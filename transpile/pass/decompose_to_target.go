package pass

import (
	"fmt"

	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile"
	"github.com/splch/qgo/transpile/decompose"
	"github.com/splch/qgo/transpile/target"
)

// maxDecomposeDepth limits recursive decomposition to prevent infinite loops.
// Depth 10 handles multi-controlled gates (e.g. C^5(X) → recursive V-gate → CCX → CX → basis).
const maxDecomposeDepth = 10

// DecomposeToTarget replaces non-basis gates with basis gate sequences.
func DecomposeToTarget(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
	eb := decompose.BasisForTarget(t.BasisGates)
	var result []ir.Operation
	for _, op := range c.Ops() {
		if op.Gate == nil {
			result = append(result, op)
			continue
		}
		if op.Gate.Name() == "barrier" {
			continue
		}

		bname := transpile.BasisName(op.Gate)
		if t.HasBasisGate(bname) {
			result = append(result, op)
			continue
		}

		decomposed, err := decomposeOp(op, t, 0, eb)
		if err != nil {
			return nil, err
		}
		result = append(result, decomposed...)
	}

	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), result, c.Metadata()), nil
}

// decomposeOp decomposes a single operation to target basis gates, recursively.
func decomposeOp(op ir.Operation, t target.Target, depth int, eb decompose.EulerBasis) ([]ir.Operation, error) {
	if depth > maxDecomposeDepth {
		return nil, fmt.Errorf("decompose: max depth exceeded for gate %q on qubits %v",
			op.Gate.Name(), op.Qubits)
	}

	bname := transpile.BasisName(op.Gate)
	if t.HasBasisGate(bname) {
		return []ir.Operation{op}, nil
	}

	// Try rule-based decomposition first.
	ruleOps := decompose.DecomposeByRule(op.Gate, op.Qubits, t.BasisGates)
	if ruleOps != nil {
		return expandAndRecurse(ruleOps, t, depth, eb)
	}

	// Try Euler decomposition for single-qubit gates.
	if op.Gate.Qubits() == 1 {
		eulerOps := decompose.EulerDecomposeForBasis(op.Gate, op.Qubits[0], eb)
		if eulerOps != nil {
			return expandAndRecurse(eulerOps, t, depth, eb)
		}
		// Identity gate: no ops needed.
		return nil, nil
	}

	// Try KAK for 2-qubit gates.
	if op.Gate.Qubits() == 2 {
		kakOps := decompose.KAKForBasis(op.Gate.Matrix(), op.Qubits[0], op.Qubits[1], eb)
		if kakOps != nil {
			return expandAndRecurse(kakOps, t, depth, eb)
		}
	}

	// Try 3-qubit: decompose to 2-qubit + 1-qubit via gate's own decomposition.
	if op.Gate.Qubits() == 3 {
		applied := op.Gate.Decompose(op.Qubits)
		if applied != nil {
			var ops []ir.Operation
			for _, a := range applied {
				ops = append(ops, ir.Operation{Gate: a.Gate, Qubits: a.Qubits})
			}
			return expandAndRecurse(ops, t, depth, eb)
		}
		// Try rule-based decomposition for 3-qubit gates to CX basis.
		ruleOps := decompose.DecomposeByRule(op.Gate, op.Qubits, []string{"CX", "H", "T", "Tdg", "S", "Sdg", "RZ", "RY"})
		if ruleOps != nil {
			return expandAndRecurse(ruleOps, t, depth, eb)
		}
	}

	// Try multi-controlled gates (>3 qubits).
	if op.Gate.Qubits() > 3 {
		ruleOps := decompose.DecomposeByRule(op.Gate, op.Qubits, t.BasisGates)
		if ruleOps != nil {
			return expandAndRecurse(ruleOps, t, depth, eb)
		}
	}

	return nil, fmt.Errorf("decompose: cannot decompose gate %q (%d qubits) to target %q basis %v",
		op.Gate.Name(), op.Gate.Qubits(), t.Name, t.BasisGates)
}

// expandAndRecurse recursively decomposes ops that are still not in basis.
func expandAndRecurse(ops []ir.Operation, t target.Target, depth int, eb decompose.EulerBasis) ([]ir.Operation, error) {
	var result []ir.Operation
	for _, op := range ops {
		if op.Gate == nil {
			result = append(result, op)
			continue
		}
		bname := transpile.BasisName(op.Gate)
		if t.HasBasisGate(bname) {
			result = append(result, op)
		} else {
			sub, err := decomposeOp(op, t, depth+1, eb)
			if err != nil {
				return nil, err
			}
			result = append(result, sub...)
		}
	}
	return result, nil
}
