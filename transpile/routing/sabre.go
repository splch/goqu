package routing

import (
	"math"
	"math/rand/v2"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

// sabrePass runs one direction of the SABRE routing algorithm with decay and
// extended-set lookahead. Reference: Li et al., arXiv:1809.02573.
// Returns routed operations, final layout, and SWAP count.
//
// The SABRE heuristic works as follows:
//  1. Maintain a "front layer" of 2-qubit gates whose data dependencies are
//     fully satisfied (all predecessor operations on those qubits have been
//     executed).
//  2. Execute any front-layer gate whose operands are already mapped to
//     adjacent physical qubits.
//  3. For each non-executable gate (operands not adjacent), evaluate all
//     candidate SWAP insertions on hardware edges adjacent to front-layer
//     qubits.
//  4. Score each candidate SWAP by the sum of post-SWAP distances for
//     front-layer gates, plus a geometrically weighted sum of distances
//     for "extended set" (lookahead) gates from deeper DAG layers.
//  5. A decay factor penalizes repeatedly swapping the same physical qubits,
//     discouraging cycles where qubits are swapped back and forth.
//  6. The lowest-scoring SWAP is inserted and the logical-to-physical layout
//     is updated accordingly.
//  7. A "release valve" breaks deadlocks: if no gate has been routed after
//     many consecutive SWAPs, the closest front-layer gate is force-routed
//     via shortest-path SWAPs.
func sabrePass(d *dag, dist [][]int, adj map[int][]int,
	initialLayout []int, opts Options, rng *rand.Rand) ([]ir.Operation, []int, int) {

	n := d.nQubits
	layout := make([]int, n)
	copy(layout, initialLayout)
	inv := InverseLayout(layout)

	numPhys := len(dist)
	decay := make([]float64, numPhys)
	for i := range decay {
		decay[i] = 1.0
	}

	releaseThreshold := opts.ReleaseValveThreshold
	if releaseThreshold == 0 {
		releaseThreshold = 10 * n
	}

	var result []ir.Operation
	swapCount := 0
	swapsSinceLastRoute := 0

	for {
		front := d.frontLayer()
		if len(front) == 0 {
			break
		}

		// Execute all directly routable ops.
		progress := true
		for progress {
			progress = false
			front = d.frontLayer()
			if len(front) == 0 {
				break
			}
			for _, idx := range front {
				op := d.ops[idx]
				if op.Gate == nil || op.Gate.Qubits() <= 1 {
					result = append(result, remapOp(op, layout))
					d.markExecuted(idx)
					progress = true
					continue
				}
				q0, q1 := op.Qubits[0], op.Qubits[1]
				p0, p1 := layout[q0], layout[q1]
				if p0 >= 0 && p0 < numPhys && p1 >= 0 && p1 < numPhys && dist[p0][p1] == 1 {
					result = append(result, remapOp(op, layout))
					d.markExecuted(idx)
					// Reset decay for the physical qubits used.
					decay[p0] = 1.0
					decay[p1] = 1.0
					swapsSinceLastRoute = 0
					progress = true
				}
			}
		}

		front = d.frontLayer()
		if len(front) == 0 {
			break
		}

		// Release valve: if stuck too long, force-route the closest gate.
		if releaseThreshold > 0 && swapsSinceLastRoute >= releaseThreshold {
			forced := releaseValveRoute(d, front, dist, adj, layout, inv, &result)
			swapCount += forced
			swapsSinceLastRoute = 0
			// Reset all decay after release valve.
			for i := range decay {
				decay[i] = 1.0
			}
			continue
		}

		// Compute extended set for lookahead.
		extSets := d.extendedSet(front, opts.ExtendedSetDepth)

		// Pre-compute geometric weights for extended set layers.
		extWeights := make([]float64, len(extSets))
		w := opts.ExtendedSetWeight
		for i := range extSets {
			extWeights[i] = w
			w *= opts.ExtendedSetWeight
		}

		// Collect candidate SWAPs: only edges adjacent to front-layer 2Q qubits.
		candidates := frontCandidates(front, d.ops, layout, adj, numPhys)

		// Evaluate candidate SWAPs.
		bestSwap := [2]int{-1, -1}
		bestCost := math.Inf(1)
		numTied := 0

		for _, cand := range candidates {
			phys0, phys1 := cand[0], cand[1]

			log0, log1 := inv[phys0], inv[phys1]
			layout[log0], layout[log1] = layout[log1], layout[log0]

			cost := decay[phys0] * decay[phys1] * layerCost(front, d.ops, layout, dist, numPhys)

			for i, extLayer := range extSets {
				cost += extWeights[i] * layerCost(extLayer, d.ops, layout, dist, numPhys)
			}

			layout[log0], layout[log1] = layout[log1], layout[log0]

			if cost < bestCost {
				bestCost = cost
				bestSwap = [2]int{phys0, phys1}
				numTied = 1
			} else if cost == bestCost {
				// Reservoir sampling for ties: pick uniformly at random.
				numTied++
				if rng.IntN(numTied) == 0 {
					bestSwap = [2]int{phys0, phys1}
				}
			}
		}

		if bestSwap[0] < 0 {
			break
		}

		// Insert SWAP and update layout.
		result = append(result, ir.Operation{
			Gate:   gate.SWAP,
			Qubits: []int{bestSwap[0], bestSwap[1]},
		})
		swapCount++
		swapsSinceLastRoute++

		log0, log1 := inv[bestSwap[0]], inv[bestSwap[1]]
		layout[log0], layout[log1] = layout[log1], layout[log0]
		inv[bestSwap[0]], inv[bestSwap[1]] = inv[bestSwap[1]], inv[bestSwap[0]]

		// Update decay.
		decay[bestSwap[0]] += opts.DecayDelta
		decay[bestSwap[1]] += opts.DecayDelta
	}

	return result, layout, swapCount
}

// frontCandidates returns deduplicated SWAP candidate edges adjacent to physical
// qubits involved in front-layer 2-qubit gates. This dramatically prunes the
// search space on large topologies.
func frontCandidates(front []int, ops []ir.Operation, layout []int, adj map[int][]int, numPhys int) [][2]int {
	// Collect physical qubits involved in front-layer 2Q gates.
	physSet := make(map[int]bool)
	for _, idx := range front {
		op := ops[idx]
		if op.Gate == nil || op.Gate.Qubits() < 2 {
			continue
		}
		for _, q := range op.Qubits {
			if q >= 0 && q < len(layout) {
				p := layout[q]
				if p >= 0 && p < numPhys {
					physSet[p] = true
				}
			}
		}
	}

	// Collect all edges where at least one endpoint is in physSet.
	type edge struct{ a, b int }
	seen := make(map[edge]bool)
	var candidates [][2]int
	for p := range physSet {
		for _, nb := range adj[p] {
			lo, hi := p, nb
			if lo > hi {
				lo, hi = hi, lo
			}
			e := edge{lo, hi}
			if !seen[e] {
				seen[e] = true
				candidates = append(candidates, [2]int{lo, hi})
			}
		}
	}
	return candidates
}

// layerCost sums distances for 2-qubit ops in the given index list.
// Unreachable pairs (dist == -1) contribute a large penalty to steer the
// algorithm away from disconnected configurations.
func layerCost(indices []int, ops []ir.Operation, layout []int, dist [][]int, numPhys int) float64 {
	cost := 0.0
	for _, idx := range indices {
		op := ops[idx]
		if op.Gate == nil || op.Gate.Qubits() < 2 {
			continue
		}
		q0, q1 := op.Qubits[0], op.Qubits[1]
		p0, p1 := layout[q0], layout[q1]
		if p0 >= 0 && p0 < numPhys && p1 >= 0 && p1 < numPhys {
			d := dist[p0][p1]
			if d >= 0 {
				cost += float64(d)
			} else {
				cost += float64(numPhys) // large penalty for unreachable
			}
		}
	}
	return cost
}

// releaseValveRoute is SABRE's deadlock-breaking mechanism. It activates when
// the algorithm has inserted many consecutive SWAPs without successfully
// routing any gate (exceeding ReleaseValveThreshold). It selects the
// front-layer 2-qubit gate whose operands are closest in the hardware
// distance matrix and greedily inserts SWAPs along the shortest path to
// bring them adjacent, then immediately executes the gate.
// Returns the number of SWAPs inserted.
func releaseValveRoute(d *dag, front []int, dist [][]int, adj map[int][]int,
	layout, inv []int, result *[]ir.Operation) int {

	numPhys := len(dist)
	// Find the front-layer 2Q gate with minimum distance.
	bestIdx := -1
	bestDist := math.MaxInt
	for _, idx := range front {
		op := d.ops[idx]
		if op.Gate == nil || op.Gate.Qubits() < 2 {
			continue
		}
		q0, q1 := op.Qubits[0], op.Qubits[1]
		p0, p1 := layout[q0], layout[q1]
		if p0 >= 0 && p0 < numPhys && p1 >= 0 && p1 < numPhys {
			dd := dist[p0][p1]
			if dd >= 0 && dd < bestDist {
				bestDist = dd
				bestIdx = idx
			}
		}
	}
	if bestIdx < 0 {
		return 0
	}

	swaps := 0
	op := d.ops[bestIdx]
	q0, q1 := op.Qubits[0], op.Qubits[1]

	// Greedily SWAP along shortest path to bring q0 and q1 adjacent.
	for {
		p0, p1 := layout[q0], layout[q1]
		if p0 >= 0 && p0 < numPhys && p1 >= 0 && p1 < numPhys && dist[p0][p1] <= 1 {
			break
		}
		// Move p0 toward p1 along shortest path.
		nextPhys := -1
		bestNext := dist[p0][p1]
		for _, nb := range adj[p0] {
			if dist[nb][p1] < bestNext {
				bestNext = dist[nb][p1]
				nextPhys = nb
			}
		}
		if nextPhys < 0 {
			break // shouldn't happen on connected graph
		}

		*result = append(*result, ir.Operation{
			Gate:   gate.SWAP,
			Qubits: []int{p0, nextPhys},
		})
		swaps++

		log0, log1 := inv[p0], inv[nextPhys]
		layout[log0], layout[log1] = layout[log1], layout[log0]
		inv[p0], inv[nextPhys] = inv[nextPhys], inv[p0]
	}

	// Now route the gate.
	*result = append(*result, remapOp(d.ops[bestIdx], layout))
	d.markExecuted(bestIdx)

	return swaps
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
