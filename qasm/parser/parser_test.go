package parser

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/splch/goqu/circuit/gate"
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
		t.Errorf("Condition = %+v, want {Register:c, Value:1}", xOp.Condition)
	}
	if xOp.Condition.Clbit != 0 {
		t.Errorf("Condition.Clbit = %d, want 0", xOp.Condition.Clbit)
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

func TestParseDelay(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[2] q;
delay[100ns] q[0];
delay[2.5us] q[1];
h q[0];
`)
	if err != nil {
		t.Fatal(err)
	}
	ops := c.Ops()
	if ops[0].Gate.Name() != "delay" {
		t.Errorf("Ops[0].Gate.Name() = %q, want delay", ops[0].Gate.Name())
	}
	d0, ok := ops[0].Gate.(gate.Delayable)
	if !ok {
		t.Fatal("delay gate should implement Delayable")
	}
	if d0.Duration() != 100 || d0.Unit() != "ns" {
		t.Errorf("delay[0] = (%g, %q), want (100, ns)", d0.Duration(), d0.Unit())
	}
	d1, ok := ops[1].Gate.(gate.Delayable)
	if !ok {
		t.Fatal("delay gate should implement Delayable")
	}
	if d1.Duration() != 2.5 || d1.Unit() != "us" {
		t.Errorf("delay[1] = (%g, %q), want (2.5, us)", d1.Duration(), d1.Unit())
	}
}

func TestParseDelayDefaultUnit(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
delay[10] q[0];
`)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := c.Ops()[0].Gate.(gate.Delayable)
	if !ok {
		t.Fatal("delay gate should implement Delayable")
	}
	if d.Unit() != "dt" {
		t.Errorf("default unit = %q, want dt", d.Unit())
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
	if math.Abs(p2-math.Pi*math.Pi) > 1e-10 {
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

func TestParse_NegativeRegisterIndex(t *testing.T) {
	_, err := ParseString(`
OPENQASM 3.0;
qubit[2] q;
h q[-1];
`)
	if err == nil {
		t.Fatal("expected error for negative qubit index")
	}
}

func TestParse_ImplicitMeasureClbitOverflow(t *testing.T) {
	_, err := ParseString(`
OPENQASM 3.0;
qubit[3] q;
bit[1] c;
measure q;
`)
	if err == nil {
		t.Fatal("expected error for implicit measurement with insufficient classical bits")
	}
}

func TestParse_DivisionByZeroInExpr(t *testing.T) {
	_, err := ParseString(`
OPENQASM 3.0;
qubit[1] q;
rx(1/0) q[0];
`)
	if err == nil {
		t.Fatal("expected error for division by zero")
	}
}

func TestParse_MissingSemicolon(t *testing.T) {
	_, err := ParseString(`
OPENQASM 3.0;
qubit[1] q
h q[0];
`)
	if err == nil {
		t.Fatal("expected error for missing semicolon")
	}
}

func TestParse_ImplicitMeasureValid(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
qubit[2] q;
bit[2] c;
measure q;
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2 measurement ops expected.
	if len(c.Ops()) != 2 {
		t.Errorf("len(Ops) = %d, want 2", len(c.Ops()))
	}
}

func TestParse_PowModifier(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
pow(2) @ h q[0];
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Ops()) != 1 {
		t.Fatalf("len(Ops) = %d, want 1", len(c.Ops()))
	}
	// H^2 = I, so the gate should be approximately identity.
	m := c.Ops()[0].Gate.Matrix()
	if len(m) != 4 {
		t.Fatalf("matrix len = %d, want 4", len(m))
	}
	// Check diagonal ~1, off-diagonal ~0.
	const tol = 1e-10
	if math.Abs(real(m[0])-1) > tol || math.Abs(real(m[3])-1) > tol {
		t.Errorf("pow(2) @ h diagonal = (%v, %v), want (1, 1)", m[0], m[3])
	}
	if math.Abs(real(m[1])) > tol || math.Abs(real(m[2])) > tol {
		t.Errorf("pow(2) @ h off-diagonal = (%v, %v), want (0, 0)", m[1], m[2])
	}
}

func TestParse_PowZero(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
pow(0) @ x q[0];
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Ops()) != 1 {
		t.Fatalf("len(Ops) = %d, want 1", len(c.Ops()))
	}
	// X^0 = I
	m := c.Ops()[0].Gate.Matrix()
	const tol = 1e-10
	if math.Abs(real(m[0])-1) > tol || math.Abs(real(m[3])-1) > tol {
		t.Errorf("pow(0) @ x should be identity")
	}
}

func TestParse_NegctrlModifier(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[2] q;
negctrl @ x q[0], q[1];
`)
	if err != nil {
		t.Fatal(err)
	}
	// negctrl @ x => X(q0), CNOT(q0,q1), X(q0)
	ops := c.Ops()
	if len(ops) != 3 {
		t.Fatalf("len(Ops) = %d, want 3 (X + CNOT + X)", len(ops))
	}
	// First and last ops should be X on q[0]
	if ops[0].Gate.Name() != "X" || ops[0].Qubits[0] != 0 {
		t.Errorf("ops[0] = %s on %v, want X on [0]", ops[0].Gate.Name(), ops[0].Qubits)
	}
	if ops[2].Gate.Name() != "X" || ops[2].Qubits[0] != 0 {
		t.Errorf("ops[2] = %s on %v, want X on [0]", ops[2].Gate.Name(), ops[2].Qubits)
	}
	// Middle op should be CNOT on q[0], q[1]
	if ops[1].Gate.Name() != "CNOT" {
		t.Errorf("ops[1].Gate.Name() = %q, want CNOT", ops[1].Gate.Name())
	}
}

func TestParse_CtrlNegctrlMixed(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[3] q;
ctrl @ negctrl @ x q[0], q[1], q[2];
`)
	if err != nil {
		t.Fatal(err)
	}
	// ctrl @ negctrl @ x => X(q1), CCX(q0,q1,q2), X(q1)
	ops := c.Ops()
	if len(ops) != 3 {
		t.Fatalf("len(Ops) = %d, want 3 (X + CCX + X)", len(ops))
	}
	// X sandwich on q[1] (the negctrl qubit, position 1)
	if ops[0].Gate.Name() != "X" || ops[0].Qubits[0] != 1 {
		t.Errorf("ops[0] = %s on %v, want X on [1]", ops[0].Gate.Name(), ops[0].Qubits)
	}
	if ops[2].Gate.Name() != "X" || ops[2].Qubits[0] != 1 {
		t.Errorf("ops[2] = %s on %v, want X on [1]", ops[2].Gate.Name(), ops[2].Qubits)
	}
	// Middle should be CCX (Toffoli)
	if ops[1].Gate.Name() != "CCX" {
		t.Errorf("ops[1].Gate.Name() = %q, want CCX", ops[1].Gate.Name())
	}
}

func TestParse_PowNonIntegerError(t *testing.T) {
	_, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
pow(0.5) @ h q[0];
`)
	if err == nil {
		t.Fatal("expected error for non-integer pow exponent")
	}
}

func TestParse_PowWithCtrl(t *testing.T) {
	c, err := ParseString(`
OPENQASM 3.0;
include "stdgates.inc";
qubit[2] q;
ctrl @ pow(2) @ s q[0], q[1];
`)
	if err != nil {
		t.Fatal(err)
	}
	// pow(2) @ s = Z, then ctrl @ Z = CZ (matrix-equivalent)
	ops := c.Ops()
	if len(ops) != 1 {
		t.Fatalf("len(Ops) = %d, want 1", len(ops))
	}
	// Verify the matrix matches CZ: diag(1, 1, 1, -1)
	m := ops[0].Gate.Matrix()
	const tol = 1e-10
	want := []complex128{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, -1}
	for i := range want {
		if math.Abs(real(m[i])-real(want[i])) > tol || math.Abs(imag(m[i])-imag(want[i])) > tol {
			t.Errorf("ctrl @ pow(2) @ s matrix[%d] = %v, want %v", i, m[i], want[i])
		}
	}
}
