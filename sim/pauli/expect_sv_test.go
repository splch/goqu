package pauli

import (
	"math"
	"math/cmplx"
	"testing"
)

const eps = 1e-10

func assertNear(t *testing.T, label string, got, want complex128) {
	t.Helper()
	if cmplx.Abs(got-want) > eps {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func assertRealNear(t *testing.T, label string, got complex128, want float64) {
	t.Helper()
	if math.Abs(real(got)-want) > eps || math.Abs(imag(got)) > eps {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// Single-qubit states.
var (
	ket0      = []complex128{1, 0}
	ket1      = []complex128{0, 1}
	ketPlus   = []complex128{complex(1/math.Sqrt2, 0), complex(1/math.Sqrt2, 0)}
	ketMinus  = []complex128{complex(1/math.Sqrt2, 0), complex(-1/math.Sqrt2, 0)}
	ketPlusI  = []complex128{complex(1/math.Sqrt2, 0), complex(0, 1/math.Sqrt2)}  // (|0>+i|1>)/sqrt2
	ketMinusI = []complex128{complex(1/math.Sqrt2, 0), complex(0, -1/math.Sqrt2)} // (|0>-i|1>)/sqrt2
)

func TestExpect_Z(t *testing.T) {
	zOp, _ := Parse("Z")

	// |0>: <Z> = +1
	assertRealNear(t, "<Z>|0>", Expect(ket0, zOp), 1)
	// |1>: <Z> = -1
	assertRealNear(t, "<Z>|1>", Expect(ket1, zOp), -1)
	// |+>: <Z> = 0
	assertRealNear(t, "<Z>|+>", Expect(ketPlus, zOp), 0)
}

func TestExpect_X(t *testing.T) {
	xOp, _ := Parse("X")

	// |+>: <X> = +1
	assertRealNear(t, "<X>|+>", Expect(ketPlus, xOp), 1)
	// |->: <X> = -1
	assertRealNear(t, "<X>|->", Expect(ketMinus, xOp), -1)
	// |0>: <X> = 0
	assertRealNear(t, "<X>|0>", Expect(ket0, xOp), 0)
}

func TestExpect_Y(t *testing.T) {
	yOp, _ := Parse("Y")

	// |+i>: <Y> = +1
	assertRealNear(t, "<Y>|+i>", Expect(ketPlusI, yOp), 1)
	// |-i>: <Y> = -1
	assertRealNear(t, "<Y>|-i>", Expect(ketMinusI, yOp), -1)
	// |0>: <Y> = 0
	assertRealNear(t, "<Y>|0>", Expect(ket0, yOp), 0)
}

func TestExpect_Identity(t *testing.T) {
	iOp, _ := Parse("I")
	assertRealNear(t, "<I>|0>", Expect(ket0, iOp), 1)
	assertRealNear(t, "<I>|+>", Expect(ketPlus, iOp), 1)

	// Multi-qubit identity.
	iiOp, _ := Parse("II")
	bell := bellPhi()
	assertRealNear(t, "<II>|phi+>", Expect(bell, iiOp), 1)
}

// bellPhi returns |Φ+> = (|00>+|11>)/sqrt2.
func bellPhi() []complex128 {
	s2 := 1 / math.Sqrt2
	return []complex128{complex(s2, 0), 0, 0, complex(s2, 0)}
}

func TestExpect_Bell_ZZ(t *testing.T) {
	bell := bellPhi()
	zz, _ := Parse("ZZ")
	// <ZZ>|Φ+> = +1
	assertRealNear(t, "<ZZ>|phi+>", Expect(bell, zz), 1)
}

func TestExpect_Bell_XX(t *testing.T) {
	bell := bellPhi()
	xx, _ := Parse("XX")
	// <XX>|Φ+> = +1
	assertRealNear(t, "<XX>|phi+>", Expect(bell, xx), 1)
}

func TestExpect_Bell_YY(t *testing.T) {
	bell := bellPhi()
	yy, _ := Parse("YY")
	// <YY>|Φ+> = -1
	assertRealNear(t, "<YY>|phi+>", Expect(bell, yy), -1)
}

func TestExpectSum(t *testing.T) {
	// |00>: <0.5*ZZ + 0.3*XI> = 0.5*1 + 0.3*0 = 0.5
	ket00 := []complex128{1, 0, 0, 0}
	zz := NewPauliString(0.5, map[int]Pauli{0: Z, 1: Z}, 2)
	xi := NewPauliString(0.3, map[int]Pauli{0: X}, 2)
	sum, err := NewPauliSum([]PauliString{zz, xi})
	if err != nil {
		t.Fatal(err)
	}
	assertRealNear(t, "<0.5*ZZ+0.3*XI>|00>", ExpectSum(ket00, sum), 0.5)
}

func TestExpect_CoeffScaling(t *testing.T) {
	// 2.0 * Z on |0>: should be 2.0
	ps := NewPauliString(2.0, map[int]Pauli{0: Z}, 1)
	assertRealNear(t, "<2Z>|0>", Expect(ket0, ps), 2.0)
}

func TestExpect_EmptyState(t *testing.T) {
	ps, _ := Parse("Z")
	got := Expect(nil, ps)
	if got != 0 {
		t.Errorf("Expect(nil) = %v, want 0", got)
	}
}

func TestExpect_ZOnly_MatchesParity(t *testing.T) {
	// Verify Z-only PauliString matches the parity-counting algorithm.
	// Bell state |Φ+>: <Z0> = 0, <Z1> = 0, <Z0Z1> = 1
	bell := bellPhi()

	z0 := ZOn([]int{0}, 2)
	z1 := ZOn([]int{1}, 2)
	z01 := ZOn([]int{0, 1}, 2)

	assertRealNear(t, "<Z0>|phi+>", Expect(bell, z0), 0)
	assertRealNear(t, "<Z1>|phi+>", Expect(bell, z1), 0)
	assertRealNear(t, "<Z0Z1>|phi+>", Expect(bell, z01), 1)
}

func TestExpect_Hermitian(t *testing.T) {
	// All single Pauli expectations on real states should be real.
	for _, op := range []string{"X", "Y", "Z"} {
		ps, _ := Parse(op)
		for _, st := range [][]complex128{ket0, ket1, ketPlus, ketMinus} {
			got := Expect(st, ps)
			if math.Abs(imag(got)) > eps {
				t.Errorf("<%s> has nonzero imag: %v", op, got)
			}
		}
	}
}

func TestExpect_XY_Bell(t *testing.T) {
	bell := bellPhi()
	xy, _ := Parse("XY")
	// <XY>|Φ+> = 0
	assertRealNear(t, "<XY>|phi+>", Expect(bell, xy), 0)
}

func TestExpect_ZI_Bell(t *testing.T) {
	bell := bellPhi()
	zi, _ := Parse("ZI")
	// <ZI>|Φ+> = <Z0>|Φ+> = 0
	assertRealNear(t, "<ZI>|phi+>", Expect(bell, zi), 0)
}
