package emitter

import (
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/qasm/parser"
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
		if !contains(s, e) {
			t.Errorf("output missing %q\nFull output:\n%s", e, s)
		}
	}
}

func TestEmitParameterized(t *testing.T) {
	c, err := builder.New("param", 1).
		WithClbits(1).
		RZ(3.14159265358979/4, 0).
		Phase(3.14159265358979/2, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s, err := EmitString(c)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(s, "rz(") {
		t.Errorf("output missing rz gate\nFull output:\n%s", s)
	}
	if !contains(s, "p(") {
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

	if !contains(s, "if (c == 1) x") {
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
