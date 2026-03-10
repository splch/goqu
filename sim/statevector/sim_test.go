package statevector

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
)

const eps = 1e-10

func TestBellState(t *testing.T) {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2

	// |Φ+> = (|00> + |11>) / √2
	want := []complex128{complex(s2, 0), 0, 0, complex(s2, 0)}
	assertStateClose(t, sv, want)
}

func TestGHZ3(t *testing.T) {
	c, err := builder.New("ghz3", 3).
		H(0).
		CNOT(0, 1).
		CNOT(1, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(3)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2
	want := []complex128{complex(s2, 0), 0, 0, 0, 0, 0, 0, complex(s2, 0)}
	assertStateClose(t, sv, want)
}

func TestSingleX(t *testing.T) {
	c, err := builder.New("x", 1).X(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	want := []complex128{0, 1}
	assertStateClose(t, sv, want)
}

func TestSingleH(t *testing.T) {
	c, err := builder.New("h", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2
	want := []complex128{complex(s2, 0), complex(s2, 0)}
	assertStateClose(t, sv, want)
}

func TestHH_Identity(t *testing.T) {
	c, err := builder.New("hh", 1).H(0).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	want := []complex128{1, 0}
	assertStateClose(t, sv, want)
}

func TestQFT3_FromZero(t *testing.T) {
	// QFT on |000> should give uniform superposition.
	c, err := builder.New("qft3-zero", 3).
		H(0).
		Apply(gate.CP(math.Pi/2), 1, 0).
		Apply(gate.CP(math.Pi/4), 2, 0).
		H(1).
		Apply(gate.CP(math.Pi/2), 2, 1).
		H(2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(3)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	amp := 1.0 / math.Sqrt(8)
	for i, v := range sv {
		if math.Abs(cmplx.Abs(v)-amp) > eps {
			t.Errorf("|sv[%d]| = %f, want %f", i, cmplx.Abs(v), amp)
		}
	}
}

func TestMeasurementCounts(t *testing.T) {
	// Bell state should produce ~50% |00> and ~50% |11>.
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	counts, err := sim.Run(c, 10000)
	if err != nil {
		t.Fatal(err)
	}

	// Should only have "00" and "11" entries.
	for k := range counts {
		if k != "00" && k != "11" {
			t.Errorf("unexpected measurement outcome: %q", k)
		}
	}
	// Each should be roughly 5000 (±500 for statistical noise).
	c00 := counts["00"]
	c11 := counts["11"]
	if c00 < 4000 || c00 > 6000 {
		t.Errorf("counts[00] = %d, expected ~5000", c00)
	}
	if c11 < 4000 || c11 > 6000 {
		t.Errorf("counts[11] = %d, expected ~5000", c11)
	}
}

func TestCCX_Toffoli(t *testing.T) {
	// CCX flips target when both controls are |1>.
	// Start: |110> = X(1), X(2) on 3-qubit register -> index 6
	// CCX(2,1,0) should give |111> = index 7
	c, err := builder.New("toffoli", 3).
		X(1).
		X(2).
		CCX(2, 1, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(3)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// Should be |111> = index 7
	for i, v := range sv {
		if i == 7 {
			if cmplx.Abs(v-1) > eps {
				t.Errorf("sv[7] = %v, want 1", v)
			}
		} else {
			if cmplx.Abs(v) > eps {
				t.Errorf("sv[%d] = %v, want 0", i, v)
			}
		}
	}
}

func TestSWAP(t *testing.T) {
	// Start: |01> (X on qubit 0), SWAP(0,1) should give |10>
	c, err := builder.New("swap", 2).
		X(0).
		SWAP(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// |10> = index 2
	want := []complex128{0, 0, 1, 0}
	assertStateClose(t, sv, want)
}

func TestExpectationValue(t *testing.T) {
	// |0> state: <Z> = +1
	sim := New(1)
	c, err := builder.New("z0", 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	ev := sim.ExpectationValue([]int{0})
	if math.Abs(ev-1.0) > eps {
		t.Errorf("<Z>|0> = %f, want 1.0", ev)
	}

	// |1> state: <Z> = -1
	c, err = builder.New("z1", 1).X(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	ev = sim.ExpectationValue([]int{0})
	if math.Abs(ev-(-1.0)) > eps {
		t.Errorf("<Z>|1> = %f, want -1.0", ev)
	}

	// |+> state: <Z> = 0
	c, err = builder.New("z+", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	ev = sim.ExpectationValue([]int{0})
	if math.Abs(ev) > eps {
		t.Errorf("<Z>|+> = %f, want 0.0", ev)
	}
}

func assertStateClose(t *testing.T, got, want []complex128) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("state size %d, want %d", len(got), len(want))
	}
	for i := range got {
		if cmplx.Abs(got[i]-want[i]) > eps {
			t.Errorf("state[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func BenchmarkSimulate16(b *testing.B) {
	// Build a 16-qubit GHZ circuit.
	bld := builder.New("ghz16", 16)
	bld.H(0)
	for i := range 15 {
		bld.CNOT(i, i+1)
	}
	c, err := bld.Build()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		sim := New(16)
		sim.Evolve(c)
	}
}
