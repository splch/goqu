package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/qasm/parser"
	"github.com/splch/goqu/sim/statevector"
	"github.com/splch/goqu/viz"
)

type result struct {
	Circuit   string `json:"circuit,omitempty"`
	Histogram string `json:"histogram,omitempty"`
	Bloch     string `json:"bloch,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runQASM(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshal(result{Error: "usage: runQASM(qasm, shots)"})
	}
	qasm := args[0].String()
	shots := args[1].Int()
	if shots < 1 {
		shots = 1024
	}

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshal(result{Error: err.Error()})
	}

	r := result{Circuit: draw.SVG(circ)}

	nq := circ.NumQubits()
	sim := statevector.New(nq)
	defer sim.Close()

	counts, err := sim.Run(circ, shots)
	if err != nil {
		return marshal(result{Error: err.Error()})
	}
	r.Histogram = viz.Histogram(counts)

	if nq == 1 {
		sim2 := statevector.New(1)
		defer sim2.Close()
		if err := sim2.Evolve(circ); err == nil {
			r.Bloch = viz.Bloch(sim2.StateVector())
		}
	}

	return marshal(r)
}

func renderBloch(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return ""
	}
	state := []complex128{
		complex(args[0].Float(), args[1].Float()),
		complex(args[2].Float(), args[3].Float()),
	}
	return viz.Bloch(state)
}

func marshal(r result) string {
	b, _ := json.Marshal(r)
	return string(b)
}

func main() {
	js.Global().Set("runQASM", js.FuncOf(runQASM))
	js.Global().Set("renderBloch", js.FuncOf(renderBloch))
	select {}
}
