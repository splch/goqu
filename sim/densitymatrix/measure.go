package densitymatrix

import (
	"math"
	"math/bits"
	"math/rand/v2"
)

// diagonalProbs extracts measurement probabilities from the density matrix diagonal.
// P(i) = Re(ρ[i*dim+i]).
func (s *Sim) diagonalProbs() []float64 {
	probs := make([]float64, s.dim)
	for i := range s.dim {
		probs[i] = real(s.rho[i*s.dim+i])
		if probs[i] < 0 {
			probs[i] = 0
		}
	}
	return probs
}

// DiagonalProbs returns the measurement probabilities (diagonal of ρ).
func (s *Sim) DiagonalProbs() []float64 {
	return s.diagonalProbs()
}

// ApplyReadoutError applies per-qubit readout errors to probabilities.
// Each qubit's readout error independently flips the classical bit.
func (s *Sim) ApplyReadoutError(probs []float64) []float64 {
	if s.noise == nil {
		return probs
	}
	dim := s.dim
	result := make([]float64, dim)
	for i := range dim {
		p := probs[i]
		if p == 0 {
			continue
		}
		// For each basis state i, apply readout errors per qubit.
		// Each qubit q in state i can flip independently.
		result[i] += p
	}

	// Apply qubit-by-qubit readout error.
	for q := range s.numQubits {
		re := s.noise.ReadoutFor(q)
		if re == nil {
			continue
		}
		newResult := make([]float64, dim)
		mask := 1 << q
		for i := range dim {
			if result[i] == 0 {
				continue
			}
			flipped := i ^ mask
			if i&mask == 0 {
				// Qubit q is in state 0: P01 chance to flip to 1.
				newResult[i] += result[i] * (1 - re.P01)
				newResult[flipped] += result[i] * re.P01
			} else {
				// Qubit q is in state 1: P10 chance to flip to 0.
				newResult[i] += result[i] * (1 - re.P10)
				newResult[flipped] += result[i] * re.P10
			}
		}
		result = newResult
	}
	return result
}

// ExpectationValue computes Tr(ρ·O) for a diagonal Pauli-Z observable
// specified as a list of qubit indices. For example, [0, 1] computes <Z0 Z1>.
// The result is rounded to 14 decimal places to clean up floating-point noise.
func (s *Sim) ExpectationValue(qubits []int) float64 {
	var mask int
	for _, q := range qubits {
		mask |= 1 << q
	}
	var ev float64
	for i := range s.dim {
		prob := real(s.rho[i*s.dim+i])
		if bits.OnesCount(uint(i&mask))%2 == 0 {
			ev += prob
		} else {
			ev -= prob
		}
	}
	return math.Round(ev*1e14) / 1e14
}

func sampleIndex(probs []float64, rng *rand.Rand) int {
	r := rng.Float64()
	cum := 0.0
	for i, p := range probs {
		cum += p
		if r < cum {
			return i
		}
	}
	return len(probs) - 1
}

func formatBitstring(idx, n int) string {
	bs := make([]byte, n)
	for i := range n {
		if idx&(1<<i) != 0 {
			bs[n-1-i] = '1'
		} else {
			bs[n-1-i] = '0'
		}
	}
	return string(bs)
}
