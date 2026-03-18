//go:build js && wasm

package main

import (
	"context"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/splch/goqu/algorithm/ansatz"
	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/algorithm/vqe"
	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/sim/pauli"
)

// vqeResult holds the output from runVQEJS.
type vqeResult struct {
	CircuitSVG    string
	Energy        float64
	History       []float64
	NumIterations int
	Error         string
}

// buildVQEAnsatz constructs an ansatz from a name string for VQE use.
func buildVQEAnsatz(name string, nQubits, layers int) (ansatz.Ansatz, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "real_amplitudes", "realamplitudes":
		return ansatz.NewRealAmplitudes(nQubits, layers, ansatz.Linear), nil
	case "efficient_su2", "efficientsu2":
		return ansatz.NewEfficientSU2(nQubits, layers, ansatz.Linear), nil
	case "strongly_entangling", "stronglyentangling":
		return ansatz.NewStronglyEntanglingLayers(nQubits, layers), nil
	default:
		return nil, fmt.Errorf("unknown ansatz: %q (supported: real_amplitudes, efficient_su2, strongly_entangling)", name)
	}
}

// parsePauliSumStr parses a Hamiltonian string like "-1.05*II+0.39*ZI+-0.39*IZ+0.01*XX"
// into a pauli.PauliSum. Terms are separated by '+' (negative coefficients use "+-0.39*ZI").
func parsePauliSumStr(s string) (pauli.PauliSum, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pauli.PauliSum{}, fmt.Errorf("empty Hamiltonian string")
	}

	parts := strings.Split(s, "+")
	var terms []pauli.PauliString
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		coeff, label, err := parsePauliTerm(part)
		if err != nil {
			return pauli.PauliSum{}, fmt.Errorf("parse term %q: %w", part, err)
		}
		ps, err := pauli.Parse(label)
		if err != nil {
			return pauli.PauliSum{}, fmt.Errorf("parse Pauli %q: %w", label, err)
		}
		terms = append(terms, ps.Scale(complex(coeff, 0)))
	}
	if len(terms) == 0 {
		return pauli.PauliSum{}, fmt.Errorf("no valid Pauli terms found")
	}
	return pauli.NewPauliSum(terms)
}

// runVQEJS runs the Variational Quantum Eigensolver.
// Args: (hamiltonianStr string, ansatzName string, nQubits int, layers int, maxIter int, dark? bool)
func runVQEJS(_ js.Value, args []js.Value) any {
	if len(args) < 5 {
		return marshalVQE(vqeResult{Error: "usage: runVQE(hamiltonianStr, ansatzName, nQubits, layers, maxIter, dark?)"})
	}
	hamiltonianStr := args[0].String()
	ansatzName := args[1].String()
	nQubits := args[2].Int()
	layers := args[3].Int()
	maxIter := args[4].Int()
	dark := len(args) >= 6 && args[5].Truthy()

	// Enforce caps.
	if nQubits < 1 {
		nQubits = 1
	}
	if nQubits > 6 {
		return marshalVQE(vqeResult{Error: "VQE limited to 6 qubits in WASM"})
	}
	if layers < 1 {
		layers = 1
	}
	if layers > 5 {
		layers = 5
	}
	if maxIter < 1 {
		maxIter = 50
	}
	if maxIter > 200 {
		maxIter = 200
	}

	// Parse Hamiltonian.
	hamiltonian, err := parsePauliSumStr(hamiltonianStr)
	if err != nil {
		return marshalVQE(vqeResult{Error: fmt.Sprintf("parse hamiltonian: %s", err.Error())})
	}

	// Validate qubit count matches.
	if hamiltonian.NumQubits() != nQubits {
		return marshalVQE(vqeResult{Error: fmt.Sprintf("Hamiltonian has %d qubits, expected %d", hamiltonian.NumQubits(), nQubits)})
	}

	// Build ansatz.
	ans, err := buildVQEAnsatz(ansatzName, nQubits, layers)
	if err != nil {
		return marshalVQE(vqeResult{Error: err.Error()})
	}

	// Configure and run VQE.
	cfg := vqe.Config{
		Hamiltonian: hamiltonian,
		Ansatz:      ans,
		Optimizer: &optim.NelderMead{},
	}

	// We need to wrap the optimizer to limit iterations.
	// VQE accepts optim.Optimizer which calls Minimize with nil opts.
	// Use a wrapping optimizer that injects MaxIter.
	cfg.Optimizer = &cappedOptimizer{
		inner:   &optim.NelderMead{},
		maxIter: maxIter,
	}

	ctx := context.Background()
	result, err := vqe.Run(ctx, cfg)
	if err != nil {
		return marshalVQE(vqeResult{Error: fmt.Sprintf("VQE: %s", err.Error())})
	}

	// Draw the ansatz circuit.
	ansCirc, err := ans.Circuit()
	if err != nil {
		return marshalVQE(vqeResult{Error: fmt.Sprintf("ansatz circuit: %s", err.Error())})
	}

	var drawOpts []draw.SVGOption
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
	}

	return marshalVQE(vqeResult{
		CircuitSVG:    draw.SVG(ansCirc, drawOpts...),
		Energy:        result.Energy,
		History:       result.History,
		NumIterations: result.NumIters,
	})
}

// cappedOptimizer wraps an inner Optimizer and injects a MaxIter cap.
type cappedOptimizer struct {
	inner   optim.Optimizer
	maxIter int
}

func (c *cappedOptimizer) Name() string { return c.inner.Name() }

func (c *cappedOptimizer) Minimize(ctx context.Context, f optim.ObjectiveFunc, x0 []float64,
	grad optim.GradientFunc, opts *optim.Options) (optim.Result, error) {
	if opts == nil {
		opts = &optim.Options{}
	}
	if opts.MaxIter <= 0 || opts.MaxIter > c.maxIter {
		opts.MaxIter = c.maxIter
	}
	return c.inner.Minimize(ctx, f, x0, grad, opts)
}
