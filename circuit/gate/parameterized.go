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

// RX returns an X-rotation gate: exp(-i * theta/2 * X).
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

// RY returns a Y-rotation gate: exp(-i * theta/2 * Y).
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

// RZ returns a Z-rotation gate: exp(-i * theta/2 * Z).
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

// U3 returns the universal single-qubit gate U(θ, φ, λ).
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

// CP returns a controlled-phase gate: diag(1, 1, 1, exp(iφ)).
func CP(phi float64) Gate {
	return &parameterized{
		name:   fmt.Sprintf("CP(%.4f)", phi),
		n:      2,
		params: []float64{phi},
		matrix: []complex128{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, 1, 0,
			0, 0, 0, cmplx.Exp(complex(0, phi)),
		},
	}
}

// CRZ returns a controlled-RZ gate.
func CRZ(theta float64) Gate {
	return &parameterized{
		name:   fmt.Sprintf("CRZ(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, cmplx.Exp(complex(0, -theta/2)), 0,
			0, 0, 0, cmplx.Exp(complex(0, theta/2)),
		},
	}
}

// CRX returns a controlled-RX gate.
func CRX(theta float64) Gate {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	return &parameterized{
		name:   fmt.Sprintf("CRX(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, complex(c, 0), complex(0, -s),
			0, 0, complex(0, -s), complex(c, 0),
		},
	}
}

// CRY returns a controlled-RY gate.
func CRY(theta float64) Gate {
	c, s := math.Cos(theta/2), math.Sin(theta/2)
	return &parameterized{
		name:   fmt.Sprintf("CRY(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, complex(c, 0), complex(-s, 0),
			0, 0, complex(s, 0), complex(c, 0),
		},
	}
}

// RXX returns the Ising XX gate: exp(-i * theta/2 * X⊗X).
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

// RYY returns the Ising YY gate: exp(-i * theta/2 * Y⊗Y).
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

// RZZ returns the Ising ZZ gate: exp(-i * theta/2 * Z⊗Z).
//
//	[[exp(-iθ/2), 0, 0, 0],
//	 [0, exp(iθ/2), 0, 0],
//	 [0, 0, exp(iθ/2), 0],
//	 [0, 0, 0, exp(-iθ/2)]]
func RZZ(theta float64) Gate {
	em := cmplx.Exp(complex(0, -theta/2))
	ep := cmplx.Exp(complex(0, theta/2))
	return &parameterized{
		name:   fmt.Sprintf("RZZ(%.4f)", theta),
		n:      2,
		params: []float64{theta},
		matrix: []complex128{
			em, 0, 0, 0,
			0, ep, 0, 0,
			0, 0, ep, 0,
			0, 0, 0, em,
		},
	}
}

// GPI returns an IonQ native GPI gate.
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

// GPI2 returns an IonQ native GPI2 gate.
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

// NOP returns an IonQ native NOP (no-operation) timing gate.
// It is a 0-qubit instruction analogous to Delay; time is in microseconds.
func NOP(time float64) Gate {
	return &parameterized{
		name:   fmt.Sprintf("NOP(%.4f)", time),
		n:      0,
		params: []float64{time},
		matrix: []complex128{1}, // 2^0 × 2^0 identity
	}
}

// MS returns an IonQ native Mølmer-Sørensen gate.
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

// U1 returns a phase gate (Qiskit compatibility alias for Phase).
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

// U2 returns the single-qubit gate U3(π/2, φ, λ).
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

// Rot returns the PennyLane-style rotation gate RZ(ω)·RY(θ)·RZ(φ).
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
// Parameters are in half-turns. Z^z = diag(1, e^{iπz}), P^a = diag(1, e^{iπa}),
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

// FSim returns the fermionic simulation gate.
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
