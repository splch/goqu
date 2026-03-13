package pulsesim

import "math"

// applyCRStep applies one cross-resonance time step.
// The target qubit sees opposite rotations conditioned on the control qubit:
//
//	control=|0>: U_target = R(+theta, phi)
//	control=|1>: U_target = R(-theta, phi)
//
// This produces ZX-like entanglement — the dominant physics of echoed CR gates.
func (s *Sim) applyCRStep(control, target int, theta, phi float64) {
	// U_0: standard drive unitary at (theta, phi)
	cosH0 := math.Cos(theta / 2)
	sinH0 := math.Sin(theta / 2)
	cosPhi := math.Cos(phi)
	sinPhi := math.Sin(phi)

	u0_00 := complex(cosH0, 0)
	u0_01 := complex(-sinH0*sinPhi, -sinH0*cosPhi)
	u0_10 := complex(sinH0*sinPhi, -sinH0*cosPhi)
	u0_11 := complex(cosH0, 0)

	// U_1: drive unitary at (-theta, phi)
	cosH1 := math.Cos(-theta / 2)
	sinH1 := math.Sin(-theta / 2)

	u1_00 := complex(cosH1, 0)
	u1_01 := complex(-sinH1*sinPhi, -sinH1*cosPhi)
	u1_10 := complex(sinH1*sinPhi, -sinH1*cosPhi)
	u1_11 := complex(cosH1, 0)

	// Block-stride iteration over 2Q subspaces.
	maskC := 1 << control
	maskT := 1 << target
	lo, hi := control, target
	if lo > hi {
		lo, hi = hi, lo
	}
	loMask := 1 << lo
	hiMask := 1 << hi
	n := len(s.state)

	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				i00 := offset                 // control=0, target=0
				i01 := offset | maskT         // control=0, target=1
				i10 := offset | maskC         // control=1, target=0
				i11 := offset | maskC | maskT // control=1, target=1

				// Apply U_0 to target when control=|0>
				a0, a1 := s.state[i00], s.state[i01]
				s.state[i00] = u0_00*a0 + u0_01*a1
				s.state[i01] = u0_10*a0 + u0_11*a1

				// Apply U_1 to target when control=|1>
				a2, a3 := s.state[i10], s.state[i11]
				s.state[i10] = u1_00*a2 + u1_01*a3
				s.state[i11] = u1_10*a2 + u1_11*a3
			}
		}
	}
}

// applyZZPhase applies the static ZZ coupling phase:
//
//	U_ZZ = diag(e^{-i*angle}, e^{+i*angle}, e^{+i*angle}, e^{-i*angle})
//
// where angle = zeta * duration / 2. The eigenvalues correspond to:
// |00> → +1, |01> → -1, |10> → -1, |11> → +1 for ZZ.
func (s *Sim) applyZZPhase(q0, q1 int, zeta, duration float64) {
	angle := zeta * duration / 2
	if math.Abs(angle) < 1e-15 {
		return
	}

	// Phase factors for each basis state.
	phaseP := complex(math.Cos(angle), -math.Sin(angle)) // e^{-i*angle} for |00>,|11>
	phaseM := complex(math.Cos(angle), math.Sin(angle))  // e^{+i*angle} for |01>,|10>

	mask0 := 1 << q0
	mask1 := 1 << q1
	lo, hi := q0, q1
	if lo > hi {
		lo, hi = hi, lo
	}
	loMask := 1 << lo
	hiMask := 1 << hi
	n := len(s.state)

	for hi0 := 0; hi0 < n; hi0 += hiMask << 1 {
		for lo0 := hi0; lo0 < hi0+hiMask; lo0 += loMask << 1 {
			for offset := lo0; offset < lo0+loMask; offset++ {
				s.state[offset] *= phaseP             // |00>
				s.state[offset|mask1] *= phaseM       // |01>
				s.state[offset|mask0] *= phaseM       // |10>
				s.state[offset|mask0|mask1] *= phaseP // |11>
			}
		}
	}
}

// applyAllZZ applies static ZZ coupling for all pairs in the coupling map.
func (s *Sim) applyAllZZ(duration float64) {
	for pair, c := range s.coupling {
		if c.ZZ != 0 {
			s.applyZZPhase(pair[0], pair[1], c.ZZ, duration)
		}
	}
}
