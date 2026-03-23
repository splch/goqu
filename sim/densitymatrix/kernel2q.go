package densitymatrix

import "sync"

// applyGate2 applies a 2-qubit gate: ρ' = U·ρ·U†.
// Two-pass: multiply U on row indices, then U† on column indices.
func (s *Sim) applyGate2(q0, q1 int, m []complex128) {
	if s.numQubits >= parallelThreshold {
		s.applyGate2Parallel(q0, q1, m)
		return
	}
	s.gate2Rows(q0, q1, m)
	s.gate2Cols(q0, q1, m)
}

// gate2Rows applies the 4x4 unitary U on the row index.
// For each column c, treat rho[:][c] as a statevector and apply U using
// the 2-qubit stride pattern on the row index.
func (s *Sim) gate2Rows(q0, q1 int, m []complex128) {
	dim := s.dim
	mask0, mask1 := 1<<q0, 1<<q1
	lo, hi := q0, q1
	if lo > hi {
		lo, hi = hi, lo
	}
	loMask := 1 << lo
	hiMask := 1 << hi

	for hi0 := 0; hi0 < dim; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				r00 := offset
				r01 := offset | mask1
				r10 := offset | mask0
				r11 := offset | mask0 | mask1
				base00 := r00 * dim
				base01 := r01 * dim
				base10 := r10 * dim
				base11 := r11 * dim
				for c := range dim {
					a0 := s.rho[base00+c]
					a1 := s.rho[base01+c]
					a2 := s.rho[base10+c]
					a3 := s.rho[base11+c]
					s.rho[base00+c] = m[0]*a0 + m[1]*a1 + m[2]*a2 + m[3]*a3
					s.rho[base01+c] = m[4]*a0 + m[5]*a1 + m[6]*a2 + m[7]*a3
					s.rho[base10+c] = m[8]*a0 + m[9]*a1 + m[10]*a2 + m[11]*a3
					s.rho[base11+c] = m[12]*a0 + m[13]*a1 + m[14]*a2 + m[15]*a3
				}
			}
		}
	}
}

// gate2Cols applies U† on the column index.
func (s *Sim) gate2Cols(q0, q1 int, m []complex128) {
	dim := s.dim
	mask0, mask1 := 1<<q0, 1<<q1
	lo, hi := q0, q1
	if lo > hi {
		lo, hi = hi, lo
	}
	loMask := 1 << lo
	hiMask := 1 << hi

	// For right-multiply by U†: use conj(U) element-wise.
	// (M·U†)[r][c] = Σ_l M[r][l] · conj(U[c_q][l_q])
	var uc [16]complex128
	for i := range 16 {
		uc[i] = conj(m[i])
	}

	for r := range dim {
		base := r * dim
		for hi0 := 0; hi0 < dim; hi0 += hiMask << 1 {
			for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
				for offset := lo0; offset < lo0+loMask; offset++ {
					c00 := offset
					c01 := offset | mask1
					c10 := offset | mask0
					c11 := offset | mask0 | mask1
					a0 := s.rho[base+c00]
					a1 := s.rho[base+c01]
					a2 := s.rho[base+c10]
					a3 := s.rho[base+c11]
					s.rho[base+c00] = uc[0]*a0 + uc[1]*a1 + uc[2]*a2 + uc[3]*a3
					s.rho[base+c01] = uc[4]*a0 + uc[5]*a1 + uc[6]*a2 + uc[7]*a3
					s.rho[base+c10] = uc[8]*a0 + uc[9]*a1 + uc[10]*a2 + uc[11]*a3
					s.rho[base+c11] = uc[12]*a0 + uc[13]*a1 + uc[14]*a2 + uc[15]*a3
				}
			}
		}
	}
}

// Parallel versions

func (s *Sim) applyGate2Parallel(q0, q1 int, m []complex128) {
	s.gate2RowsParallel(q0, q1, m)
	s.gate2ColsParallel(q0, q1, m)
}

func (s *Sim) gate2RowsParallel(q0, q1 int, m []complex128) {
	dim := s.dim
	mask0, mask1 := 1<<q0, 1<<q1
	lo, hi := q0, q1
	if lo > hi {
		lo, hi = hi, lo
	}
	loMask := 1 << lo
	hiMask := 1 << hi
	hiStep := hiMask << 1

	nBlocks := dim / hiStep
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
		sb := w * blocksPerWorker
		eb := sb + blocksPerWorker
		if w == nWorkers-1 {
			eb = nBlocks
		}
		go func(sb, eb int) {
			defer wg.Done()
			for b := sb; b < eb; b++ {
				hi0 := b * hiStep
				for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
					for offset := lo0; offset < lo0+loMask; offset++ {
						r00 := offset
						r01 := offset | mask1
						r10 := offset | mask0
						r11 := offset | mask0 | mask1
						base00 := r00 * dim
						base01 := r01 * dim
						base10 := r10 * dim
						base11 := r11 * dim
						for c := range dim {
							a0 := s.rho[base00+c]
							a1 := s.rho[base01+c]
							a2 := s.rho[base10+c]
							a3 := s.rho[base11+c]
							s.rho[base00+c] = m[0]*a0 + m[1]*a1 + m[2]*a2 + m[3]*a3
							s.rho[base01+c] = m[4]*a0 + m[5]*a1 + m[6]*a2 + m[7]*a3
							s.rho[base10+c] = m[8]*a0 + m[9]*a1 + m[10]*a2 + m[11]*a3
							s.rho[base11+c] = m[12]*a0 + m[13]*a1 + m[14]*a2 + m[15]*a3
						}
					}
				}
			}
		}(sb, eb)
	}
	wg.Wait()
}

func (s *Sim) gate2ColsParallel(q0, q1 int, m []complex128) {
	dim := s.dim
	mask0, mask1 := 1<<q0, 1<<q1
	lo, hi := q0, q1
	if lo > hi {
		lo, hi = hi, lo
	}
	loMask := 1 << lo
	hiMask := 1 << hi

	// Element-wise conjugate (matching serial gate2Cols), NOT conjugate-transpose.
	var ud [16]complex128
	for i := range 16 {
		ud[i] = conj(m[i])
	}

	nWorkers := optimalWorkers(s.numQubits)
	if dim < nWorkers {
		nWorkers = dim
	}
	if nWorkers < 1 {
		nWorkers = 1
	}
	rowsPerWorker := dim / nWorkers

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	for w := range nWorkers {
		sr := w * rowsPerWorker
		er := sr + rowsPerWorker
		if w == nWorkers-1 {
			er = dim
		}
		go func(sr, er int) {
			defer wg.Done()
			for r := sr; r < er; r++ {
				base := r * dim
				for hi0 := 0; hi0 < dim; hi0 += hiMask << 1 {
					for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
						for offset := lo0; offset < lo0+loMask; offset++ {
							c00 := offset
							c01 := offset | mask1
							c10 := offset | mask0
							c11 := offset | mask0 | mask1
							a0 := s.rho[base+c00]
							a1 := s.rho[base+c01]
							a2 := s.rho[base+c10]
							a3 := s.rho[base+c11]
							s.rho[base+c00] = ud[0]*a0 + ud[1]*a1 + ud[2]*a2 + ud[3]*a3
							s.rho[base+c01] = ud[4]*a0 + ud[5]*a1 + ud[6]*a2 + ud[7]*a3
							s.rho[base+c10] = ud[8]*a0 + ud[9]*a1 + ud[10]*a2 + ud[11]*a3
							s.rho[base+c11] = ud[12]*a0 + ud[13]*a1 + ud[14]*a2 + ud[15]*a3
						}
					}
				}
			}
		}(sr, er)
	}
	wg.Wait()
}
