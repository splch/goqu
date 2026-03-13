package mitigation

import (
	"fmt"

	"github.com/splch/goqu/circuit/ir"
)

// ScaleMethod selects the noise-scaling strategy for ZNE.
type ScaleMethod int

const (
	// UnitaryFolding scales noise by appending C†C pairs to the full circuit:
	// C → C (C† C)^((s-1)/2). Preserves the logical unitary.
	UnitaryFolding ScaleMethod = iota
	// LocalFolding scales noise by replacing each gate G with
	// G (G† G)^((s-1)/2). Preserves the logical unitary per gate.
	LocalFolding
)

// FoldCircuit returns a new circuit with noise scaled by scaleFactor.
// scaleFactor must be a positive odd integer (1, 3, 5, ...).
// A scaleFactor of 1 returns an unmodified copy.
func FoldCircuit(circuit *ir.Circuit, scaleFactor int, method ScaleMethod) (*ir.Circuit, error) {
	if scaleFactor < 1 || scaleFactor%2 == 0 {
		return nil, fmt.Errorf("mitigation.FoldCircuit: scaleFactor must be a positive odd integer, got %d", scaleFactor)
	}
	if scaleFactor == 1 {
		// Return an identical copy.
		return ir.New(circuit.Name(), circuit.NumQubits(), circuit.NumClbits(),
			circuit.Ops(), circuit.Metadata()), nil
	}

	switch method {
	case UnitaryFolding:
		return foldUnitary(circuit, scaleFactor)
	case LocalFolding:
		return foldLocalFolding(circuit, scaleFactor)
	default:
		return nil, fmt.Errorf("mitigation.FoldCircuit: unknown method %d", method)
	}
}

// foldUnitary implements C → C (C† C)^((s-1)/2).
func foldUnitary(circuit *ir.Circuit, scaleFactor int) (*ir.Circuit, error) {
	nPairs := (scaleFactor - 1) / 2
	inv := ir.Inverse(circuit)

	result := circuit
	var err error
	for range nPairs {
		// Append C†.
		result, err = ir.Compose(result, inv, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("mitigation.FoldCircuit: compose inverse: %w", err)
		}
		// Append C.
		result, err = ir.Compose(result, circuit, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("mitigation.FoldCircuit: compose forward: %w", err)
		}
	}
	return result, nil
}

// foldLocalFolding implements per-gate folding:
// each gate G → G (G† G)^((s-1)/2).
func foldLocalFolding(circuit *ir.Circuit, scaleFactor int) (*ir.Circuit, error) {
	nPairs := (scaleFactor - 1) / 2
	ops := circuit.Ops()
	result := make([]ir.Operation, 0, len(ops)*scaleFactor)

	for _, op := range ops {
		// Keep non-gate ops (measurements, barriers) as-is.
		if op.Gate == nil {
			result = append(result, op)
			continue
		}
		name := op.Gate.Name()
		if name == "reset" || name == "barrier" {
			result = append(result, op)
			continue
		}

		// Original gate.
		result = append(result, op)

		// Append (G† G) pairs.
		invGate := op.Gate.Inverse()
		for range nPairs {
			result = append(result, ir.Operation{
				Gate:   invGate,
				Qubits: op.Qubits,
			})
			result = append(result, ir.Operation{
				Gate:   op.Gate,
				Qubits: op.Qubits,
			})
		}
	}

	return ir.New(circuit.Name(), circuit.NumQubits(), circuit.NumClbits(),
		result, circuit.Metadata()), nil
}
