package emitter

import "strings"

// sigType classifies the LLVM function signature for a QIR intrinsic.
type sigType int

const (
	// sig1Q: void(ptr) - single-qubit gate with no parameters.
	sig1Q sigType = iota
	// sig1QParam1: void(double, ptr) - single-qubit gate with one parameter.
	sig1QParam1
	// sig2Q: void(ptr, ptr) - two-qubit gate with no parameters.
	sig2Q
	// sig2QParam1: void(double, ptr, ptr) - two-qubit gate with one parameter.
	sig2QParam1
	// sig3Q: void(ptr, ptr, ptr) - three-qubit gate with no parameters.
	sig3Q
	// sigMeasure: void(ptr, ptr writeonly) - measurement.
	sigMeasure
	// sigReset: void(ptr) - qubit reset.
	sigReset
)

// intrinsic describes a QIR gate intrinsic.
type intrinsic struct {
	name string  // e.g. "__quantum__qis__h__body"
	sig  sigType // function signature category
}

// gateMap maps goqu gate base names to QIR intrinsic descriptors.
// Base names are derived by stripping parameter suffixes and dagger marks.
var gateMap = map[string]intrinsic{
	// Fixed single-qubit gates.
	"H":  {name: "__quantum__qis__h__body", sig: sig1Q},
	"X":  {name: "__quantum__qis__x__body", sig: sig1Q},
	"Y":  {name: "__quantum__qis__y__body", sig: sig1Q},
	"Z":  {name: "__quantum__qis__z__body", sig: sig1Q},
	"S":  {name: "__quantum__qis__s__body", sig: sig1Q},
	"Sd": {name: "__quantum__qis__s__adj", sig: sig1Q},
	"T":  {name: "__quantum__qis__t__body", sig: sig1Q},
	"Td": {name: "__quantum__qis__t__adj", sig: sig1Q},

	// Parameterized single-qubit gates.
	"RX": {name: "__quantum__qis__rx__body", sig: sig1QParam1},
	"RY": {name: "__quantum__qis__ry__body", sig: sig1QParam1},
	"RZ": {name: "__quantum__qis__rz__body", sig: sig1QParam1},

	// Fixed two-qubit gates.
	"CX":   {name: "__quantum__qis__cnot__body", sig: sig2Q},
	"CNOT": {name: "__quantum__qis__cnot__body", sig: sig2Q},
	"CY":   {name: "__quantum__qis__cy__body", sig: sig2Q},
	"CZ":   {name: "__quantum__qis__cz__body", sig: sig2Q},
	"SWAP": {name: "__quantum__qis__swap__body", sig: sig2Q},

	// Parameterized two-qubit gates.
	"CRX": {name: "__quantum__qis__crx__body", sig: sig2QParam1},
	"CRY": {name: "__quantum__qis__cry__body", sig: sig2QParam1},
	"CRZ": {name: "__quantum__qis__crz__body", sig: sig2QParam1},

	// Three-qubit gates.
	"CCX":   {name: "__quantum__qis__ccx__body", sig: sig3Q},
	"CSWAP": {name: "__quantum__qis__cswap__body", sig: sig3Q},
}

// lookupGate returns the QIR intrinsic for the given goqu gate name.
// It normalizes the name by stripping parameter parenthetical suffixes
// and mapping dagger variants. Returns the intrinsic and true if found.
func lookupGate(name string) (intrinsic, bool) {
	// Handle dagger gates: "S\u2020" -> "Sd", "T\u2020" -> "Td".
	if base, ok := strings.CutSuffix(name, "\u2020"); ok {
		// Strip params from dagger name too.
		if prefix, _, found := strings.Cut(base, "("); found {
			base = prefix
		}
		key := base + "d"
		if intr, ok := gateMap[key]; ok {
			return intr, true
		}
	}

	// Strip parameter suffix: "RZ(1.5708)" -> "RZ".
	base, _, _ := strings.Cut(name, "(")

	intr, ok := gateMap[base]
	return intr, ok
}

// sigDeclaration returns the LLVM IR declare statement for the given intrinsic.
func sigDeclaration(intr intrinsic) string {
	switch intr.sig {
	case sig1Q, sigReset:
		return "declare void @" + intr.name + "(ptr)"
	case sig1QParam1:
		return "declare void @" + intr.name + "(double, ptr)"
	case sig2Q:
		return "declare void @" + intr.name + "(ptr, ptr)"
	case sig2QParam1:
		return "declare void @" + intr.name + "(double, ptr, ptr)"
	case sig3Q:
		return "declare void @" + intr.name + "(ptr, ptr, ptr)"
	case sigMeasure:
		return "declare void @" + intr.name + "(ptr, ptr writeonly) #1"
	default:
		return "declare void @" + intr.name + "(ptr)"
	}
}

// qirBasisGates lists the gate names accepted directly by the QIR emitter
// without decomposition.
var qirBasisGates = []string{
	"H", "X", "Y", "Z", "S", "Sd", "T", "Td", "I",
	"RX", "RY", "RZ",
	"CX", "CY", "CZ", "SWAP",
	"CRX", "CRY", "CRZ",
	"CCX", "CSWAP",
}
