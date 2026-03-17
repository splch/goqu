package statevector

import (
	"sort"
	"sync"

	"github.com/splch/goqu/circuit/gate"
)

// dispatchControlled routes a ControlledGate to the appropriate kernel.
func (s *Sim) dispatchControlled(cg gate.ControlledGate, qubits []int) {
	nControls := cg.NumControls()
	controls := qubits[:nControls]
	targets := qubits[nControls:]

	switch cg.Inner().Qubits() {
	case 1:
		m := cg.Inner().Matrix()
		if s.numQubits >= 17 {
			s.applyControlledGate1Parallel(controls, targets[0], m)
		} else {
			s.applyControlledGate1(controls, targets[0], m)
		}
	case 2:
		m := cg.Inner().Matrix()
		if s.numQubits >= 17 {
			s.applyControlledGate2Parallel(controls, targets[0], targets[1], m)
		} else {
			s.applyControlledGate2(controls, targets[0], targets[1], m)
		}
	default:
		// Generic fallback: construct full matrix and use generic N-qubit kernel.
		s.applyControlledGateN(cg, qubits)
	}
}

// applyControlledGate1 applies a multi-controlled single-qubit gate using the
// control-mask pattern. A bitmask is built from all control qubit positions.
// The iteration scans every basis-state index and applies two filters:
//
//  1. Skip indices where any control bit is 0: (i & controlMask) != controlMask.
//     This ensures the inner gate only acts on the subspace where all controls
//     are |1>, which is the defining behavior of a controlled gate.
//  2. Among matching indices, only process the canonical pair representative
//     where the target bit is 0: (i & targetBit) == 0. The partner index
//     i | targetBit has the target bit = 1. Together they form the 2D subspace
//     on which the inner 2x2 unitary acts.
//
// This brute-force scan is simpler than a block-stride approach for arbitrary
// control configurations. It visits all 2^n indices but skips most via the
// bitmask check, which is a single AND + compare (branch-predicted well on
// modern CPUs).
func (s *Sim) applyControlledGate1(controls []int, target int, m []complex128) {
	var controlMask int
	for _, c := range controls {
		controlMask |= 1 << c
	}

	// Collect all qubit positions and sort for block-stride iteration.
	allQubits := make([]int, 0, len(controls)+1)
	allQubits = append(allQubits, controls...)
	allQubits = append(allQubits, target)
	sort.Ints(allQubits)

	targetBit := 1 << target
	n := len(s.state)

	// Iterate using the highest-bit block stride, skipping processed pairs.
	// Simple approach: iterate all basis states, process only canonical pairs.
	for i := 0; i < n; i++ {
		// Skip if any control bit is not set.
		if i&controlMask != controlMask {
			continue
		}
		// Only process if target bit is 0 (canonical pair representative).
		if i&targetBit != 0 {
			continue
		}
		i0 := i
		i1 := i | targetBit
		a0, a1 := s.state[i0], s.state[i1]
		s.state[i0] = m[0]*a0 + m[1]*a1
		s.state[i1] = m[2]*a0 + m[3]*a1
	}
}

// applyControlledGate1Parallel is the parallel version for 17+ qubits.
func (s *Sim) applyControlledGate1Parallel(controls []int, target int, m []complex128) {
	var controlMask int
	for _, c := range controls {
		controlMask |= 1 << c
	}
	targetBit := 1 << target
	n := len(s.state)

	nWorkers := optimalWorkers(s.numQubits)
	chunkSize := n / nWorkers
	if chunkSize < 1 {
		chunkSize = n
		nWorkers = 1
	}

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	for w := range nWorkers {
		start := w * chunkSize
		end := start + chunkSize
		if w == nWorkers-1 {
			end = n
		}
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				if i&controlMask != controlMask {
					continue
				}
				if i&targetBit != 0 {
					continue
				}
				i0 := i
				i1 := i | targetBit
				a0, a1 := s.state[i0], s.state[i1]
				s.state[i0] = m[0]*a0 + m[1]*a1
				s.state[i1] = m[2]*a0 + m[3]*a1
			}
		}(start, end)
	}
	wg.Wait()
}

// applyControlledGate2 applies a controlled 2-qubit gate.
func (s *Sim) applyControlledGate2(controls []int, t0, t1 int, m []complex128) {
	var controlMask int
	for _, c := range controls {
		controlMask |= 1 << c
	}
	mask0 := 1 << t0
	mask1 := 1 << t1
	targetMask := mask0 | mask1
	n := len(s.state)

	for i := 0; i < n; i++ {
		if i&controlMask != controlMask {
			continue
		}
		// Only process canonical representative (both target bits 0).
		if i&targetMask != 0 {
			continue
		}
		idx := [4]int{
			i,                 // 00
			i | mask1,         // 01
			i | mask0,         // 10
			i | mask0 | mask1, // 11
		}
		var a [4]complex128
		for j := range 4 {
			a[j] = s.state[idx[j]]
		}
		for r := range 4 {
			var sum complex128
			for c := range 4 {
				sum += m[r*4+c] * a[c]
			}
			s.state[idx[r]] = sum
		}
	}
}

// applyControlledGate2Parallel is the parallel version for 17+ qubits.
func (s *Sim) applyControlledGate2Parallel(controls []int, t0, t1 int, m []complex128) {
	var controlMask int
	for _, c := range controls {
		controlMask |= 1 << c
	}
	mask0 := 1 << t0
	mask1 := 1 << t1
	targetMask := mask0 | mask1
	n := len(s.state)

	nWorkers := optimalWorkers(s.numQubits)
	chunkSize := n / nWorkers
	if chunkSize < 1 {
		chunkSize = n
		nWorkers = 1
	}

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	for w := range nWorkers {
		start := w * chunkSize
		end := start + chunkSize
		if w == nWorkers-1 {
			end = n
		}
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				if i&controlMask != controlMask {
					continue
				}
				if i&targetMask != 0 {
					continue
				}
				idx := [4]int{
					i,
					i | mask1,
					i | mask0,
					i | mask0 | mask1,
				}
				var a [4]complex128
				for j := range 4 {
					a[j] = s.state[idx[j]]
				}
				for r := range 4 {
					var sum complex128
					for c := range 4 {
						sum += m[r*4+c] * a[c]
					}
					s.state[idx[r]] = sum
				}
			}
		}(start, end)
	}
	wg.Wait()
}

// applyControlledGateN is the generic fallback for controlled gates whose inner
// gate acts on more than 2 qubits. It works by constructing the full
// 2^totalQubits x 2^totalQubits unitary and performing explicit matrix-vector
// multiplication on each group of basis states.
//
// For each canonical base index (where all involved qubit bits are 0), it
// builds the full set of 2^totalQubits indices by OR-ing bitmasks for each
// qubit position. The qubit-to-index mapping uses big-endian bit order within
// the gate's local space (bit totalQubits-1-j maps to qubit j), matching the
// row-major convention of the gate matrix. The amplitudes at those indices are
// gathered, multiplied by the full matrix, and scattered back.
//
// This kernel panics for gates wider than 10 qubits (gate.Matrix limitation)
// and is O(2^n * 4^k) where k = totalQubits, so it is only used as a last
// resort.
func (s *Sim) applyControlledGateN(cg gate.ControlledGate, qubits []int) {
	totalQubits := len(qubits)
	dim := 1 << totalQubits
	m := cg.Matrix() // will panic if > 10 qubits total

	n := len(s.state)
	masks := make([]int, totalQubits)
	for i, q := range qubits {
		masks[i] = 1 << q
	}

	// Build all-bits-zero mask for canonical iteration.
	var allMask int
	for _, q := range qubits {
		allMask |= 1 << q
	}

	indices := make([]int, dim)
	a := make([]complex128, dim)
	for base := 0; base < n; base++ {
		if base&allMask != 0 {
			continue
		}
		for r := range dim {
			idx := base
			for bit := range totalQubits {
				if r&(1<<(totalQubits-1-bit)) != 0 {
					idx |= masks[bit]
				}
			}
			indices[r] = idx
		}
		for j := range dim {
			a[j] = s.state[indices[j]]
		}
		for r := range dim {
			var sum complex128
			for c := range dim {
				sum += m[r*dim+c] * a[c]
			}
			s.state[indices[r]] = sum
		}
	}
}
