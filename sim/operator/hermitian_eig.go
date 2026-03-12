package operator

import (
	"math"
	"math/cmplx"
	"sort"
)

const eigTol = 1e-12
const maxIter = 1000

// hermitianEig computes eigenvalues and eigenvectors of an n x n Hermitian matrix
// using Jacobi iteration. Returns eigenvalues sorted descending and the
// corresponding eigenvectors stored as columns in a flat n x n matrix.
//
// Practical limit: 1-2 qubit channels (dim^2 up to 16x16).
func hermitianEig(m []complex128, n int) (eigenvalues []float64, eigenvectors []complex128) {
	// Work on a copy.
	a := make([]complex128, n*n)
	copy(a, m)

	// Initialize eigenvectors to identity.
	v := identityMatrix(n)

	for range maxIter {
		// Find largest off-diagonal element.
		maxVal := 0.0
		p, q := 0, 1
		for i := range n {
			for j := i + 1; j < n; j++ {
				val := cmplx.Abs(a[i*n+j])
				if val > maxVal {
					maxVal = val
					p = i
					q = j
				}
			}
		}
		if maxVal < eigTol {
			break
		}

		// Compute Jacobi rotation to zero out a[p,q].
		jacobiRotate(a, v, n, p, q)
	}

	// Extract eigenvalues from diagonal.
	eigenvalues = make([]float64, n)
	for i := range n {
		eigenvalues[i] = real(a[i*n+i])
	}

	// Sort eigenvalues descending, reorder eigenvector columns accordingly.
	indices := make([]int, n)
	for i := range n {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return eigenvalues[indices[i]] > eigenvalues[indices[j]]
	})

	sortedEvals := make([]float64, n)
	sortedEvecs := make([]complex128, n*n)
	for newIdx, oldIdx := range indices {
		sortedEvals[newIdx] = eigenvalues[oldIdx]
		for row := range n {
			sortedEvecs[row*n+newIdx] = v[row*n+oldIdx]
		}
	}

	return sortedEvals, sortedEvecs
}

// jacobiRotate applies a Jacobi rotation to zero out the (p,q) element of a
// Hermitian matrix a, updating the eigenvector matrix v accordingly.
func jacobiRotate(a, v []complex128, n, p, q int) {
	// For Hermitian matrix: a[p,q] = conj(a[q,p]).
	// 2x2 subproblem:
	//   [app, apq]
	//   [conj(apq), aqq]
	app := real(a[p*n+p])
	aqq := real(a[q*n+q])
	apq := a[p*n+q]

	// Phase rotation to make apq real.
	absApq := cmplx.Abs(apq)
	if absApq < eigTol {
		return
	}
	phase := apq / complex(absApq, 0)

	// Now the effective 2x2 real symmetric problem:
	//   [app, |apq|]
	//   [|apq|, aqq]
	// tan(2*theta) = 2*|apq| / (app - aqq)
	diff := app - aqq
	var t float64 // tan(theta)
	if math.Abs(diff) < eigTol {
		t = 1.0 // theta = pi/4
	} else {
		tau := diff / (2 * absApq)
		// t = sign(tau) / (|tau| + sqrt(1 + tau^2))
		// This is the "smaller" root for numerical stability.
		t = 1.0 / (math.Abs(tau) + math.Sqrt(1+tau*tau))
		if tau < 0 {
			t = -t
		}
	}

	c := 1.0 / math.Sqrt(1+t*t) // cos(theta)
	s := t * c                  // sin(theta)
	cc := complex(c, 0)
	// The full rotation includes the phase: G[p,q] = -s * conj(phase), G[q,p] = s * phase
	sPhase := complex(s, 0) * phase
	sPhaseConj := cmplx.Conj(sPhase)

	// Update matrix: A' = G-dagger * A * G
	// First update rows p and q for all columns.
	for j := range n {
		if j == p || j == q {
			continue
		}
		ajp := a[j*n+p]
		ajq := a[j*n+q]
		a[j*n+p] = cc*ajp + sPhaseConj*ajq
		a[j*n+q] = -sPhase*ajp + cc*ajq
		// Hermitian: a[p,j] = conj(a[j,p]), a[q,j] = conj(a[j,q])
		a[p*n+j] = cmplx.Conj(a[j*n+p])
		a[q*n+j] = cmplx.Conj(a[j*n+q])
	}

	// Update diagonal and off-diagonal of the 2x2 block.
	// a'[p,p] = c^2*app + 2*c*s*|apq| + s^2*aqq
	// a'[q,q] = s^2*app - 2*c*s*|apq| + c^2*aqq
	a[p*n+p] = complex(c*c*app+2*c*s*absApq+s*s*aqq, 0)
	a[q*n+q] = complex(s*s*app-2*c*s*absApq+c*c*aqq, 0)
	a[p*n+q] = 0
	a[q*n+p] = 0

	// Update eigenvectors.
	for i := range n {
		vip := v[i*n+p]
		viq := v[i*n+q]
		v[i*n+p] = cc*vip + sPhaseConj*viq
		v[i*n+q] = -sPhase*vip + cc*viq
	}
}
