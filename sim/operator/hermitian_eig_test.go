package operator

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestHermitianEig_Diagonal(t *testing.T) {
	// Diagonal matrix: eigenvalues are the diagonal entries.
	m := []complex128{
		3, 0, 0,
		0, 1, 0,
		0, 0, 2,
	}
	evals, evecs := hermitianEig(m, 3)

	// Sorted descending: 3, 2, 1
	expected := []float64{3, 2, 1}
	for i, ev := range expected {
		if math.Abs(evals[i]-ev) > testTol {
			t.Errorf("eval[%d] = %f, want %f", i, evals[i], ev)
		}
	}

	// Verify eigenvectors are orthonormal.
	verifyOrthonormal(t, evecs, 3)
}

func TestHermitianEig_2x2(t *testing.T) {
	// [[2, 1], [1, 2]] -> eigenvalues 3, 1
	m := []complex128{2, 1, 1, 2}
	evals, evecs := hermitianEig(m, 2)

	if math.Abs(evals[0]-3.0) > testTol {
		t.Errorf("eval[0] = %f, want 3.0", evals[0])
	}
	if math.Abs(evals[1]-1.0) > testTol {
		t.Errorf("eval[1] = %f, want 1.0", evals[1])
	}
	verifyOrthonormal(t, evecs, 2)
}

func TestHermitianEig_PauliZ(t *testing.T) {
	// Z = [[1, 0], [0, -1]] -> eigenvalues 1, -1
	m := []complex128{1, 0, 0, -1}
	evals, _ := hermitianEig(m, 2)

	if math.Abs(evals[0]-1.0) > testTol {
		t.Errorf("eval[0] = %f, want 1.0", evals[0])
	}
	if math.Abs(evals[1]-(-1.0)) > testTol {
		t.Errorf("eval[1] = %f, want -1.0", evals[1])
	}
}

func TestHermitianEig_Complex(t *testing.T) {
	// [[2, 1+i], [1-i, 3]]: trace=5, det=6-2=4. Eigenvalues: (5 +/- sqrt(25-16))/2 = (5+/-3)/2 = 4, 1
	m := []complex128{2, 1 + 1i, 1 - 1i, 3}
	evals, evecs := hermitianEig(m, 2)

	if math.Abs(evals[0]-4.0) > testTol {
		t.Errorf("eval[0] = %f, want 4.0", evals[0])
	}
	if math.Abs(evals[1]-1.0) > testTol {
		t.Errorf("eval[1] = %f, want 1.0", evals[1])
	}
	verifyOrthonormal(t, evecs, 2)
	verifyEigenvectors(t, m, evals, evecs, 2)
}

func TestHermitianEig_4x4(t *testing.T) {
	// Identity should give all eigenvalues = 1.
	m := identityMatrix(4)
	evals, _ := hermitianEig(m, 4)
	for i, ev := range evals {
		if math.Abs(ev-1.0) > testTol {
			t.Errorf("eval[%d] = %f, want 1.0", i, ev)
		}
	}
}

func TestHermitianEig_ChoiIdentity(t *testing.T) {
	// Choi matrix of identity channel on 1 qubit:
	// [[1,0,0,1],[0,0,0,0],[0,0,0,0],[1,0,0,1]]
	// Rank 1, eigenvalue 2 (with eigenvector [1,0,0,1]/sqrt(2)).
	k := identity1Q()
	c := KrausToChoi(k)
	evals, _ := hermitianEig(c.matrix, 4)

	// Should have one eigenvalue = 2 and three = 0.
	if math.Abs(evals[0]-2.0) > testTol {
		t.Errorf("largest eigenvalue = %f, want 2.0", evals[0])
	}
	for i := 1; i < 4; i++ {
		if math.Abs(evals[i]) > testTol {
			t.Errorf("eval[%d] = %f, want 0.0", i, evals[i])
		}
	}
}

func TestHermitianEig_ZeroMatrix(t *testing.T) {
	m := make([]complex128, 4)
	evals, _ := hermitianEig(m, 2)
	for i, ev := range evals {
		if math.Abs(ev) > testTol {
			t.Errorf("eval[%d] = %f, want 0.0", i, ev)
		}
	}
}

// verifyOrthonormal checks that columns of the n x n matrix v are orthonormal.
func verifyOrthonormal(t *testing.T, v []complex128, n int) {
	t.Helper()
	for i := range n {
		for j := range n {
			var dot complex128
			for r := range n {
				dot += cmplx.Conj(v[r*n+i]) * v[r*n+j]
			}
			expected := complex(0, 0)
			if i == j {
				expected = 1
			}
			if cmplx.Abs(dot-expected) > 1e-8 {
				t.Errorf("dot(col%d, col%d) = %v, want %v", i, j, dot, expected)
			}
		}
	}
}

// verifyEigenvectors checks A*v_i = lambda_i * v_i.
func verifyEigenvectors(t *testing.T, a []complex128, evals []float64, evecs []complex128, n int) {
	t.Helper()
	for col := range n {
		// Extract eigenvector column.
		v := make([]complex128, n)
		for r := range n {
			v[r] = evecs[r*n+col]
		}
		// Compute A*v.
		av := make([]complex128, n)
		for r := range n {
			for c := range n {
				av[r] += a[r*n+c] * v[c]
			}
		}
		// Check A*v = lambda*v.
		lambda := complex(evals[col], 0)
		for r := range n {
			diff := cmplx.Abs(av[r] - lambda*v[r])
			if diff > 1e-8 {
				t.Errorf("eigenvector %d, row %d: A*v = %v, lambda*v = %v",
					col, r, av[r], lambda*v[r])
			}
		}
	}
}
