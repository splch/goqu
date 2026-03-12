package pauli

import (
	"math/cmplx"
	"testing"
)

func TestPauliSum_Add(t *testing.T) {
	x, _ := Parse("X")
	z, _ := Parse("Z")
	s1, _ := NewPauliSum([]PauliString{x})
	s2, _ := NewPauliSum([]PauliString{z})
	sum := s1.Add(s2)
	if len(sum.Terms()) != 2 {
		t.Fatalf("len(Terms) = %d, want 2", len(sum.Terms()))
	}
	if sum.Terms()[0].Op(0) != X {
		t.Errorf("term[0] = %v, want X", sum.Terms()[0].Op(0))
	}
	if sum.Terms()[1].Op(0) != Z {
		t.Errorf("term[1] = %v, want Z", sum.Terms()[1].Op(0))
	}
}

func TestPauliSum_Scale(t *testing.T) {
	x, _ := Parse("X")
	z, _ := Parse("Z")
	s, _ := NewPauliSum([]PauliString{x, z})
	scaled := s.Scale(2i)
	for _, term := range scaled.Terms() {
		if term.Coeff() != 2i {
			t.Errorf("Coeff = %v, want 2i", term.Coeff())
		}
	}
	// Original unchanged.
	for _, term := range s.Terms() {
		if term.Coeff() != 1 {
			t.Errorf("original Coeff = %v, want 1", term.Coeff())
		}
	}
}

func TestPauliSum_Mul(t *testing.T) {
	// (X + Z)(X - Z):
	// X*X = I, X*(-Z) = -(-iY) = iY, Z*X = iY, Z*(-Z) = -I
	// = I - I + iY + iY = 2iY
	x := FromLabel("X")
	z := FromLabel("Z")
	zNeg := z.Scale(-1)

	s1, _ := NewPauliSum([]PauliString{x, z})
	s2, _ := NewPauliSum([]PauliString{x, zNeg})

	prod := s1.Mul(s2)
	simplified := prod.Simplify()

	terms := simplified.Terms()
	if len(terms) != 1 {
		t.Fatalf("len(Terms) = %d, want 1, terms: %v", len(terms), terms)
	}
	if terms[0].Op(0) != Y {
		t.Errorf("Op(0) = %v, want Y", terms[0].Op(0))
	}
	if terms[0].Coeff() != 2i {
		t.Errorf("Coeff = %v, want 2i", terms[0].Coeff())
	}
}

func TestPauliSum_Mul_NonTrivial(t *testing.T) {
	// (X + Y) * X = XX + YX = I + (-i)Z = I - iZ
	x := FromLabel("X")
	y := FromLabel("Y")

	s1, _ := NewPauliSum([]PauliString{x, y})
	s2, _ := NewPauliSum([]PauliString{x})

	prod := s1.Mul(s2).Simplify()
	terms := prod.Terms()

	// Should have 2 terms: I with coeff 1, Z with coeff -i
	if len(terms) != 2 {
		t.Fatalf("len(Terms) = %d, want 2", len(terms))
	}

	foundI := false
	foundZ := false
	for _, term := range terms {
		if term.Op(0) == I && term.Coeff() == 1 {
			foundI = true
		}
		if term.Op(0) == Z && term.Coeff() == -1i {
			foundZ = true
		}
	}
	if !foundI {
		t.Error("expected term I with coeff 1")
	}
	if !foundZ {
		t.Error("expected term Z with coeff -1i")
	}
}

func TestPauliSum_Simplify(t *testing.T) {
	// 2X + 3X = 5X
	x1 := FromLabel("X").Scale(2)
	x2 := FromLabel("X").Scale(3)
	s, _ := NewPauliSum([]PauliString{x1, x2})
	simplified := s.Simplify()
	if len(simplified.Terms()) != 1 {
		t.Fatalf("len(Terms) = %d, want 1", len(simplified.Terms()))
	}
	if simplified.Terms()[0].Op(0) != X {
		t.Errorf("Op(0) = %v, want X", simplified.Terms()[0].Op(0))
	}
	if simplified.Terms()[0].Coeff() != 5 {
		t.Errorf("Coeff = %v, want 5", simplified.Terms()[0].Coeff())
	}
}

func TestPauliSum_Simplify_DropsZero(t *testing.T) {
	// X + (-1)X = 0 -> should be dropped
	x := FromLabel("X")
	xNeg := FromLabel("X").Scale(-1)
	s, _ := NewPauliSum([]PauliString{x, xNeg})
	simplified := s.Simplify()
	// All remaining terms should have near-zero coefficient.
	for _, term := range simplified.Terms() {
		if cmplx.Abs(term.Coeff()) > 1e-10 {
			t.Errorf("expected zero terms, got %v", term)
		}
	}
}

func TestPauliSum_Simplify_MultiQubit(t *testing.T) {
	// XY + 2*XY + ZI = 3*XY + ZI
	xy1 := FromLabel("XY")
	xy2 := FromLabel("XY").Scale(2)
	zi := FromLabel("ZI")
	s, _ := NewPauliSum([]PauliString{xy1, xy2, zi})
	simplified := s.Simplify()
	if len(simplified.Terms()) != 2 {
		t.Fatalf("len(Terms) = %d, want 2", len(simplified.Terms()))
	}
	// Check the terms (order preserved: XY first, ZI second).
	if simplified.Terms()[0].Coeff() != 3 {
		t.Errorf("XY coeff = %v, want 3", simplified.Terms()[0].Coeff())
	}
	if simplified.Terms()[1].Coeff() != 1 {
		t.Errorf("ZI coeff = %v, want 1", simplified.Terms()[1].Coeff())
	}
}
