//go:build js && wasm

package main

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/splch/goqu/algorithm/mitigation"
	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/qasm/parser"
)

// zneResult holds the output from runZNEJS.
type zneResult struct {
	ScaleFactors       []float64
	NoisyValues        []float64
	LinearExtrapolated float64
	Ideal              float64
	Error              string
}

// ddResult holds the output from insertDDJS.
type ddResult struct {
	BeforeCircuitSVG string
	AfterCircuitSVG  string
	Error            string
}

// twirlResult holds the output from twirlCircuitJS.
type twirlResult struct {
	OriginalCircuitSVG string
	TwirledCircuitSVG  string
	Error              string
}

// parseScaleFactors parses a comma-separated string of positive odd integers
// like "1,3,5" into a []float64.
func parseScaleFactors(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []float64{1, 3, 5}, nil
	}
	parts := strings.Split(s, ",")
	factors := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid scale factor %q: %w", p, err)
		}
		iv := int(v)
		if iv < 1 || iv%2 == 0 {
			return nil, fmt.Errorf("scale factor %v must be a positive odd integer", v)
		}
		factors = append(factors, v)
	}
	if len(factors) < 2 {
		return nil, fmt.Errorf("need at least 2 scale factors, got %d", len(factors))
	}
	return factors, nil
}

// runZNEJS demonstrates zero-noise extrapolation.
// Args: (qasm string, pauliStr string, noiseModel string, noiseParam float64,
//
//	scaleFactorsStr string, dark? bool)
func runZNEJS(_ js.Value, args []js.Value) any {
	if len(args) < 5 {
		return marshalZNE(zneResult{Error: "usage: runZNE(qasm, pauliStr, noiseModel, noiseParam, scaleFactorsStr, dark?)"})
	}
	qasmStr := args[0].String()
	pauliStr := args[1].String()
	noiseModel := args[2].String()
	noiseParam := args[3].Float()
	scaleFactorsStr := args[4].String()
	_ = len(args) >= 6 && args[5].Truthy() // dark (unused for ZNE data-only result)

	circ, err := parser.ParseString(qasmStr)
	if err != nil {
		return marshalZNE(zneResult{Error: fmt.Sprintf("parse QASM: %s", err.Error())})
	}

	nq := circ.NumQubits()
	if nq > 8 {
		return marshalZNE(zneResult{Error: "ZNE limited to 8 qubits"})
	}

	// Parse the Pauli observable.
	hamiltonian, err := parsePauliSumStr(pauliStr)
	if err != nil {
		return marshalZNE(zneResult{Error: fmt.Sprintf("parse observable: %s", err.Error())})
	}
	if hamiltonian.NumQubits() != nq {
		return marshalZNE(zneResult{Error: fmt.Sprintf("observable has %d qubits, circuit has %d", hamiltonian.NumQubits(), nq)})
	}

	// Parse scale factors.
	scaleFactors, err := parseScaleFactors(scaleFactorsStr)
	if err != nil {
		return marshalZNE(zneResult{Error: fmt.Sprintf("scale factors: %s", err.Error())})
	}

	// Build noise model.
	nm, err := buildNoiseModel(noiseModel, noiseParam)
	if err != nil {
		return marshalZNE(zneResult{Error: err.Error()})
	}

	// Build executors.
	noisyExec := mitigation.DensityMatrixExecutor(hamiltonian, nm)
	idealExec := mitigation.StatevectorExecutor(hamiltonian)

	// Run ZNE using the mitigation package.
	ctx := context.Background()
	zneRes, err := mitigation.RunZNE(ctx, mitigation.ZNEConfig{
		Circuit:      circ,
		Executor:     noisyExec,
		ScaleFactors: scaleFactors,
		ScaleMethod:  mitigation.UnitaryFolding,
		Extrapolator: mitigation.LinearExtrapolator,
	})
	if err != nil {
		return marshalZNE(zneResult{Error: fmt.Sprintf("ZNE: %s", err.Error())})
	}

	// Compute ideal expectation value.
	ideal, err := idealExec(ctx, circ)
	if err != nil {
		return marshalZNE(zneResult{Error: fmt.Sprintf("ideal: %s", err.Error())})
	}

	return marshalZNE(zneResult{
		ScaleFactors:       zneRes.ScaleFactors,
		NoisyValues:        zneRes.NoisyValues,
		LinearExtrapolated: zneRes.MitigatedValue,
		Ideal:              ideal,
	})
}

// insertDDJS inserts dynamical decoupling sequences into idle periods.
// Args: (qasm string, sequence string, dark? bool)
func insertDDJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshalDD(ddResult{Error: "usage: insertDD(qasm, sequence, dark?)"})
	}
	qasmStr := args[0].String()
	seqStr := args[1].String()
	dark := len(args) >= 3 && args[2].Truthy()

	circ, err := parser.ParseString(qasmStr)
	if err != nil {
		return marshalDD(ddResult{Error: fmt.Sprintf("parse QASM: %s", err.Error())})
	}

	nq := circ.NumQubits()
	if nq > 8 {
		return marshalDD(ddResult{Error: "DD limited to 8 qubits"})
	}

	// Select DD sequence.
	var seq mitigation.DDSequence
	switch strings.ToLower(strings.TrimSpace(seqStr)) {
	case "xy4", "xyxy":
		seq = mitigation.DDXY4
	default:
		seq = mitigation.DDXX
	}

	// Draw options.
	var drawOpts []draw.SVGOption
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
	}

	beforeSVG := draw.SVG(circ, drawOpts...)

	ddCirc, err := mitigation.InsertDD(mitigation.DDConfig{
		Circuit:  circ,
		Sequence: seq,
	})
	if err != nil {
		return marshalDD(ddResult{
			BeforeCircuitSVG: beforeSVG,
			Error:            fmt.Sprintf("InsertDD: %s", err.Error()),
		})
	}

	return marshalDD(ddResult{
		BeforeCircuitSVG: beforeSVG,
		AfterCircuitSVG:  draw.SVG(ddCirc, drawOpts...),
	})
}

// twirlCircuitJS applies Pauli twirling to a circuit.
// Args: (qasm string, dark? bool)
func twirlCircuitJS(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalTwirl(twirlResult{Error: "usage: twirlCircuit(qasm, dark?)"})
	}
	qasmStr := args[0].String()
	dark := len(args) >= 2 && args[1].Truthy()

	circ, err := parser.ParseString(qasmStr)
	if err != nil {
		return marshalTwirl(twirlResult{Error: fmt.Sprintf("parse QASM: %s", err.Error())})
	}

	nq := circ.NumQubits()
	if nq > 8 {
		return marshalTwirl(twirlResult{Error: "twirling limited to 8 qubits"})
	}

	// Draw options.
	var drawOpts []draw.SVGOption
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
	}

	originalSVG := draw.SVG(circ, drawOpts...)

	rng := rand.New(rand.NewSource(42))
	twirled, err := mitigation.TwirlCircuit(circ, rng)
	if err != nil {
		return marshalTwirl(twirlResult{
			OriginalCircuitSVG: originalSVG,
			Error:              fmt.Sprintf("TwirlCircuit: %s", err.Error()),
		})
	}

	return marshalTwirl(twirlResult{
		OriginalCircuitSVG: originalSVG,
		TwirledCircuitSVG:  draw.SVG(twirled, drawOpts...),
	})
}

