package operator

import (
	"math"
	"testing"

	"github.com/splch/goqu/sim/noise"
)

func TestKrausToPTM_Identity(t *testing.T) {
	k := identity1Q()
	p := KrausToPTM(k)

	// PTM of identity channel should be the 4x4 identity matrix.
	d2 := 4
	for i := range d2 {
		for j := range d2 {
			expected := 0.0
			if i == j {
				expected = 1.0
			}
			if math.Abs(p.matrix[i*d2+j]-expected) > testTol {
				t.Errorf("PTM[%d,%d] = %f, want %f", i, j, p.matrix[i*d2+j], expected)
			}
		}
	}
}

func TestKrausToPTM_Depolarizing(t *testing.T) {
	// Depolarizing channel: PTM = diag(1, 1-4p/3, 1-4p/3, 1-4p/3)
	p := 0.3
	ch := noise.Depolarizing1Q(p)
	k := FromChannel(ch)
	ptm := KrausToPTM(k)

	lambda := 1 - 4*p/3
	expected := []float64{
		1, 0, 0, 0,
		0, lambda, 0, 0,
		0, 0, lambda, 0,
		0, 0, 0, lambda,
	}
	for i := range expected {
		if math.Abs(ptm.matrix[i]-expected[i]) > testTol {
			t.Errorf("PTM[%d] = %f, want %f", i, ptm.matrix[i], expected[i])
		}
	}
}

func TestKrausToPTM_BitFlip(t *testing.T) {
	// Bit-flip channel with probability p.
	// BitFlip(p): sqrt(1-p)*I + sqrt(p)*X
	// E(I) = I, E(X) = X, E(Y) = (1-2p)*Y, E(Z) = (1-2p)*Z
	// R[I,I] = Tr(I*E(I))/2 = Tr(I)/2 = 1
	// R[X,X] = Tr(X*E(X))/2 = Tr(X*X)/2 = Tr(I)/2 = 1
	// R[Y,Y] = Tr(Y*E(Y))/2 = (1-2p)*Tr(Y*Y)/2 = (1-2p)
	// R[Z,Z] = Tr(Z*E(Z))/2 = (1-2p)*Tr(Z*Z)/2 = (1-2p)
	p := 0.2
	ch := noise.BitFlip(p)
	k := FromChannel(ch)
	ptm := KrausToPTM(k)

	lambda := 1 - 2*p
	expected := []float64{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, lambda, 0,
		0, 0, 0, lambda,
	}
	for i := range expected {
		if math.Abs(ptm.matrix[i]-expected[i]) > testTol {
			t.Errorf("PTM[%d] = %f, want %f (bit-flip)", i, ptm.matrix[i], expected[i])
		}
	}
}

func TestKrausToPTM_PhaseFlip(t *testing.T) {
	// Phase-flip channel: sqrt(1-p)*I, sqrt(p)*Z
	// E(I) = I, E(X) = (1-2p)*X, E(Y) = (1-2p)*Y, E(Z) = Z
	p := 0.15
	ch := noise.PhaseFlip(p)
	k := FromChannel(ch)
	ptm := KrausToPTM(k)

	lambda := 1 - 2*p
	expected := []float64{
		1, 0, 0, 0,
		0, lambda, 0, 0,
		0, 0, lambda, 0,
		0, 0, 0, 1,
	}
	for i := range expected {
		if math.Abs(ptm.matrix[i]-expected[i]) > testTol {
			t.Errorf("PTM[%d] = %f, want %f (phase-flip)", i, ptm.matrix[i], expected[i])
		}
	}
}

func TestPTMRoundTrip(t *testing.T) {
	// Kraus -> PTM -> Choi -> Kraus -> PTM, compare.
	ch := noise.Depolarizing1Q(0.25)
	k := FromChannel(ch)
	ptm1 := KrausToPTM(k)
	k2 := PTMToKraus(ptm1)
	ptm2 := KrausToPTM(k2)

	for i := range ptm1.matrix {
		if math.Abs(ptm1.matrix[i]-ptm2.matrix[i]) > 1e-7 {
			t.Errorf("PTM roundtrip mismatch at [%d]: %f vs %f", i, ptm1.matrix[i], ptm2.matrix[i])
		}
	}
}

func TestPTMToChoi_Identity(t *testing.T) {
	k := identity1Q()
	ptm := KrausToPTM(k)
	c := PTMToChoi(ptm)
	cDirect := KrausToChoi(k)

	if !matClose(c.matrix, cDirect.matrix, 1e-8) {
		t.Error("PTMToChoi(identity) does not match KrausToChoi(identity)")
		for i := range c.matrix {
			t.Logf("  [%d]: %v vs %v", i, c.matrix[i], cDirect.matrix[i])
		}
	}
}

func TestPauliBasis_1Q(t *testing.T) {
	basis := pauliBasis(1)
	if len(basis) != 4 {
		t.Fatalf("expected 4 Pauli basis elements, got %d", len(basis))
	}
	// Each should be 2x2.
	for i, b := range basis {
		if len(b) != 4 {
			t.Errorf("basis[%d] has %d elements, want 4", i, len(b))
		}
	}
}

func TestPauliBasis_2Q(t *testing.T) {
	basis := pauliBasis(2)
	if len(basis) != 16 {
		t.Fatalf("expected 16 Pauli basis elements for 2 qubits, got %d", len(basis))
	}
	// Each should be 4x4.
	for i, b := range basis {
		if len(b) != 16 {
			t.Errorf("basis[%d] has %d elements, want 16", i, len(b))
		}
	}
}

func TestChoiToPTM(t *testing.T) {
	ch := noise.PhaseDamping(0.3)
	k := FromChannel(ch)

	ptm1 := KrausToPTM(k)
	c := KrausToChoi(k)
	ptm2 := ChoiToPTM(c)

	for i := range ptm1.matrix {
		if math.Abs(ptm1.matrix[i]-ptm2.matrix[i]) > 1e-7 {
			t.Errorf("ChoiToPTM mismatch at [%d]: %f vs %f", i, ptm1.matrix[i], ptm2.matrix[i])
		}
	}
}

func TestSuperOpToPTM(t *testing.T) {
	ch := noise.AmplitudeDamping(0.2)
	k := FromChannel(ch)

	ptm1 := KrausToPTM(k)
	s := KrausToSuperOp(k)
	ptm2 := SuperOpToPTM(s)

	for i := range ptm1.matrix {
		if math.Abs(ptm1.matrix[i]-ptm2.matrix[i]) > 1e-7 {
			t.Errorf("SuperOpToPTM mismatch at [%d]: %f vs %f", i, ptm1.matrix[i], ptm2.matrix[i])
		}
	}
}
