package operator

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/goqu/sim/noise"
)

const testTol = 1e-9

// identity1Q returns the single-qubit identity Kraus channel.
func identity1Q() *Kraus {
	return NewKraus(1, [][]complex128{{1, 0, 0, 1}})
}

func TestKrausToSuperOp_Identity(t *testing.T) {
	k := identity1Q()
	s := KrausToSuperOp(k)

	// For identity channel: S = I (x) conj(I) = I_4
	// S should be the 4x4 identity matrix.
	dim := 2
	d2 := dim * dim
	for i := range d2 {
		for j := range d2 {
			expected := complex(0, 0)
			if i == j {
				expected = 1
			}
			if cmplx.Abs(s.matrix[i*d2+j]-expected) > testTol {
				t.Errorf("S[%d,%d] = %v, want %v", i, j, s.matrix[i*d2+j], expected)
			}
		}
	}
}

func TestKrausToChoi_Identity(t *testing.T) {
	k := identity1Q()
	c := KrausToChoi(k)

	// For identity channel on 1 qubit:
	// Choi = |Phi+><Phi+| where |Phi+> = (|00> + |11>)/sqrt(2)
	// vec(I) = [1, 0, 0, 1] (column-stacked), outer product:
	// [1,0,0,1]^T [1,0,0,1]* = [[1,0,0,1],[0,0,0,0],[0,0,0,0],[1,0,0,1]]
	expected := []complex128{
		1, 0, 0, 1,
		0, 0, 0, 0,
		0, 0, 0, 0,
		1, 0, 0, 1,
	}
	for i, v := range expected {
		if cmplx.Abs(c.matrix[i]-v) > testTol {
			t.Errorf("Choi[%d] = %v, want %v", i, c.matrix[i], v)
		}
	}
}

func TestRoundTrip_KrausChoiKraus(t *testing.T) {
	// Use depolarizing channel.
	ch := noise.Depolarizing1Q(0.3)
	k := FromChannel(ch)

	// Kraus -> Choi -> Kraus
	c := KrausToChoi(k)
	k2 := ChoiToKraus(c)

	// Verify by converting both back to SuperOp and comparing.
	s1 := KrausToSuperOp(k)
	s2 := KrausToSuperOp(k2)

	if !matClose(s1.matrix, s2.matrix, 1e-8) {
		t.Error("Kraus->Choi->Kraus roundtrip produced different SuperOp")
		for i := range s1.matrix {
			if cmplx.Abs(s1.matrix[i]-s2.matrix[i]) > 1e-8 {
				t.Logf("  [%d]: %v vs %v", i, s1.matrix[i], s2.matrix[i])
			}
		}
	}
}

func TestRoundTrip_KrausSuperOpChoi(t *testing.T) {
	// Full triangle: Kraus -> SuperOp -> Choi -> SuperOp, compare.
	ch := noise.AmplitudeDamping(0.2)
	k := FromChannel(ch)

	s := KrausToSuperOp(k)
	c1 := SuperOpToChoi(s)
	s2 := ChoiToSuperOp(c1)

	if !matClose(s.matrix, s2.matrix, 1e-8) {
		t.Error("SuperOp->Choi->SuperOp roundtrip mismatch")
	}

	// Also verify Kraus -> Choi directly matches SuperOp -> Choi.
	c2 := KrausToChoi(k)
	if !matClose(c1.matrix, c2.matrix, 1e-8) {
		t.Error("KrausToChoi vs SuperOpToChoi mismatch")
	}
}

func TestSuperOpToChoi_Depolarizing(t *testing.T) {
	ch := noise.Depolarizing1Q(0.5)
	k := FromChannel(ch)

	// Compute Choi two ways: directly and via SuperOp.
	cDirect := KrausToChoi(k)
	s := KrausToSuperOp(k)
	cViaSuperOp := SuperOpToChoi(s)

	if !matClose(cDirect.matrix, cViaSuperOp.matrix, 1e-8) {
		t.Error("Choi via direct vs via SuperOp mismatch for depolarizing")
	}
}

func TestChoiToKraus_ReconstructsSuperOp(t *testing.T) {
	// Test that ChoiToKraus produces operators that yield the same SuperOp.
	channels := []noise.Channel{
		noise.Depolarizing1Q(0.1),
		noise.AmplitudeDamping(0.3),
		noise.PhaseDamping(0.2),
		noise.BitFlip(0.15),
		noise.PhaseFlip(0.25),
	}
	for _, ch := range channels {
		k := FromChannel(ch)
		sOrig := KrausToSuperOp(k)

		c := KrausToChoi(k)
		k2 := ChoiToKraus(c)
		sRecov := KrausToSuperOp(k2)

		if !matClose(sOrig.matrix, sRecov.matrix, 1e-7) {
			t.Errorf("Choi->Kraus roundtrip failed for %s", ch.Name())
		}
	}
}

func TestKrausToSuperOp_Depolarizing(t *testing.T) {
	// Depolarizing channel with p: E(rho) = (1-p)*rho + (p/3)*(X rho X + Y rho Y + Z rho Z)
	// SuperOp: S = (1-4p/3)*I_4 + (4p/3) * (I_4 projected to identity-component)
	// Verify by applying to known density matrices.
	p := 0.3
	ch := noise.Depolarizing1Q(p)
	k := FromChannel(ch)
	s := KrausToSuperOp(k)

	// Apply SuperOp to vec(|0><0|) = [1,0,0,0]
	vecRho := []complex128{1, 0, 0, 0}
	d2 := 4
	result := make([]complex128, d2)
	for i := range d2 {
		for j := range d2 {
			result[i] += s.matrix[i*d2+j] * vecRho[j]
		}
	}
	// Expected: E(|0><0|) = (1-p)|0><0| + (p/3)(|1><1| + |1><1| + |0><0|)
	// = (1-p+p/3)|0><0| + (2p/3)|1><1|
	// = (1-2p/3)|0><0| + (2p/3)|1><1|
	// vec = [1-2p/3, 0, 0, 2p/3]
	expected := []complex128{complex(1-2*p/3, 0), 0, 0, complex(2*p/3, 0)}
	for i := range d2 {
		if cmplx.Abs(result[i]-expected[i]) > testTol {
			t.Errorf("result[%d] = %v, want %v", i, result[i], expected[i])
		}
	}
}

// Test 2-qubit identity conversion.
func TestKrausToSuperOp_Identity2Q(t *testing.T) {
	id4 := identityMatrix(4)
	k := NewKraus(2, [][]complex128{id4})
	s := KrausToSuperOp(k)

	// SuperOp of 2-qubit identity should be I_16.
	d2 := 16
	for i := range d2 {
		for j := range d2 {
			expected := complex(0, 0)
			if i == j {
				expected = 1
			}
			if cmplx.Abs(s.matrix[i*d2+j]-expected) > testTol {
				t.Errorf("S2Q[%d,%d] = %v, want %v", i, j, s.matrix[i*d2+j], expected)
			}
		}
	}
}

// Verify matClose utility.
func TestMatClose(t *testing.T) {
	a := []complex128{1, 2, 3, 4}
	b := []complex128{1, 2, 3, 4}
	if !matClose(a, b, 1e-10) {
		t.Error("identical matrices should be close")
	}
	c := []complex128{1, 2, 3, 4.1}
	if matClose(a, c, 0.01) {
		t.Error("different matrices should not be close at tol=0.01")
	}
	if !matClose(a, c, 0.2) {
		t.Error("should be close at tol=0.2")
	}
	// Different lengths.
	if matClose(a, []complex128{1, 2}, 1) {
		t.Error("different lengths should not be close")
	}
}

// Verify adjoint utility.
func TestAdjoint(t *testing.T) {
	m := []complex128{1 + 1i, 2, 3 - 1i, 4}
	adj := adjoint(m, 2)
	expected := []complex128{1 - 1i, 3 + 1i, 2, 4}
	for i := range expected {
		if cmplx.Abs(adj[i]-expected[i]) > 1e-15 {
			t.Errorf("adjoint[%d] = %v, want %v", i, adj[i], expected[i])
		}
	}
}

// Verify matMul utility.
func TestMatMul(t *testing.T) {
	a := []complex128{1, 2, 3, 4}
	b := []complex128{5, 6, 7, 8}
	c := matMul(a, b, 2)
	expected := []complex128{19, 22, 43, 50}
	for i := range expected {
		if cmplx.Abs(c[i]-expected[i]) > 1e-15 {
			t.Errorf("matMul[%d] = %v, want %v", i, c[i], expected[i])
		}
	}
}

// Verify trace utility.
func TestTrace(t *testing.T) {
	m := []complex128{1 + 1i, 2, 3, 4 - 2i}
	tr := trace(m, 2)
	expected := complex(5, -1)
	if cmplx.Abs(tr-expected) > 1e-15 {
		t.Errorf("trace = %v, want %v", tr, expected)
	}
}

// _ suppress unused import warnings for math in tests.
var _ = math.Sqrt
