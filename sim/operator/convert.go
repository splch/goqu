package operator

import (
	"math"
	"math/cmplx"
)

// KrausToSuperOp converts a Kraus representation to a superoperator.
// S = sum_k (E_k (x) conj(E_k)).
func KrausToSuperOp(k *Kraus) *SuperOp {
	dim := 1 << k.nq
	d2 := dim * dim
	result := make([]complex128, d2*d2)
	for _, e := range k.operators {
		tc := tensorConj(e, dim)
		for i := range result {
			result[i] += tc[i]
		}
	}
	return &SuperOp{nq: k.nq, matrix: result}
}

// KrausToChoi converts a Kraus representation to a Choi matrix.
// Lambda = sum_k vec(E_k) * vec(E_k)-dagger
// where vec(E) stacks columns: vec[j*dim+i] = E[i,j].
func KrausToChoi(k *Kraus) *Choi {
	dim := 1 << k.nq
	d2 := dim * dim
	result := make([]complex128, d2*d2)
	for _, e := range k.operators {
		vop := vecOuterProduct(e, dim)
		for i := range result {
			result[i] += vop[i]
		}
	}
	return &Choi{nq: k.nq, matrix: result}
}

// SuperOpToChoi converts a superoperator to a Choi matrix.
// The superoperator has S[a*dim+b, c*dim+d] = E[a,c] * conj(E[b,d]).
// The Choi matrix has Lambda[c*dim+a, d*dim+b] = E[a,c] * conj(E[b,d]).
// Reshuffling: S[a*dim+b, c*dim+d] -> Lambda[c*dim+a, d*dim+b].
func SuperOpToChoi(s *SuperOp) *Choi {
	dim := 1 << s.nq
	d2 := dim * dim
	result := make([]complex128, d2*d2)
	for a := range dim {
		for b := range dim {
			for c := range dim {
				for d := range dim {
					sIdx := (a*dim+b)*d2 + (c*dim + d)
					cIdx := (c*dim+a)*d2 + (d*dim + b)
					result[cIdx] = s.matrix[sIdx]
				}
			}
		}
	}
	return &Choi{nq: s.nq, matrix: result}
}

// ChoiToSuperOp converts a Choi matrix to a superoperator.
// Inverse reshuffling: Lambda[c*dim+a, d*dim+b] -> S[a*dim+b, c*dim+d].
func ChoiToSuperOp(c *Choi) *SuperOp {
	dim := 1 << c.nq
	d2 := dim * dim
	result := make([]complex128, d2*d2)
	for a := range dim {
		for b := range dim {
			for cc := range dim {
				for d := range dim {
					cIdx := (cc*dim+a)*d2 + (d*dim + b)
					sIdx := (a*dim+b)*d2 + (cc*dim + d)
					result[sIdx] = c.matrix[cIdx]
				}
			}
		}
	}
	return &SuperOp{nq: c.nq, matrix: result}
}

// ChoiToKraus converts a Choi matrix to Kraus operators.
// Eigendecomposes the Choi matrix, then reshapes eigenvectors
// corresponding to positive eigenvalues into Kraus operators.
func ChoiToKraus(c *Choi) *Kraus {
	dim := 1 << c.nq
	d2 := dim * dim

	evals, evecs := hermitianEig(c.matrix, d2)

	var operators [][]complex128
	for idx, ev := range evals {
		if ev < 1e-12 {
			continue
		}
		scale := complex(math.Sqrt(ev), 0)
		// Extract eigenvector (column idx of evecs matrix)
		vec := make([]complex128, d2)
		for r := range d2 {
			vec[r] = evecs[r*d2+idx]
		}
		// Reshape vec to dim x dim operator.
		// vec[j*dim+i] = E[i,j], so E[i,j] = scale * vec[j*dim+i].
		op := make([]complex128, d2)
		for i := range dim {
			for j := range dim {
				op[i*dim+j] = scale * vec[j*dim+i]
			}
		}
		operators = append(operators, op)
	}
	if len(operators) == 0 {
		// Degenerate case: return zero operator.
		operators = [][]complex128{make([]complex128, d2)}
	}
	return &Kraus{nq: c.nq, operators: operators}
}

// tensorConj computes E (x) conj(E) for a dim x dim matrix E.
// Result is d2 x d2 where d2 = dim*dim.
// (E (x) conj(E))[ab,cd] = E[a,c] * conj(E[b,d])
func tensorConj(e []complex128, dim int) []complex128 {
	d2 := dim * dim
	result := make([]complex128, d2*d2)
	for a := range dim {
		for b := range dim {
			for c := range dim {
				for d := range dim {
					row := a*dim + b
					col := c*dim + d
					result[row*d2+col] = e[a*dim+c] * cmplx.Conj(e[b*dim+d])
				}
			}
		}
	}
	return result
}

// vecOuterProduct computes vec(E) * vec(E)-dagger.
// vec(E) stacks columns: vec[j*dim+i] = E[i,j].
func vecOuterProduct(e []complex128, dim int) []complex128 {
	d2 := dim * dim
	vec := make([]complex128, d2)
	for j := range dim {
		for i := range dim {
			vec[j*dim+i] = e[i*dim+j]
		}
	}
	result := make([]complex128, d2*d2)
	for i := range d2 {
		for j := range d2 {
			result[i*d2+j] = vec[i] * cmplx.Conj(vec[j])
		}
	}
	return result
}

// matMul multiplies two n x n flat row-major complex matrices.
func matMul(a, b []complex128, n int) []complex128 {
	result := make([]complex128, n*n)
	for i := range n {
		for k := range n {
			aik := a[i*n+k]
			if aik == 0 {
				continue
			}
			for j := range n {
				result[i*n+j] += aik * b[k*n+j]
			}
		}
	}
	return result
}

// adjoint returns the conjugate transpose of an n x n flat row-major matrix.
func adjoint(m []complex128, n int) []complex128 {
	result := make([]complex128, n*n)
	for i := range n {
		for j := range n {
			result[j*n+i] = cmplx.Conj(m[i*n+j])
		}
	}
	return result
}

// trace returns the trace of an n x n flat row-major complex matrix.
func trace(m []complex128, n int) complex128 {
	var t complex128
	for i := range n {
		t += m[i*n+i]
	}
	return t
}

// identityMatrix returns an n x n identity matrix flat row-major.
func identityMatrix(n int) []complex128 {
	m := make([]complex128, n*n)
	for i := range n {
		m[i*n+i] = 1
	}
	return m
}

// matClose returns true if two n x n matrices are element-wise close within tol.
func matClose(a, b []complex128, tol float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if cmplx.Abs(a[i]-b[i]) > tol {
			return false
		}
	}
	return true
}
