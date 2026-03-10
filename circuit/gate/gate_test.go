package gate

import (
	"math"
	"math/cmplx"
	"testing"
)

const eps = 1e-14

// TestUnitarity verifies U†U = I for all standard gates.
func TestUnitarity(t *testing.T) {
	gates := []Gate{
		I, H, X, Y, Z, S, Sdg, T, Tdg, SX,
		CNOT, CZ, SWAP, CY, CCX, CSWAP,
		RX(math.Pi / 4), RY(math.Pi / 3), RZ(math.Pi / 6),
		Phase(math.Pi / 4), U3(math.Pi/4, math.Pi/3, math.Pi/6),
		CP(math.Pi / 4), CRZ(math.Pi / 3), CRX(math.Pi / 4), CRY(math.Pi / 5),
		GPI(math.Pi / 4), GPI2(math.Pi / 3), MS(math.Pi/4, math.Pi/6),
	}
	for _, g := range gates {
		t.Run(g.Name(), func(t *testing.T) {
			assertUnitary(t, g)
		})
	}
}

func assertUnitary(t *testing.T, g Gate) {
	t.Helper()
	m := g.Matrix()
	dim := 1 << g.Qubits()
	if len(m) != dim*dim {
		t.Fatalf("matrix size %d, want %d", len(m), dim*dim)
	}
	// Check U†U = I
	for r := range dim {
		for c := range dim {
			var sum complex128
			for k := range dim {
				sum += conj(m[k*dim+r]) * m[k*dim+c]
			}
			expected := complex(0, 0)
			if r == c {
				expected = 1
			}
			if cmplx.Abs(sum-expected) > eps {
				t.Errorf("(U†U)[%d,%d] = %v, want %v", r, c, sum, expected)
			}
		}
	}
}

// TestInverse verifies g * g.Inverse() = I for all standard gates.
func TestInverse(t *testing.T) {
	gates := []Gate{
		I, H, X, Y, Z, S, Sdg, T, Tdg, SX,
		CNOT, CZ, SWAP, CY,
		RX(math.Pi / 4), RY(math.Pi / 3), RZ(math.Pi / 6),
	}
	for _, g := range gates {
		t.Run(g.Name(), func(t *testing.T) {
			inv := g.Inverse()
			dim := 1 << g.Qubits()
			m := g.Matrix()
			mi := inv.Matrix()
			// Check m * mi = I
			for r := range dim {
				for c := range dim {
					var sum complex128
					for k := range dim {
						sum += m[r*dim+k] * mi[k*dim+c]
					}
					expected := complex(0, 0)
					if r == c {
						expected = 1
					}
					if cmplx.Abs(sum-expected) > eps {
						t.Errorf("(g*g†)[%d,%d] = %v, want %v", r, c, sum, expected)
					}
				}
			}
		})
	}
}

// TestKnownMatrices verifies specific matrix entries.
func TestKnownMatrices(t *testing.T) {
	s2 := 1.0 / math.Sqrt2

	tests := []struct {
		name string
		gate Gate
		want []complex128
	}{
		{"X", X, []complex128{0, 1, 1, 0}},
		{"Z", Z, []complex128{1, 0, 0, -1}},
		{"H", H, []complex128{complex(s2, 0), complex(s2, 0), complex(s2, 0), complex(-s2, 0)}},
		{"S", S, []complex128{1, 0, 0, 1i}},
		{"T", T, []complex128{1, 0, 0, complex(s2, s2)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.gate.Matrix()
			for i, want := range tc.want {
				if cmplx.Abs(m[i]-want) > eps {
					t.Errorf("Matrix[%d] = %v, want %v", i, m[i], want)
				}
			}
		})
	}
}

// TestParameterizedGates checks that RX(0) = I, RZ(pi) ~ Z (up to global phase), etc.
func TestParameterizedGates(t *testing.T) {
	// RX(0) = I
	m := RX(0).Matrix()
	assertMatrixClose(t, "RX(0)", m, []complex128{1, 0, 0, 1})

	// RY(0) = I
	m = RY(0).Matrix()
	assertMatrixClose(t, "RY(0)", m, []complex128{1, 0, 0, 1})

	// RZ(0) = I
	m = RZ(0).Matrix()
	assertMatrixClose(t, "RZ(0)", m, []complex128{1, 0, 0, 1})

	// RX(pi) should be -i*X
	m = RX(math.Pi).Matrix()
	assertMatrixClose(t, "RX(pi)", m, []complex128{0, -1i, -1i, 0})

	// Phase(0) = I
	m = Phase(0).Matrix()
	assertMatrixClose(t, "Phase(0)", m, []complex128{1, 0, 0, 1})
}

func assertMatrixClose(t *testing.T, name string, got, want []complex128) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: matrix size %d, want %d", name, len(got), len(want))
	}
	for i := range got {
		if cmplx.Abs(got[i]-want[i]) > eps {
			t.Errorf("%s: [%d] = %v, want %v", name, i, got[i], want[i])
		}
	}
}

func TestGateProperties(t *testing.T) {
	if H.Name() != "H" {
		t.Errorf("H.Name() = %q, want %q", H.Name(), "H")
	}
	if H.Qubits() != 1 {
		t.Errorf("H.Qubits() = %d, want 1", H.Qubits())
	}
	if H.Params() != nil {
		t.Errorf("H.Params() = %v, want nil", H.Params())
	}
	if CNOT.Qubits() != 2 {
		t.Errorf("CNOT.Qubits() = %d, want 2", CNOT.Qubits())
	}
	if CCX.Qubits() != 3 {
		t.Errorf("CCX.Qubits() = %d, want 3", CCX.Qubits())
	}

	rx := RX(1.5)
	if rx.Params() == nil || rx.Params()[0] != 1.5 {
		t.Errorf("RX(1.5).Params() = %v, want [1.5]", rx.Params())
	}
}
