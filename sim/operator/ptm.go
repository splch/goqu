package operator

import "math"

// Single-qubit Pauli matrices (flat 2x2 row-major).
// Defined locally to avoid coupling with sim/pauli.
var paulis1Q = [4][]complex128{
	{1, 0, 0, 1},    // I
	{0, 1, 1, 0},    // X
	{0, -1i, 1i, 0}, // Y
	{1, 0, 0, -1},   // Z
}

// KrausToPTM converts a Kraus representation to a Pauli transfer matrix.
// R_ij = Tr(sigma_i * E(sigma_j)) / d
// where E(sigma_j) = sum_k E_k * sigma_j * E_k-dagger.
func KrausToPTM(k *Kraus) *PTM {
	dim := 1 << k.nq
	d2 := dim * dim

	// Build the Pauli basis for nq qubits.
	basis := pauliBasis(k.nq)

	result := make([]float64, d2*d2)
	for j := range d2 {
		// Compute E(sigma_j) = sum_k E_k * sigma_j * E_k-dagger
		eSigJ := applyChannel(k.operators, basis[j], dim)
		for i := range d2 {
			// R_ij = Tr(sigma_i * E(sigma_j)) / d
			prod := matMul(basis[i], eSigJ, dim)
			tr := trace(prod, dim)
			result[i*d2+j] = real(tr) / float64(dim)
		}
	}
	return &PTM{nq: k.nq, matrix: result}
}

// PTMToKraus converts a PTM to Kraus operators via Choi decomposition.
// First converts PTM -> Choi, then Choi -> Kraus.
func PTMToKraus(p *PTM) *Kraus {
	return ChoiToKraus(PTMToChoi(p))
}

// PTMToChoi converts a PTM to a Choi matrix.
// Lambda = (1/d) * sum_ij R_ij * (sigma_j^T (x) sigma_i)
// Note: we use the relation that the Choi matrix can be reconstructed from the PTM
// via the Pauli basis.
func PTMToChoi(p *PTM) *Choi {
	dim := 1 << p.nq
	d2 := dim * dim

	basis := pauliBasis(p.nq)
	result := make([]complex128, d2*d2)

	scale := 1.0 / float64(dim)

	for i := range d2 {
		for j := range d2 {
			rij := p.matrix[i*d2+j]
			if math.Abs(rij) < 1e-15 {
				continue
			}
			// sigma_j^T (x) sigma_i
			sigJT := transpose(basis[j], dim)
			tp := tensorProduct(sigJT, basis[i], dim)
			coeff := complex(scale*rij, 0)
			for k := range result {
				result[k] += coeff * tp[k]
			}
		}
	}
	return &Choi{nq: p.nq, matrix: result}
}

// ChoiToPTM converts a Choi matrix to a PTM.
func ChoiToPTM(c *Choi) *PTM {
	return KrausToPTM(ChoiToKraus(c))
}

// SuperOpToPTM converts a superoperator to a PTM.
func SuperOpToPTM(s *SuperOp) *PTM {
	return KrausToPTM(ChoiToKraus(SuperOpToChoi(s)))
}

// pauliBasis generates the d^2 Pauli basis matrices for nq qubits.
// Each is a dim x dim flat row-major matrix.
// The basis is ordered by tensor product indices.
func pauliBasis(nq int) [][]complex128 {
	if nq == 0 {
		return [][]complex128{{1}}
	}
	// Start with single-qubit Paulis.
	basis := make([][]complex128, 4)
	for i := range 4 {
		basis[i] = make([]complex128, 4)
		copy(basis[i], paulis1Q[i])
	}
	// Tensor product for additional qubits.
	for q := 1; q < nq; q++ {
		prevDim := 1 << q
		newDim := prevDim * 2
		newBasis := make([][]complex128, len(basis)*4)
		for i, prev := range basis {
			for j := range 4 {
				newBasis[i*4+j] = tensorProduct(prev, paulis1Q[j], prevDim)
				_ = newDim // dimension of result
			}
		}
		basis = newBasis
	}
	return basis
}

// tensorProduct computes the tensor product of an n x n matrix and an m x m matrix.
// Both are flat row-major. The second matrix uses the locally-known Pauli size.
func tensorProduct(a []complex128, b []complex128, dimA int) []complex128 {
	dimB := isqrt(len(b))
	dimR := dimA * dimB
	result := make([]complex128, dimR*dimR)
	for i := range dimA {
		for j := range dimA {
			aij := a[i*dimA+j]
			if aij == 0 {
				continue
			}
			for k := range dimB {
				for l := range dimB {
					row := i*dimB + k
					col := j*dimB + l
					result[row*dimR+col] = aij * b[k*dimB+l]
				}
			}
		}
	}
	return result
}

// transpose returns the transpose of an n x n flat row-major matrix.
func transpose(m []complex128, n int) []complex128 {
	result := make([]complex128, n*n)
	for i := range n {
		for j := range n {
			result[j*n+i] = m[i*n+j]
		}
	}
	return result
}

// applyChannel computes E(rho) = sum_k E_k * rho * E_k-dagger.
func applyChannel(operators [][]complex128, rho []complex128, dim int) []complex128 {
	result := make([]complex128, dim*dim)
	for _, ek := range operators {
		ekd := adjoint(ek, dim)
		tmp := matMul(ek, rho, dim)
		prod := matMul(tmp, ekd, dim)
		for i := range result {
			result[i] += prod[i]
		}
	}
	return result
}

// isqrt returns the integer square root of n.
func isqrt(n int) int {
	r := int(math.Sqrt(float64(n)))
	if r*r == n {
		return r
	}
	// Try neighbors.
	if (r+1)*(r+1) == n {
		return r + 1
	}
	return r
}
