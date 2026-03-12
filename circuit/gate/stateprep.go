package gate

import (
	"fmt"
	"math"
	"math/cmplx"
)

// StatePrepable is implemented by gates that prepare a specific quantum state.
type StatePrepable interface {
	Amplitudes() []complex128
}

type statePrepGate struct {
	amplitudes []complex128
	nQubits    int
}

// StatePrep creates a state preparation gate from normalized amplitudes.
// The length of amplitudes must be a power of 2 (2^n for n qubits).
// The amplitudes must be normalized (sum of |a_i|^2 = 1).
func StatePrep(amplitudes []complex128) (Gate, error) {
	n := len(amplitudes)
	if n == 0 || n&(n-1) != 0 {
		return nil, fmt.Errorf("gate.StatePrep: amplitudes length %d is not a power of 2", n)
	}
	nq := 0
	for n>>nq > 1 {
		nq++
	}
	// Check normalization.
	var norm float64
	for _, a := range amplitudes {
		norm += real(a)*real(a) + imag(a)*imag(a)
	}
	if math.Abs(norm-1.0) > 1e-8 {
		return nil, fmt.Errorf("gate.StatePrep: amplitudes not normalized (norm=%f)", norm)
	}
	amps := make([]complex128, len(amplitudes))
	copy(amps, amplitudes)
	return &statePrepGate{amplitudes: amps, nQubits: nq}, nil
}

// MustStatePrep creates a state preparation gate, panicking on error.
func MustStatePrep(amplitudes []complex128) Gate {
	g, err := StatePrep(amplitudes)
	if err != nil {
		panic(err)
	}
	return g
}

func (g *statePrepGate) Name() string         { return "StatePrep" }
func (g *statePrepGate) Qubits() int          { return g.nQubits }
func (g *statePrepGate) Matrix() []complex128 { return nil } // pseudo-gate
func (g *statePrepGate) Params() []float64    { return nil }
func (g *statePrepGate) Amplitudes() []complex128 {
	out := make([]complex128, len(g.amplitudes))
	copy(out, g.amplitudes)
	return out
}

func (g *statePrepGate) Inverse() Gate {
	return &statePrepInvGate{amplitudes: g.amplitudes, nQubits: g.nQubits}
}

// Decompose returns a Mottonen-style RY+CNOT cascade.
func (g *statePrepGate) Decompose(qubits []int) []Applied {
	if len(qubits) != g.nQubits {
		return nil
	}
	return mottonenDecompose(g.amplitudes, qubits)
}

type statePrepInvGate struct {
	amplitudes []complex128
	nQubits    int
}

func (g *statePrepInvGate) Name() string         { return "StatePrep†" }
func (g *statePrepInvGate) Qubits() int          { return g.nQubits }
func (g *statePrepInvGate) Matrix() []complex128 { return nil }
func (g *statePrepInvGate) Params() []float64    { return nil }
func (g *statePrepInvGate) Inverse() Gate {
	return &statePrepGate{amplitudes: g.amplitudes, nQubits: g.nQubits}
}

func (g *statePrepInvGate) Decompose(qubits []int) []Applied {
	if len(qubits) != g.nQubits {
		return nil
	}
	fwd := mottonenDecompose(g.amplitudes, qubits)
	if fwd == nil {
		return nil
	}
	inv := make([]Applied, len(fwd))
	for i, a := range fwd {
		inv[len(fwd)-1-i] = Applied{Gate: a.Gate.Inverse(), Qubits: a.Qubits}
	}
	return inv
}

// mottonenDecompose implements a simplified Mottonen decomposition.
// For 1 qubit: RY(2*atan2(|a1|, |a0|)) then RZ for phase.
// For n qubits: recursive uniformly-controlled rotations.
func mottonenDecompose(amplitudes []complex128, qubits []int) []Applied {
	n := len(qubits)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return decompose1Q(amplitudes, qubits[0])
	}

	var ops []Applied

	// Work from least significant qubit to most significant.
	// For each level, compute RY angles to merge pairs.
	amps := make([]complex128, len(amplitudes))
	copy(amps, amplitudes)

	for level := 0; level < n; level++ {
		target := qubits[n-1-level]
		halfSize := 1 << level
		numPairs := len(amps) / (2 * halfSize)

		angles := make([]float64, numPairs)
		newAmps := make([]complex128, numPairs*halfSize)

		for p := range numPairs {
			groupSize := halfSize
			var normSq0, normSq1 float64
			for k := range groupSize {
				idx0 := p*2*groupSize + k
				idx1 := p*2*groupSize + groupSize + k
				a0 := amps[idx0]
				a1 := amps[idx1]
				normSq0 += real(a0)*real(a0) + imag(a0)*imag(a0)
				normSq1 += real(a1)*real(a1) + imag(a1)*imag(a1)
			}
			norm0 := math.Sqrt(normSq0)
			norm1 := math.Sqrt(normSq1)
			angles[p] = 2 * math.Atan2(norm1, norm0)

			totalNorm := math.Sqrt(normSq0 + normSq1)
			for k := range groupSize {
				idx0 := p*2*groupSize + k
				if totalNorm > 1e-15 {
					if norm0 > 1e-15 {
						newAmps[p*groupSize+k] = amps[idx0] * complex(totalNorm/norm0, 0)
					} else {
						if k == 0 {
							newAmps[p*groupSize+k] = complex(totalNorm, 0)
						} else {
							newAmps[p*groupSize+k] = 0
						}
					}
				}
			}
		}

		// Apply uniformly controlled RY.
		controls := make([]int, 0, n-1-level)
		for ci := 0; ci < n-1-level; ci++ {
			controls = append(controls, qubits[ci])
		}
		if numPairs == 1 {
			if math.Abs(angles[0]) > 1e-10 {
				ops = append(ops, Applied{Gate: RY(angles[0]), Qubits: []int{target}})
			}
		} else {
			ops = append(ops, uniformlyControlledRY(angles, controls, target)...)
		}

		amps = newAmps
	}

	// Handle remaining phases.
	if len(amps) == 1 {
		phase := cmplx.Phase(amps[0])
		if math.Abs(phase) > 1e-10 {
			ops = append(ops, Applied{Gate: RZ(phase), Qubits: []int{qubits[0]}})
		}
	}

	return ops
}

func decompose1Q(amps []complex128, qubit int) []Applied {
	var ops []Applied
	theta := 2 * math.Atan2(cmplx.Abs(amps[1]), cmplx.Abs(amps[0]))
	if math.Abs(theta) > 1e-10 {
		ops = append(ops, Applied{Gate: RY(theta), Qubits: []int{qubit}})
	}
	// Phase difference.
	if cmplx.Abs(amps[0]) > 1e-10 && cmplx.Abs(amps[1]) > 1e-10 {
		phaseDiff := cmplx.Phase(amps[1]) - cmplx.Phase(amps[0])
		if math.Abs(phaseDiff) > 1e-10 {
			ops = append(ops, Applied{Gate: RZ(phaseDiff), Qubits: []int{qubit}})
		}
	} else if cmplx.Abs(amps[1]) > 1e-10 {
		phase := cmplx.Phase(amps[1])
		if math.Abs(phase) > 1e-10 {
			ops = append(ops, Applied{Gate: RZ(phase), Qubits: []int{qubit}})
		}
	}
	// Global phase from amps[0] is unobservable and already handled
	// by the RZ gate's half-angle convention. Adding a Phase gate here
	// would corrupt the relative phase between |0⟩ and |1⟩.
	return ops
}

// uniformlyControlledRY implements a uniformly controlled RY gate.
// For N controls, there are 2^N angles, one for each computational basis state of the controls.
// Uses recursive decomposition into CNOT + RY pairs.
func uniformlyControlledRY(angles []float64, controls []int, target int) []Applied {
	n := len(angles)
	if n == 0 {
		return nil
	}
	if n == 1 {
		if math.Abs(angles[0]) > 1e-10 {
			return []Applied{{Gate: RY(angles[0]), Qubits: []int{target}}}
		}
		return nil
	}

	// Recursive decomposition: split angles into halves.
	// theta_even = (angles[2k] + angles[2k+1]) / 2
	// theta_odd  = (angles[2k] - angles[2k+1]) / 2
	half := n / 2
	even := make([]float64, half)
	odd := make([]float64, half)
	for k := range half {
		even[k] = (angles[2*k] + angles[2*k+1]) / 2
		odd[k] = (angles[2*k] - angles[2*k+1]) / 2
	}

	var ops []Applied

	// Recursively decompose even part (without last control).
	if len(controls) > 1 {
		ops = append(ops, uniformlyControlledRY(even, controls[:len(controls)-1], target)...)
	} else if math.Abs(even[0]) > 1e-10 {
		ops = append(ops, Applied{Gate: RY(even[0]), Qubits: []int{target}})
	}

	// CNOT from last control to target.
	ops = append(ops, Applied{Gate: CNOT, Qubits: []int{controls[len(controls)-1], target}})

	// Recursively decompose odd part (without last control).
	if len(controls) > 1 {
		ops = append(ops, uniformlyControlledRY(odd, controls[:len(controls)-1], target)...)
	} else if math.Abs(odd[0]) > 1e-10 {
		ops = append(ops, Applied{Gate: RY(odd[0]), Qubits: []int{target}})
	}

	// CNOT from last control to target.
	ops = append(ops, Applied{Gate: CNOT, Qubits: []int{controls[len(controls)-1], target}})

	return ops
}
