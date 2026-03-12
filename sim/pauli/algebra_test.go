package pauli

import "testing"

func TestMul_SingleQubit(t *testing.T) {
	// All 16 single-qubit products.
	tests := []struct {
		a, b      string
		wantOp    string
		wantCoeff complex128
	}{
		// I * {I,X,Y,Z}
		{"I", "I", "I", 1}, {"I", "X", "X", 1}, {"I", "Y", "Y", 1}, {"I", "Z", "Z", 1},
		// X * {I,X,Y,Z}
		{"X", "I", "X", 1}, {"X", "X", "I", 1}, {"X", "Y", "Z", 1i}, {"X", "Z", "Y", -1i},
		// Y * {I,X,Y,Z}
		{"Y", "I", "Y", 1}, {"Y", "X", "Z", -1i}, {"Y", "Y", "I", 1}, {"Y", "Z", "X", 1i},
		// Z * {I,X,Y,Z}
		{"Z", "I", "Z", 1}, {"Z", "X", "Y", 1i}, {"Z", "Y", "X", -1i}, {"Z", "Z", "I", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"*"+tt.b, func(t *testing.T) {
			a := FromLabel(tt.a)
			b := FromLabel(tt.b)
			got := Mul(a, b)
			wantOps := FromLabel(tt.wantOp)
			if got.Op(0) != wantOps.Op(0) {
				t.Errorf("Mul(%s,%s) op = %v, want %v", tt.a, tt.b, got.Op(0), wantOps.Op(0))
			}
			if got.Coeff() != tt.wantCoeff {
				t.Errorf("Mul(%s,%s) coeff = %v, want %v", tt.a, tt.b, got.Coeff(), tt.wantCoeff)
			}
		})
	}
}

func TestMul_MultiQubit(t *testing.T) {
	tests := []struct {
		name      string
		a, b      string
		wantOp    string
		wantCoeff complex128
	}{
		{"XI*IX=XX", "XI", "IX", "XX", 1},
		{"XY*YX=ZZ", "XY", "YX", "ZZ", 1}, // phase: (1+3)%4=0 => coeff=1
		{"XX*XX=II", "XX", "XX", "II", 1},
		{"ZZ*ZZ=II", "ZZ", "ZZ", "II", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := FromLabel(tt.a)
			b := FromLabel(tt.b)
			got := Mul(a, b)
			want := FromLabel(tt.wantOp)
			for i := 0; i < got.NumQubits(); i++ {
				if got.Op(i) != want.Op(i) {
					t.Errorf("Op(%d) = %v, want %v", i, got.Op(i), want.Op(i))
				}
			}
			if got.Coeff() != tt.wantCoeff {
				t.Errorf("Coeff = %v, want %v", got.Coeff(), tt.wantCoeff)
			}
		})
	}
}

func TestMul_Associativity(t *testing.T) {
	// (AB)C should equal A(BC) for various Pauli strings.
	triples := [][3]string{
		{"XYZ", "ZXY", "YZX"},
		{"XXX", "YYY", "ZZZ"},
		{"XIZ", "ZIX", "YIY"},
	}
	for _, triple := range triples {
		a := FromLabel(triple[0])
		b := FromLabel(triple[1])
		c := FromLabel(triple[2])

		ab := Mul(a, b)
		ab_c := Mul(ab, c)

		bc := Mul(b, c)
		a_bc := Mul(a, bc)

		for i := 0; i < a.NumQubits(); i++ {
			if ab_c.Op(i) != a_bc.Op(i) {
				t.Errorf("%v: Op(%d) mismatch: (AB)C=%v, A(BC)=%v", triple, i, ab_c.Op(i), a_bc.Op(i))
			}
		}
		if ab_c.Coeff() != a_bc.Coeff() {
			t.Errorf("%v: Coeff mismatch: (AB)C=%v, A(BC)=%v", triple, ab_c.Coeff(), a_bc.Coeff())
		}
	}
}

func TestCommutes(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"XX", "XX", true}, // same operators always commute
		{"XI", "IX", true}, // disjoint support commutes
		{"XZ", "ZX", true}, // anticommute at both positions => 2 odd => even => commute
		{"XY", "YX", true}, // position 0: X*Y anticommute, position 1: Y*X anticommute => 2 odd => even => commute
		{"X", "Z", false},  // single qubit X,Z anticommute
		{"X", "X", true},   // single qubit X,X commute
		{"II", "II", true},
	}
	for _, tt := range tests {
		t.Run(tt.a+","+tt.b, func(t *testing.T) {
			a := FromLabel(tt.a)
			b := FromLabel(tt.b)
			got := Commutes(a, b)
			if got != tt.want {
				t.Errorf("Commutes(%s,%s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestAntiCommutes(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"X", "Z", true},
		{"X", "Y", true},
		{"Y", "Z", true},
		{"X", "X", false},
		{"XZ", "ZX", false}, // position 0: X,Z anticommute; position 1: Z,X anticommute => 2 odd => even => commute
		{"XI", "ZI", true},  // position 0: X,Z anticommute; position 1: I,I commute => 1 odd => anticommute
	}
	for _, tt := range tests {
		t.Run(tt.a+","+tt.b, func(t *testing.T) {
			a := FromLabel(tt.a)
			b := FromLabel(tt.b)
			got := AntiCommutes(a, b)
			if got != tt.want {
				t.Errorf("AntiCommutes(%s,%s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestTensor(t *testing.T) {
	a := FromLabel("X")
	b := FromLabel("Z")
	got := Tensor(a, b)
	if got.NumQubits() != 2 {
		t.Fatalf("NumQubits = %d, want 2", got.NumQubits())
	}
	if got.Op(0) != X {
		t.Errorf("Op(0) = %v, want X", got.Op(0))
	}
	if got.Op(1) != Z {
		t.Errorf("Op(1) = %v, want Z", got.Op(1))
	}
	if got.Coeff() != 1 {
		t.Errorf("Coeff = %v, want 1", got.Coeff())
	}
}

func TestTensor_Coefficients(t *testing.T) {
	a := FromLabel("X").Scale(2)
	b := FromLabel("Z").Scale(3i)
	got := Tensor(a, b)
	if got.Coeff() != 6i {
		t.Errorf("Coeff = %v, want 6i", got.Coeff())
	}
}

func TestTensor_Larger(t *testing.T) {
	a := FromLabel("XY")
	b := FromLabel("ZI")
	got := Tensor(a, b)
	if got.NumQubits() != 4 {
		t.Fatalf("NumQubits = %d, want 4", got.NumQubits())
	}
	want := FromLabel("XYZI")
	for i := 0; i < 4; i++ {
		if got.Op(i) != want.Op(i) {
			t.Errorf("Op(%d) = %v, want %v", i, got.Op(i), want.Op(i))
		}
	}
}
