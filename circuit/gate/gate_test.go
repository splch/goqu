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
		ISWAP, ECR, DCX, CH, CSX, CCZ, Sycamore,
		RX(math.Pi / 4), RY(math.Pi / 3), RZ(math.Pi / 6),
		Phase(math.Pi / 4), U3(math.Pi/4, math.Pi/3, math.Pi/6),
		U1(math.Pi / 4), U2(math.Pi/4, math.Pi/6),
		Rot(math.Pi/4, math.Pi/3, math.Pi/6),
		PhasedXZ(0.5, 0.3, 0.1), GlobalPhase(math.Pi / 4),
		CP(math.Pi / 4), CRZ(math.Pi / 3), CRX(math.Pi / 4), CRY(math.Pi / 5),
		RXX(math.Pi / 4), RYY(math.Pi / 3), RZZ(math.Pi / 6),
		FSim(math.Pi/4, math.Pi/6), PSwap(math.Pi / 4),
		GPI(math.Pi / 4), GPI2(math.Pi / 3), MS(math.Pi/4, math.Pi/6), ZZ(math.Pi / 4),
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
		CNOT, CZ, SWAP, CY, CCX, CSWAP,
		ISWAP, ECR, DCX, CH, CSX, CCZ, Sycamore,
		RX(math.Pi / 4), RY(math.Pi / 3), RZ(math.Pi / 6),
		U1(math.Pi / 4), U2(math.Pi/4, math.Pi/6),
		Rot(math.Pi/4, math.Pi/3, math.Pi/6),
		PhasedXZ(0.5, 0.3, 0.1), GlobalPhase(math.Pi / 4),
		RXX(math.Pi / 4), RYY(math.Pi / 3), RZZ(math.Pi / 6),
		FSim(math.Pi/4, math.Pi/6), PSwap(math.Pi / 4),
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
		{"iSWAP", ISWAP, []complex128{
			1, 0, 0, 0,
			0, 0, 1i, 0,
			0, 1i, 0, 0,
			0, 0, 0, 1,
		}},
		{"ECR", ECR, []complex128{
			0, 0, complex(s2, 0), complex(0, s2),
			0, 0, complex(0, s2), complex(s2, 0),
			complex(s2, 0), complex(0, -s2), 0, 0,
			complex(0, -s2), complex(s2, 0), 0, 0,
		}},
		{"DCX", DCX, []complex128{
			1, 0, 0, 0,
			0, 0, 0, 1,
			0, 1, 0, 0,
			0, 0, 1, 0,
		}},
		{"CCZ[7,7]", CCZ, []complex128{
			1, 0, 0, 0, 0, 0, 0, 0,
			0, 1, 0, 0, 0, 0, 0, 0,
			0, 0, 1, 0, 0, 0, 0, 0,
			0, 0, 0, 1, 0, 0, 0, 0,
			0, 0, 0, 0, 1, 0, 0, 0,
			0, 0, 0, 0, 0, 1, 0, 0,
			0, 0, 0, 0, 0, 0, 1, 0,
			0, 0, 0, 0, 0, 0, 0, -1,
		}},
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

	// RXX(0) = I (4x4)
	m = RXX(0).Matrix()
	assertMatrixClose(t, "RXX(0)", m, []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})

	// RYY(0) = I (4x4)
	m = RYY(0).Matrix()
	assertMatrixClose(t, "RYY(0)", m, []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})

	// RZZ(0) = I (4x4)
	m = RZZ(0).Matrix()
	assertMatrixClose(t, "RZZ(0)", m, []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})

	// RZZ(pi) = diag(-i, i, i, -i)
	m = RZZ(math.Pi).Matrix()
	assertMatrixClose(t, "RZZ(pi)", m, []complex128{
		-1i, 0, 0, 0,
		0, 1i, 0, 0,
		0, 0, 1i, 0,
		0, 0, 0, -1i,
	})

	// U1(0) = I
	m = U1(0).Matrix()
	assertMatrixClose(t, "U1(0)", m, []complex128{1, 0, 0, 1})

	// U1(lambda) should match Phase(lambda)
	m = U1(math.Pi / 4).Matrix()
	mp := Phase(math.Pi / 4).Matrix()
	assertMatrixClose(t, "U1==Phase", m, mp)

	// U2(0,0) = (1/sqrt2) * [[1, -1], [1, 1]]
	s2v := 1.0 / math.Sqrt2
	m = U2(0, 0).Matrix()
	assertMatrixClose(t, "U2(0,0)", m, []complex128{
		complex(s2v, 0), complex(-s2v, 0),
		complex(s2v, 0), complex(s2v, 0),
	})

	// Rot(0,0,0) = I
	m = Rot(0, 0, 0).Matrix()
	assertMatrixClose(t, "Rot(0,0,0)", m, []complex128{1, 0, 0, 1})

	// GlobalPhase(0) = I
	m = GlobalPhase(0).Matrix()
	assertMatrixClose(t, "GlobalPhase(0)", m, []complex128{1, 0, 0, 1})

	// FSim(0,0) = I
	m = FSim(0, 0).Matrix()
	assertMatrixClose(t, "FSim(0,0)", m, []complex128{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})

	// FSim(pi/2, pi/6) should match Sycamore matrix
	m = FSim(math.Pi/2, math.Pi/6).Matrix()
	assertMatrixClose(t, "FSim==Sycamore", m, Sycamore.Matrix())

	// PSwap(0) = SWAP
	m = PSwap(0).Matrix()
	assertMatrixClose(t, "PSwap(0)", m, SWAP.Matrix())
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

// TestCCZMatchesMCZ2 verifies CCZ matrix matches Controlled(Z, 2).
func TestCCZMatchesMCZ2(t *testing.T) {
	assertMatrixClose(t, "CCZ==MCZ(2)", CCZ.Matrix(), MCZ(2).Matrix())
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

func TestDelay(t *testing.T) {
	d := Delay(100, UnitNs)
	if d.Name() != "delay" {
		t.Errorf("Name() = %q, want delay", d.Name())
	}
	if d.Qubits() != 1 {
		t.Errorf("Qubits() = %d, want 1", d.Qubits())
	}
	if d.Matrix() != nil {
		t.Error("Matrix() should be nil for delay")
	}
	if p := d.Params(); len(p) != 1 || p[0] != 100 {
		t.Errorf("Params() = %v, want [100]", p)
	}
	if d.Inverse().Name() != "delay" {
		t.Error("Inverse() should return self")
	}
	if d.Decompose([]int{0}) != nil {
		t.Error("Decompose() should return nil")
	}

	// Delayable interface.
	dl, ok := d.(Delayable)
	if !ok {
		t.Fatal("Delay gate should implement Delayable")
	}
	if dl.Duration() != 100 {
		t.Errorf("Duration() = %g, want 100", dl.Duration())
	}
	if dl.Unit() != UnitNs {
		t.Errorf("Unit() = %q, want %q", dl.Unit(), UnitNs)
	}
	if math.Abs(dl.Seconds()-100e-9) > 1e-20 {
		t.Errorf("Seconds() = %g, want 1e-7", dl.Seconds())
	}
}

func TestDelaySeconds(t *testing.T) {
	tests := []struct {
		dur  float64
		unit string
		want float64
	}{
		{1, UnitS, 1},
		{1, UnitMs, 1e-3},
		{1, UnitUs, 1e-6},
		{1, UnitNs, 1e-9},
	}
	for _, tc := range tests {
		d := Delay(tc.dur, tc.unit).(Delayable)
		if math.Abs(d.Seconds()-tc.want) > 1e-20 {
			t.Errorf("Delay(%g, %q).Seconds() = %g, want %g", tc.dur, tc.unit, d.Seconds(), tc.want)
		}
	}
}

func TestDelaySecondsPanicDt(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Delay with dt unit should panic on Seconds()")
		}
	}()
	Delay(1, UnitDt).(Delayable).Seconds()
}
