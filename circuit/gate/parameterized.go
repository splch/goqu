package gate

import (
	"fmt"
	"math"
	"math/cmplx"
)

// parameterized is a gate constructed from rotation parameters.
type parameterized struct {
	name   string
	n      int
	params []float64
	matrix []complex128
}

func (g *parameterized) Name() string                { return g.name }
func (g *parameterized) Qubits() int                 { return g.n }
func (g *parameterized) Matrix() []complex128        { return g.matrix }
func (g *parameterized) Params() []float64           { return g.params }
func (g *parameterized) Decompose(_ []int) []Applied { return nil }

func (g *parameterized) Inverse() Gate {
	dim := 1 << g.n
	inv := make([]complex128, dim*dim)
	for r := range dim {
		for c := range dim {
			inv[r*dim+c] = conj(g.matrix[c*dim+r])
		}
	}
	negParams := make([]float64, len(g.params))
	for i, p := range g.params {
		negParams[i] = -p
	}
	return &parameterized{
		name:   g.name + "†",
		n:      g.n,
		params: negParams,
		matrix: inv,
	}
}

// Wrapper types that embed parameterized and implement the optional dispatch
// interfaces. These let the simulator select optimized kernels via type
// assertions instead of parsing gate names.

// controlDiag2Q implements ControlDiagonal2Q for gates like CP and CRZ
// whose matrix is diag(1, 1, d10, d11).
type controlDiag2Q struct{ *parameterized }

func (g *controlDiag2Q) ControlDiagonal() (d10, d11 complex128) {
	return g.matrix[10], g.matrix[15]
}

func (g *controlDiag2Q) Inverse() Gate {
	base := g.parameterized.Inverse().(*parameterized)
	return &controlDiag2Q{base}
}

// diagonal2Q implements Diagonal2Q for fully diagonal 2-qubit gates like RZZ.
type diagonal2Q struct{ *parameterized }

func (g *diagonal2Q) Diagonal() (d00, d01, d10, d11 complex128) {
	return g.matrix[0], g.matrix[5], g.matrix[10], g.matrix[15]
}

func (g *diagonal2Q) Inverse() Gate {
	base := g.parameterized.Inverse().(*parameterized)
	return &diagonal2Q{base}
}

// controlU2Q implements ControlU2Q for controlled-unitary gates like CRX and CRY.
type controlU2Q struct{ *parameterized }

func (g *controlU2Q) ControlSubmatrix() (u00, u01, u10, u11 complex128) {
	return g.matrix[10], g.matrix[11], g.matrix[14], g.matrix[15]
}

func (g *controlU2Q) Inverse() Gate {
	base := g.parameterized.Inverse().(*parameterized)
	return &controlU2Q{base}
}

// RX returns an X-rotation gate: exp(-i * theta/2 * X). It rotates the
// qubit state by angle theta around the X axis of the Bloch sphere. At
// theta=pi, RX equals -i*X (a bit-flip with global phase). At theta=pi/2,
// it creates a superposition equivalent to H up to phase.
//
//	[[cos(θ/2), -i·sin(θ/2)],
//	 [-i·sin(θ/2), cos(θ/2)]]
func RX(theta float64) Gate {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	return &parameterized{
		name:   fmt.Sprintf("RX(%.4f)", theta),
		n:      1,
		params: []float64{theta},
		matrix: []complex128{
			complex(c, 0), complex(0, -s),
			complex(0, -s), complex(c, 0),
		},
	}
}

// RY returns a Y-rotation gate: exp(-i * theta/2 * Y). It rotates the
// qubit state by angle theta around the Y axis of the Bloch sphere. Unlike
// RX and RZ, RY has only real entries, making it useful for state preparation
// (converting amplitudes between |0> and |1> without introducing phase).
//
//	[[cos(θ/2), -sin(θ/2)],
//	 [sin(θ/2), cos(θ/2)]]
func RY(theta float64) Gate {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	return &parameterized{
		name:   fmt.Sprintf("RY(%.4f)", theta),
		n:      1,
		params: []float64{theta},
		matrix: []complex128{
			complex(c, 0), complex(-s, 0),
			complex(s, 0), complex(c, 0),
		},
	}
}

// RZ returns a Z-rotation gate: exp(-i * theta/2 * Z). It rotates the
// qubit state by angle theta around the Z axis of the Bloch sphere (pure
// phase rotation). RZ is diagonal and commutes with measurement in the
// computational basis. On IBM hardware, RZ is implemented as a virtual gate
// (frame change) with zero error, making it a preferred building block.
//
//	[[exp(-iθ/2), 0],
//	 [0, exp(iθ/2)]]
func RZ(theta float64) Gate {
	return &parameterized{
		name:   fmt.Sprintf("RZ(%.4f)", theta),
		n:      1,
		params: []float64{theta},
		matrix: []complex128{
			cmplx.Exp(complex(0, -theta/2)), 0,
			0, cmplx.Exp(complex(0, theta/2)),
		},
	}
}

// Phase returns a phase gate: diag(1, exp(iφ)).
func Phase(phi float64) Gate {
	return &parameterized{
		name:   fmt.Sprintf("P(%.4f)", phi),
		n:      1,
		params: []float64{phi},
		matrix: []complex128{
			1, 0,
			0, cmplx.Exp(complex(0, phi)),
		},
	}
}

// U3 returns the universal single-qubit gate U(θ, φ, λ). Any single-qubit
// unitary can be expressed as U3 (up to global phase), making it the most
// general single-qubit rotation. Special cases: U3(θ,0,0) = RY(θ) up to
// phase; U3(0,0,λ) = Phase(λ) up to phase.
//
//	[[cos(θ/2), -exp(iλ)·sin(θ/2)],
//	 [exp(iφ)·sin(θ/2), exp(i(φ+λ))·cos(θ/2)]]
func U3(theta, phi, lambda float64) Gate {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	return &parameterized{
		name:   fmt.Sprintf("U3(%.4f,%.4f,%.4f)", theta, phi, lambda),
		n:      1,
		params: []float64{theta, phi, lambda},
		matrix: []complex128{
			complex(c, 0),
			-cmplx.Exp(complex(0, lambda)) * complex(s, 0),
			cmplx.Exp(complex(0, phi)) * complex(s, 0),
			cmplx.Exp(complex(0, phi+lambda)) * complex(c, 0),
		},
	}
}

// CP returns a controlled-phase gate: diag(1, 1, 1, exp(iφ)). It applies a
// phase exp(iφ) to the |11> state and leaves all others unchanged. Symmetric
// in q0,q1. At φ=π, CP equals CZ. Used in QFT (Quantum Fourier Transform)
// with φ=2π/2^k for increasing k.
func CP(phi float64) Gate {
	return &controlDiag2Q{&parameterized{
		name:   fmt.Sprintf("CP(%.4f)", phi),
		n:      2,
		params: []float64{phi},
		matrix: []complex128{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, 1, 0,
			0, 0, 0, cmplx.Exp(complex(0, phi)),
		},
	}}
}

// CRZ returns a controlled-RZ gate.
func CRZ(theta float64) Gate {
	return &controlDiag2Q{&parameterized{
		name:   fmt.Sprintf("CRZ(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, cmplx.Exp(complex(0, -theta/2)), 0,
			0, 0, 0, cmplx.Exp(complex(0, theta/2)),
		},
	}}
}

// CRX returns a controlled-RX gate.
func CRX(theta float64) Gate {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	return &controlU2Q{&parameterized{
		name:   fmt.Sprintf("CRX(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, complex(c, 0), complex(0, -s),
			0, 0, complex(0, -s), complex(c, 0),
		},
	}}
}

// CRY returns a controlled-RY gate.
func CRY(theta float64) Gate {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	return &controlU2Q{&parameterized{
		name:   fmt.Sprintf("CRY(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, complex(c, 0), complex(-s, 0),
			0, 0, complex(s, 0), complex(c, 0),
		},
	}}
}

// RXX returns the Ising XX coupling gate: exp(-i * theta/2 * X⊗X). It
// models a two-qubit interaction where both qubits rotate simultaneously
// around the X axis. Native on trapped-ion platforms (IonQ) where the
// Mølmer-Sørensen interaction naturally produces XX coupling.
//
//	c = cos(θ/2), s = sin(θ/2)
//	[[c, 0, 0, -is],
//	 [0, c, -is, 0],
//	 [0, -is, c, 0],
//	 [-is, 0, 0, c]]
func RXX(theta float64) Gate {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	is := complex(0, -s)
	cc := complex(c, 0)
	return &parameterized{
		name:   fmt.Sprintf("RXX(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			cc, 0, 0, is,
			0, cc, is, 0,
			0, is, cc, 0,
			is, 0, 0, cc,
		},
	}
}

// RYY returns the Ising YY coupling gate: exp(-i * theta/2 * Y⊗Y). It
// models a two-qubit YY interaction, used in variational ansatze and
// Hamiltonian simulation for systems with YY coupling terms.
//
//	c = cos(θ/2), s = sin(θ/2)
//	[[c, 0, 0, is],
//	 [0, c, -is, 0],
//	 [0, -is, c, 0],
//	 [is, 0, 0, c]]
func RYY(theta float64) Gate {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	is := complex(0, s)
	nis := complex(0, -s)
	cc := complex(c, 0)
	return &parameterized{
		name:   fmt.Sprintf("RYY(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			cc, 0, 0, is,
			0, cc, nis, 0,
			0, nis, cc, 0,
			is, 0, 0, cc,
		},
	}
}

// RZZ returns the Ising ZZ coupling gate: exp(-i * theta/2 * Z⊗Z). It is
// fully diagonal, applying phases based on the parity of the two qubits.
// Appears naturally in Hamiltonian simulation of spin chains and in QAOA
// for combinatorial optimization (encoding problem clauses). Since it is
// diagonal, it commutes with measurements in the computational basis.
//
//	[[exp(-iθ/2), 0, 0, 0],
//	 [0, exp(iθ/2), 0, 0],
//	 [0, 0, exp(iθ/2), 0],
//	 [0, 0, 0, exp(-iθ/2)]]
func RZZ(theta float64) Gate {
	em := cmplx.Exp(complex(0, -theta/2))
	ep := cmplx.Exp(complex(0, theta/2))
	return &diagonal2Q{&parameterized{
		name:   fmt.Sprintf("RZZ(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			em, 0, 0, 0,
			0, ep, 0, 0,
			0, 0, ep, 0,
			0, 0, 0, em,
		},
	}}
}

// ZZ returns the IonQ native ZZ entangling gate. It has the same matrix as
// [RZZ] but is recognized as a native gate by the IonQ serializer. On Forte
// hardware the ZZ gate is the native two-qubit entangling interaction.
//
//	diag(exp(-iθ/2), exp(iθ/2), exp(iθ/2), exp(-iθ/2))
func ZZ(theta float64) Gate {
	em := cmplx.Exp(complex(0, -theta/2))
	ep := cmplx.Exp(complex(0, theta/2))
	return &diagonal2Q{&parameterized{
		name:   fmt.Sprintf("ZZ(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			em, 0, 0, 0,
			0, ep, 0, 0,
			0, 0, ep, 0,
			0, 0, 0, em,
		},
	}}
}

// GPI returns an IonQ native GPI gate. On trapped-ion hardware, GPI
// implements a pi rotation (180 degrees) around an axis in the XY plane of
// the Bloch sphere at azimuthal angle phi. Together with [GPI2] and an
// entangling gate ([MS] on Aria, [ZZ] on Forte), GPI forms a universal
// native gate set for IonQ processors.
//
//	[[0, exp(-iφ)],
//	 [exp(iφ), 0]]
func GPI(phi float64) Gate {
	return &parameterized{
		name:   fmt.Sprintf("GPI(%.4f)", phi),
		n:      1,
		params: []float64{phi},
		matrix: []complex128{
			0, cmplx.Exp(complex(0, -phi)),
			cmplx.Exp(complex(0, phi)), 0,
		},
	}
}

// GPI2 returns an IonQ native GPI2 gate. It implements a pi/2 rotation
// (90 degrees) around an axis in the XY plane at azimuthal angle phi.
// GPI2 is the "half gate" counterpart to [GPI] and is used to build
// arbitrary single-qubit rotations on IonQ hardware.
//
//	(1/√2) * [[1, -i·exp(-iφ)],
//	           [-i·exp(iφ), 1]]
func GPI2(phi float64) Gate {
	inv := complex(s2, 0)
	return &parameterized{
		name:   fmt.Sprintf("GPI2(%.4f)", phi),
		n:      1,
		params: []float64{phi},
		matrix: []complex128{
			inv, inv * complex(0, -1) * cmplx.Exp(complex(0, -phi)),
			inv * complex(0, -1) * cmplx.Exp(complex(0, phi)), inv,
		},
	}
}

// NOP returns an IonQ native NOP (no-operation) timing gate. It is a
// 0-qubit instruction analogous to [Delay]; time is in microseconds. On
// IonQ hardware, NOP inserts an idle period used for timing synchronization
// between qubits.
func NOP(time float64) Gate {
	return &parameterized{
		name:   fmt.Sprintf("NOP(%.4f)", time),
		n:      0,
		params: []float64{time},
		matrix: []complex128{1}, // 2^0 × 2^0 identity
	}
}

// MS returns an IonQ native Mølmer-Sørensen gate. The MS gate generates
// entanglement between two trapped ions by driving both with lasers that
// couple through their shared motional mode. It is the native two-qubit
// gate on IonQ processors and is equivalent to RXX(pi/2) up to local
// rotations when phi0=phi1=0.
func MS(phi0, phi1 float64) Gate {
	inv := complex(s2, 0)
	ep := cmplx.Exp(complex(0, phi0+phi1))
	em := cmplx.Exp(complex(0, phi0-phi1))
	return &parameterized{
		name:   fmt.Sprintf("MS(%.4f,%.4f)", phi0, phi1),
		n:      2,
		params: []float64{phi0, phi1},
		matrix: []complex128{
			inv, 0, 0, inv * complex(0, -1) * conj(ep),
			0, inv, inv * complex(0, -1) * em, 0,
			0, inv * complex(0, -1) * conj(em), inv, 0,
			inv * complex(0, -1) * ep, 0, 0, inv,
		},
	}
}

// U1 returns a phase gate, equivalent to [Phase]. This is the Qiskit naming
// convention: U1(λ) = Phase(λ) = RZ(λ) up to global phase.
//
//	diag(1, exp(iλ))
func U1(lambda float64) Gate {
	return &parameterized{
		name:   fmt.Sprintf("U1(%.4f)", lambda),
		n:      1,
		params: []float64{lambda},
		matrix: []complex128{
			1, 0,
			0, cmplx.Exp(complex(0, lambda)),
		},
	}
}

// U2 returns the single-qubit gate U3(π/2, φ, λ). It always creates a
// superposition (theta is fixed at π/2) with tunable phases. U2(0, π) = H
// up to global phase.
//
//	(1/√2)·[[1, -exp(iλ)], [exp(iφ), exp(i(φ+λ))]]
func U2(phi, lambda float64) Gate {
	return &parameterized{
		name:   fmt.Sprintf("U2(%.4f,%.4f)", phi, lambda),
		n:      1,
		params: []float64{phi, lambda},
		matrix: []complex128{
			complex(s2, 0),
			-cmplx.Exp(complex(0, lambda)) * complex(s2, 0),
			cmplx.Exp(complex(0, phi)) * complex(s2, 0),
			cmplx.Exp(complex(0, phi+lambda)) * complex(s2, 0),
		},
	}
}

// Rot returns the PennyLane-style rotation gate RZ(ω)·RY(θ)·RZ(φ). This
// is the ZYZ Euler decomposition: any single-qubit unitary can be expressed
// as three successive rotations RZ, RY, RZ (up to global phase). This is
// PennyLane's default parameterization for single-qubit rotations.
func Rot(phi, theta, omega float64) Gate {
	// Compute the 2x2 matrix as product: RZ(omega) * RY(theta) * RZ(phi).
	cth, sth := math.Cos(theta/2), math.Sin(theta/2)
	eipo := cmplx.Exp(complex(0, (phi+omega)/2))
	eimo := cmplx.Exp(complex(0, (phi-omega)/2))
	// RZ(w)*RY(t)*RZ(p) =
	// [[ exp(-i(p+w)/2)*cos(t/2), -exp(i(p-w)/2)*sin(t/2)],
	//  [ exp(-i(p-w)/2)*sin(t/2),  exp(i(p+w)/2)*cos(t/2)]]
	return &parameterized{
		name:   fmt.Sprintf("Rot(%.4f,%.4f,%.4f)", phi, theta, omega),
		n:      1,
		params: []float64{phi, theta, omega},
		matrix: []complex128{
			conj(eipo) * complex(cth, 0),
			-conj(eimo) * complex(sth, 0),
			eimo * complex(sth, 0),
			eipo * complex(cth, 0),
		},
	}
}

// PhasedXZ returns the Cirq-style PhasedXZ gate: Z^z · P^a · X^x · (P^a)†.
// This is Cirq's canonical single-qubit representation: any single-qubit
// unitary can be written as PhasedXZ (up to global phase) using at most 3
// half-turn parameters. Parameters are in half-turns (1.0 = 180 degrees).
// Z^z = diag(1, e^{iπz}), P^a = diag(1, e^{iπa}),
// X^x has matrix [[cos(πx/2), i·sin(πx/2)], [i·sin(πx/2), cos(πx/2)]].
func PhasedXZ(xExp, zExp, axisPhaseExp float64) Gate {
	cx := math.Cos(math.Pi * xExp / 2)
	sx := math.Sin(math.Pi * xExp / 2)
	ez := cmplx.Exp(complex(0, math.Pi*zExp))
	ea := cmplx.Exp(complex(0, math.Pi*axisPhaseExp))
	return &parameterized{
		name:   fmt.Sprintf("PhasedXZ(%.4f,%.4f,%.4f)", xExp, zExp, axisPhaseExp),
		n:      1,
		params: []float64{xExp, zExp, axisPhaseExp},
		matrix: []complex128{
			complex(cx, 0),
			conj(ea) * complex(0, sx),
			ez * ea * complex(0, sx),
			ez * complex(cx, 0),
		},
	}
}

// GlobalPhase returns a gate applying scalar phase e^(iφ) to a single qubit.
//
//	e^(iφ)·I = [[e^(iφ), 0], [0, e^(iφ)]]
func GlobalPhase(phi float64) Gate {
	ep := cmplx.Exp(complex(0, phi))
	return &parameterized{
		name:   fmt.Sprintf("GlobalPhase(%.4f)", phi),
		n:      1,
		params: []float64{phi},
		matrix: []complex128{
			ep, 0,
			0, ep,
		},
	}
}

// FSim returns the fermionic simulation gate. It models the interaction
// between two fermionic modes with a hopping term (theta) and a conditional
// phase (phi). FSim is the native interaction on superconducting processors
// with tunable couplers (Google Sycamore). Special cases: FSim(pi/2, 0) =
// iSWAP; FSim(pi/2, pi/6) = Sycamore gate; FSim(0, phi) = CP(-phi).
//
//	[[1, 0, 0, 0],
//	 [0, cos θ, -i·sin θ, 0],
//	 [0, -i·sin θ, cos θ, 0],
//	 [0, 0, 0, exp(-iφ)]]
func FSim(theta, phi float64) Gate {
	ct, st := math.Cos(theta), math.Sin(theta)
	return &parameterized{
		name:   fmt.Sprintf("FSim(%.4f,%.4f)", theta, phi),
		n:      2,
		params: []float64{theta, phi},
		matrix: []complex128{
			1, 0, 0, 0,
			0, complex(ct, 0), complex(0, -st), 0,
			0, complex(0, -st), complex(ct, 0), 0,
			0, 0, 0, cmplx.Exp(complex(0, -phi)),
		},
	}
}

// PSwap returns the parameterized SWAP gate.
//
//	[[1, 0, 0, 0],
//	 [0, 0, exp(iφ), 0],
//	 [0, exp(iφ), 0, 0],
//	 [0, 0, 0, 1]]
func PSwap(phi float64) Gate {
	ep := cmplx.Exp(complex(0, phi))
	return &parameterized{
		name:   fmt.Sprintf("PSwap(%.4f)", phi),
		n:      2,
		params: []float64{phi},
		matrix: []complex128{
			1, 0, 0, 0,
			0, 0, ep, 0,
			0, ep, 0, 0,
			0, 0, 0, 1,
		},
	}
}
