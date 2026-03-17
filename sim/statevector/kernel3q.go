package statevector

import (
	"sync"

	"github.com/splch/goqu/circuit/gate"
)

// dispatchGate3 selects an optimized kernel for the given 3-qubit gate.
func (s *Sim) dispatchGate3(g gate.Gate, q0, q1, q2 int) {
	parallel := s.numQubits >= 17

	switch g {
	case gate.CCX:
		if parallel {
			s.kernel3qCCXParallel(q0, q1, q2)
		} else {
			s.kernel3qCCX(q0, q1, q2)
		}
		return
	case gate.CSWAP:
		if parallel {
			s.kernel3qCSWAPParallel(q0, q1, q2)
		} else {
			s.kernel3qCSWAP(q0, q1, q2)
		}
		return
	case gate.CCZ:
		if parallel {
			s.kernel3qCCZParallel(q0, q1, q2)
		} else {
			s.kernel3qCCZ(q0, q1, q2)
		}
		return
	}

	// Generic fallback.
	m := g.Matrix()
	if parallel {
		s.kernel3qGenericParallel(q0, q1, q2, m)
	} else {
		s.kernel3qGeneric(q0, q1, q2, m)
	}
}

// sortThree returns three ints in ascending order.
func sortThree(a, b, c int) (lo, mid, hi int) {
	lo, mid, hi = a, b, c
	if lo > mid {
		lo, mid = mid, lo
	}
	if mid > hi {
		mid, hi = hi, mid
	}
	if lo > mid {
		lo, mid = mid, lo
	}
	return
}

// blockStride3 computes loop parameters for block-stride 3Q iteration.
func blockStride3(q0, q1, q2 int) (mask0, mask1, mask2, lo, mid, hi int) {
	mask0 = 1 << q0
	mask1 = 1 << q1
	mask2 = 1 << q2
	lo, mid, hi = sortThree(q0, q1, q2)
	return
}

// --- Serial kernels ---

// kernel3qCCX: CCX swaps |110> and |111> (both controls set).
func (s *Sim) kernel3qCCX(q0, q1, q2 int) {
	mask0, mask1, mask2, lo, mid, hi := blockStride3(q0, q1, q2)
	n := len(s.state)
	loMask := 1 << lo
	midMask := 1 << mid
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for mid0 := hi0; mid0 < hi0+hiMask; mid0 += midMask << 1 {
			for lo0 := mid0; lo0 < mid0+midMask; lo0 += loMask << 1 {
				for offset := lo0; offset < lo0+loMask; offset++ {
					i110 := offset | mask0 | mask1
					i111 := offset | mask0 | mask1 | mask2
					s.state[i110], s.state[i111] = s.state[i111], s.state[i110]
				}
			}
		}
	}
}

// kernel3qCSWAP: CSWAP swaps |101> and |110> (control q0 set).
func (s *Sim) kernel3qCSWAP(q0, q1, q2 int) {
	mask0, mask1, mask2, lo, mid, hi := blockStride3(q0, q1, q2)
	n := len(s.state)
	loMask := 1 << lo
	midMask := 1 << mid
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for mid0 := hi0; mid0 < hi0+hiMask; mid0 += midMask << 1 {
			for lo0 := mid0; lo0 < mid0+midMask; lo0 += loMask << 1 {
				for offset := lo0; offset < lo0+loMask; offset++ {
					i101 := offset | mask0 | mask2
					i110 := offset | mask0 | mask1
					s.state[i101], s.state[i110] = s.state[i110], s.state[i101]
				}
			}
		}
	}
}

// kernel3qCCZ: CCZ negates |111> amplitude (all controls set).
func (s *Sim) kernel3qCCZ(q0, q1, q2 int) {
	mask0, mask1, mask2, lo, mid, hi := blockStride3(q0, q1, q2)
	n := len(s.state)
	loMask := 1 << lo
	midMask := 1 << mid
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for mid0 := hi0; mid0 < hi0+hiMask; mid0 += midMask << 1 {
			for lo0 := mid0; lo0 < mid0+midMask; lo0 += loMask << 1 {
				for offset := lo0; offset < lo0+loMask; offset++ {
					i111 := offset | mask0 | mask1 | mask2
					s.state[i111] = -s.state[i111]
				}
			}
		}
	}
}

// kernel3qGeneric handles arbitrary 3Q gates with full 8×8 matmul.
func (s *Sim) kernel3qGeneric(q0, q1, q2 int, m []complex128) {
	mask0, mask1, mask2, lo, mid, hi := blockStride3(q0, q1, q2)
	n := len(s.state)
	loMask := 1 << lo
	midMask := 1 << mid
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for mid0 := hi0; mid0 < hi0+hiMask; mid0 += midMask << 1 {
			for lo0 := mid0; lo0 < mid0+midMask; lo0 += loMask << 1 {
				for offset := lo0; offset < lo0+loMask; offset++ {
					var indices [8]int
					for r := range 8 {
						idx := offset
						if r&4 != 0 {
							idx |= mask0
						}
						if r&2 != 0 {
							idx |= mask1
						}
						if r&1 != 0 {
							idx |= mask2
						}
						indices[r] = idx
					}
					var a [8]complex128
					for j := range 8 {
						a[j] = s.state[indices[j]]
					}
					for r := range 8 {
						var sum complex128
						row := r * 8
						for c := range 8 {
							sum += m[row+c] * a[c]
						}
						s.state[indices[r]] = sum
					}
				}
			}
		}
	}
}

// --- Parallel kernels ---

// kernel3qFunc is the per-offset body of a 3-qubit kernel. It receives
// the statevector, the base offset (all three qubit bits = 0), and the
// three qubit bitmasks.
type kernel3qFunc func(state []complex128, offset, mask0, mask1, mask2 int)

// runParallel3q distributes the 3-qubit block-stride iteration across
// goroutines, analogous to [runParallel2q] for 2-qubit gates.
func (s *Sim) runParallel3q(q0, q1, q2 int, body kernel3qFunc) {
	mask0, mask1, mask2, lo, mid, hi := blockStride3(q0, q1, q2)
	loMask := 1 << lo
	midMask := 1 << mid
	hiMask := 1 << hi
	hiStep := hiMask << 1
	n := len(s.state)
	nBlocks := n / hiStep
	nWorkers := optimalWorkers(s.numQubits)
	if nBlocks < nWorkers {
		nWorkers = nBlocks
	}
	if nWorkers < 1 {
		nWorkers = 1
	}
	blocksPerWorker := nBlocks / nWorkers

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	for w := range nWorkers {
		startBlock := w * blocksPerWorker
		endBlock := startBlock + blocksPerWorker
		if w == nWorkers-1 {
			endBlock = nBlocks
		}
		go func(sb, eb int) {
			defer wg.Done()
			for b := sb; b < eb; b++ {
				hi0 := b * hiStep
				for mid0 := hi0; mid0 < hi0+hiMask; mid0 += midMask << 1 {
					for lo0 := mid0; lo0 < mid0+midMask; lo0 += loMask << 1 {
						for offset := lo0; offset < lo0+loMask; offset++ {
							body(s.state, offset, mask0, mask1, mask2)
						}
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

func (s *Sim) kernel3qCCXParallel(q0, q1, q2 int) {
	s.runParallel3q(q0, q1, q2, func(state []complex128, offset, mask0, mask1, mask2 int) {
		i110 := offset | mask0 | mask1
		i111 := offset | mask0 | mask1 | mask2
		state[i110], state[i111] = state[i111], state[i110]
	})
}

func (s *Sim) kernel3qCSWAPParallel(q0, q1, q2 int) {
	s.runParallel3q(q0, q1, q2, func(state []complex128, offset, mask0, mask1, mask2 int) {
		i101 := offset | mask0 | mask2
		i110 := offset | mask0 | mask1
		state[i101], state[i110] = state[i110], state[i101]
	})
}

func (s *Sim) kernel3qCCZParallel(q0, q1, q2 int) {
	s.runParallel3q(q0, q1, q2, func(state []complex128, offset, mask0, mask1, mask2 int) {
		i111 := offset | mask0 | mask1 | mask2
		state[i111] = -state[i111]
	})
}

func (s *Sim) kernel3qGenericParallel(q0, q1, q2 int, m []complex128) {
	s.runParallel3q(q0, q1, q2, func(state []complex128, offset, mask0, mask1, mask2 int) {
		var indices [8]int
		for r := range 8 {
			idx := offset
			if r&4 != 0 {
				idx |= mask0
			}
			if r&2 != 0 {
				idx |= mask1
			}
			if r&1 != 0 {
				idx |= mask2
			}
			indices[r] = idx
		}
		var a [8]complex128
		for j := range 8 {
			a[j] = state[indices[j]]
		}
		for r := range 8 {
			var sum complex128
			row := r * 8
			for c := range 8 {
				sum += m[row+c] * a[c]
			}
			state[indices[r]] = sum
		}
	})
}
