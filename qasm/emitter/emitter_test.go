package emitter

import (
	"math"
	"strings"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/qasm/parser"
)

func TestEmitBell(t *testing.T) {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if s == "" {
		t.Fatal("empty output")
	}

	// Verify it contains expected elements.
	expects := []string{
		"OPENQASM 3.0;",
		"qubit[2] q;",
		"bit[2] c;",
		"h q[0];",
		"cx q[0], q[1];",
	}
	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitParameterized(t *testing.T) {
	c, err := builder.New("param", 1).
		WithClbits(1).
		RZ(math.Pi/4, 0).
		Phase(math.Pi/2, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "rz(") {
		t.Errorf("output missing rz gate\nFull output:\n%s", s)
	}
	if !strings.Contains(s, "p(") {
		t.Errorf("output missing phase gate\nFull output:\n%s", s)
	}
}

func TestEmitConditional(t *testing.T) {
	c, err := parser.ParseString(`
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

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s, "if (c == 1) x") {
		t.Errorf("output missing conditional\nFull output:\n%s", s)
	}
}

func TestRoundTrip(t *testing.T) {
	sources := []string{
		`OPENQASM 3.0;
include "stdgates.inc";
qubit[2] q;
bit[2] c;
h q[0];
cx q[0], q[1];
c = measure q;`,
		`OPENQASM 3.0;
include "stdgates.inc";
qubit[3] q;
bit[3] c;
h q[0];
cx q[0], q[1];
cx q[1], q[2];
c = measure q;`,
	}

	for i, src := range sources {
		// Parse original.
		c1, err := parser.ParseString(src)
		if err != nil {
			t.Fatalf("source %d parse error: %v", i, err)
		}

		// Emit.
		emitted, err := EmitString(c1)
		if err != nil {
			t.Fatalf("source %d emit error: %v", i, err)
		}

		// Re-parse.
		c2, err := parser.ParseString(emitted)
		if err != nil {
			t.Fatalf("source %d re-parse error: %v\nEmitted:\n%s", i, err, emitted)
		}

		// Compare structure.
		if c1.NumQubits() != c2.NumQubits() {
			t.Errorf("source %d: NumQubits mismatch: %d vs %d", i, c1.NumQubits(), c2.NumQubits())
		}
		if c1.NumClbits() != c2.NumClbits() {
			t.Errorf("source %d: NumClbits mismatch: %d vs %d", i, c1.NumClbits(), c2.NumClbits())
		}
		if len(c1.Ops()) != len(c2.Ops()) {
			t.Errorf("source %d: Ops count mismatch: %d vs %d", i, len(c1.Ops()), len(c2.Ops()))
		}
	}
}

func TestEmitControlled(t *testing.T) {
	// Test that multi-controlled gates emit ctrl(N) @ syntax.
	c, err := builder.New("ctrl-test", 4).
		Apply(gate.MCX(3), 0, 1, 2, 3).
		Apply(gate.Controlled(gate.H, 2), 0, 1, 2).
		Apply(gate.MCZ(2), 0, 1, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	expects := []string{
		"ctrl(3) @ x q[0], q[1], q[2], q[3];",
		"ctrl(2) @ h q[0], q[1], q[2];",
		"ctrl(2) @ z q[0], q[1], q[2];",
	}
	for _, e := range expects {
		if !strings.Contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestControlledRoundTrip(t *testing.T) {
	// Build circuit with controlled gate, emit QASM, parse back, compare.
	c, err := builder.New("ctrl-rt", 4).
		Apply(gate.MCX(3), 0, 1, 2, 3).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	emitted, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	c2, err := parser.ParseString(emitted)
	if err != nil {
		t.Fatalf("re-parse error: %v\nEmitted:\n%s", err, emitted)
	}

	if c.NumQubits() != c2.NumQubits() {
		t.Errorf("NumQubits mismatch: %d vs %d", c.NumQubits(), c2.NumQubits())
	}
	if len(c.Ops()) != len(c2.Ops()) {
		t.Errorf("Ops count mismatch: %d vs %d", len(c.Ops()), len(c2.Ops()))
	}
	// Verify the gate is still a ControlledGate.
	ops2 := c2.Ops()
	if len(ops2) > 0 {
		if _, ok := ops2[0].Gate.(gate.ControlledGate); !ok {
			t.Error("parsed gate is not a ControlledGate")
		}
	}
}

func TestRoundTripParameterized(t *testing.T) {
	// Build a circuit with parameterized gates, emit, re-parse, verify.
	c, err := builder.New("roundtrip", 2).
		WithClbits(2).
		Apply(gate.RX(1.5707963267948966), 0).
		Apply(gate.CP(0.7853981633974483), 0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	emitted, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	c2, err := parser.ParseString(emitted)
	if err != nil {
		t.Fatalf("re-parse error: %v\nEmitted:\n%s", err, emitted)
	}

	if c.NumQubits() != c2.NumQubits() {
		t.Errorf("NumQubits mismatch: %d vs %d", c.NumQubits(), c2.NumQubits())
	}
	if len(c.Ops()) != len(c2.Ops()) {
		t.Errorf("Ops count mismatch: %d vs %d", len(c.Ops()), len(c2.Ops()))
	}
}

func TestEmitDelay(t *testing.T) {
	c, err := builder.New("delay", 1).
		Delay(0, 100, gate.UnitNs).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	out, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "delay[100ns]") {
		t.Errorf("expected delay[100ns] in output, got:\n%s", out)
	}

	// Round-trip.
	c2, err := parser.ParseString(out)
	if err != nil {
		t.Fatalf("re-parse error: %v\nEmitted:\n%s", err, out)
	}
	if len(c2.Ops()) != 1 || c2.Ops()[0].Gate.Name() != "delay" {
		t.Errorf("round-trip failed: %d ops", len(c2.Ops()))
	}
	d, ok := c2.Ops()[0].Gate.(gate.Delayable)
	if !ok {
		t.Fatal("round-tripped gate should implement Delayable")
	}
	if d.Duration() != 100 || d.Unit() != "ns" {
		t.Errorf("round-trip delay = (%g, %q), want (100, ns)", d.Duration(), d.Unit())
	}
}
