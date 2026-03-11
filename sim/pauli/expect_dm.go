package pauli

// ExpectDM computes Tr(rho * P) for a PauliString P and density matrix rho.
// The density matrix is stored as a flat []complex128 of length dim*dim where
// dim = 2^numQubits.
//
// The algorithm reads only one element per row of rho: rho[i, i XOR xMask],
// giving O(2^n) complexity instead of O(2^2n).
func ExpectDM(rho []complex128, dim int, ps PauliString) complex128 {
	xm := ps.xMask()
	zm := ps.zMask()
	yPhase := iPow(ps.numY())

	var sum complex128
	for i := range dim {
		j := i ^ xm
		s := sign(i & zm)
		sum += rho[i*dim+j] * complex(s, 0) * yPhase
	}
	return ps.coeff * sum
}

// ExpectSumDM computes Tr(rho * H) for a PauliSum H (Hamiltonian).
func ExpectSumDM(rho []complex128, dim int, ps PauliSum) complex128 {
	var total complex128
	for _, term := range ps.terms {
		total += ExpectDM(rho, dim, term)
	}
	return total
}
