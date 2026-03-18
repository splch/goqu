//go:build js && wasm

package main

import (
	"context"
	"fmt"
	"syscall/js"

	"github.com/splch/goqu/algorithm/trotter"
	"github.com/splch/goqu/circuit/draw"
	qasmemitter "github.com/splch/goqu/qasm/emitter"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// trotterResult holds the output from runTrotterJS.
type trotterResult struct {
	CircuitSVG  string
	QASMCode    string
	Expectation float64
	Steps       int
	Order       int
	Error       string
}

// runTrotterJS builds a Trotter circuit and computes the final expectation value.
// Args: (hamiltonianStr string, time float64, steps int, order int, dark? bool)
func runTrotterJS(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return marshalTrotter(trotterResult{Error: "usage: runTrotter(hamiltonian, time, steps, order, dark?)"})
	}
	hamiltonianStr := args[0].String()
	time := args[1].Float()
	steps := args[2].Int()
	order := args[3].Int()
	dark := len(args) >= 5 && args[4].Truthy()

	hamiltonian, err := parsePauliSumStr(hamiltonianStr)
	if err != nil {
		return marshalTrotter(trotterResult{Error: fmt.Sprintf("parse Hamiltonian: %s", err.Error())})
	}

	nq := hamiltonian.NumQubits()
	if nq > 8 {
		return marshalTrotter(trotterResult{Error: "Trotter limited to 8 qubits"})
	}
	if steps < 1 {
		steps = 1
	}
	if steps > 20 {
		steps = 20
	}

	trotterOrder := trotter.First
	if order == 2 {
		trotterOrder = trotter.Second
	}

	ctx := context.Background()
	result, err := trotter.Run(ctx, trotter.Config{
		Hamiltonian: hamiltonian,
		Time:        time,
		Steps:       steps,
		Order:       trotterOrder,
	})
	if err != nil {
		return marshalTrotter(trotterResult{Error: err.Error()})
	}

	// Draw circuit.
	var drawOpts []draw.SVGOption
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
	}
	circSVG := draw.SVG(result.Circuit, drawOpts...)

	// Emit QASM.
	qasmCode, _ := qasmemitter.EmitString(result.Circuit)

	// Evolve and compute expectation value.
	sim := statevector.New(nq)
	defer sim.Close()
	if err := sim.Evolve(result.Circuit); err != nil {
		return marshalTrotter(trotterResult{Error: fmt.Sprintf("evolve: %s", err.Error())})
	}
	sv := sim.StateVector()
	expectation := real(pauli.ExpectSum(sv, hamiltonian))

	return marshalTrotter(trotterResult{
		CircuitSVG:  circSVG,
		QASMCode:    qasmCode,
		Expectation: expectation,
		Steps:       steps,
		Order:       int(trotterOrder),
	})
}
