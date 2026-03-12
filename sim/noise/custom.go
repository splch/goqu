package noise

import (
	"fmt"
	"math"
	"math/cmplx"
)

// Custom creates a noise channel from user-provided Kraus operators.
// Each operator must be a flat row-major (2^nQubits)x(2^nQubits) matrix.
// Returns an error if the operators are not trace-preserving
// (i.e. if sum_k E_k-dagger E_k != I within 1e-10 tolerance).
func Custom(name string, nQubits int, kraus [][]complex128) (Channel, error) {
	if nQubits < 1 {
		return nil, fmt.Errorf("noise.Custom: nQubits must be >= 1, got %d", nQubits)
	}
	dim := 1 << nQubits
	expected := dim * dim
	if len(kraus) == 0 {
		return nil, fmt.Errorf("noise.Custom: at least one Kraus operator is required")
	}
	for i, ek := range kraus {
		if len(ek) != expected {
			return nil, fmt.Errorf("noise.Custom: Kraus[%d] length %d, want %d", i, len(ek), expected)
		}
	}
	if err := validateTP(kraus, dim); err != nil {
		return nil, err
	}
	// Defensive copy.
	ops := make([][]complex128, len(kraus))
	for i, ek := range kraus {
		cp := make([]complex128, len(ek))
		copy(cp, ek)
		ops[i] = cp
	}
	return &channel{name: name, nq: nQubits, kraus: ops}, nil
}

// MustCustom is like Custom but panics on error.
func MustCustom(name string, nQubits int, kraus [][]complex128) Channel {
	ch, err := Custom(name, nQubits, kraus)
	if err != nil {
		panic(fmt.Sprintf("noise.MustCustom: %v", err))
	}
	return ch
}

// validateTP checks that sum_k E_k-dagger E_k = I within tolerance.
func validateTP(kraus [][]complex128, dim int) error {
	const tol = 1e-10
	sum := make([]complex128, dim*dim)
	for _, ek := range kraus {
		for i := range dim {
			for j := range dim {
				// (E_k† E_k)[i][j] = sum_l conj(E_k[l][i]) * E_k[l][j]
				var v complex128
				for l := range dim {
					v += cmplx.Conj(ek[l*dim+i]) * ek[l*dim+j]
				}
				sum[i*dim+j] += v
			}
		}
	}
	for i := range dim {
		for j := range dim {
			want := complex(0, 0)
			if i == j {
				want = 1
			}
			diff := sum[i*dim+j] - want
			if math.Abs(real(diff)) > tol || math.Abs(imag(diff)) > tol {
				return fmt.Errorf("noise.Custom: channel is not trace-preserving: (sum E_k†E_k)[%d][%d] = %v, want %v", i, j, sum[i*dim+j], want)
			}
		}
	}
	return nil
}
