package ionq

import (
	"fmt"
	"math"
	"strings"

	"github.com/splch/qgo/circuit/ir"
)

// nativeGates are IonQ hardware-level gate names.
var nativeGates = map[string]bool{
	"GPI": true, "GPI2": true, "MS": true, "ZZ": true,
}

// qisGates are the abstract gates supported by IonQ's QIS gateset.
var qisGates = map[string]bool{
	"H": true, "X": true, "Y": true, "Z": true,
	"S": true, "S†": true, "T": true, "T†": true,
	"SX": true, "SX†": true, "I": true,
	"CNOT": true, "SWAP": true,
	"RX": true, "RY": true, "RZ": true,
}

// marshalCircuit converts a circuit IR to IonQ input format.
func marshalCircuit(c *ir.Circuit) (*ionqInput, error) {
	gateset, err := detectGateset(c)
	if err != nil {
		return nil, err
	}

	var gates []ionqGate
	for _, op := range c.Ops() {
		if op.Gate == nil {
			continue // skip measurements
		}
		name := basisName(op.Gate.Name())
		if name == "barrier" || name == "I" {
			continue
		}

		g, err := marshalGate(op, gateset)
		if err != nil {
			return nil, err
		}
		gates = append(gates, g)
	}

	return &ionqInput{
		Qubits:  c.NumQubits(),
		Gateset: gateset,
		Circuit: gates,
	}, nil
}

// detectGateset determines whether the circuit uses native or QIS gates.
func detectGateset(c *ir.Circuit) (string, error) {
	hasNative, hasQIS := false, false
	for _, op := range c.Ops() {
		if op.Gate == nil {
			continue
		}
		name := basisName(op.Gate.Name())
		if name == "barrier" || name == "I" {
			continue
		}
		if nativeGates[name] {
			hasNative = true
		} else if qisGates[name] || qisGates[op.Gate.Name()] {
			hasQIS = true
		} else {
			return "", fmt.Errorf("ionq: unsupported gate %q", op.Gate.Name())
		}
	}
	if hasNative && hasQIS {
		return "", fmt.Errorf("ionq: circuit mixes native and abstract gates")
	}
	if hasNative {
		return "native", nil
	}
	return "qis", nil
}

// marshalGate converts a single operation to an IonQ gate entry.
func marshalGate(op ir.Operation, gateset string) (ionqGate, error) {
	if gateset == "native" {
		return marshalNativeGate(op)
	}
	return marshalQISGate(op)
}

func marshalQISGate(op ir.Operation) (ionqGate, error) {
	name := op.Gate.Name()
	basis := basisName(name)

	switch basis {
	case "H":
		return singleTarget("h", op), nil
	case "X":
		return singleTarget("x", op), nil
	case "Y":
		return singleTarget("y", op), nil
	case "Z":
		return singleTarget("z", op), nil
	case "S":
		if strings.HasSuffix(name, "†") {
			return singleTarget("si", op), nil
		}
		return singleTarget("s", op), nil
	case "T":
		if strings.HasSuffix(name, "†") {
			return singleTarget("ti", op), nil
		}
		return singleTarget("t", op), nil
	case "SX":
		if strings.HasSuffix(name, "†") {
			return singleTarget("vi", op), nil
		}
		return singleTarget("v", op), nil
	case "RX":
		return singleRotation("rx", op), nil
	case "RY":
		return singleRotation("ry", op), nil
	case "RZ":
		return singleRotation("rz", op), nil
	case "CNOT":
		ctrl, tgt := op.Qubits[0], op.Qubits[1]
		return ionqGate{Gate: "cnot", Control: &ctrl, Target: &tgt}, nil
	case "SWAP":
		return ionqGate{Gate: "swap", Targets: []int{op.Qubits[0], op.Qubits[1]}}, nil
	default:
		return ionqGate{}, fmt.Errorf("ionq: gate %q not supported in QIS gateset", name)
	}
}

func marshalNativeGate(op ir.Operation) (ionqGate, error) {
	basis := basisName(op.Gate.Name())
	params := op.Gate.Params()

	switch basis {
	case "GPI":
		t := op.Qubits[0]
		phase := radiansToTurns(params[0])
		return ionqGate{Gate: "gpi", Target: &t, Phase: &phase}, nil
	case "GPI2":
		t := op.Qubits[0]
		phase := radiansToTurns(params[0])
		return ionqGate{Gate: "gpi2", Target: &t, Phase: &phase}, nil
	case "MS":
		p0 := radiansToTurns(params[0])
		p1 := radiansToTurns(params[1])
		angle := 0.25 // default: maximally entangling (π/2)
		return ionqGate{
			Gate:    "ms",
			Targets: []int{op.Qubits[0], op.Qubits[1]},
			Phases:  []float64{p0, p1},
			Angle:   &angle,
		}, nil
	case "ZZ":
		angle := radiansToTurns(params[0])
		return ionqGate{
			Gate:    "zz",
			Targets: []int{op.Qubits[0], op.Qubits[1]},
			Angle:   &angle,
		}, nil
	default:
		return ionqGate{}, fmt.Errorf("ionq: gate %q not supported in native gateset", op.Gate.Name())
	}
}

// basisName strips parameters: "RZ(0.7854)" → "RZ", "S†" → "S".
func basisName(name string) string {
	if idx := strings.IndexByte(name, '('); idx >= 0 {
		return name[:idx]
	}
	// Strip dagger suffix for basis lookup.
	return strings.TrimSuffix(name, "†")
}

func singleTarget(ionqName string, op ir.Operation) ionqGate {
	t := op.Qubits[0]
	return ionqGate{Gate: ionqName, Target: &t}
}

func singleRotation(ionqName string, op ir.Operation) ionqGate {
	t := op.Qubits[0]
	r := op.Gate.Params()[0]
	return ionqGate{Gate: ionqName, Target: &t, Rotation: &r}
}

// radiansToTurns converts radians to turns (1 turn = 2π radians).
func radiansToTurns(rad float64) float64 {
	return rad / (2 * math.Pi)
}

// bitstring converts an integer key from IonQ results to a bitstring.
// IonQ returns big-endian integer keys: "3" for 2 qubits → "11".
func bitstring(key int, numQubits int) string {
	bs := make([]byte, numQubits)
	for i := range numQubits {
		if key&(1<<(numQubits-1-i)) != 0 {
			bs[i] = '1'
		} else {
			bs[i] = '0'
		}
	}
	return string(bs)
}
