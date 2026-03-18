//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/qasm/parser"
	"github.com/splch/goqu/sim/clifford"
	"github.com/splch/goqu/viz"
)

type cliffordResult struct {
	CircuitSVG   string
	HistogramSVG string
	Stabilizers  []string
	Error        string
}

type cliffordStep struct {
	Gate        string
	Qubits      []int
	Stabilizers []string
}

type cliffordStepsResult struct {
	Steps      []cliffordStep
	TotalSteps int
	Error      string
}

func runCliffordJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshalClifford(cliffordResult{Error: "usage: runClifford(qasm, shots, dark?)"})
	}
	qasm := args[0].String()
	shots := args[1].Int()
	if shots < 1 {
		shots = 1024
	}
	dark := len(args) >= 3 && args[2].Truthy()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalClifford(cliffordResult{Error: err.Error()})
	}

	var drawOpts []draw.SVGOption
	var vizOpts []viz.Option
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
		vizOpts = append(vizOpts, viz.WithStyle(viz.DarkStyle()))
	}

	r := cliffordResult{CircuitSVG: draw.SVG(circ, drawOpts...)}

	nq := circ.NumQubits()

	// Run the circuit for histogram counts.
	sim := clifford.New(nq)
	counts, err := sim.Run(circ, shots)
	if err != nil {
		return marshalClifford(cliffordResult{Error: err.Error()})
	}
	r.HistogramSVG = viz.Histogram(counts, vizOpts...)

	// Evolve a second sim (without measurement) to get stabilizers.
	sim2 := clifford.New(nq)
	if err := sim2.Evolve(circ); err != nil {
		return marshalClifford(cliffordResult{Error: err.Error()})
	}
	r.Stabilizers = sim2.Stabilizers()

	return marshalClifford(r)
}

func cliffordStepThroughJS(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalCliffordSteps(cliffordStepsResult{Error: "usage: cliffordStepThrough(qasm)"})
	}
	qasm := args[0].String()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalCliffordSteps(cliffordStepsResult{Error: err.Error()})
	}

	nq := circ.NumQubits()
	ops := circ.Ops()

	// Filter to gate ops (skip barriers, measurements, nil gates).
	var gateOps []ir.Operation
	for _, op := range ops {
		if op.Gate == nil || op.Gate.Name() == "barrier" {
			continue
		}
		gateOps = append(gateOps, op)
	}

	sim := clifford.New(nq)
	steps := make([]cliffordStep, len(gateOps))

	for i, op := range gateOps {
		if err := sim.ApplyOp(op); err != nil {
			return marshalCliffordSteps(cliffordStepsResult{Error: err.Error()})
		}
		qubits := make([]int, len(op.Qubits))
		copy(qubits, op.Qubits)
		steps[i] = cliffordStep{
			Gate:        op.Gate.Name(),
			Qubits:      qubits,
			Stabilizers: sim.Stabilizers(),
		}
	}

	return marshalCliffordSteps(cliffordStepsResult{
		Steps:      steps,
		TotalSteps: len(steps),
	})
}
