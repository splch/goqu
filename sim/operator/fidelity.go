package operator

// ProcessFidelity computes the entanglement fidelity (process fidelity) of a
// channel relative to the identity channel.
// F_pro = Tr(Choi_identity-dagger * Choi_actual) / d^2
// For the identity channel, Choi_identity[ik,jl] = delta_ij * delta_kl / d,
// which means F_pro = sum_{i,j} Choi_actual[ii, jj] / d^2 = (1/d^2) * sum of
// the "diagonal blocks" diagonal entries.
// Equivalently, F_pro = (1/d) * sum_k |Tr(E_k)|^2 / d.
func ProcessFidelity(actual *Kraus) float64 {
	dim := 1 << actual.nq
	d := float64(dim)

	// F_pro = (1/d^2) * sum_k |Tr(E_k)|^2
	var sum float64
	for _, ek := range actual.operators {
		tr := trace(ek, dim)
		sum += real(tr)*real(tr) + imag(tr)*imag(tr)
	}
	return sum / (d * d)
}

// AverageGateFidelity computes the average gate fidelity of a channel
// relative to the identity channel.
// F_avg = (d * F_pro + 1) / (d + 1)
func AverageGateFidelity(actual *Kraus) float64 {
	dim := 1 << actual.nq
	d := float64(dim)
	fPro := ProcessFidelity(actual)
	return (d*fPro + 1) / (d + 1)
}
