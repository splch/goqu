package noise

import (
	"fmt"
	"math"
)

// channel is a concrete implementation of Channel.
type channel struct {
	name  string
	nq    int
	kraus [][]complex128
}

func (c *channel) Name() string          { return c.name }
func (c *channel) Qubits() int           { return c.nq }
func (c *channel) Kraus() [][]complex128 { return c.kraus }

// Depolarizing1Q returns a single-qubit depolarizing channel.
//
// Models isotropic noise: with probability p, the qubit is randomly subjected
// to one of the three Pauli errors (X, Y, Z) with equal probability p/3.
// With probability 1-p, the qubit is left unchanged. At p=3/4 (maximum
// mixing), the output is the maximally mixed state I/2 regardless of input.
//
// Kraus operators: sqrt(1-p)*I, sqrt(p/3)*X, sqrt(p/3)*Y, sqrt(p/3)*Z.
func Depolarizing1Q(p float64) Channel {
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("noise.Depolarizing1Q: p=%f out of range [0,1]", p))
	}
	s0 := complex(math.Sqrt(1-p), 0)
	sp := complex(math.Sqrt(p/3), 0)
	return &channel{
		name: fmt.Sprintf("depolarizing1q(%.4f)", p),
		nq:   1,
		kraus: [][]complex128{
			{s0, 0, 0, s0},            // sqrt(1-p) * I
			{0, sp, sp, 0},            // sqrt(p/3) * X
			{0, -sp * 1i, sp * 1i, 0}, // sqrt(p/3) * Y
			{sp, 0, 0, -sp},           // sqrt(p/3) * Z
		},
	}
}

// Depolarizing2Q returns a two-qubit depolarizing channel.
//
// Extends depolarizing noise to two qubits: with probability p, one of the
// 15 non-identity two-qubit Pauli operators {I,X,Y,Z}^2 \ {II} is applied
// with equal probability p/15. With probability 1-p, no error occurs.
func Depolarizing2Q(p float64) Channel {
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("noise.Depolarizing2Q: p=%f out of range [0,1]", p))
	}
	// 16 Kraus operators: tensor products of {I,X,Y,Z} x {I,X,Y,Z}
	pauli1 := [][]complex128{
		{1, 0, 0, 1},    // I
		{0, 1, 1, 0},    // X
		{0, -1i, 1i, 0}, // Y
		{1, 0, 0, -1},   // Z
	}

	s0 := math.Sqrt(1 - p)
	sp := math.Sqrt(p / 15)

	kraus := make([][]complex128, 16)
	idx := 0
	for i := range 4 {
		for j := range 4 {
			k := tensorProduct2x2(pauli1[i], pauli1[j])
			scale := complex(sp, 0)
			if i == 0 && j == 0 {
				scale = complex(s0, 0)
			}
			for m := range k {
				k[m] *= scale
			}
			kraus[idx] = k
			idx++
		}
	}
	return &channel{
		name:  fmt.Sprintf("depolarizing2q(%.4f)", p),
		nq:    2,
		kraus: kraus,
	}
}

// tensorProduct2x2 computes the tensor product of two 2x2 matrices (both flat row-major).
// Result is a flat 4x4 matrix.
func tensorProduct2x2(a, b []complex128) []complex128 {
	result := make([]complex128, 16)
	for i := range 2 {
		for j := range 2 {
			for k := range 2 {
				for l := range 2 {
					row := i*2 + k
					col := j*2 + l
					result[row*4+col] = a[i*2+j] * b[k*2+l]
				}
			}
		}
	}
	return result
}

// AmplitudeDamping returns an amplitude damping channel (T1 decay).
//
// Models energy dissipation -- the spontaneous decay of an excited qubit
// from |1> to |0>, analogous to a photon being emitted. The parameter
// gamma = 1 - exp(-t/T1) where t is the gate duration and T1 is the energy
// relaxation time. At gamma=1, the qubit is fully decayed to |0>.
//
// Kraus: E0 = [[1,0],[0,sqrt(1-gamma)]], E1 = [[0,sqrt(gamma)],[0,0]]
func AmplitudeDamping(gamma float64) Channel {
	if gamma < 0 || gamma > 1 {
		panic(fmt.Sprintf("noise.AmplitudeDamping: gamma=%f out of range [0,1]", gamma))
	}
	sg := complex(math.Sqrt(gamma), 0)
	s1 := complex(math.Sqrt(1-gamma), 0)
	return &channel{
		name: fmt.Sprintf("amplitude_damping(%.4f)", gamma),
		nq:   1,
		kraus: [][]complex128{
			{1, 0, 0, s1},
			{0, sg, 0, 0},
		},
	}
}

// PhaseDamping returns a phase damping channel (T2 dephasing without energy loss).
//
// Models pure dephasing -- the loss of phase coherence between |0> and |1>
// without any change in the population (probability of being in |0> or |1>).
// Physically, this arises from fluctuations in the qubit's transition
// frequency caused by environmental noise. The off-diagonal elements of the
// density matrix decay by a factor of sqrt(1-lambda).
//
// Kraus: E0 = [[1,0],[0,sqrt(1-lambda)]], E1 = [[0,0],[0,sqrt(lambda)]]
func PhaseDamping(lambda float64) Channel {
	if lambda < 0 || lambda > 1 {
		panic(fmt.Sprintf("noise.PhaseDamping: lambda=%f out of range [0,1]", lambda))
	}
	sl := complex(math.Sqrt(lambda), 0)
	s1 := complex(math.Sqrt(1-lambda), 0)
	return &channel{
		name: fmt.Sprintf("phase_damping(%.4f)", lambda),
		nq:   1,
		kraus: [][]complex128{
			{1, 0, 0, s1},
			{0, 0, 0, sl},
		},
	}
}

// BitFlip returns a bit-flip channel.
//
// Models a random bit-flip error: with probability p, the qubit is flipped
// (X gate applied), mapping |0> to |1> and vice versa. This is the quantum
// analogue of a classical binary symmetric channel.
//
// Kraus: sqrt(1-p)*I, sqrt(p)*X
func BitFlip(p float64) Channel {
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("noise.BitFlip: p=%f out of range [0,1]", p))
	}
	s0 := complex(math.Sqrt(1-p), 0)
	sp := complex(math.Sqrt(p), 0)
	return &channel{
		name: fmt.Sprintf("bit_flip(%.4f)", p),
		nq:   1,
		kraus: [][]complex128{
			{s0, 0, 0, s0},
			{0, sp, sp, 0},
		},
	}
}

// PhaseFlip returns a phase-flip channel.
//
// Models a random phase-flip error: with probability p, a Z gate is applied,
// flipping the sign of the |1> component while leaving |0> unchanged. This
// has no classical analogue -- it corrupts quantum phase information only.
//
// Kraus: sqrt(1-p)*I, sqrt(p)*Z
func PhaseFlip(p float64) Channel {
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("noise.PhaseFlip: p=%f out of range [0,1]", p))
	}
	s0 := complex(math.Sqrt(1-p), 0)
	sp := complex(math.Sqrt(p), 0)
	return &channel{
		name: fmt.Sprintf("phase_flip(%.4f)", p),
		nq:   1,
		kraus: [][]complex128{
			{s0, 0, 0, s0},
			{sp, 0, 0, -sp},
		},
	}
}

// GeneralizedAmplitudeDamping returns a generalized amplitude damping channel.
//
// Models thermal relaxation to a finite-temperature bath. The parameter p is
// the probability of the environment being in the ground state, related to
// temperature by p = 1/(1 + exp(-hbar*omega/(k_B*T))). At p=1 (zero
// temperature), this reduces to standard amplitude damping. At p<1, the
// qubit can be thermally excited from |0> to |1>, and the steady state is
// a thermal mixture rather than pure |0>.
//
// gamma is the damping rate [0,1], p is the thermal population [0,1].
// Kraus operators:
//
//	E0 = sqrt(p)   * [[1,0],[0,sqrt(1-gamma)]]
//	E1 = sqrt(p)   * [[0,sqrt(gamma)],[0,0]]
//	E2 = sqrt(1-p) * [[sqrt(1-gamma),0],[0,1]]
//	E3 = sqrt(1-p) * [[0,0],[sqrt(gamma),0]]
func GeneralizedAmplitudeDamping(p, gamma float64) Channel {
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("noise.GeneralizedAmplitudeDamping: p=%f out of range [0,1]", p))
	}
	if gamma < 0 || gamma > 1 {
		panic(fmt.Sprintf("noise.GeneralizedAmplitudeDamping: gamma=%f out of range [0,1]", gamma))
	}
	sp := complex(math.Sqrt(p), 0)
	s1p := complex(math.Sqrt(1-p), 0)
	sg := complex(math.Sqrt(gamma), 0)
	s1g := complex(math.Sqrt(1-gamma), 0)
	return &channel{
		name: fmt.Sprintf("generalized_amplitude_damping(%.4f,%.4f)", p, gamma),
		nq:   1,
		kraus: [][]complex128{
			{sp, 0, 0, sp * s1g},   // E0 = sqrt(p) * [[1,0],[0,sqrt(1-gamma)]]
			{0, sp * sg, 0, 0},     // E1 = sqrt(p) * [[0,sqrt(gamma)],[0,0]]
			{s1p * s1g, 0, 0, s1p}, // E2 = sqrt(1-p) * [[sqrt(1-gamma),0],[0,1]]
			{0, 0, s1p * sg, 0},    // E3 = sqrt(1-p) * [[0,0],[sqrt(gamma),0]]
		},
	}
}

// ThermalRelaxation returns a combined T1/T2 relaxation channel.
// t1 is the relaxation time, t2 is the dephasing time, time is the gate duration.
// Requires t2 <= 2*t1, and t1, t2 > 0, time >= 0.
//
// The channel is composed as amplitude damping (T1 decay) followed by
// residual phase damping to match the target T2 coherence.
func ThermalRelaxation(t1, t2, time float64) Channel {
	if t2 > 2*t1 {
		panic(fmt.Sprintf("noise.ThermalRelaxation: t2=%f > 2*t1=%f is unphysical", t2, 2*t1))
	}
	if t1 <= 0 || t2 <= 0 || time < 0 {
		panic("noise.ThermalRelaxation: t1, t2 must be positive, time must be non-negative")
	}

	// Probability of decay from |1> to |0>
	gamma := 1 - math.Exp(-time/t1)

	// Effective additional dephasing beyond T1 contribution.
	// Amplitude damping alone gives off-diagonal decay of sqrt(1-gamma).
	// T2 requires off-diagonal decay of exp(-time/t2).
	// The residual phase damping lambda satisfies:
	//   sqrt(1-gamma) * sqrt(1-lambda) = exp(-time/t2)
	eLambda := math.Exp(-time / t2)
	lambda := 0.0
	if gamma < 1 {
		idealCoherence := math.Sqrt(1 - gamma)
		if eLambda < idealCoherence {
			ratio := eLambda / idealCoherence
			lambda = 1 - ratio*ratio
		}
	}

	// Compose amplitude damping(gamma) then phase damping(lambda).
	// AD Kraus: A0=[[1,0],[0,sqrt(1-g)]], A1=[[0,sqrt(g)],[0,0]]
	// PD Kraus: P0=[[1,0],[0,sqrt(1-l)]], P1=[[0,0],[0,sqrt(l)]]
	// Products:
	//   P0*A0 = [[1,0],[0,sqrt(1-g)*sqrt(1-l)]]
	//   P0*A1 = [[0,sqrt(g)],[0,0]]
	//   P1*A0 = [[0,0],[0,sqrt(l)*sqrt(1-g)]]
	//   P1*A1 = [[0,0],[0,0]]  (zero, skip)
	sg := math.Sqrt(gamma)
	s1g := math.Sqrt(1 - gamma)
	sl := math.Sqrt(lambda)
	s1l := math.Sqrt(1 - lambda)

	kraus := [][]complex128{
		{1, 0, 0, complex(s1l*s1g, 0)}, // P0*A0
		{0, complex(sg, 0), 0, 0},      // P0*A1
	}
	if lambda > 1e-15 {
		kraus = append(kraus, []complex128{0, 0, 0, complex(sl*s1g, 0)}) // P1*A0
	}

	return &channel{
		name:  fmt.Sprintf("thermal_relaxation(%.4g,%.4g,%.4g)", t1, t2, time),
		nq:    1,
		kraus: kraus,
	}
}
