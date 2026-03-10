package statevector

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/qgo/qasm/parser"
)

// TestEndToEnd_BellQASM: parse QASM → simulate → verify statevector.
func TestEndToEnd_BellQASM(t *testing.T) {
	c, err := parser.ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[2] q;
bit[2] c;
h q[0];
cx q[0], q[1];
c = measure q;
`)
	if err != nil {
		t.Fatal(err)
	}

	sim := New(c.NumQubits())
	counts, err := sim.Run(c, 10000)
	if err != nil {
		t.Fatal(err)
	}

	// Bell state: should only see "00" and "11".
	for k := range counts {
		if k != "00" && k != "11" {
			t.Errorf("unexpected outcome: %q", k)
		}
	}
}

// TestEndToEnd_GHZ3QASM: parse GHZ circuit → simulate.
func TestEndToEnd_GHZ3QASM(t *testing.T) {
	c, err := parser.ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[3] q;
bit[3] c;
h q[0];
cx q[0], q[1];
cx q[1], q[2];
c = measure q;
`)
	if err != nil {
		t.Fatal(err)
	}

	sim := New(c.NumQubits())
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2
	want := []complex128{complex(s2, 0), 0, 0, 0, 0, 0, 0, complex(s2, 0)}
	for i := range sv {
		if cmplx.Abs(sv[i]-want[i]) > 1e-10 {
			t.Errorf("sv[%d] = %v, want %v", i, sv[i], want[i])
		}
	}
}

// TestEndToEnd_QFT4QASM: parse QFT circuit from file, simulate, verify uniform magnitudes.
func TestEndToEnd_QFT4QASM(t *testing.T) {
	c, err := parser.ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[4] q;
bit[4] c;
x q[0];
x q[2];
barrier q;
h q[0];
cp(pi/2) q[1], q[0];
h q[1];
cp(pi/4) q[2], q[0];
cp(pi/2) q[2], q[1];
h q[2];
cp(pi/8) q[3], q[0];
cp(pi/4) q[3], q[1];
cp(pi/2) q[3], q[2];
h q[3];
c = measure q;
`)
	if err != nil {
		t.Fatal(err)
	}

	sim := New(c.NumQubits())
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// After QFT, all amplitudes should have equal magnitude 1/4.
	expectedMag := 0.25
	for i, v := range sv {
		mag := cmplx.Abs(v)
		if math.Abs(mag-expectedMag) > 1e-10 {
			t.Errorf("|sv[%d]| = %f, want %f", i, mag, expectedMag)
		}
	}
}

// TestEndToEnd_ParameterizedQASM: parse parameterized circuit and simulate.
func TestEndToEnd_ParameterizedQASM(t *testing.T) {
	c, err := parser.ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[2] q;
bit[2] c;
rx(pi/4) q[0];
ry(pi/3) q[0];
rz(pi/6) q[1];
cp(pi/2) q[0], q[1];
c = measure q;
`)
	if err != nil {
		t.Fatal(err)
	}

	sim := New(c.NumQubits())
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	// Just verify the statevector is normalized.
	sv := sim.StateVector()
	var norm float64
	for _, v := range sv {
		norm += real(v)*real(v) + imag(v)*imag(v)
	}
	if math.Abs(norm-1.0) > 1e-10 {
		t.Errorf("norm = %f, want 1.0", norm)
	}
}
