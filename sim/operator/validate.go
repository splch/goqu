package operator

// IsCP checks if a channel (given as Choi matrix) is completely positive.
// A channel is CP if and only if its Choi matrix is positive semidefinite.
// All eigenvalues must be >= -tol.
func IsCP(c *Choi, tol float64) bool {
	dim := 1 << c.nq
	d2 := dim * dim
	evals, _ := hermitianEig(c.matrix, d2)
	for _, ev := range evals {
		if ev < -tol {
			return false
		}
	}
	return true
}

// IsTP checks if a channel (given as Kraus operators) is trace-preserving.
// A channel is TP if sum_k E_k-dagger * E_k = I.
func IsTP(k *Kraus, tol float64) bool {
	dim := 1 << k.nq
	sum := make([]complex128, dim*dim)
	for _, ek := range k.operators {
		ekd := adjoint(ek, dim)
		prod := matMul(ekd, ek, dim)
		for i := range sum {
			sum[i] += prod[i]
		}
	}
	id := identityMatrix(dim)
	return matClose(sum, id, tol)
}

// IsCPTP checks if a channel (given as Kraus operators) is both CP and TP.
func IsCPTP(k *Kraus, tol float64) bool {
	return IsCP(KrausToChoi(k), tol) && IsTP(k, tol)
}
