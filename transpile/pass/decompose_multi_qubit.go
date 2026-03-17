package pass

import (
	"fmt"

	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/decompose"
	"github.com/splch/goqu/transpile/target"
)

// DecomposeMultiQubit decomposes gates with more than 2 qubits into sequences
// of 1-qubit and 2-qubit gates. This must run before qubit routing so that
// SABRE can see all 2-qubit interactions.
func DecomposeMultiQubit(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
	var result []ir.Operation
	changed := false
	for _, op := range c.Ops() {
		if op.Gate == nil || op.Gate.Qubits() <= 2 {
			result = append(result, op)
			continue
		}
		decomposed := decomposeToTwoQubit(op, 0)
		if decomposed == nil {
			return nil, fmt.Errorf("decompose_multi_qubit: cannot decompose %d-qubit gate %q",
				op.Gate.Qubits(), op.Gate.Name())
		}
		if op.Condition != nil {
			for i := range decomposed {
				decomposed[i].Condition = op.Condition
			}
		}
		result = append(result, decomposed...)
		changed = true
	}
	if !changed {
		return c, nil
	}
	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), result, c.Metadata()), nil
}

// decomposeToTwoQubit recursively breaks a >2-qubit gate into ≤2-qubit gates.
func decomposeToTwoQubit(op ir.Operation, depth int) []ir.Operation {
	if depth > maxDecomposeDepth {
		return nil
	}
	if op.Gate.Qubits() <= 2 {
		return []ir.Operation{op}
	}

	// Try the gate's own Decompose method first.
	applied := op.Gate.Decompose(op.Qubits)
	if applied != nil {
		var result []ir.Operation
		for _, a := range applied {
			sub := ir.Operation{Gate: a.Gate, Qubits: a.Qubits}
			if a.Gate.Qubits() <= 2 {
				result = append(result, sub)
			} else {
				inner := decomposeToTwoQubit(sub, depth+1)
				if inner == nil {
					return nil
				}
				result = append(result, inner...)
			}
		}
		return result
	}

	// Fall back to rule-based decomposition with a generic gate set.
	ruleOps := decompose.DecomposeByRule(op.Gate, op.Qubits,
		[]string{"CX", "CNOT", "H", "T", "Tdg", "S", "Sdg", "RZ", "RY", "SX", "X"})
	if ruleOps == nil {
		return nil
	}
	var result []ir.Operation
	for _, sub := range ruleOps {
		if sub.Gate.Qubits() <= 2 {
			result = append(result, sub)
		} else {
			inner := decomposeToTwoQubit(sub, depth+1)
			if inner == nil {
				return nil
			}
			result = append(result, inner...)
		}
	}
	return result
}
