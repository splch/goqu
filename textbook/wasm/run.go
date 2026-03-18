//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/qasm/parser"
	"github.com/splch/goqu/sim/statevector"
	"github.com/splch/goqu/viz"
)

type runResult struct {
	Circuit   string `json:"circuit,omitempty"`
	Histogram string `json:"histogram,omitempty"`
	Bloch     string `json:"bloch,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runQASMJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshal(runResult{Error: "usage: runQASM(qasm, shots, dark?)"})
	}
	qasm := args[0].String()
	shots := args[1].Int()
	if shots < 1 {
		shots = 1024
	}
	dark := len(args) >= 3 && args[2].Truthy()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshal(runResult{Error: err.Error()})
	}

	var drawOpts []draw.SVGOption
	var vizOpts []viz.Option
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
		vizOpts = append(vizOpts, viz.WithStyle(viz.DarkStyle()))
	}

	r := runResult{Circuit: draw.SVG(circ, drawOpts...)}

	nq := circ.NumQubits()
	sim := statevector.New(nq)
	defer sim.Close()

	counts, err := sim.Run(circ, shots)
	if err != nil {
		return marshal(runResult{Error: err.Error()})
	}
	r.Histogram = viz.Histogram(counts, vizOpts...)

	if nq == 1 {
		sim2 := statevector.New(1)
		defer sim2.Close()
		if err := sim2.Evolve(circ); err == nil {
			r.Bloch = viz.Bloch(sim2.StateVector(), vizOpts...)
		}
	}

	return marshal(r)
}

func marshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
