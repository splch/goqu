//go:build darwin

package metal

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/statevector"
)

// Metal uses float32 precision; allow ~1e-5 tolerance.
const eps = 1e-5

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

func TestCrossValidation(t *testing.T) {
	bld := builder.New("cross", 4)
	bld.H(0)
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
	defer gpuSim.Close()

	if err := gpuSim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	gpuSV := gpuSim.StateVector()

	assertStateClose(t, gpuSV, cpuSV)
}

func TestReversedQubits(t *testing.T) {
	// CNOT with control > target to test qubit-swap path.
	c, err := builder.New("rev", 3).
		H(2).
		CNOT(2, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	cpuSim := statevector.New(3)
	if err := cpuSim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	gpuSim, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	defer gpuSim.Close()

	if err := gpuSim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	assertStateClose(t, gpuSim.StateVector(), cpuSim.StateVector())
}

func TestRun(t *testing.T) {
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

	counts, err := sim.Run(c, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// Bell state should only produce "00" and "11".
	for bs := range counts {
		if bs != "00" && bs != "11" {
			t.Errorf("unexpected bitstring %q", bs)
		}
	}
	if counts["00"]+counts["11"] != 1000 {
		t.Errorf("total shots = %d, want 1000", counts["00"]+counts["11"])
	}
}

func TestGHZ(t *testing.T) {
	// 5-qubit GHZ state: (|00000> + |11111>) / sqrt(2)
	bld := builder.New("ghz", 5)
	bld.H(0)
	for i := range 4 {
		bld.CNOT(i, i+1)
	}
	c, err := bld.Build()
	if err != nil {
		t.Fatal(err)
	}

	cpuSim := statevector.New(5)
	if err := cpuSim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	gpuSim, err := New(5)
	if err != nil {
		t.Fatal(err)
	}
	defer gpuSim.Close()

	if err := gpuSim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	assertStateClose(t, gpuSim.StateVector(), cpuSim.StateVector())
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
