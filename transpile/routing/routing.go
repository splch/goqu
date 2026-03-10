package routing

import (
	"math"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile/target"
)

// Route inserts SWAP gates to satisfy target connectivity constraints.
// Uses the SABRE algorithm (Li et al., 2019).
// Returns the circuit unchanged for all-to-all targets.
func Route(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
	if t.Connectivity == nil {
		return c, nil
	}

	dist := t.DistanceMatrix()
	adj := t.AdjacencyMap()

	// Forward pass.
	fwdOps, fwdLayout := sabrePass(c, t, dist, adj, TrivialLayout(c.NumQubits()), false)

	// Backward pass: reverse the circuit, use forward's final layout.
	revOps, _ := sabrePass(c, t, dist, adj, fwdLayout, true)

	// Use whichever produced fewer SWAPs.
	fwdSwaps := countSwaps(fwdOps)
	revSwaps := countSwaps(revOps)

	var resultOps []ir.Operation
	if revSwaps < fwdSwaps {
		// Reverse revOps back to forward order.
		for i, j := 0, len(revOps)-1; i < j; i, j = i+1, j-1 {
			revOps[i], revOps[j] = revOps[j], revOps[i]
		}
		resultOps = revOps
	} else {
		resultOps = fwdOps
	}

	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), resultOps, c.Metadata()), nil
}

// sabrePass runs one direction of the SABRE algorithm.
func sabrePass(c *ir.Circuit, t target.Target, dist [][]int, adj map[int][]int, initialLayout []int, reverse bool) ([]ir.Operation, []int) {
	ops := c.Ops()
	if reverse {
		// Reverse the operation order.
		rev := make([]ir.Operation, len(ops))
		for i, op := range ops {
			rev[len(ops)-1-i] = op
		}
		ops = rev
	}

	n := c.NumQubits()
	layout := make([]int, n) // logical → physical
	copy(layout, initialLayout)
	inv := InverseLayout(layout) // physical → logical

	// Build dependency tracking: for each op, count of unexecuted predecessors.
	numOps := len(ops)
	executed := make([]bool, numOps)

	// Per-qubit: track which ops touch each qubit in order.
	qubitOps := make([][]int, n)
	for idx, op := range ops {
		for _, q := range op.Qubits {
			if q < n {
				qubitOps[q] = append(qubitOps[q], idx)
			}
		}
	}

	// For each op, count predecessors (previous ops on same qubit).
	predCount := make([]int, numOps)
	for q := range n {
		for k := 1; k < len(qubitOps[q]); k++ {
			idx := qubitOps[q][k]
			// Count all prior ops on this qubit as predecessors.
			predCount[idx] += k
		}
	}

	var result []ir.Operation

	for {
		// Build front layer: ops with all predecessors executed.
		var front []int
		for i := range numOps {
			if !executed[i] && predCount[i] == 0 {
				front = append(front, i)
			}
		}
		if len(front) == 0 {
			break
		}

		// Execute ops that are directly connected.
		progress := false
		for _, idx := range front {
			op := ops[idx]
			if op.Gate == nil || op.Gate.Qubits() <= 1 {
				// Single-qubit or measurement: remap qubit and execute.
				mappedOp := remapOp(op, layout)
				result = append(result, mappedOp)
				markExecuted(idx, ops, executed, predCount, qubitOps, n)
				progress = true
				continue
			}

			// Multi-qubit gate: check connectivity.
			q0, q1 := op.Qubits[0], op.Qubits[1]
			p0, p1 := layout[q0], layout[q1]
			if p0 >= 0 && p0 < len(dist) && p1 >= 0 && p1 < len(dist) && dist[p0][p1] == 1 {
				mappedOp := remapOp(op, layout)
				result = append(result, mappedOp)
				markExecuted(idx, ops, executed, predCount, qubitOps, n)
				progress = true
			}
		}

		if progress {
			continue
		}

		// No directly executable ops: find best SWAP.
		bestSwap := [2]int{-1, -1}
		bestCost := math.Inf(1)

		// Build lookahead layer (next ops after front).
		var lookahead []int
		for _, idx := range front {
			op := ops[idx]
			for _, q := range op.Qubits {
				for _, nextIdx := range qubitOps[q] {
					if nextIdx > idx && !executed[nextIdx] {
						lookahead = append(lookahead, nextIdx)
						break
					}
				}
			}
		}

		// Evaluate candidate SWAPs on connected physical pairs.
		for phys0, neighbors := range adj {
			for _, phys1 := range neighbors {
				if phys0 >= phys1 {
					continue // avoid duplicates
				}

				// Simulate the SWAP.
				log0, log1 := inv[phys0], inv[phys1]
				// Temporarily swap.
				layout[log0], layout[log1] = layout[log1], layout[log0]

				cost := 0.0
				for _, idx := range front {
					op := ops[idx]
					if op.Gate != nil && op.Gate.Qubits() >= 2 {
						q0, q1 := op.Qubits[0], op.Qubits[1]
						p0, p1 := layout[q0], layout[q1]
						if p0 >= 0 && p0 < len(dist) && p1 >= 0 && p1 < len(dist) && dist[p0][p1] >= 0 {
							cost += float64(dist[p0][p1])
						}
					}
				}
				for _, idx := range lookahead {
					op := ops[idx]
					if op.Gate != nil && op.Gate.Qubits() >= 2 {
						q0, q1 := op.Qubits[0], op.Qubits[1]
						p0, p1 := layout[q0], layout[q1]
						if p0 >= 0 && p0 < len(dist) && p1 >= 0 && p1 < len(dist) && dist[p0][p1] >= 0 {
							cost += 0.5 * float64(dist[p0][p1])
						}
					}
				}

				// Undo swap.
				layout[log0], layout[log1] = layout[log1], layout[log0]

				if cost < bestCost {
					bestCost = cost
					bestSwap = [2]int{phys0, phys1}
				}
			}
		}

		if bestSwap[0] < 0 {
			// No valid SWAP found; skip remaining blocked ops.
			break
		}

		// Insert SWAP.
		result = append(result, ir.Operation{
			Gate:   gate.SWAP,
			Qubits: []int{bestSwap[0], bestSwap[1]},
		})

		// Update layout.
		log0, log1 := inv[bestSwap[0]], inv[bestSwap[1]]
		layout[log0], layout[log1] = layout[log1], layout[log0]
		inv[bestSwap[0]], inv[bestSwap[1]] = inv[bestSwap[1]], inv[bestSwap[0]]
	}

	return result, layout
}

// markExecuted marks an operation as executed and updates predecessor counts.
func markExecuted(idx int, ops []ir.Operation, executed []bool, predCount []int, qubitOps [][]int, n int) {
	executed[idx] = true
	op := ops[idx]
	for _, q := range op.Qubits {
		if q >= n {
			continue
		}
		// Decrement predCount of subsequent ops on this qubit.
		found := false
		for _, nextIdx := range qubitOps[q] {
			if nextIdx == idx {
				found = true
				continue
			}
			if found && !executed[nextIdx] {
				predCount[nextIdx]--
				break
			}
		}
	}
}

// remapOp remaps logical qubits to physical qubits using the layout.
func remapOp(op ir.Operation, layout []int) ir.Operation {
	mapped := ir.Operation{
		Gate:      op.Gate,
		Clbits:    op.Clbits,
		Condition: op.Condition,
	}
	mapped.Qubits = make([]int, len(op.Qubits))
	for i, q := range op.Qubits {
		if q >= 0 && q < len(layout) {
			mapped.Qubits[i] = layout[q]
		} else {
			mapped.Qubits[i] = q
		}
	}
	return mapped
}

// countSwaps counts SWAP gates in an operation list.
func countSwaps(ops []ir.Operation) int {
	count := 0
	for _, op := range ops {
		if op.Gate != nil && op.Gate.Name() == "SWAP" {
			count++
		}
	}
	return count
}
