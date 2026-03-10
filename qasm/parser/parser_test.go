package parser

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestParseBell(t *testing.T) {
	c, err := ParseString(`
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
	if c.NumQubits() != 2 {
		t.Errorf("NumQubits = %d, want 2", c.NumQubits())
	}
	if c.NumClbits() != 2 {
		t.Errorf("NumClbits = %d, want 2", c.NumClbits())
	}
	// h + cx + 2 measurements = 4 ops
	if len(c.Ops()) != 4 {
		t.Errorf("len(Ops) = %d, want 4", len(c.Ops()))
	}
	if c.Ops()[0].Gate.Name() != "H" {
		t.Errorf("Ops[0].Gate.Name() = %q, want H", c.Ops()[0].Gate.Name())
	}
	if c.Ops()[1].Gate.Name() != "CNOT" {
		t.Errorf("Ops[1].Gate.Name() = %q, want CNOT", c.Ops()[1].Gate.Name())
	}
}

func TestParseLegacySyntax(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qreg q[2];
creg c[2];
h q[0];
cx q[0], q[1];
measure q -> c;
`)
	if err != nil {
		t.Fatal(err)
	}
	if c.NumQubits() != 2 {
		t.Errorf("NumQubits = %d, want 2", c.NumQubits())
	}
	// h + cx + 2 measurements = 4 ops
	if len(c.Ops()) != 4 {
		t.Errorf("len(Ops) = %d, want 4", len(c.Ops()))
	}
}

func TestParseParameterized(t *testing.T) {
	c, err := ParseString(`
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
	// rx + ry + rz + cp + 2 measurements = 6 ops
	if len(c.Ops()) != 6 {
		t.Errorf("len(Ops) = %d, want 6", len(c.Ops()))
	}
	// Check rx parameter.
	rxParams := c.Ops()[0].Gate.Params()
	if rxParams == nil || math.Abs(rxParams[0]-math.Pi/4) > 1e-10 {
		t.Errorf("rx param = %v, want pi/4", rxParams)
	}
}

func TestParseConditional(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[2] q;
bit[2] c;
h q[0];
c[0] = measure q[0];
if (c == 1) {
    x q[1];
}
c[1] = measure q[1];
`)
	if err != nil {
		t.Fatal(err)
	}
	// h + measure + conditional x + measure = 4 ops
	if len(c.Ops()) != 4 {
		t.Errorf("len(Ops) = %d, want 4", len(c.Ops()))
	}
	// The conditional X should have a condition.
	xOp := c.Ops()[2]
	if xOp.Gate.Name() != "X" {
		t.Errorf("Ops[2].Gate.Name() = %q, want X", xOp.Gate.Name())
	}
	if xOp.Condition == nil {
		t.Fatal("Ops[2].Condition is nil, want non-nil")
	}
	if xOp.Condition.Register != "c" || xOp.Condition.Value != 1 {
		t.Errorf("Condition = %+v, want {c, 1}", xOp.Condition)
	}
}

func TestParseGateDecl(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
gate mygate a, b {
    h a;
    cx a, b;
}
qubit[2] q;
bit[2] c;
mygate q[0], q[1];
c = measure q;
`)
	if err != nil {
		t.Fatal(err)
	}
	// mygate (opaque) + 2 measurements = 3 ops
	if len(c.Ops()) != 3 {
		t.Errorf("len(Ops) = %d, want 3", len(c.Ops()))
	}
	if c.Ops()[0].Gate.Name() != "mygate" {
		t.Errorf("Ops[0].Gate.Name() = %q, want mygate", c.Ops()[0].Gate.Name())
	}
}

func TestParseReset(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[2] q;
reset q;
h q[0];
`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Ops()[0].Gate.Name() != "reset" {
		t.Errorf("Ops[0].Gate.Name() = %q, want reset", c.Ops()[0].Gate.Name())
	}
}

func TestParseBarrier(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[3] q;
h q[0];
barrier q;
cx q[0], q[1];
`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Ops()[1].Gate.Name() != "barrier" {
		t.Errorf("Ops[1].Gate.Name() = %q, want barrier", c.Ops()[1].Gate.Name())
	}
}

func TestParseExpressions(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
rx(2 * pi / 3) q[0];
ry(-pi / 4) q[0];
rz(pi ** 2) q[0];
`)
	if err != nil {
		t.Fatal(err)
	}
	ops := c.Ops()

	p0 := ops[0].Gate.Params()[0]
	if math.Abs(p0-2*math.Pi/3) > 1e-10 {
		t.Errorf("rx param = %v, want 2*pi/3", p0)
	}

	p1 := ops[1].Gate.Params()[0]
	if math.Abs(p1-(-math.Pi/4)) > 1e-10 {
		t.Errorf("ry param = %v, want -pi/4", p1)
	}

	p2 := ops[2].Gate.Params()[0]
	if math.Abs(p2-math.Pow(math.Pi, 2)) > 1e-10 {
		t.Errorf("rz param = %v, want pi^2", p2)
	}
}

func TestParseTrigFunctions(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
rx(cos(0)) q[0];
ry(sin(pi/2)) q[0];
rz(sqrt(2)) q[0];
`)
	if err != nil {
		t.Fatal(err)
	}
	ops := c.Ops()

	if math.Abs(ops[0].Gate.Params()[0]-1.0) > 1e-10 {
		t.Errorf("cos(0) = %v, want 1.0", ops[0].Gate.Params()[0])
	}
	if math.Abs(ops[1].Gate.Params()[0]-1.0) > 1e-10 {
		t.Errorf("sin(pi/2) = %v, want 1.0", ops[1].Gate.Params()[0])
	}
	if math.Abs(ops[2].Gate.Params()[0]-math.Sqrt(2)) > 1e-10 {
		t.Errorf("sqrt(2) = %v, want %v", ops[2].Gate.Params()[0], math.Sqrt(2))
	}
}

func TestParseQASMFiles(t *testing.T) {
	files, err := filepath.Glob("../testdata/*.qasm")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no test QASM files found")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			c, err := ParseString(string(data))
			if err != nil {
				t.Fatal(err)
			}
			if c.NumQubits() == 0 {
				t.Error("parsed circuit has 0 qubits")
			}
		})
	}
}

func TestParseComments(t *testing.T) {
	c, err := ParseString(`
// This is a comment
OPENQASM 3.0;
include "stdgates.inc";
/* Block comment */
qubit[1] q;
h q[0]; // inline comment
`)
	if err != nil {
		t.Fatal(err)
	}
	if c.NumQubits() != 1 {
		t.Errorf("NumQubits = %d, want 1", c.NumQubits())
	}
}

func TestParseU3Gate(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
U(0.3, 0.2, 0.1) q[0];
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Ops()) != 1 {
		t.Fatalf("len(Ops) = %d, want 1", len(c.Ops()))
	}
	params := c.Ops()[0].Gate.Params()
	if len(params) != 3 {
		t.Fatalf("params len = %d, want 3", len(params))
	}
	if math.Abs(params[0]-0.3) > 1e-10 || math.Abs(params[1]-0.2) > 1e-10 || math.Abs(params[2]-0.1) > 1e-10 {
		t.Errorf("U3 params = %v, want [0.3, 0.2, 0.1]", params)
	}
}

func TestParseErrorUndefinedRegister(t *testing.T) {
	_, err := ParseString(`
OPENQASM 3.0;
qubit[1] q;
h r[0];
`)
	if err == nil {
		t.Fatal("expected error for undefined register")
	}
}

func TestParseErrorBadDesignator(t *testing.T) {
	_, err := ParseString(`
OPENQASM 3.0;
qubit[0] q;
`)
	if err == nil {
		t.Fatal("expected error for zero-size register")
	}
}
