package parser

import (
	"testing"

	"github.com/splch/goqu/circuit/ir"
)

func TestParseWhile(t *testing.T) {
	src := `OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
bit[1] c;

h q[0];
c[0] = measure q[0];
while (c == 1) {
  x q[0];
  c[0] = measure q[0];
}
`
	c, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	// h, measure, while
	found := false
	for _, op := range ops {
		if op.ControlFlow != nil && op.ControlFlow.Type == ir.ControlFlowWhile {
			found = true
			if len(op.ControlFlow.Bodies[0]) != 2 {
				t.Errorf("while body has %d ops, want 2", len(op.ControlFlow.Bodies[0]))
			}
		}
	}
	if !found {
		t.Error("no while control flow found in parsed circuit")
	}
}

func TestParseFor(t *testing.T) {
	src := `OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;

for int i in [0:3] {
  x q[0];
}
`
	c, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	found := false
	for _, op := range ops {
		if op.ControlFlow != nil && op.ControlFlow.Type == ir.ControlFlowFor {
			found = true
			r := op.ControlFlow.ForRange
			if r.Start != 0 || r.End != 3 || r.Step != 1 {
				t.Errorf("ForRange = {%d, %d, %d}, want {0, 3, 1}", r.Start, r.End, r.Step)
			}
		}
	}
	if !found {
		t.Error("no for control flow found")
	}
}

func TestParseForWithStep(t *testing.T) {
	src := `OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;

for int i in [0:10:2] {
  x q[0];
}
`
	c, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	found := false
	for _, op := range ops {
		if op.ControlFlow != nil && op.ControlFlow.Type == ir.ControlFlowFor {
			found = true
			r := op.ControlFlow.ForRange
			if r.Step != 2 {
				t.Errorf("Step = %d, want 2", r.Step)
			}
		}
	}
	if !found {
		t.Error("no for control flow found")
	}
}

func TestParseIfElse(t *testing.T) {
	src := `OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
bit[1] c;

h q[0];
c[0] = measure q[0];
if (c == 1) {
  x q[0];
} else {
  z q[0];
}
`
	c, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	found := false
	for _, op := range ops {
		if op.ControlFlow != nil && op.ControlFlow.Type == ir.ControlFlowIfElse {
			found = true
			if len(op.ControlFlow.Bodies) != 2 {
				t.Errorf("Bodies = %d, want 2", len(op.ControlFlow.Bodies))
			}
		}
	}
	if !found {
		t.Error("no if-else control flow found")
	}
}

func TestParseSwitch(t *testing.T) {
	src := `OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
bit[2] c;

switch (c) {
case 0: {
  x q[0];
}
case 1: {
  y q[0];
}
default: {
  z q[0];
}
}
`
	c, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	found := false
	for _, op := range ops {
		if op.ControlFlow != nil && op.ControlFlow.Type == ir.ControlFlowSwitch {
			found = true
			sa := op.ControlFlow.SwitchArg
			if len(sa.Cases) != 2 {
				t.Errorf("Cases = %d, want 2", len(sa.Cases))
			}
			// 2 cases + 1 default = 3 bodies.
			if len(op.ControlFlow.Bodies) != 3 {
				t.Errorf("Bodies = %d, want 3", len(op.ControlFlow.Bodies))
			}
		}
	}
	if !found {
		t.Error("no switch control flow found")
	}
}

// TestParseIfNoElse verifies backward compat: if without else uses legacy per-op conditioning.
func TestParseIfNoElse(t *testing.T) {
	src := `OPENQASM 3.0;
include "stdgates.inc";
qubit[1] q;
bit[1] c;

if (c == 1) {
  x q[0];
}
`
	c, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	// Should use legacy conditioning, not ControlFlow.
	for _, op := range ops {
		if op.ControlFlow != nil {
			t.Error("if-without-else should use legacy per-op conditioning, not ControlFlow")
		}
	}
	// Find the X gate with condition.
	found := false
	for _, op := range ops {
		if op.Gate != nil && op.Gate.Name() == "X" && op.Condition != nil {
			found = true
			if op.Condition.Value != 1 {
				t.Errorf("Condition.Value = %d, want 1", op.Condition.Value)
			}
		}
	}
	if !found {
		t.Error("no conditioned X gate found")
	}
}
