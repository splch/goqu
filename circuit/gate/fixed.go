package gate

import "math"

// fixed is a non-parameterized gate with a precomputed matrix.
type fixed struct {
	name   string
	n      int
	matrix []complex128
}

func (g *fixed) Name() string        { return g.name }
func (g *fixed) Qubits() int         { return g.n }
func (g *fixed) Matrix() []complex128 { return g.matrix }
func (g *fixed) Params() []float64   { return nil }

func (g *fixed) Inverse() Gate {
	// Compute conjugate transpose.
	dim := 1 << g.n
	inv := make([]complex128, dim*dim)
	for r := range dim {
		for c := range dim {
			// adjoint: swap indices and conjugate
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
	I = &fixed{name: "I", n: 1, matrix: []complex128{
		1, 0,
		0, 1,
	}}

	H = &fixed{name: "H", n: 1, matrix: []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}}

	X = &fixed{name: "X", n: 1, matrix: []complex128{
		0, 1,
		1, 0,
	}}

	Y = &fixed{name: "Y", n: 1, matrix: []complex128{
		0, -1i,
		1i, 0,
	}}

	Z = &fixed{name: "Z", n: 1, matrix: []complex128{
		1, 0,
		0, -1,
	}}

	S = &fixed{name: "S", n: 1, matrix: []complex128{
		1, 0,
		0, 1i,
	}}

	Sdg = &fixed{name: "S†", n: 1, matrix: []complex128{
		1, 0,
		0, -1i,
	}}

	T = &fixed{name: "T", n: 1, matrix: []complex128{
		1, 0,
		0, complex(s2, s2),
	}}

	Tdg = &fixed{name: "T†", n: 1, matrix: []complex128{
		1, 0,
		0, complex(s2, -s2),
	}}

	SX = &fixed{name: "SX", n: 1, matrix: []complex128{
		complex(0.5, 0.5), complex(0.5, -0.5),
		complex(0.5, -0.5), complex(0.5, 0.5),
	}}
)

// Standard two-qubit gates.
var (
	CNOT = &fixed{name: "CNOT", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 0, 1,
		0, 0, 1, 0,
	}}

	CZ = &fixed{name: "CZ", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, -1,
	}}

	SWAP = &fixed{name: "SWAP", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 0, 1, 0,
		0, 1, 0, 0,
		0, 0, 0, 1,
	}}

	CY = &fixed{name: "CY", n: 2, matrix: []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 0, -1i,
		0, 0, 1i, 0,
	}}
)

// Standard three-qubit gates.
var (
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
)
