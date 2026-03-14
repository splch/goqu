//go:build cuda

package cuda

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/sim/statevector"
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

	sim, err := New(2)
	if err != nil {
		t.Fatal(err)
	}
	defer sim.Close()

	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2
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

	sim, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	defer sim.Close()

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

	sim, err := New(1)
	if err != nil {
		t.Fatal(err)
	}
	defer sim.Close()

	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	want := []complex128{0, 1}
	assertStateClose(t, sv, want)
}

func TestMeasurementCounts(t *testing.T) {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim, err := New(2)
	if err != nil {
		t.Fatal(err)
	}
	defer sim.Close()

	counts, err := sim.Run(c, 10000)
	if err != nil {
		t.Fatal(err)
	}

	for k := range counts {
		if k != "00" && k != "11" {
			t.Errorf("unexpected measurement outcome: %q", k)
		}
	}
	c00 := counts["00"]
	c11 := counts["11"]
	if c00 < 4000 || c00 > 6000 {
		t.Errorf("counts[00] = %d, expected ~5000", c00)
	}
	if c11 < 4000 || c11 > 6000 {
		t.Errorf("counts[11] = %d, expected ~5000", c11)
	}
}

func TestSWAP(t *testing.T) {
	c, err := builder.New("swap", 2).
		X(0).
		SWAP(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim, err := New(2)
	if err != nil {
		t.Fatal(err)
	}
	defer sim.Close()

	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	want := []complex128{0, 0, 1, 0}
	assertStateClose(t, sv, want)
}

func TestCCX_Toffoli(t *testing.T) {
	c, err := builder.New("toffoli", 3).
		X(1).
		X(2).
		CCX(2, 1, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	defer sim.Close()

	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
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

// TestCrossValidation runs random circuits on both CPU and GPU, asserting state vectors match.
func TestCrossValidation(t *testing.T) {
	gates1Q := []func(int) *builder.Builder{
		func(q int) *builder.Builder { return builder.New("", 4).H(q) },
		func(q int) *builder.Builder { return builder.New("", 4).X(q) },
		func(q int) *builder.Builder { return builder.New("", 4).Y(q) },
		func(q int) *builder.Builder { return builder.New("", 4).Z(q) },
		func(q int) *builder.Builder { return builder.New("", 4).S(q) },
		func(q int) *builder.Builder { return builder.New("", 4).T(q) },
	}

	for i, gf := range gates1Q {
		bld := gf(i % 4)
		bld.CNOT(0, 1)
		bld.H(2)
		bld.CNOT(2, 3)

		c, err := bld.Build()
		if err != nil {
			t.Fatal(err)
		}

		cpuSim := statevector.New(4)
		if err := cpuSim.Evolve(c); err != nil {
			t.Fatal(err)
		}
		cpuSV := cpuSim.StateVector()

		gpuSim, err := New(4)
		if err != nil {
			t.Fatal(err)
		}
		if err := gpuSim.Evolve(c); err != nil {
			gpuSim.Close()
			t.Fatal(err)
		}
		gpuSV := gpuSim.StateVector()
		gpuSim.Close()

		assertStateClose(t, gpuSV, cpuSV)
	}
}

// TestQFT3 verifies QFT on |000> produces uniform superposition.
func TestQFT3(t *testing.T) {
	c, err := builder.New("qft3", 3).
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

	sim, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	defer sim.Close()

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
