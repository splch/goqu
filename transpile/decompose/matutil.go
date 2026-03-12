// Package decompose provides gate decomposition algorithms.
package decompose

import (
	"math"
	"math/cmplx"
)

// MatMul multiplies two n×n matrices stored as flat row-major slices.
func MatMul(a, b []complex128, n int) []complex128 {
	c := make([]complex128, n*n)
	for i := range n {
		for k := range n {
			aik := a[i*n+k]
			if aik == 0 {
				continue
			}
			for j := range n {
				c[i*n+j] += aik * b[k*n+j]
			}
		}
	}
	return c
}

// MatAdj returns the conjugate transpose of an n×n matrix.
func MatAdj(m []complex128, n int) []complex128 {
	adj := make([]complex128, n*n)
	for r := range n {
		for c := range n {
			adj[r*n+c] = cmplx.Conj(m[c*n+r])
		}
	}
	return adj
}

// Tensor computes the Kronecker product of matrices a (dimA×dimA) and b (dimB×dimB).
func Tensor(a []complex128, dimA int, b []complex128, dimB int) []complex128 {
	dim := dimA * dimB
	out := make([]complex128, dim*dim)
	for ra := range dimA {
		for ca := range dimA {
			aval := a[ra*dimA+ca]
			if aval == 0 {
				continue
			}
			for rb := range dimB {
				for cb := range dimB {
					r := ra*dimB + rb
					c := ca*dimB + cb
					out[r*dim+c] = aval * b[rb*dimB+cb]
				}
			}
		}
	}
	return out
}

// Det2x2 returns the determinant of a 2×2 matrix.
func Det2x2(m []complex128) complex128 {
	return m[0]*m[3] - m[1]*m[2]
}

// ToSU2 normalizes a 2×2 matrix to SU(2) (determinant = 1).
// Handles both phase and magnitude normalization.
func ToSU2(m []complex128) []complex128 {
	det := Det2x2(m)
	if cmplx.Abs(det) < 1e-30 {
		return []complex128{1, 0, 0, 1}
	}
	// factor = det^{-1/2}: divides out both phase and magnitude.
	factor := 1 / cmplx.Sqrt(det)
	su2 := make([]complex128, 4)
	for i := range 4 {
		su2[i] = m[i] * factor
	}
	return su2
}

// IsIdentity checks if an n×n matrix is the identity within tolerance.
func IsIdentity(m []complex128, n int, tol float64) bool {
	for r := range n {
		for c := range n {
			expected := complex(0, 0)
			if r == c {
				expected = 1
			}
			if cmplx.Abs(m[r*n+c]-expected) > tol {
				return false
			}
		}
	}
	return true
}

// MatClose checks if two flat matrices are element-wise close within tolerance.
func MatClose(a, b []complex128, tol float64) bool {
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

// GlobalPhase checks if a = e^{iφ}·b and returns φ.
func GlobalPhase(a, b []complex128, tol float64) (float64, bool) {
	if len(a) != len(b) {
		return 0, false
	}
	// Find first non-negligible element to determine the phase.
	phase := 0.0
	found := false
	for i := range a {
		if cmplx.Abs(b[i]) < tol {
			if cmplx.Abs(a[i]) > tol {
				return 0, false
			}
			continue
		}
		ratio := a[i] / b[i]
		if math.Abs(cmplx.Abs(ratio)-1) > tol {
			return 0, false
		}
		if !found {
			phase = cmplx.Phase(ratio)
			found = true
		} else {
			// Check consistency of the phase.
			p := cmplx.Phase(ratio)
			diff := math.Abs(phase - p)
			// Handle wrapping around ±π.
			if diff > math.Pi {
				diff = 2*math.Pi - diff
			}
			if diff > tol*100 { // slightly more relaxed for phase comparison
				return 0, false
			}
		}
	}
	if !found {
		// Both are zero matrices.
		return 0, true
	}
	return phase, true
}

// Eye returns the n×n identity matrix.
func Eye(n int) []complex128 {
	m := make([]complex128, n*n)
	for i := range n {
		m[i*n+i] = 1
	}
	return m
}

// matMul2 multiplies two 2×2 matrices stored as flat [4]complex128 slices.
func matMul2(a, b []complex128) []complex128 {
	return []complex128{
		a[0]*b[0] + a[1]*b[2], a[0]*b[1] + a[1]*b[3],
		a[2]*b[0] + a[3]*b[2], a[2]*b[1] + a[3]*b[3],
	}
}

// matAdj2 returns the conjugate transpose of a 2×2 matrix.
func matAdj2(m []complex128) []complex128 {
	return []complex128{
		cmplx.Conj(m[0]), cmplx.Conj(m[2]),
		cmplx.Conj(m[1]), cmplx.Conj(m[3]),
	}
}

// rzMat returns the 2×2 Rz(theta) matrix: diag(e^{-i*theta/2}, e^{i*theta/2}).
func rzMat(theta float64) []complex128 {
	h := theta / 2
	return []complex128{
		cmplx.Exp(complex(0, -h)), 0,
		0, cmplx.Exp(complex(0, h)),
	}
}

// hMat returns the 2×2 Hadamard matrix.
func hMat() []complex128 {
	s := 1.0 / math.Sqrt(2)
	return []complex128{complex(s, 0), complex(s, 0), complex(s, 0), complex(-s, 0)}
}

// ryMat returns the 2×2 Ry(theta) matrix.
func ryMat(theta float64) []complex128 {
	c := math.Cos(theta / 2)
	s := math.Sin(theta / 2)
	return []complex128{complex(c, 0), complex(-s, 0), complex(s, 0), complex(c, 0)}
}

// MatScale multiplies every element of m by scalar s.
func MatScale(m []complex128, s complex128) []complex128 {
	out := make([]complex128, len(m))
	for i := range m {
		out[i] = m[i] * s
	}
	return out
}
