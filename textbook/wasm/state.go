//go:build js && wasm

package main

import (
	"math/cmplx"
	"syscall/js"

	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/qasm/parser"
	"github.com/splch/goqu/sim/statevector"
	"github.com/splch/goqu/viz"
)

type complexNum struct {
	Re float64
	Im float64
}

type blochVector struct {
	X float64
	Y float64
	Z float64
}

type stateResult struct {
	Amplitudes    []complexNum
	Probabilities []float64
	BlochVectors  []blochVector
	Error         string
}

type probResult struct {
	Probabilities []float64
	Labels        []string
	Histogram     string
	Circuit       string
	Error         string
}

// getStateVectorJS returns the full statevector after circuit evolution (no measurement).
func getStateVectorJS(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalState(stateResult{Error: "usage: getStateVector(qasm)"})
	}
	qasm := args[0].String()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalState(stateResult{Error: err.Error()})
	}

	nq := circ.NumQubits()
	sim := statevector.New(nq)
	defer sim.Close()

	if err := sim.Evolve(circ); err != nil {
		return marshalState(stateResult{Error: err.Error()})
	}

	sv := sim.StateVector()
	r := stateResult{
		Amplitudes:    make([]complexNum, len(sv)),
		Probabilities: make([]float64, len(sv)),
	}

	for i, amp := range sv {
		r.Amplitudes[i] = complexNum{Re: real(amp), Im: imag(amp)}
		r.Probabilities[i] = real(amp)*real(amp) + imag(amp)*imag(amp)
	}

	// Compute per-qubit Bloch vectors via partial trace for small systems.
	if nq <= 6 {
		r.BlochVectors = make([]blochVector, nq)
		for q := range nq {
			bx, by, bz := qubitBlochCoords(sv, nq, q)
			r.BlochVectors[q] = blochVector{X: bx, Y: by, Z: bz}
		}
	}

	return marshalState(r)
}

// getProbabilitiesJS returns exact probabilities and a histogram SVG.
func getProbabilitiesJS(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalProb(probResult{Error: "usage: getProbabilities(qasm, dark?)"})
	}
	qasm := args[0].String()
	dark := len(args) >= 2 && args[1].Truthy()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalProb(probResult{Error: err.Error()})
	}

	var drawOpts []draw.SVGOption
	var vizOpts []viz.Option
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
		vizOpts = append(vizOpts, viz.WithStyle(viz.DarkStyle()))
	}

	nq := circ.NumQubits()
	sim := statevector.New(nq)
	defer sim.Close()

	if err := sim.Evolve(circ); err != nil {
		return marshalProb(probResult{Error: err.Error()})
	}

	sv := sim.StateVector()
	probs := make([]float64, len(sv))
	labels := make([]string, len(sv))
	probMap := make(map[string]float64)

	for i, amp := range sv {
		p := real(amp)*real(amp) + imag(amp)*imag(amp)
		probs[i] = p
		label := formatBinary(i, nq)
		labels[i] = label
		if p > 1e-10 {
			probMap[label] = p
		}
	}

	r := probResult{
		Probabilities: probs,
		Labels:        labels,
		Histogram:     viz.HistogramProb(probMap, vizOpts...),
		Circuit:       draw.SVG(circ, drawOpts...),
	}

	return marshalProb(r)
}

// qubitBlochCoords computes the Bloch vector for qubit q in a multi-qubit statevector
// by tracing out all other qubits.
func qubitBlochCoords(sv []complex128, nq, q int) (x, y, z float64) {
	dim := 1 << nq
	var rho00, rho11 float64
	var rho01 complex128

	for i := range dim {
		bit := (i >> (nq - 1 - q)) & 1
		paired := i ^ (1 << (nq - 1 - q))
		amp := sv[i]
		prob := real(amp)*real(amp) + imag(amp)*imag(amp)

		if bit == 0 {
			rho00 += prob
			rho01 += amp * cmplx.Conj(sv[paired])
		} else {
			rho11 += prob
		}
	}

	x = 2 * real(rho01)
	y = -2 * imag(rho01)
	z = rho00 - rho11
	return
}

// formatBinary returns i as an nq-bit binary string.
func formatBinary(i, nq int) string {
	s := make([]byte, nq)
	for b := range nq {
		if (i>>(nq-1-b))&1 == 1 {
			s[b] = '1'
		} else {
			s[b] = '0'
		}
	}
	return string(s)
}
