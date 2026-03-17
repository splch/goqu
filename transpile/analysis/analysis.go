// Package analysis provides circuit analysis helpers for transpilation passes.
package analysis

import "github.com/splch/goqu/circuit/ir"

// QubitTimeline tracks the operation indices that touch a given qubit.
type QubitTimeline struct {
	Qubit int
	Ops   []int // operation indices in circuit order
}

// BuildTimelines returns per-qubit timelines for all qubits in the circuit.
func BuildTimelines(c *ir.Circuit) []QubitTimeline {
	tl := make([]QubitTimeline, c.NumQubits())
	for i := range tl {
		tl[i].Qubit = i
	}
	c.RangeOps(func(idx int, op ir.Operation) bool {
		for _, q := range op.Qubits {
			if q < len(tl) {
				tl[q].Ops = append(tl[q].Ops, idx)
			}
		}
		return true
	})
	return tl
}

// NextOnQubit returns the next operation index on the given qubit after afterIdx.
// Returns -1 if none.
func NextOnQubit(timelines []QubitTimeline, qubit, afterIdx int) int {
	if qubit < 0 || qubit >= len(timelines) {
		return -1
	}
	for _, idx := range timelines[qubit].Ops {
		if idx > afterIdx {
			return idx
		}
	}
	return -1
}

// PrevOnQubit returns the previous operation index on the given qubit before beforeIdx.
// Returns -1 if none.
func PrevOnQubit(timelines []QubitTimeline, qubit, beforeIdx int) int {
	if qubit < 0 || qubit >= len(timelines) {
		return -1
	}
	ops := timelines[qubit].Ops
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i] < beforeIdx {
			return ops[i]
		}
	}
	return -1
}
