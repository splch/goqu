package statevector

import (
	"sync"

	"github.com/splch/goqu/circuit/gate"
)

// dispatchGate2 selects an optimized kernel for the given 2-qubit gate
// using a three-tier dispatch hierarchy:
//
//  1. Pointer equality: fixed singleton gate objects (CNOT, CZ, SWAP, CY,
//     ISWAP, Sycamore) are compared by pointer to route to hand-written
//     kernels that avoid loading the full 4x4 matrix. For example, CNOT
//     is just a swap of two amplitudes, and CZ is a sign flip on one.
//  2. Interface-based dispatch: parameterized gates implement optional
//     interfaces (ControlDiagonal2Q, Diagonal2Q, ControlU2Q) to signal
//     their matrix structure, enabling optimized kernels without string parsing.
//  3. Generic fallback: an arbitrary 4x4 matrix-vector multiply on all four
//     basis-state amplitudes per block.
//
// Each tier selects between serial and parallel implementations based on
// the 17-qubit threshold.
func (s *Sim) dispatchGate2(g gate.Gate, q0, q1 int) {
	parallel := s.numQubits >= 17

	// Pointer equality for fixed singletons.
	switch g {
	case gate.CNOT:
		if parallel {
			s.kernel2qCNOTParallel(q0, q1)
		} else {
			s.kernel2qCNOT(q0, q1)
		}
		return
	case gate.CZ:
		if parallel {
			s.kernel2qCZParallel(q0, q1)
		} else {
			s.kernel2qCZ(q0, q1)
		}
		return
	case gate.SWAP:
		if parallel {
			s.kernel2qSWAPParallel(q0, q1)
		} else {
			s.kernel2qSWAP(q0, q1)
		}
		return
	case gate.CY:
		if parallel {
			s.kernel2qCYParallel(q0, q1)
		} else {
			s.kernel2qCY(q0, q1)
		}
		return
	case gate.ISWAP:
		if parallel {
			s.kernel2qISWAPParallel(q0, q1)
		} else {
			s.kernel2qISWAP(q0, q1)
		}
		return
	case gate.Sycamore:
		if parallel {
			s.kernel2qSycamoreParallel(q0, q1)
		} else {
			s.kernel2qSycamore(q0, q1)
		}
		return
	}

	// Interface-based dispatch for parameterized gates. Gates implement
	// optional interfaces (ControlDiagonal2Q, Diagonal2Q, ControlU2Q) to
	// signal their structure, enabling optimized kernels without string parsing.
	if cd, ok := g.(gate.ControlDiagonal2Q); ok {
		d10, d11 := cd.ControlDiagonal()
		if parallel {
			s.kernel2qDiagonalParallel(q0, q1, d10, d11)
		} else {
			s.kernel2qDiagonal(q0, q1, d10, d11)
		}
		return
	}
	if dg, ok := g.(gate.Diagonal2Q); ok {
		d0, d1, d2, d3 := dg.Diagonal()
		if parallel {
			s.kernel2qFullDiagonalParallel(q0, q1, d0, d1, d2, d3)
		} else {
			s.kernel2qFullDiagonal(q0, q1, d0, d1, d2, d3)
		}
		return
	}
	if cu, ok := g.(gate.ControlU2Q); ok {
		u00, u01, u10, u11 := cu.ControlSubmatrix()
		if parallel {
			s.kernel2qControlledParallel(q0, q1, u00, u01, u10, u11)
		} else {
			s.kernel2qControlled(q0, q1, u00, u01, u10, u11)
		}
		return
	}

	// Generic fallback.
	m := g.Matrix()
	if parallel {
		s.kernel2qGenericParallel(q0, q1, m)
	} else {
		s.kernel2qGeneric(q0, q1, m)
	}
}

// blockStride2 computes loop parameters for block-stride 2-qubit iteration.
//
// For a 2-qubit gate on qubits q0 and q1, we need to iterate all basis
// states where both qubit bits are 0 (the canonical representative), then
// address the four combinations |00>, |01>, |10>, |11> via OR with mask0
// and mask1. The two bit positions are sorted into lo < hi so the nested
// loop structure works correctly:
//
//	Outer loop:  stride over blocks of size 2^(hi+1), stepping by hiMask<<1.
//	Middle loop: within each outer block, stride over sub-blocks of size
//	             2^(lo+1), stepping by loMask<<1.
//	Inner loop:  process consecutive indices where BOTH qubit bits are 0,
//	             running from the sub-block start for loMask iterations.
//
// This 3-level nesting ensures every basis state is visited exactly once,
// and the four states in each group are accessed by ORing the base index
// with mask0, mask1, or both.
//
// Returns mask0, mask1 (original qubit bitmasks) and lo, hi (sorted positions).
func blockStride2(q0, q1 int) (mask0, mask1, lo, hi int) {
	mask0 = 1 << q0
	mask1 = 1 << q1
	lo, hi = q0, q1
	if lo > hi {
		lo, hi = hi, lo
	}
	return
}

// --- Serial kernels ---
//
// Each serial kernel implements the 3-level block-stride loop (see
// blockStride2) with a gate-specific inner body. The inner body is the
// physics: it reads/writes the 2-4 amplitudes involved in the gate.

// kernel2qCNOT: swap amplitudes at |10> and |11> (bit-flip of target when control=1).
func (s *Sim) kernel2qCNOT(q0, q1 int) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i10 := offset | mask0
				i11 := offset | mask0 | mask1
				s.state[i10], s.state[i11] = s.state[i11], s.state[i10]
			}
		}
	}
}

// kernel2qCZ: negate the |11> amplitude (phase-flip when both qubits are |1>).
func (s *Sim) kernel2qCZ(q0, q1 int) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i11 := offset | mask0 | mask1
				s.state[i11] = -s.state[i11]
			}
		}
	}
}

// kernel2qSWAP: exchange amplitudes at |01> and |10>.
func (s *Sim) kernel2qSWAP(q0, q1 int) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i01 := offset | mask1
				i10 := offset | mask0
				s.state[i01], s.state[i10] = s.state[i10], s.state[i01]
			}
		}
	}
}

// kernel2qCY: controlled-Y on the |10>,|11> pair: |10> -> i|11>, |11> -> -i|10>.
func (s *Sim) kernel2qCY(q0, q1 int) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i10 := offset | mask0
				i11 := offset | mask0 | mask1
				a2, a3 := s.state[i10], s.state[i11]
				s.state[i10] = -1i * a3
				s.state[i11] = 1i * a2
			}
		}
	}
}

// kernel2qDiagonal handles gates with matrix diag(1,1,d2,d3): CP, CRZ.
// Only the |10> and |11> amplitudes are multiplied by phase factors.
func (s *Sim) kernel2qDiagonal(q0, q1 int, d2, d3 complex128) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i10 := offset | mask0
				i11 := offset | mask0 | mask1
				s.state[i10] *= d2
				s.state[i11] *= d3
			}
		}
	}
}

// kernel2qFullDiagonal handles fully diagonal 2Q gates (e.g., RZZ) where each
// of the 4 basis states gets a phase factor. ~4x faster than generic.
func (s *Sim) kernel2qFullDiagonal(q0, q1 int, d0, d1, d2, d3 complex128) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				s.state[offset] *= d0
				s.state[offset|mask1] *= d1
				s.state[offset|mask0] *= d2
				s.state[offset|mask0|mask1] *= d3
			}
		}
	}
}

// kernel2qControlled handles controlled-U gates (CRX, CRY) where only the
// |10>,|11> subspace is non-trivial: a 2x2 matmul on those amplitudes.
func (s *Sim) kernel2qControlled(q0, q1 int, u00, u01, u10, u11 complex128) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i10 := offset | mask0
				i11 := offset | mask0 | mask1
				a2, a3 := s.state[i10], s.state[i11]
				s.state[i10] = u00*a2 + u01*a3
				s.state[i11] = u10*a2 + u11*a3
			}
		}
	}
}

// kernel2qGeneric handles arbitrary 2-qubit gates with a full 4x4
// matrix-vector multiply. For each block (set of indices where both qubit
// bits are 0), it gathers the four amplitudes at |00>, |01>, |10>, |11>,
// performs U * [a00, a01, a10, a11]^T using the row-major 4x4 matrix m,
// and scatters the results back. This is the slowest 2Q kernel (16 complex
// multiply-adds per block) but handles any unitary.
func (s *Sim) kernel2qGeneric(q0, q1 int, m []complex128) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i00 := offset
				i01 := offset | mask1
				i10 := offset | mask0
				i11 := offset | mask0 | mask1
				a0, a1, a2, a3 := s.state[i00], s.state[i01], s.state[i10], s.state[i11]
				s.state[i00] = m[0]*a0 + m[1]*a1 + m[2]*a2 + m[3]*a3
				s.state[i01] = m[4]*a0 + m[5]*a1 + m[6]*a2 + m[7]*a3
				s.state[i10] = m[8]*a0 + m[9]*a1 + m[10]*a2 + m[11]*a3
				s.state[i11] = m[12]*a0 + m[13]*a1 + m[14]*a2 + m[15]*a3
			}
		}
	}
}

// kernel2qISWAP: iSWAP swaps |01>↔|10> with factor i.
func (s *Sim) kernel2qISWAP(q0, q1 int) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i01 := offset | mask1
				i10 := offset | mask0
				a1, a2 := s.state[i01], s.state[i10]
				s.state[i01] = 1i * a2
				s.state[i10] = 1i * a1
			}
		}
	}
}

// kernel2qSycamore: FSim(pi/2, pi/6) — iSWAP on |01>↔|10> plus phase on |11>.
func (s *Sim) kernel2qSycamore(q0, q1 int) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	n := len(s.state)
	loMask := 1 << lo
	hiMask := 1 << hi
	sm := gate.Sycamore.Matrix()
	d11 := sm[15]
	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i01 := offset | mask1
				i10 := offset | mask0
				i11 := offset | mask0 | mask1
				a1, a2 := s.state[i01], s.state[i10]
				s.state[i01] = -1i * a2
				s.state[i10] = -1i * a1
				s.state[i11] *= d11
			}
		}
	}
}

// --- Parallel kernels ---
//
// The parallel kernels share a common structure: split the outer-loop blocks
// across goroutines, with each goroutine running the same 3-level loop on
// its assigned range. The runParallel2q helper encapsulates this scaffolding
// so each kernel only provides its gate-specific inner body.

// kernel2qFunc is the per-offset body of a 2-qubit kernel. It receives
// the statevector, the base offset (both qubit bits = 0), and the two
// qubit bitmasks.
type kernel2qFunc func(state []complex128, offset, mask0, mask1 int)

// runParallel2q distributes the 2-qubit block-stride iteration across
// goroutines. Each worker processes a contiguous range of outer-loop blocks,
// calling body for every offset within its blocks. This eliminates the
// ~25 lines of boilerplate that was previously duplicated in every parallel
// kernel.
func (s *Sim) runParallel2q(q0, q1 int, body kernel2qFunc) {
	mask0, mask1, lo, hi := blockStride2(q0, q1)
	loMask := 1 << lo
	hiMask := 1 << hi
	hiStep := hiMask << 1
	nBlocks, nWorkers := s.parallelBlocks2(hiMask)
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
				for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
					for offset := lo0; offset < lo0+loMask; offset++ {
						body(s.state, offset, mask0, mask1)
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

// parallelBlocks2 computes worker distribution for 2Q parallel kernels.
// The outer loop count is n / (hiMask<<1), so we split that among workers.
func (s *Sim) parallelBlocks2(hiMask int) (nBlocks, nWorkers int) {
	n := len(s.state)
	nBlocks = n / (hiMask << 1)
	nWorkers = optimalWorkers(s.numQubits)
	if nBlocks < nWorkers {
		nWorkers = nBlocks
	}
	if nWorkers < 1 {
		nWorkers = 1
	}
	return
}

func (s *Sim) kernel2qCNOTParallel(q0, q1 int) {
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		i10 := offset | mask0
		i11 := offset | mask0 | mask1
		state[i10], state[i11] = state[i11], state[i10]
	})
}

func (s *Sim) kernel2qCZParallel(q0, q1 int) {
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		i11 := offset | mask0 | mask1
		state[i11] = -state[i11]
	})
}

func (s *Sim) kernel2qSWAPParallel(q0, q1 int) {
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		i01 := offset | mask1
		i10 := offset | mask0
		state[i01], state[i10] = state[i10], state[i01]
	})
}

func (s *Sim) kernel2qCYParallel(q0, q1 int) {
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		i10 := offset | mask0
		i11 := offset | mask0 | mask1
		a2, a3 := state[i10], state[i11]
		state[i10] = -1i * a3
		state[i11] = 1i * a2
	})
}

func (s *Sim) kernel2qDiagonalParallel(q0, q1 int, d2, d3 complex128) {
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		i10 := offset | mask0
		i11 := offset | mask0 | mask1
		state[i10] *= d2
		state[i11] *= d3
	})
}

func (s *Sim) kernel2qFullDiagonalParallel(q0, q1 int, d0, d1, d2, d3 complex128) {
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		state[offset] *= d0
		state[offset|mask1] *= d1
		state[offset|mask0] *= d2
		state[offset|mask0|mask1] *= d3
	})
}

func (s *Sim) kernel2qControlledParallel(q0, q1 int, u00, u01, u10, u11 complex128) {
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		i10 := offset | mask0
		i11 := offset | mask0 | mask1
		a2, a3 := state[i10], state[i11]
		state[i10] = u00*a2 + u01*a3
		state[i11] = u10*a2 + u11*a3
	})
}

func (s *Sim) kernel2qGenericParallel(q0, q1 int, m []complex128) {
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		i00 := offset
		i01 := offset | mask1
		i10 := offset | mask0
		i11 := offset | mask0 | mask1
		a0, a1, a2, a3 := state[i00], state[i01], state[i10], state[i11]
		state[i00] = m[0]*a0 + m[1]*a1 + m[2]*a2 + m[3]*a3
		state[i01] = m[4]*a0 + m[5]*a1 + m[6]*a2 + m[7]*a3
		state[i10] = m[8]*a0 + m[9]*a1 + m[10]*a2 + m[11]*a3
		state[i11] = m[12]*a0 + m[13]*a1 + m[14]*a2 + m[15]*a3
	})
}

func (s *Sim) kernel2qISWAPParallel(q0, q1 int) {
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		i01 := offset | mask1
		i10 := offset | mask0
		a1, a2 := state[i01], state[i10]
		state[i01] = 1i * a2
		state[i10] = 1i * a1
	})
}

func (s *Sim) kernel2qSycamoreParallel(q0, q1 int) {
	sm := gate.Sycamore.Matrix()
	d11 := sm[15]
	s.runParallel2q(q0, q1, func(state []complex128, offset, mask0, mask1 int) {
		i01 := offset | mask1
		i10 := offset | mask0
		i11 := offset | mask0 | mask1
		a1, a2 := state[i01], state[i10]
		state[i01] = -1i * a2
		state[i10] = -1i * a1
		state[i11] *= d11
	})
}
