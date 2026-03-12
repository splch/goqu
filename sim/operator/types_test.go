package operator

import (
	"testing"

	"github.com/splch/qgo/sim/noise"
)

func TestNewKraus(t *testing.T) {
	// Identity channel: single Kraus operator = I
	ops := [][]complex128{{1, 0, 0, 1}}
	k := NewKraus(1, ops)
	if k.NumQubits() != 1 {
		t.Fatalf("expected 1 qubit, got %d", k.NumQubits())
	}
	got := k.Operators()
	if len(got) != 1 {
		t.Fatalf("expected 1 operator, got %d", len(got))
	}
	// Verify defensive copy: modifying original should not affect Kraus.
	ops[0][0] = 42
	if k.operators[0][0] == 42 {
		t.Fatal("NewKraus did not make a defensive copy")
	}
	// Verify Operators() returns a defensive copy.
	got[0][0] = 99
	if k.operators[0][0] == 99 {
		t.Fatal("Operators() did not return a defensive copy")
	}
}

func TestNewSuperOp(t *testing.T) {
	// 1 qubit: dim=2, d2=4, superop is 4x4=16 elements
	m := make([]complex128, 16)
	m[0] = 1
	m[5] = 1
	m[10] = 1
	m[15] = 1
	s := NewSuperOp(1, m)
	if s.NumQubits() != 1 {
		t.Fatalf("expected 1 qubit, got %d", s.NumQubits())
	}
	got := s.Matrix()
	if len(got) != 16 {
		t.Fatalf("expected length 16, got %d", len(got))
	}
	// Defensive copy.
	got[0] = 99
	if s.matrix[0] == 99 {
		t.Fatal("Matrix() did not return a defensive copy")
	}
}

func TestNewChoi(t *testing.T) {
	// 1 qubit: dim=2, d2=4, Choi is 4x4=16 elements
	m := make([]complex128, 16)
	m[0] = 0.5
	m[5] = 0.5
	m[10] = 0.5
	m[15] = 0.5
	c := NewChoi(1, m)
	if c.NumQubits() != 1 {
		t.Fatalf("expected 1 qubit, got %d", c.NumQubits())
	}
}

func TestNewPTM(t *testing.T) {
	// 1 qubit: dim=2, d2=4, PTM is 4x4=16 elements
	m := make([]float64, 16)
	m[0] = 1
	m[5] = 1
	m[10] = 1
	m[15] = 1
	p := NewPTM(1, m)
	if p.NumQubits() != 1 {
		t.Fatalf("expected 1 qubit, got %d", p.NumQubits())
	}
	got := p.Matrix()
	if len(got) != 16 {
		t.Fatalf("expected length 16, got %d", len(got))
	}
}

func TestFromChannel(t *testing.T) {
	ch := noise.Depolarizing1Q(0.1)
	k := FromChannel(ch)
	if k.NumQubits() != 1 {
		t.Fatalf("expected 1 qubit, got %d", k.NumQubits())
	}
	ops := k.Operators()
	if len(ops) != 4 {
		t.Fatalf("expected 4 operators, got %d", len(ops))
	}
}

func TestNewKraus_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for mismatched operator size")
		}
	}()
	NewKraus(1, [][]complex128{{1, 0, 0}}) // 3 elements, need 4
}

func TestNewSuperOp_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for mismatched matrix size")
		}
	}()
	NewSuperOp(1, []complex128{1, 0}) // 2 elements, need 16
}
