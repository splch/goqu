package pauli

import (
	"testing"
)

func TestPauliString(t *testing.T) {
	t.Run("I", func(t *testing.T) {
		if I.String() != "I" {
			t.Errorf("I.String() = %q", I.String())
		}
	})
	t.Run("X", func(t *testing.T) {
		if X.String() != "X" {
			t.Errorf("X.String() = %q", X.String())
		}
	})
	t.Run("Y", func(t *testing.T) {
		if Y.String() != "Y" {
			t.Errorf("Y.String() = %q", Y.String())
		}
	})
	t.Run("Z", func(t *testing.T) {
		if Z.String() != "Z" {
			t.Errorf("Z.String() = %q", Z.String())
		}
	})
}

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		want  []Pauli
	}{
		{"I", []Pauli{I}},
		{"X", []Pauli{X}},
		{"Y", []Pauli{Y}},
		{"Z", []Pauli{Z}},
		{"XZI", []Pauli{X, Z, I}},
		{"XYZI", []Pauli{X, Y, Z, I}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ps, err := Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if ps.NumQubits() != len(tt.want) {
				t.Fatalf("NumQubits() = %d, want %d", ps.NumQubits(), len(tt.want))
			}
			for i, w := range tt.want {
				if ps.Op(i) != w {
					t.Errorf("Op(%d) = %v, want %v", i, ps.Op(i), w)
				}
			}
			if ps.Coeff() != 1 {
				t.Errorf("Coeff() = %v, want 1", ps.Coeff())
			}
		})
	}
}

func TestParseError(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Error("expected error for empty string")
	}
	_, err = Parse("XAZ")
	if err == nil {
		t.Error("expected error for invalid character")
	}
}

func TestNewPauliString(t *testing.T) {
	ps := NewPauliString(0.5, map[int]Pauli{0: X, 2: Z}, 3)
	if ps.NumQubits() != 3 {
		t.Fatalf("NumQubits() = %d", ps.NumQubits())
	}
	if ps.Op(0) != X {
		t.Errorf("Op(0) = %v", ps.Op(0))
	}
	if ps.Op(1) != I {
		t.Errorf("Op(1) = %v", ps.Op(1))
	}
	if ps.Op(2) != Z {
		t.Errorf("Op(2) = %v", ps.Op(2))
	}
	if ps.Coeff() != 0.5 {
		t.Errorf("Coeff() = %v", ps.Coeff())
	}
}

func TestZOn(t *testing.T) {
	ps := ZOn([]int{0, 2}, 3)
	if ps.Op(0) != Z || ps.Op(1) != I || ps.Op(2) != Z {
		t.Errorf("ZOn: ops = %v %v %v", ps.Op(0), ps.Op(1), ps.Op(2))
	}
	if ps.Coeff() != 1 {
		t.Errorf("Coeff() = %v", ps.Coeff())
	}
}

func TestIsIdentity(t *testing.T) {
	ps, _ := Parse("III")
	if !ps.IsIdentity() {
		t.Error("expected identity")
	}
	ps, _ = Parse("IXI")
	if ps.IsIdentity() {
		t.Error("expected non-identity")
	}
}

func TestScale(t *testing.T) {
	ps, _ := Parse("XZ")
	scaled := ps.Scale(2i)
	if scaled.Coeff() != 2i {
		t.Errorf("Coeff() = %v, want 2i", scaled.Coeff())
	}
	// Original unchanged.
	if ps.Coeff() != 1 {
		t.Errorf("original Coeff() = %v, want 1", ps.Coeff())
	}
}

func TestMasks(t *testing.T) {
	// XYZ on qubits 0,1,2
	ps, _ := Parse("XYZ")
	xm := ps.xMask()
	zm := ps.zMask()

	// X: x=1,z=0; Y: x=1,z=1; Z: x=0,z=1
	// xMask: bit0(X)=1, bit1(Y)=1, bit2(Z)=0 -> 0b011 = 3
	if xm != 3 {
		t.Errorf("xMask = %b, want 011", xm)
	}
	// zMask: bit0(X)=0, bit1(Y)=1, bit2(Z)=1 -> 0b110 = 6
	if zm != 6 {
		t.Errorf("zMask = %b, want 110", zm)
	}
}

func TestNewPauliSum(t *testing.T) {
	t1, _ := Parse("XZ")
	t2, _ := Parse("ZX")
	sum, err := NewPauliSum([]PauliString{t1, t2})
	if err != nil {
		t.Fatal(err)
	}
	if sum.NumQubits() != 2 {
		t.Errorf("NumQubits() = %d", sum.NumQubits())
	}
	if len(sum.Terms()) != 2 {
		t.Errorf("Terms() len = %d", len(sum.Terms()))
	}
}

func TestNewPauliSumMismatch(t *testing.T) {
	t1, _ := Parse("XZ")
	t2, _ := Parse("XZI")
	_, err := NewPauliSum([]PauliString{t1, t2})
	if err == nil {
		t.Error("expected error for mismatched qubit counts")
	}
}

func TestNewPauliSumEmpty(t *testing.T) {
	_, err := NewPauliSum(nil)
	if err == nil {
		t.Error("expected error for empty terms")
	}
}

func TestStringRepr(t *testing.T) {
	ps := NewPauliString(0.5, map[int]Pauli{0: X, 1: Z, 2: I}, 3)
	s := ps.String()
	if s != "(0.5+0i)*XZI" {
		t.Errorf("String() = %q", s)
	}
	ps2, _ := Parse("XZ")
	if ps2.String() != "XZ" {
		t.Errorf("String() = %q, want XZ", ps2.String())
	}
}

func TestIPow(t *testing.T) {
	tests := []struct {
		n    int
		want complex128
	}{
		{0, 1},
		{1, 1i},
		{2, -1},
		{3, -1i},
		{4, 1},
		{5, 1i},
	}
	for _, tt := range tests {
		got := iPow(tt.n)
		if got != tt.want {
			t.Errorf("iPow(%d) = %v, want %v", tt.n, got, tt.want)
		}
	}
}
