package gate

import (
	"math"
	"math/cmplx"
)

// fixed is a non-parameterized gate with a precomputed matrix.
type fixed struct {
	name   string
	n      int
	matrix []complex128
}

func (g *fixed) Name() string         { return g.name }
func (g *fixed) Qubits() int          { return g.n }
func (g *fixed) Matrix() []complex128 { return g.matrix }
func (g *fixed) Params() []float64    { return nil }

func (g *fixed) Inverse() Gate {
	// Self-adjoint gates return themselves.
	switch g.name {
	case "I", "H", "X", "Y", "Z", "CNOT", "CZ", "SWAP", "CCX", "CSWAP", "ECR", "CCZ":
		return g
	case "S":
		return Sdg
	case "S†":
		return S
	case "T":
		return Tdg
	case "T†":
		return T
	}
	// General case: compute conjugate transpose.
	dim := 1 << g.n
	inv := make([]complex128, dim*dim)
	for r := range dim {
		for c := range dim {
			inv[r*dim+c] = conj(g.matrix[c*dim+r])
		}
	}
	return &fixed{name: g.name + "†", n: g.n, matrix: inv}
}

func (g *fixed) Decompose(_ []int) []Applied { return nil }

func conj(c complex128) complex128 {
	return complex(real(c), -imag(c))
}

var s2 = 1.0 / math.Sqrt2

// Standard single-qubit gates.
var (
	// I is the single-qubit identity gate. It leaves the qubit state unchanged
	// and is used as a placeholder or for padding in circuit layouts.
	//
	//   [[1, 0],
	//    [0, 1]]
	I = &fixed{name: "I", n: 1, matrix: []complex128{
		1, 0,
		0, 1,
	}}

	// H is the Hadamard gate. It maps |0> to (|0>+|1>)/sqrt(2) and |1> to
	// (|0>-|1>)/sqrt(2), creating an equal superposition from a basis state.
	// It is its own inverse (H^2 = I) and is the starting point for most
	// quantum algorithms. On the Bloch sphere, H is a 180-degree rotation
	// around the axis halfway between X and Z.
	//
	//   (1/sqrt(2)) * [[1,  1],
	//                   [1, -1]]
	H = &fixed{name: "H", n: 1, matrix: []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}}

	// X is the Pauli-X gate (quantum NOT / bit-flip). It swaps |0> and |1>,
	// equivalent to a 180-degree rotation around the X axis of the Bloch sphere.
	// Self-inverse: X^2 = I. Together with Y and Z, the Pauli gates form a
	// basis for all single-qubit operations.
	//
	//   [[0, 1],
	//    [1, 0]]
	X = &fixed{name: "X", n: 1, matrix: []complex128{
		0, 1,
		1, 0,
	}}

	// Y is the Pauli-Y gate. It flips |0> to i|1> and |1> to -i|0>,
	// combining a bit-flip with a phase-flip. Equivalent to a 180-degree
	// rotation around the Y axis of the Bloch sphere. Self-inverse: Y^2 = I.
	//
	//   [[0, -i],
	//    [i,  0]]
	Y = &fixed{name: "Y", n: 1, matrix: []complex128{
		0, -1i,
		1i, 0,
	}}

	// Z is the Pauli-Z gate (phase-flip). It leaves |0> unchanged and maps
	// |1> to -|1>, equivalent to a 180-degree rotation around the Z axis of
	// the Bloch sphere. Self-inverse: Z^2 = I.
	//
	//   [[1,  0],
	//    [0, -1]]
	Z = &fixed{name: "Z", n: 1, matrix: []complex128{
		1, 0,
		0, -1,
	}}

	// S is the S gate (sqrt-Z / phase gate / P(pi/2)). It applies a
	// quarter-turn (90 degrees) around the Z axis: |0> -> |0>, |1> -> i|1>.
	// Two applications give Z: S^2 = Z. Inverse: [Sdg].
	//
	//   [[1, 0],
	//    [0, i]]
	S = &fixed{name: "S", n: 1, matrix: []complex128{
		1, 0,
		0, 1i,
	}}

	// Sdg is the S-dagger gate (inverse of [S]). It applies a -90 degree
	// rotation around the Z axis: |0> -> |0>, |1> -> -i|1>.
	//
	//   [[1,  0],
	//    [0, -i]]
	Sdg = &fixed{name: "S†", n: 1, matrix: []complex128{
		1, 0,
		0, -1i,
	}}

	// T is the T gate (pi/8 gate / fourth-root of Z). It applies a pi/4
	// (45-degree) phase: |0> -> |0>, |1> -> exp(i*pi/4)|1>. The T gate is
	// essential for universal quantum computation — the Clifford+T gate set
	// can approximate any unitary to arbitrary precision (Solovay-Kitaev).
	// Inverse: [Tdg]. T^2 = S, T^4 = Z.
	//
	//   [[1, 0],
	//    [0, exp(i*pi/4)]]
	T = &fixed{name: "T", n: 1, matrix: []complex128{
		1, 0,
		0, complex(s2, s2),
	}}

	// Tdg is the T-dagger gate (inverse of [T]). It applies a -pi/4 phase.
	//
	//   [[1, 0],
	//    [0, exp(-i*pi/4)]]
	Tdg = &fixed{name: "T†", n: 1, matrix: []complex128{
		1, 0,
		0, complex(s2, -s2),
	}}

	// SX is the sqrt-X gate. It is the square root of the Pauli-X gate:
	// SX^2 = X. IBM quantum processors use SX as a native gate (alongside
	// CNOT and RZ), so many transpilation flows decompose into {SX, RZ, CNOT}.
	//
	//   (1/2) * [[1+i, 1-i],
	//             [1-i, 1+i]]
	SX = &fixed{name: "SX", n: 1, matrix: []complex128{
		complex(0.5, 0.5), complex(0.5, -0.5),
		complex(0.5, -0.5), complex(0.5, 0.5),
	}}
)

// Standard two-qubit gates.
//
// Convention: q0 is the control (or first operand), q1 is the target (or
// second operand). The 4x4 matrix is indexed as |q0,q1> where q0 is the
// MSB: |00>=0, |01>=1, |10>=2, |11>=3.
var (
	// CNOT (Controlled-NOT / CX) flips the target qubit if and only if the
	// control qubit is |1>. It is the standard entangling gate: applying
	// H(q0) then CNOT(q0,q1) to |00> produces the Bell state
	// (|00>+|11>)/sqrt(2). Self-inverse: CNOT^2 = I. Most hardware platforms
	// use CNOT (or an equivalent like CZ) as their native two-qubit gate.
	//
	//   q0: ──●──     [[1, 0, 0, 0],
	//         |        [0, 1, 0, 0],
	//   q1: ──X──      [0, 0, 0, 1],
	//                   [0, 0, 1, 0]]
	CNOT = &fixed{name: "CNOT", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 0, 1,
		0, 0, 1, 0,
	}}

	// CZ (Controlled-Z) applies a Z gate to the target when the control is |1>,
	// equivalently flipping the phase of |11> to -|11>. Unlike CNOT, CZ is
	// symmetric: CZ(q0,q1) = CZ(q1,q0). Self-inverse. Native gate on many
	// superconducting platforms (Google, Rigetti).
	//
	//   q0: ──●──     [[1, 0, 0,  0],
	//         |        [0, 1, 0,  0],
	//   q1: ──Z──      [0, 0, 1,  0],
	//                   [0, 0, 0, -1]]
	CZ = &fixed{name: "CZ", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, -1,
	}}

	// SWAP exchanges the states of two qubits: |01> <-> |10>. Equivalent to
	// three CNOTs: CNOT(0,1) CNOT(1,0) CNOT(0,1). Self-inverse. The
	// transpiler inserts SWAPs to route qubits on hardware with limited
	// connectivity.
	//
	//   q0: ──X──     [[1, 0, 0, 0],
	//         |        [0, 0, 1, 0],
	//   q1: ──X──      [0, 1, 0, 0],
	//                   [0, 0, 0, 1]]
	SWAP = &fixed{name: "SWAP", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 0, 1, 0,
		0, 1, 0, 0,
		0, 0, 0, 1,
	}}

	// CY (Controlled-Y) applies a Pauli-Y gate to the target when the control
	// is |1>. Maps |10> to i|11> and |11> to -i|10>.
	//
	//   [[1, 0, 0,   0],
	//    [0, 1, 0,   0],
	//    [0, 0, 0, -i ],
	//    [0, 0, i,   0]]
	CY = &fixed{name: "CY", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 0, -1i,
		0, 0, 1i, 0,
	}}

	// ISWAP (imaginary SWAP) swaps |01> and |10> with a factor of i:
	// |01> -> i|10>, |10> -> i|01>. It is the native two-qubit gate on
	// Google's superconducting processors (with the Sycamore gate being
	// ISWAP + conditional phase). ISWAP^2 = SWAP (up to global phase).
	//
	//   [[1, 0,  0, 0],
	//    [0, 0, i,  0],
	//    [0, i,  0, 0],
	//    [0, 0,  0, 1]]
	ISWAP = &fixed{name: "iSWAP", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 0, 1i, 0,
		0, 1i, 0, 0,
		0, 0, 0, 1,
	}}

	// ECR (Echoed Cross-Resonance) is IBM's native two-qubit gate on Eagle
	// and newer processors. It is equivalent to a CNOT up to single-qubit
	// rotations and is self-inverse. The cross-resonance interaction drives
	// one qubit at the frequency of another, creating an entangling ZX
	// interaction.
	//
	//   (1/sqrt(2)) * [[0, 0,  1,  i],
	//                   [0, 0,  i,  1],
	//                   [1, -i, 0,  0],
	//                   [-i, 1, 0,  0]]
	ECR = &fixed{name: "ECR", n: 2, matrix: []complex128{
		0, 0, complex(s2, 0), complex(0, s2),
		0, 0, complex(0, s2), complex(s2, 0),
		complex(s2, 0), complex(0, -s2), 0, 0,
		complex(0, -s2), complex(s2, 0), 0, 0,
	}}

	// DCX (Double-CNOT) applies CNOT(q0,q1) followed by CNOT(q1,q0).
	// It is a Clifford gate that swaps |01> -> |11> -> |10> -> |01>
	// (a 3-cycle on the computational basis states).
	//
	//   [[1, 0, 0, 0],
	//    [0, 0, 0, 1],
	//    [0, 1, 0, 0],
	//    [0, 0, 1, 0]]
	DCX = &fixed{name: "DCX", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 0, 0, 1,
		0, 1, 0, 0,
		0, 0, 1, 0,
	}}

	// CH (Controlled-Hadamard) applies a Hadamard gate to the target when
	// the control is |1>. Useful in algorithms that create conditional
	// superposition.
	//
	//   [[1, 0, 0,           0          ],
	//    [0, 1, 0,           0          ],
	//    [0, 0, 1/sqrt(2),   1/sqrt(2)  ],
	//    [0, 0, 1/sqrt(2),  -1/sqrt(2)  ]]
	CH = &fixed{name: "CH", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, complex(s2, 0), complex(s2, 0),
		0, 0, complex(s2, 0), complex(-s2, 0),
	}}

	// CSX (Controlled-sqrt-X) applies a sqrt-X ([SX]) gate to the target
	// when the control is |1>.
	//
	//   [[1, 0, 0,          0         ],
	//    [0, 1, 0,          0         ],
	//    [0, 0, (1+i)/2,   (1-i)/2    ],
	//    [0, 0, (1-i)/2,   (1+i)/2    ]]
	CSX = &fixed{name: "CSX", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, complex(0.5, 0.5), complex(0.5, -0.5),
		0, 0, complex(0.5, -0.5), complex(0.5, 0.5),
	}}
)

// Standard three-qubit gates.
//
// Convention: q0 and q1 are controls, q2 is the target. The 8x8 matrix is
// indexed as |q0,q1,q2> where q0 is MSB.
var (
	// CCX (Toffoli gate) flips the target qubit if and only if both controls
	// are |1>. It is the quantum analog of the classical AND gate (up to
	// phase) and is universal for reversible classical computation. Decomposed
	// into 6 CNOTs + single-qubit gates for hardware execution. Self-inverse.
	//
	//   q0: ──●──
	//         |
	//   q1: ──●──
	//         |
	//   q2: ──X──
	CCX = &fixed{name: "CCX", n: 3, matrix: []complex128{
		1, 0, 0, 0, 0, 0, 0, 0,
		0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 1, 0, 0, 0, 0, 0,
		0, 0, 0, 1, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 0, 0, 0,
		0, 0, 0, 0, 0, 1, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 1,
		0, 0, 0, 0, 0, 0, 1, 0,
	}}

	// CSWAP (Fredkin gate) swaps q1 and q2 when the control q0 is |1>.
	// It is a universal reversible gate: any classical computation can be
	// built from Fredkin gates alone. Self-inverse.
	//
	//   q0: ──●──
	//         |
	//   q1: ──X──
	//         |
	//   q2: ──X──
	CSWAP = &fixed{name: "CSWAP", n: 3, matrix: []complex128{
		1, 0, 0, 0, 0, 0, 0, 0,
		0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 1, 0, 0, 0, 0, 0,
		0, 0, 0, 1, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 1, 0,
		0, 0, 0, 0, 0, 1, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 1,
	}}

	// CCZ (doubly-controlled Z) flips the phase of |111> to -|111> and leaves
	// all other basis states unchanged. Equivalent to CCX with Hadamards on
	// the target: H * CCX * H = CCZ. Self-inverse. Symmetric in all three
	// qubits.
	CCZ = &fixed{name: "CCZ", n: 3, matrix: []complex128{
		1, 0, 0, 0, 0, 0, 0, 0,
		0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 1, 0, 0, 0, 0, 0,
		0, 0, 0, 1, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 0, 0, 0,
		0, 0, 0, 0, 0, 1, 0, 0,
		0, 0, 0, 0, 0, 0, 1, 0,
		0, 0, 0, 0, 0, 0, 0, -1,
	}}
)

// Special gates.
var (
	// Sycamore is Google's native two-qubit gate, FSim(pi/2, pi/6). It
	// combines an iSWAP-like interaction (|01> <-> -i|10>) with a conditional
	// phase exp(-i*pi/6) on |11>. This gate was used in Google's 2019
	// quantum computational advantage experiment on the Sycamore processor.
	//
	//   [[1,  0,   0,              0            ],
	//    [0,  0,  -i,              0            ],
	//    [0, -i,   0,              0            ],
	//    [0,  0,   0,  exp(-i*pi/6)             ]]
	Sycamore = &fixed{name: "Sycamore", n: 2, matrix: func() []complex128 {
		phi := math.Pi / 6
		return []complex128{
			1, 0, 0, 0,
			0, 0, -1i, 0,
			0, -1i, 0, 0,
			0, 0, 0, cmplx.Exp(complex(0, -phi)),
		}
	}()}
)
