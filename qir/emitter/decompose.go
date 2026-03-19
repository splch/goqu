package emitter

import (
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/pass"
	"github.com/splch/goqu/transpile/target"
)

// qirTarget defines the QIR basis gate set as a transpilation target.
// All-to-all connectivity, unlimited qubits.
var qirTarget = target.Target{
	Name: "QIR",
	BasisGates: []string{
		// Single-qubit.
		"H", "X", "Y", "Z", "S", "Sd", "T", "Td", "I",
		"RX", "RY", "RZ",
		// Two-qubit.
		"CX", "CY", "CZ", "SWAP",
		"CRX", "CRY", "CRZ",
		// Three-qubit.
		"CCX", "CSWAP",
		// Pseudo-gates (handled directly by the emitter).
		"reset",
	},
}

// decomposeToQIR reduces a circuit to the QIR basis gate set using the
// existing transpilation infrastructure (Euler, KAK, rule-based decomposition).
// Barriers and delays are stripped. Global phase gates are dropped.
func decomposeToQIR(c *ir.Circuit) (*ir.Circuit, error) {
	return pass.DecomposeToTarget(c, qirTarget)
}
