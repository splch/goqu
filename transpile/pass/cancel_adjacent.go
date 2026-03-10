package pass

import (
	"math"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile/analysis"
	"github.com/splch/qgo/transpile/target"
)

// CancelAdjacent cancels adjacent inverse gate pairs on the same qubits.
// Iterates to fixpoint.
func CancelAdjacent(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
	ops := c.Ops()
	for {
		cancelled := false
		ops, cancelled = cancelOnce(ops, c.NumQubits(), c.NumClbits())
		if !cancelled {
			break
		}
	}
	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), ops, c.Metadata()), nil
}

func cancelOnce(ops []ir.Operation, numQubits, numClbits int) ([]ir.Operation, bool) {
	if len(ops) == 0 {
		return ops, false
	}

	// Build timelines to find adjacent ops on same qubits.
	tmp := ir.New("", numQubits, numClbits, ops, nil)
	timelines := analysis.BuildTimelines(tmp)

	removed := make([]bool, len(ops))
	cancelled := false

	for i := range ops {
		if removed[i] || ops[i].Gate == nil {
			continue
		}
		op := ops[i]
		if op.Gate.Qubits() != 1 && op.Gate.Qubits() != 2 {
			continue
		}

		// Find the next op on the same qubit(s) with no intervening ops.
		j := findNextAdjacent(timelines, ops, i, removed)
		if j < 0 {
			continue
		}

		if areInverse(op.Gate, ops[j].Gate) && sameQubits(op.Qubits, ops[j].Qubits) {
			removed[i] = true
			removed[j] = true
			cancelled = true
		}
	}

	if !cancelled {
		return ops, false
	}
	var result []ir.Operation
	for i, op := range ops {
		if !removed[i] {
			result = append(result, op)
		}
	}
	return result, true
}

// findNextAdjacent finds the next operation that touches the same qubits as ops[i],
// with no intervening operations on any of those qubits.
func findNextAdjacent(timelines []analysis.QubitTimeline, ops []ir.Operation, i int, removed []bool) int {
	op := ops[i]
	if len(op.Qubits) == 0 {
		return -1
	}

	// For single-qubit: find next on same qubit.
	// For two-qubit: find next on both qubits, ensuring no intervening ops.
	candidates := make(map[int]int) // op index -> count of matching qubits
	for _, q := range op.Qubits {
		next := analysis.NextOnQubit(timelines, q, i)
		for next >= 0 && removed[next] {
			next = analysis.NextOnQubit(timelines, q, next)
		}
		if next >= 0 {
			candidates[next]++
		}
	}

	// For a valid adjacent pair, the candidate must be next on ALL shared qubits.
	for j, count := range candidates {
		if count == len(op.Qubits) {
			return j
		}
	}
	return -1
}

var selfInverseGates = map[gate.Gate]bool{
	gate.H: true, gate.X: true, gate.Y: true, gate.Z: true,
	gate.CNOT: true, gate.CZ: true, gate.SWAP: true,
}

// areInverse checks if two gates are inverse of each other.
func areInverse(a, b gate.Gate) bool {
	// Fixed gate pairs: pointer equality for self-inverse.
	if a == b && selfInverseGates[a] {
		return true
	}

	// S/Sdg pair.
	if (a == gate.S && b == gate.Sdg) || (a == gate.Sdg && b == gate.S) {
		return true
	}
	// T/Tdg pair.
	if (a == gate.T && b == gate.Tdg) || (a == gate.Tdg && b == gate.T) {
		return true
	}
	// SX/SX† pair.
	if a == gate.SX && b.Name() == "SX†" {
		return true
	}
	if b == gate.SX && a.Name() == "SX†" {
		return true
	}

	// Parameterized: same base name with negated parameters.
	pa, pb := a.Params(), b.Params()
	if pa != nil && pb != nil && len(pa) == len(pb) {
		nameA := baseName(a)
		nameB := baseName(b)
		if nameA == nameB {
			allNeg := true
			for k := range pa {
				if math.Abs(pa[k]+pb[k]) > 1e-12 {
					allNeg = false
					break
				}
			}
			if allNeg {
				return true
			}
		}
	}

	return false
}

func sameQubits(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// baseName extracts the gate name without parameters.
func baseName(g gate.Gate) string {
	name := g.Name()
	for i := range len(name) {
		if name[i] == '(' {
			return name[:i]
		}
	}
	return name
}
