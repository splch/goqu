package operator

import "github.com/splch/qgo/sim/noise"

// Kraus represents a quantum channel via Kraus operators.
// Each operator is a dim x dim flat row-major complex matrix where dim = 2^nq.
type Kraus struct {
	nq        int
	operators [][]complex128
}

// SuperOp represents a quantum channel as a superoperator matrix.
// S = sum_k (E_k (x) conj(E_k)), acts on vec(rho).
// The matrix is dim^2 x dim^2 flat row-major.
type SuperOp struct {
	nq     int
	matrix []complex128
}

// Choi represents a quantum channel via its Choi matrix (Choi-Jamiolkowski).
// Lambda = sum_k vec(E_k) * vec(E_k)-dagger.
// The matrix is dim^2 x dim^2 flat row-major.
type Choi struct {
	nq     int
	matrix []complex128
}

// PTM represents a quantum channel as a Pauli transfer matrix.
// R_ij = Tr(sigma_i * E(sigma_j)) / d.
// The matrix is dim^2 x dim^2 flat row-major real.
type PTM struct {
	nq     int
	matrix []float64
}

// NewKraus creates a Kraus representation from the given operators.
// Each operator must be a dim x dim flat row-major matrix where dim = 2^nq.
func NewKraus(nq int, operators [][]complex128) *Kraus {
	dim := 1 << nq
	d2 := dim * dim
	ops := make([][]complex128, len(operators))
	for i, op := range operators {
		if len(op) != d2 {
			panic("operator.NewKraus: operator size mismatch")
		}
		ops[i] = make([]complex128, d2)
		copy(ops[i], op)
	}
	return &Kraus{nq: nq, operators: ops}
}

// NewSuperOp creates a SuperOp from a dim^2 x dim^2 flat row-major matrix.
func NewSuperOp(nq int, matrix []complex128) *SuperOp {
	dim := 1 << nq
	d2 := dim * dim
	if len(matrix) != d2*d2 {
		panic("operator.NewSuperOp: matrix size mismatch")
	}
	m := make([]complex128, len(matrix))
	copy(m, matrix)
	return &SuperOp{nq: nq, matrix: m}
}

// NewChoi creates a Choi from a dim^2 x dim^2 flat row-major matrix.
func NewChoi(nq int, matrix []complex128) *Choi {
	dim := 1 << nq
	d2 := dim * dim
	if len(matrix) != d2*d2 {
		panic("operator.NewChoi: matrix size mismatch")
	}
	m := make([]complex128, len(matrix))
	copy(m, matrix)
	return &Choi{nq: nq, matrix: m}
}

// NewPTM creates a PTM from a dim^2 x dim^2 flat row-major real matrix.
func NewPTM(nq int, matrix []float64) *PTM {
	dim := 1 << nq
	d2 := dim * dim
	if len(matrix) != d2*d2 {
		panic("operator.NewPTM: matrix size mismatch")
	}
	m := make([]float64, len(matrix))
	copy(m, matrix)
	return &PTM{nq: nq, matrix: m}
}

// FromChannel bridges an existing noise.Channel to a Kraus representation.
func FromChannel(ch noise.Channel) *Kraus {
	return NewKraus(ch.Qubits(), ch.Kraus())
}

// NumQubits returns the number of qubits the channel acts on.
func (k *Kraus) NumQubits() int { return k.nq }

// Operators returns a defensive copy of the Kraus operators.
func (k *Kraus) Operators() [][]complex128 {
	ops := make([][]complex128, len(k.operators))
	for i, op := range k.operators {
		ops[i] = make([]complex128, len(op))
		copy(ops[i], op)
	}
	return ops
}

// NumQubits returns the number of qubits the channel acts on.
func (s *SuperOp) NumQubits() int { return s.nq }

// Matrix returns a defensive copy of the superoperator matrix.
func (s *SuperOp) Matrix() []complex128 {
	m := make([]complex128, len(s.matrix))
	copy(m, s.matrix)
	return m
}

// NumQubits returns the number of qubits the channel acts on.
func (c *Choi) NumQubits() int { return c.nq }

// Matrix returns a defensive copy of the Choi matrix.
func (c *Choi) Matrix() []complex128 {
	m := make([]complex128, len(c.matrix))
	copy(m, c.matrix)
	return m
}

// NumQubits returns the number of qubits the channel acts on.
func (p *PTM) NumQubits() int { return p.nq }

// Matrix returns a defensive copy of the PTM matrix.
func (p *PTM) Matrix() []float64 {
	m := make([]float64, len(p.matrix))
	copy(m, p.matrix)
	return m
}
