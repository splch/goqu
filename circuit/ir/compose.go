package ir

import (
	"fmt"
	"strconv"
)

// Compose appends c2's operations after c1's, with optional qubit/clbit remapping.
// nil maps use identity mapping (c2 indices used as-is; c2 must fit within c1 dimensions).
// Non-nil maps must cover every index used by c2's operations, and all targets
// must be in range of the resulting circuit.
func Compose(c1, c2 *Circuit, qubitMap, clbitMap map[int]int) (*Circuit, error) {
	numQ := c1.NumQubits()
	numC := c1.NumClbits()

	// Validate and determine result dimensions.
	if qubitMap != nil {
		for _, target := range qubitMap {
			if target < 0 {
				return nil, fmt.Errorf("ir.Compose: negative target qubit %d", target)
			}
			if target+1 > numQ {
				numQ = target + 1
			}
		}
	} else if c2.NumQubits() > c1.NumQubits() {
		return nil, fmt.Errorf("ir.Compose: c2 has %d qubits, exceeds c1's %d (provide qubitMap)", c2.NumQubits(), c1.NumQubits())
	}
	if clbitMap != nil {
		for _, target := range clbitMap {
			if target < 0 {
				return nil, fmt.Errorf("ir.Compose: negative target clbit %d", target)
			}
			if target+1 > numC {
				numC = target + 1
			}
		}
	} else if c2.NumClbits() > c1.NumClbits() {
		return nil, fmt.Errorf("ir.Compose: c2 has %d clbits, exceeds c1's %d (provide clbitMap)", c2.NumClbits(), c1.NumClbits())
	}

	result := make([]Operation, c1.NumOps(), c1.NumOps()+c2.NumOps())
	for i := range c1.NumOps() {
		result[i] = c1.Op(i)
	}

	for i := range c2.NumOps() {
		op := c2.Op(i)
		remapped, err := remapOp(op, qubitMap, clbitMap)
		if err != nil {
			return nil, fmt.Errorf("ir.Compose: op %d: %w", i, err)
		}
		result = append(result, remapped)
	}

	return New(c1.Name(), numQ, numC, result, c1.Metadata()), nil
}

// Tensor returns a new circuit with c1 and c2 on disjoint qubit/clbit spaces.
// c2's indices are shifted by c1.NumQubits() and c1.NumClbits() respectively.
func Tensor(c1, c2 *Circuit) *Circuit {
	qShift := c1.NumQubits()
	cShift := c1.NumClbits()

	result := make([]Operation, c1.NumOps(), c1.NumOps()+c2.NumOps())
	for i := range c1.NumOps() {
		result[i] = c1.Op(i)
	}

	for i := range c2.NumOps() {
		op := c2.Op(i)
		result = append(result, shiftOp(op, qShift, cShift))
	}

	name := c1.Name() + "⊗" + c2.Name()
	return New(name, c1.NumQubits()+c2.NumQubits(), c1.NumClbits()+c2.NumClbits(), result, c1.Metadata())
}

// Inverse reverses operation order and adjoints each gate.
// Measurements, resets, barriers, and control flow ops are dropped (irreversible / non-unitary).
func Inverse(c *Circuit) *Circuit {
	var result []Operation
	for i := c.NumOps() - 1; i >= 0; i-- {
		op := c.Op(i)
		// Drop control flow operations.
		if op.ControlFlow != nil {
			continue
		}
		// Drop measurements (nil gate with clbits).
		if op.Gate == nil {
			continue
		}
		// Drop resets and barriers.
		name := op.Gate.Name()
		if name == "reset" || name == "barrier" || name == "delay" {
			continue
		}
		result = append(result, Operation{
			Gate:      op.Gate.Inverse(),
			Qubits:    op.Qubits,
			Condition: op.Condition,
		})
	}

	return New(c.Name()+"†", c.NumQubits(), c.NumClbits(), result, c.Metadata())
}

// Repeat concatenates c's operations n times sequentially. n must be >= 1.
func Repeat(c *Circuit, n int) (*Circuit, error) {
	if n < 1 {
		return nil, fmt.Errorf("ir.Repeat: n must be >= 1, got %d", n)
	}
	nOps := c.NumOps()
	result := make([]Operation, 0, nOps*n)
	for range n {
		for i := range nOps {
			result = append(result, c.Op(i))
		}
	}

	name := c.Name() + "×" + strconv.Itoa(n)
	return New(name, c.NumQubits(), c.NumClbits(), result, c.Metadata()), nil
}

// remapSlice applies a mapping to a slice of indices.
func remapSlice(indices []int, mapping map[int]int) ([]int, error) {
	if mapping == nil {
		out := make([]int, len(indices))
		copy(out, indices)
		return out, nil
	}
	out := make([]int, len(indices))
	for i, idx := range indices {
		target, ok := mapping[idx]
		if !ok {
			return nil, fmt.Errorf("index %d has no mapping", idx)
		}
		out[i] = target
	}
	return out, nil
}

// remapOp returns a copy of op with qubit/clbit indices remapped.
func remapOp(op Operation, qubitMap, clbitMap map[int]int) (Operation, error) {
	// Control flow operations: recurse into bodies.
	if op.ControlFlow != nil {
		cf := *op.ControlFlow
		cf.Bodies = make([][]Operation, len(op.ControlFlow.Bodies))
		for i, body := range op.ControlFlow.Bodies {
			remapped := make([]Operation, len(body))
			for j, bop := range body {
				r, err := remapOp(bop, qubitMap, clbitMap)
				if err != nil {
					return Operation{}, err
				}
				remapped[j] = r
			}
			cf.Bodies[i] = remapped
		}
		// Remap condition clbit.
		if cf.Type == ControlFlowWhile || cf.Type == ControlFlowIfElse {
			if clbitMap != nil {
				target, ok := clbitMap[cf.Condition.Clbit]
				if !ok {
					return Operation{}, fmt.Errorf("control flow condition clbit %d has no mapping", cf.Condition.Clbit)
				}
				cf.Condition.Clbit = target
			}
		}
		// Remap switch clbits.
		if cf.SwitchArg != nil {
			sa := *cf.SwitchArg
			remapped, err := remapSlice(sa.Clbits, clbitMap)
			if err != nil {
				return Operation{}, fmt.Errorf("switch clbit remap: %w", err)
			}
			sa.Clbits = remapped
			cf.SwitchArg = &sa
		}
		return Operation{ControlFlow: &cf}, nil
	}

	qubits, err := remapSlice(op.Qubits, qubitMap)
	if err != nil {
		return Operation{}, fmt.Errorf("qubit remap: %w", err)
	}
	clbits, err := remapSlice(op.Clbits, clbitMap)
	if err != nil {
		return Operation{}, fmt.Errorf("clbit remap: %w", err)
	}

	result := Operation{
		Gate:   op.Gate,
		Qubits: qubits,
		Clbits: clbits,
	}
	if op.Condition != nil {
		remappedClbit := op.Condition.Clbit
		if clbitMap != nil {
			target, ok := clbitMap[op.Condition.Clbit]
			if !ok {
				return Operation{}, fmt.Errorf("condition clbit %d has no mapping", op.Condition.Clbit)
			}
			remappedClbit = target
		}
		result.Condition = &Condition{
			Clbit: remappedClbit,
			Value: op.Condition.Value,
		}
	}
	return result, nil
}

// shiftOp returns a copy of op with all qubit/clbit indices shifted by the given offsets.
// Used by Tensor to place operations on disjoint index spaces.
func shiftOp(op Operation, qShift, cShift int) Operation {
	if op.ControlFlow != nil {
		cf := *op.ControlFlow
		cf.Bodies = make([][]Operation, len(op.ControlFlow.Bodies))
		for i, body := range op.ControlFlow.Bodies {
			shifted := make([]Operation, len(body))
			for j, bop := range body {
				shifted[j] = shiftOp(bop, qShift, cShift)
			}
			cf.Bodies[i] = shifted
		}
		if cf.Type == ControlFlowWhile || cf.Type == ControlFlowIfElse {
			cf.Condition.Clbit += cShift
		}
		if cf.SwitchArg != nil {
			sa := *cf.SwitchArg
			sa.Clbits = make([]int, len(cf.SwitchArg.Clbits))
			for i, c := range cf.SwitchArg.Clbits {
				sa.Clbits[i] = c + cShift
			}
			cf.SwitchArg = &sa
		}
		return Operation{ControlFlow: &cf}
	}

	shifted := Operation{Gate: op.Gate}
	if len(op.Qubits) > 0 {
		shifted.Qubits = make([]int, len(op.Qubits))
		for j, q := range op.Qubits {
			shifted.Qubits[j] = q + qShift
		}
	}
	if len(op.Clbits) > 0 {
		shifted.Clbits = make([]int, len(op.Clbits))
		for j, c := range op.Clbits {
			shifted.Clbits[j] = c + cShift
		}
	}
	if op.Condition != nil {
		shifted.Condition = &Condition{
			Clbit: op.Condition.Clbit + cShift,
			Value: op.Condition.Value,
		}
	}
	return shifted
}
