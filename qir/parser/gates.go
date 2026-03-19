package parser

import "github.com/splch/goqu/circuit/gate"

// gateFactory constructs a goqu gate from QIR parameters.
type gateFactory struct {
	fixedGate  gate.Gate               // non-nil for fixed (unparameterized) gates
	paramGate1 func(float64) gate.Gate // non-nil for single-parameter gates
	nQubits    int                     // number of qubits this gate acts on
}

// reverseMap maps QIR intrinsic function names to goqu gate factories.
var reverseMap = map[string]gateFactory{
	// Fixed single-qubit gates.
	"__quantum__qis__h__body":  {fixedGate: gate.H, nQubits: 1},
	"__quantum__qis__x__body":  {fixedGate: gate.X, nQubits: 1},
	"__quantum__qis__y__body":  {fixedGate: gate.Y, nQubits: 1},
	"__quantum__qis__z__body":  {fixedGate: gate.Z, nQubits: 1},
	"__quantum__qis__s__body":  {fixedGate: gate.S, nQubits: 1},
	"__quantum__qis__s__adj":   {fixedGate: gate.Sdg, nQubits: 1},
	"__quantum__qis__t__body":  {fixedGate: gate.T, nQubits: 1},
	"__quantum__qis__t__adj":   {fixedGate: gate.Tdg, nQubits: 1},
	"__quantum__qis__id__body": {fixedGate: gate.I, nQubits: 1},

	// Parameterized single-qubit gates.
	"__quantum__qis__rx__body": {paramGate1: gate.RX, nQubits: 1},
	"__quantum__qis__ry__body": {paramGate1: gate.RY, nQubits: 1},
	"__quantum__qis__rz__body": {paramGate1: gate.RZ, nQubits: 1},

	// Fixed two-qubit gates.
	"__quantum__qis__cnot__body": {fixedGate: gate.CNOT, nQubits: 2},
	"__quantum__qis__cx__body":   {fixedGate: gate.CNOT, nQubits: 2}, // alias
	"__quantum__qis__cy__body":   {fixedGate: gate.CY, nQubits: 2},
	"__quantum__qis__cz__body":   {fixedGate: gate.CZ, nQubits: 2},
	"__quantum__qis__swap__body": {fixedGate: gate.SWAP, nQubits: 2},

	// Parameterized two-qubit gates.
	"__quantum__qis__crx__body": {paramGate1: gate.CRX, nQubits: 2},
	"__quantum__qis__cry__body": {paramGate1: gate.CRY, nQubits: 2},
	"__quantum__qis__crz__body": {paramGate1: gate.CRZ, nQubits: 2},

	// Three-qubit gates.
	"__quantum__qis__ccx__body":   {fixedGate: gate.CCX, nQubits: 3},
	"__quantum__qis__ccnot__body": {fixedGate: gate.CCX, nQubits: 3}, // alias
	"__quantum__qis__cswap__body": {fixedGate: gate.CSWAP, nQubits: 3},
}

// lookupIntrinsic returns the gate factory for the given QIR intrinsic name.
func lookupIntrinsic(name string) (gateFactory, bool) {
	f, ok := reverseMap[name]
	return f, ok
}
