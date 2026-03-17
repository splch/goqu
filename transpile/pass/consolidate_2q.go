package pass

import (
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/decompose"
	"github.com/splch/goqu/transpile/target"
)

// Consolidate2QBlocks finds maximal runs of 1Q+2Q operations on the same two
// qubits, computes their combined 4x4 unitary, and re-synthesizes via KAK
// decomposition. Replaces the block only if the new sequence uses fewer 2Q
// gates or fewer total operations.
func Consolidate2QBlocks(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
	eb := decompose.BasisForTarget(t.BasisGates)
	ops := c.Ops()
	blocks := findBlocks(ops)
	if len(blocks) == 0 {
		return c, nil
	}

	// Build replacement map: first op index -> new ops; other indices -> remove.
	replacements := make(map[int][]ir.Operation)
	removals := make(map[int]bool)

	for _, blk := range blocks {
		blockOps := make([]ir.Operation, len(blk.opIndices))
		for i, idx := range blk.opIndices {
			blockOps[i] = ops[idx]
		}

		unitary := decompose.OpsToUnitary4(blockOps, blk.q0, blk.q1)

		if decompose.IsIdentity(unitary, 4, 1e-10) {
			for _, idx := range blk.opIndices {
				removals[idx] = true
			}
			continue
		}

		newOps := decompose.KAKForBasis(unitary, blk.q0, blk.q1, eb)
		if newOps == nil {
			continue // KAK failed; leave block unchanged
		}

		orig2Q := count2QOps(blockOps)
		new2Q := count2QOps(newOps)
		if new2Q < orig2Q || (new2Q == orig2Q && len(newOps) < len(blockOps)) {
			replacements[blk.opIndices[0]] = newOps
			for _, idx := range blk.opIndices[1:] {
				removals[idx] = true
			}
		}
	}

	if len(replacements) == 0 && len(removals) == 0 {
		return c, nil
	}

	var result []ir.Operation
	for i, op := range ops {
		if removals[i] {
			continue
		}
		if rep, ok := replacements[i]; ok {
			result = append(result, rep...)
		} else {
			result = append(result, op)
		}
	}
	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), result, c.Metadata()), nil
}

// block2q represents a maximal run of 1Q+2Q operations on the same qubit pair.
type block2q struct {
	q0, q1    int   // canonical pair (q0 < q1)
	opIndices []int // indices into the circuit ops slice
}

// findBlocks walks the operation list linearly and identifies maximal 2Q blocks.
func findBlocks(ops []ir.Operation) []block2q {
	// active maps qubit -> the block it's currently part of (if any).
	active := make(map[int]*block2q)
	var finished []block2q

	finishQubit := func(q int) {
		blk := active[q]
		if blk == nil {
			return
		}
		delete(active, blk.q0)
		delete(active, blk.q1)
		if len(blk.opIndices) >= 2 && has2QGate(ops, blk.opIndices) {
			finished = append(finished, *blk)
		}
	}

	for i, op := range ops {
		if op.Gate == nil || op.Condition != nil || op.ControlFlow != nil {
			// Measurement, condition, or control flow: close blocks on affected qubits.
			for _, q := range op.Qubits {
				finishQubit(q)
			}
			continue
		}
		name := op.Gate.Name()
		if name == "barrier" || name == "reset" || name == "delay" {
			for _, q := range op.Qubits {
				finishQubit(q)
			}
			continue
		}
		if op.Gate.Matrix() == nil {
			// Non-unitary gate (e.g. StatePrep): break blocks on affected qubits.
			for _, q := range op.Qubits {
				finishQubit(q)
			}
			continue
		}

		nq := op.Gate.Qubits()

		if nq == 1 {
			q := op.Qubits[0]
			if blk := active[q]; blk != nil {
				blk.opIndices = append(blk.opIndices, i)
			}
			continue
		}

		if nq == 2 {
			qa, qb := op.Qubits[0], op.Qubits[1]
			q0, q1 := qa, qb
			if q0 > q1 {
				q0, q1 = q1, q0
			}

			blk0 := active[q0]
			blk1 := active[q1]

			if blk0 != nil && blk0 == blk1 && blk0.q0 == q0 && blk0.q1 == q1 {
				// Same active block on this pair — extend.
				blk0.opIndices = append(blk0.opIndices, i)
				continue
			}

			// Close any existing blocks on these qubits.
			if blk0 != nil {
				finishQubit(q0)
			}
			if blk1 != nil {
				finishQubit(q1)
			}

			// Start a new block.
			blk := &block2q{q0: q0, q1: q1, opIndices: []int{i}}
			active[q0] = blk
			active[q1] = blk
			continue
		}

		// >2Q gate: close blocks on all affected qubits.
		for _, q := range op.Qubits {
			finishQubit(q)
		}
	}

	// Close remaining active blocks.
	closed := make(map[*block2q]bool)
	for _, blk := range active {
		if blk != nil && !closed[blk] {
			closed[blk] = true
			if len(blk.opIndices) >= 2 && has2QGate(ops, blk.opIndices) {
				finished = append(finished, *blk)
			}
		}
	}
	return finished
}

func has2QGate(ops []ir.Operation, indices []int) bool {
	for _, idx := range indices {
		if ops[idx].Gate != nil && ops[idx].Gate.Qubits() == 2 {
			return true
		}
	}
	return false
}

func count2QOps(ops []ir.Operation) int {
	n := 0
	for _, op := range ops {
		if op.Gate != nil && op.Gate.Qubits() >= 2 {
			n++
		}
	}
	return n
}
