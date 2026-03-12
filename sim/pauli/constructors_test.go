package pauli

import "testing"

func TestXOn(t *testing.T) {
	ps := XOn([]int{0, 2}, 3)
	if ps.Op(0) != X || ps.Op(1) != I || ps.Op(2) != X {
		t.Errorf("XOn: ops = %v %v %v", ps.Op(0), ps.Op(1), ps.Op(2))
	}
	if ps.Coeff() != 1 {
		t.Errorf("Coeff() = %v, want 1", ps.Coeff())
	}
}

func TestYOn(t *testing.T) {
	ps := YOn([]int{1}, 3)
	if ps.Op(0) != I || ps.Op(1) != Y || ps.Op(2) != I {
		t.Errorf("YOn: ops = %v %v %v", ps.Op(0), ps.Op(1), ps.Op(2))
	}
	if ps.Coeff() != 1 {
		t.Errorf("Coeff() = %v, want 1", ps.Coeff())
	}
}

func TestIdentity(t *testing.T) {
	ps := Identity(4)
	if ps.NumQubits() != 4 {
		t.Fatalf("NumQubits() = %d, want 4", ps.NumQubits())
	}
	for i := 0; i < 4; i++ {
		if ps.Op(i) != I {
			t.Errorf("Op(%d) = %v, want I", i, ps.Op(i))
		}
	}
	if ps.Coeff() != 1 {
		t.Errorf("Coeff() = %v, want 1", ps.Coeff())
	}
	if !ps.IsIdentity() {
		t.Error("expected IsIdentity() to be true")
	}
}

func TestFromLabel(t *testing.T) {
	ps := FromLabel("XYZ")
	if ps.NumQubits() != 3 {
		t.Fatalf("NumQubits() = %d, want 3", ps.NumQubits())
	}
	if ps.Op(0) != X || ps.Op(1) != Y || ps.Op(2) != Z {
		t.Errorf("ops = %v %v %v", ps.Op(0), ps.Op(1), ps.Op(2))
	}
	if ps.Coeff() != 1 {
		t.Errorf("Coeff() = %v, want 1", ps.Coeff())
	}
}

func TestFromLabel_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid label")
		}
	}()
	FromLabel("ABC")
}
