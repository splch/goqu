package pauli

import "math/bits"

// ExpectFromCounts estimates the Z-basis expectation value of a Z-product
// observable from measurement counts. qubits specifies which qubits are
// measured in the Z basis; the expectation is (-1)^(parity of specified
// qubits) averaged over all shots.
//
// For X or Y observables, the caller must apply basis-change rotations
// (H for X, S†H for Y) before measurement.
func ExpectFromCounts(counts map[string]int, qubits []int) float64 {
	var mask int
	for _, q := range qubits {
		mask |= 1 << q
	}

	var total, sum float64
	for bs, count := range counts {
		// Convert bitstring to integer index.
		// Bitstring convention: leftmost character = highest qubit.
		idx := 0
		n := len(bs)
		for i, ch := range bs {
			if ch == '1' {
				idx |= 1 << (n - 1 - i)
			}
		}
		fc := float64(count)
		total += fc
		if bits.OnesCount(uint(idx&mask))%2 == 0 {
			sum += fc
		} else {
			sum -= fc
		}
	}
	if total == 0 {
		return 0
	}
	return sum / total
}
