package routing

import "github.com/splch/goqu/circuit/ir"

// dag is a dependency-ordered directed acyclic graph of circuit operations.
// It orders operations so that each operation executes only after all prior
// operations on its qubits have completed. This enables SABRE to identify
// which gates are ready to execute (the "front layer") at any point during
// routing.
type dag struct {
	ops       []ir.Operation
	nQubits   int
	qubitOps  [][]int // per-qubit ordered op indices
	predCount []int   // immediate unexecuted predecessors (one per qubit line)
	executed  []bool
}

// newDAG builds a dependency DAG from operations.
// If reverse is true, operations are traversed in reverse order.
func newDAG(ops []ir.Operation, nQubits int, reverse bool) *dag {
	work := ops
	if reverse {
		work = make([]ir.Operation, len(ops))
		for i, op := range ops {
			work[len(ops)-1-i] = op
		}
	}

	numOps := len(work)
	d := &dag{
		ops:       work,
		nQubits:   nQubits,
		qubitOps:  make([][]int, nQubits),
		predCount: make([]int, numOps),
		executed:  make([]bool, numOps),
	}

	for idx, op := range work {
		for _, q := range op.Qubits {
			if q >= 0 && q < nQubits {
				d.qubitOps[q] = append(d.qubitOps[q], idx)
			}
		}
	}

	// Each op's predecessor count = number of qubit lines where it is not first.
	// This is exactly 1 per qubit line that has a prior op (not k, just 1).
	for q := range nQubits {
		for k := 1; k < len(d.qubitOps[q]); k++ {
			d.predCount[d.qubitOps[q][k]]++
		}
	}

	return d
}

// frontLayer returns indices of ops with all predecessors executed.
func (d *dag) frontLayer() []int {
	var front []int
	for i := range len(d.ops) {
		if !d.executed[i] && d.predCount[i] == 0 {
			front = append(front, i)
		}
	}
	return front
}

// markExecuted marks an op as executed and decrements successor predCounts.
func (d *dag) markExecuted(idx int) {
	d.executed[idx] = true
	op := d.ops[idx]
	for _, q := range op.Qubits {
		if q < 0 || q >= d.nQubits {
			continue
		}
		found := false
		for _, nextIdx := range d.qubitOps[q] {
			if nextIdx == idx {
				found = true
				continue
			}
			if found && !d.executed[nextIdx] {
				d.predCount[nextIdx]--
				break
			}
		}
	}
}

// extendedSet returns layers of future 2-qubit ops reachable from the front
// layer via successor edges. depth controls how many layers to explore.
//
// The extended set contains 2-qubit gates reachable from the front layer
// within a configurable lookahead depth. Including these future gates in
// SWAP scoring helps SABRE make globally better routing decisions: a SWAP
// that looks suboptimal for the immediate front layer may be beneficial
// when considering upcoming gates. Each deeper layer is weighted
// geometrically less (via ExtendedSetWeight) so that nearer gates have
// more influence on the routing decision.
func (d *dag) extendedSet(front []int, depth int) [][]int {
	if depth <= 0 {
		return nil
	}

	// Collect the set of ops in the current front layer.
	inFront := make(map[int]bool, len(front))
	for _, idx := range front {
		inFront[idx] = true
	}

	visited := make(map[int]bool, len(front))
	for _, idx := range front {
		visited[idx] = true
	}

	layers := make([][]int, 0, depth)
	currentLayer := front

	for layer := 0; layer < depth; layer++ {
		var nextLayer []int
		seen := make(map[int]bool)
		for _, idx := range currentLayer {
			op := d.ops[idx]
			for _, q := range op.Qubits {
				if q < 0 || q >= d.nQubits {
					continue
				}
				// Find the next unexecuted, unvisited op on this qubit.
				found := false
				for _, succIdx := range d.qubitOps[q] {
					if succIdx == idx {
						found = true
						continue
					}
					if found && !d.executed[succIdx] && !visited[succIdx] && !inFront[succIdx] {
						if !seen[succIdx] {
							seen[succIdx] = true
							nextLayer = append(nextLayer, succIdx)
						}
						break
					}
				}
			}
		}

		// Filter to only 2-qubit ops for the extended set.
		var twoQ []int
		for _, idx := range nextLayer {
			op := d.ops[idx]
			if op.Gate != nil && op.Gate.Qubits() >= 2 {
				twoQ = append(twoQ, idx)
			}
		}
		if len(twoQ) > 0 {
			layers = append(layers, twoQ)
		}

		if len(nextLayer) == 0 {
			break
		}
		for _, idx := range nextLayer {
			visited[idx] = true
		}
		currentLayer = nextLayer
	}

	return layers
}

// allExecuted returns true if all ops have been executed.
func (d *dag) allExecuted() bool {
	for _, e := range d.executed {
		if !e {
			return false
		}
	}
	return true
}
