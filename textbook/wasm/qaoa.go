//go:build js && wasm

package main

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/algorithm/qaoa"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
	qasmemitter "github.com/splch/goqu/qasm/emitter"
	"github.com/splch/goqu/sim/statevector"
	"github.com/splch/goqu/viz"
)

// qaoaResult holds the output from runQAOAJS.
type qaoaResult struct {
	CircuitSVG    string
	HistogramSVG  string
	QASMCode      string
	BestBitstring string
	Energy        float64
	History       []float64
	Error         string
}

// parseEdges parses a comma-separated edge string like "0-1,1-2,2-0"
// into a slice of [2]int pairs.
func parseEdges(s string) ([][2]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty edge string")
	}
	parts := strings.Split(s, ",")
	edges := make([][2]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		idx := strings.Index(p, "-")
		if idx < 0 {
			return nil, fmt.Errorf("invalid edge %q: expected format i-j", p)
		}
		a, err := strconv.Atoi(strings.TrimSpace(p[:idx]))
		if err != nil {
			return nil, fmt.Errorf("invalid edge %q: %w", p, err)
		}
		b, err := strconv.Atoi(strings.TrimSpace(p[idx+1:]))
		if err != nil {
			return nil, fmt.Errorf("invalid edge %q: %w", p, err)
		}
		edges = append(edges, [2]int{a, b})
	}
	if len(edges) == 0 {
		return nil, fmt.Errorf("no valid edges found")
	}
	return edges, nil
}

// runQAOAJS runs the Quantum Approximate Optimization Algorithm for MaxCut.
// Args: (graphEdgesStr string, nQubits int, layers int, shots int, dark? bool)
func runQAOAJS(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return marshalQAOA(qaoaResult{Error: "usage: runQAOA(graphEdgesStr, nQubits, layers, shots, dark?)"})
	}
	graphEdgesStr := args[0].String()
	nQubits := args[1].Int()
	layers := args[2].Int()
	shots := args[3].Int()
	dark := len(args) >= 5 && args[4].Truthy()

	// Enforce caps.
	if nQubits < 2 {
		nQubits = 2
	}
	if nQubits > 6 {
		return marshalQAOA(qaoaResult{Error: "QAOA limited to 6 qubits in WASM"})
	}
	if layers < 1 {
		layers = 1
	}
	if layers > 3 {
		layers = 3
	}
	if shots < 1 {
		shots = 1024
	}

	edges, err := parseEdges(graphEdgesStr)
	if err != nil {
		return marshalQAOA(qaoaResult{Error: fmt.Sprintf("parse edges: %s", err.Error())})
	}

	// Build cost Hamiltonian.
	costH, err := qaoa.MaxCutHamiltonian(edges, nQubits)
	if err != nil {
		return marshalQAOA(qaoaResult{Error: fmt.Sprintf("hamiltonian: %s", err.Error())})
	}

	// Build the parameterized QAOA circuit manually so we can capture
	// the optimization history (qaoa.Run does not expose it).
	p := layers
	gammas := param.NewVector("g", p)
	betas := param.NewVector("b", p)

	bld := builder.New("QAOA", nQubits)
	for q := range nQubits {
		bld.H(q)
	}
	for k := range p {
		// Cost layer: RZZ for each ZZ term, RZ for each Z term.
		for _, term := range costH.Terms() {
			coeff := real(term.Coeff())
			if coeff == 0 {
				continue
			}
			var zQubits []int
			allZorI := true
			for q := range nQubits {
				op := term.Op(q)
				switch op {
				case 0x01: // Z
					zQubits = append(zQubits, q)
				case 0x00: // I
					// ok
				default:
					allZorI = false
				}
			}
			if !allZorI || len(zQubits) == 0 {
				continue
			}
			angle := param.Mul(param.Literal(2*coeff), gammas.At(k).Expr())
			switch len(zQubits) {
			case 1:
				bld.SymRZ(angle, zQubits[0])
			case 2:
				bld.SymRZZ(angle, zQubits[0], zQubits[1])
			}
		}
		// Mixer layer: default X-mixer with RX(2*beta).
		for q := range nQubits {
			bld.SymRX(param.Mul(param.Literal(2), betas.At(k).Expr()), q)
		}
	}

	circ, err := bld.Build()
	if err != nil {
		return marshalQAOA(qaoaResult{Error: fmt.Sprintf("circuit build: %s", err.Error())})
	}

	paramNames := ir.FreeParameters(circ)

	// Build cost function with history tracking.
	var history []float64
	cost := optim.ObjectiveFunc(func(x []float64) float64 {
		bindings := make(map[string]float64, len(paramNames))
		for i, name := range paramNames {
			bindings[name] = x[i]
		}
		bound, err := ir.Bind(circ, bindings)
		if err != nil {
			return math.Inf(1)
		}
		sim := statevector.New(nQubits)
		if err := sim.Evolve(bound); err != nil {
			return math.Inf(1)
		}
		v := sim.ExpectPauliSum(costH)
		history = append(history, v)
		return v
	})

	// Initial parameters: heuristic.
	x0 := make([]float64, 2*p)
	for i := range p {
		x0[i] = 0.5
		x0[p+i] = math.Pi / 4
	}

	optimizer := &optim.NelderMead{}
	ctx := context.Background()
	res, err := optimizer.Minimize(ctx, cost, x0, nil, &optim.Options{MaxIter: 200})
	if err != nil {
		return marshalQAOA(qaoaResult{Error: fmt.Sprintf("optimization: %s", err.Error())})
	}

	// Bind the optimal parameters to get the final circuit.
	bindings := make(map[string]float64, len(paramNames))
	for i, name := range paramNames {
		bindings[name] = res.X[i]
	}
	bound, err := ir.Bind(circ, bindings)
	if err != nil {
		return marshalQAOA(qaoaResult{Error: fmt.Sprintf("final bind: %s", err.Error())})
	}

	// Draw options.
	var drawOpts []draw.SVGOption
	var vizOpts []viz.Option
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
		vizOpts = append(vizOpts, viz.WithStyle(viz.DarkStyle()))
	}

	r := qaoaResult{
		CircuitSVG: draw.SVG(bound, drawOpts...),
		Energy:     res.Fun,
		History:    history,
	}

	// Sample to get histogram and best bitstring.
	measBld := builder.New("QAOA-Meas", nQubits)
	measBld.Compose(bound, nil)
	measBld.MeasureAll()
	measCirc, err := measBld.Build()
	if err != nil {
		r.Error = fmt.Sprintf("measurement circuit: %s", err.Error())
		return marshalQAOA(r)
	}

	sim := statevector.New(nQubits)
	counts, err := sim.Run(measCirc, shots)
	if err != nil {
		r.Error = fmt.Sprintf("sampling: %s", err.Error())
		return marshalQAOA(r)
	}

	r.HistogramSVG = viz.Histogram(counts, vizOpts...)

	// Find best bitstring (most frequent).
	bestBS := ""
	bestCount := 0
	for bs, cnt := range counts {
		if cnt > bestCount {
			bestCount = cnt
			bestBS = bs
		}
	}
	r.BestBitstring = bestBS

	// Emit QASM for the bound circuit.
	qasmStr, err := qasmemitter.EmitString(bound)
	if err != nil {
		r.QASMCode = fmt.Sprintf("// QASM emission error: %v", err)
	} else {
		r.QASMCode = qasmStr
	}

	return marshalQAOA(r)
}
