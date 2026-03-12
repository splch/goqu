package gate

import (
	"math/cmplx"
	"testing"
)

func TestPow_Identity(t *testing.T) {
	// H^0 = I
	g := Pow(H, 0)
	m := g.Matrix()
	if cmplx.Abs(m[0]-1) > 1e-10 || cmplx.Abs(m[3]-1) > 1e-10 {
		t.Errorf("H^0 should be identity, got %v", m)
	}
	if cmplx.Abs(m[1]) > 1e-10 || cmplx.Abs(m[2]) > 1e-10 {
		t.Errorf("H^0 off-diagonal should be zero, got %v", m)
	}
}

func TestPow_One(t *testing.T) {
	// H^1 = H
	g := Pow(H, 1)
	if g != H {
		t.Error("Pow(H, 1) should return H unchanged")
	}
}

func TestPow_SelfInverse(t *testing.T) {
	// H^2 = I (H is self-inverse)
	g := Pow(H, 2)
	m := g.Matrix()
	for i := range 2 {
		for j := range 2 {
			want := complex(0, 0)
			if i == j {
				want = 1
			}
			if cmplx.Abs(m[i*2+j]-want) > 1e-10 {
				t.Errorf("H^2[%d][%d] = %v, want %v", i, j, m[i*2+j], want)
			}
		}
	}
}

func TestPow_Negative(t *testing.T) {
	// S^-1 = S† (phase gate inverse)
	g := Pow(S, -1)
	sdg := S.Inverse()
	gm := g.Matrix()
	sm := sdg.Matrix()
	for i := range 4 {
		if cmplx.Abs(gm[i]-sm[i]) > 1e-10 {
			t.Errorf("S^-1[%d] = %v, want %v", i, gm[i], sm[i])
		}
	}
}

func TestPow_Three(t *testing.T) {
	// S^3 = S * S * S = S† (since S^4 = I)
	g := Pow(S, 3)
	sdg := S.Inverse()
	gm := g.Matrix()
	sm := sdg.Matrix()
	for i := range 4 {
		if cmplx.Abs(gm[i]-sm[i]) > 1e-10 {
			t.Errorf("S^3[%d] = %v, want S†[%d] = %v", i, gm[i], i, sm[i])
		}
	}
}

func TestPow_TwoQubit(t *testing.T) {
	// CNOT^2 = I (CNOT is self-inverse)
	g := Pow(CNOT, 2)
	m := g.Matrix()
	for i := range 4 {
		for j := range 4 {
			want := complex(0, 0)
			if i == j {
				want = 1
			}
			if cmplx.Abs(m[i*4+j]-want) > 1e-10 {
				t.Errorf("CNOT^2[%d][%d] = %v, want %v", i, j, m[i*4+j], want)
			}
		}
	}
}

func TestPow_X_Even_Odd(t *testing.T) {
	// X^2 = I, X^3 = X
	g2 := Pow(X, 2)
	m2 := g2.Matrix()
	if cmplx.Abs(m2[0]-1) > 1e-10 || cmplx.Abs(m2[3]-1) > 1e-10 {
		t.Error("X^2 should be I")
	}

	g3 := Pow(X, 3)
	m3 := g3.Matrix()
	xm := X.Matrix()
	for i := range 4 {
		if cmplx.Abs(m3[i]-xm[i]) > 1e-10 {
			t.Errorf("X^3[%d] = %v, want %v", i, m3[i], xm[i])
		}
	}
}

func TestPow_NegativeTwo(t *testing.T) {
	// S^-2 = (S†)^2 = (S†)(S†) = S^2† = Z† = Z (since Z is self-inverse)
	g := Pow(S, -2)
	m := g.Matrix()
	zm := Z.Matrix()
	for i := range 4 {
		if cmplx.Abs(m[i]-zm[i]) > 1e-10 {
			t.Errorf("S^-2[%d] = %v, want Z[%d] = %v", i, m[i], i, zm[i])
		}
	}
}
