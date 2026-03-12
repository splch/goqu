package statevector

import (
	"strings"
	"sync"

	"github.com/splch/qgo/circuit/gate"
)

// dispatchGate2 selects an optimized kernel for the given 2-qubit gate.
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
	}

	// Name-based dispatch for parameterized gates.
	name := g.Name()
	if strings.HasPrefix(name, "CP(") || strings.HasPrefix(name, "CRZ(") {
		m := g.Matrix()
		if parallel {
			s.kernel2qDiagonalParallel(q0, q1, m[10], m[15])
		} else {
			s.kernel2qDiagonal(q0, q1, m[10], m[15])
		}
		return
	}
	if strings.HasPrefix(name, "RZZ(") {
		m := g.Matrix()
		// Fully diagonal: apply phase factors to all 4 basis states.
		d0, d1, d2, d3 := m[0], m[5], m[10], m[15]
		if parallel {
			s.kernel2qFullDiagonalParallel(q0, q1, d0, d1, d2, d3)
		} else {
			s.kernel2qFullDiagonal(q0, q1, d0, d1, d2, d3)
		}
		return
	}
	if strings.HasPrefix(name, "CRX(") || strings.HasPrefix(name, "CRY(") {
		m := g.Matrix()
		if parallel {
			s.kernel2qControlledParallel(q0, q1, m[10], m[11], m[14], m[15])
		} else {
			s.kernel2qControlled(q0, q1, m[10], m[11], m[14], m[15])
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

// blockStride2 computes loop parameters for block-stride 2Q iteration.
// Returns mask0, mask1 (original qubit masks) and lo, hi (sorted bit positions).
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

func (s *Sim) kernel2qCZ(q0, q1 int) {
	_, mask1, lo, hi := blockStride2(q0, q1)
	_ = mask1
	mask0 := 1 << q0
	mask1 = 1 << q1
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
// |10>,|11> subspace is non-trivial: a 2×2 matmul on those amplitudes.
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

// kernel2qGeneric handles arbitrary 2Q gates with full 4×4 matmul.
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

// --- Parallel kernels ---

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
						i10 := offset | mask0
						i11 := offset | mask0 | mask1
						s.state[i10], s.state[i11] = s.state[i11], s.state[i10]
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

func (s *Sim) kernel2qCZParallel(q0, q1 int) {
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
						i11 := offset | mask0 | mask1
						s.state[i11] = -s.state[i11]
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

func (s *Sim) kernel2qSWAPParallel(q0, q1 int) {
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
						i01 := offset | mask1
						i10 := offset | mask0
						s.state[i01], s.state[i10] = s.state[i10], s.state[i01]
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

func (s *Sim) kernel2qCYParallel(q0, q1 int) {
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
						i10 := offset | mask0
						i11 := offset | mask0 | mask1
						a2, a3 := s.state[i10], s.state[i11]
						s.state[i10] = -1i * a3
						s.state[i11] = 1i * a2
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

func (s *Sim) kernel2qDiagonalParallel(q0, q1 int, d2, d3 complex128) {
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
						i10 := offset | mask0
						i11 := offset | mask0 | mask1
						s.state[i10] *= d2
						s.state[i11] *= d3
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

func (s *Sim) kernel2qFullDiagonalParallel(q0, q1 int, d0, d1, d2, d3 complex128) {
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
						s.state[offset] *= d0
						s.state[offset|mask1] *= d1
						s.state[offset|mask0] *= d2
						s.state[offset|mask0|mask1] *= d3
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

func (s *Sim) kernel2qControlledParallel(q0, q1 int, u00, u01, u10, u11 complex128) {
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
						i10 := offset | mask0
						i11 := offset | mask0 | mask1
						a2, a3 := s.state[i10], s.state[i11]
						s.state[i10] = u00*a2 + u01*a3
						s.state[i11] = u10*a2 + u11*a3
					}
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

func (s *Sim) kernel2qGenericParallel(q0, q1 int, m []complex128) {
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
		}(startBlock, endBlock)
	}
	wg.Wait()
}
