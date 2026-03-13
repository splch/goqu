//go:build metal

package metal

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/goqu/circuit/builder"
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
