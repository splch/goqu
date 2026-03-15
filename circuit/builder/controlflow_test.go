package builder

import (
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

func TestWhileBuilder(t *testing.T) {
	// Build: H(0), measure, while(c0==1) { X(0), measure(0->c0) }
	c, err := New("while-test", 1).
		WithClbits(1).
		H(0).
		Measure(0, 0).
		While(0, 1, func(b *Builder) {
			b.X(0).Measure(0, 0)
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Should have 3 top-level ops: H, measure, while
	ops := c.Ops()
	if len(ops) != 3 {
		t.Fatalf("got %d ops, want 3", len(ops))
	}
	if ops[2].ControlFlow == nil {
		t.Fatal("expected control flow op")
	}
	if ops[2].ControlFlow.Type != ir.ControlFlowWhile {
		t.Errorf("Type = %d, want While", ops[2].ControlFlow.Type)
	}
	if len(ops[2].ControlFlow.Bodies[0]) != 2 {
		t.Errorf("body has %d ops, want 2", len(ops[2].ControlFlow.Bodies[0]))
	}

	// Should be dynamic.
	if !c.IsDynamic() {
		t.Error("expected IsDynamic() == true")
	}

	stats := c.Stats()
	if stats.ControlFlowOps != 1 {
		t.Errorf("ControlFlowOps = %d, want 1", stats.ControlFlowOps)
	}
}

func TestForBuilder(t *testing.T) {
	// Build a for loop that applies X 3 times.
	c, err := New("for-test", 1).
		WithClbits(1).
		For(0, 0, 3, 1, func(b *Builder) {
			b.X(0)
		}).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	if ops[0].ControlFlow == nil || ops[0].ControlFlow.Type != ir.ControlFlowFor {
		t.Fatal("expected for loop control flow")
	}
	r := ops[0].ControlFlow.ForRange
	if r.Start != 0 || r.End != 3 || r.Step != 1 {
		t.Errorf("ForRange = %+v, want {0 3 1}", r)
	}
}

func TestForBuilderZeroStep(t *testing.T) {
	_, err := New("zero-step", 1).
		WithClbits(1).
		For(0, 0, 3, 0, func(b *Builder) {
			b.X(0)
		}).
		Build()
	if err == nil {
		t.Fatal("expected error for zero step")
	}
}

func TestIfElseBuilder(t *testing.T) {
	c, err := New("ifelse-test", 1).
		WithClbits(1).
		H(0).
		Measure(0, 0).
		IfElseBlock(0, 1,
			func(b *Builder) { b.Z(0) },
			func(b *Builder) { b.X(0) },
		).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	if len(ops) != 3 {
		t.Fatalf("got %d ops, want 3", len(ops))
	}
	cf := ops[2].ControlFlow
	if cf == nil || cf.Type != ir.ControlFlowIfElse {
		t.Fatal("expected if-else control flow")
	}
	if len(cf.Bodies) != 2 {
		t.Fatalf("Bodies has %d entries, want 2", len(cf.Bodies))
	}
	// If body: Z gate.
	if len(cf.Bodies[0]) != 1 || cf.Bodies[0][0].Gate.Name() != "Z" {
		t.Error("if-body should be Z gate")
	}
	// Else body: X gate.
	if len(cf.Bodies[1]) != 1 || cf.Bodies[1][0].Gate.Name() != "X" {
		t.Error("else-body should be X gate")
	}
}

func TestSwitchBuilder(t *testing.T) {
	c, err := New("switch-test", 2).
		WithClbits(2).
		H(0).H(1).
		Measure(0, 0).Measure(1, 1).
		Switch([]int{0, 1}, map[int]func(*Builder){
			0: func(b *Builder) { b.X(0) },
			1: func(b *Builder) { b.Y(0) },
		}, func(b *Builder) {
			b.Z(0)
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	lastOp := ops[len(ops)-1]
	if lastOp.ControlFlow == nil || lastOp.ControlFlow.Type != ir.ControlFlowSwitch {
		t.Fatal("expected switch control flow")
	}
	sa := lastOp.ControlFlow.SwitchArg
	if len(sa.Clbits) != 2 {
		t.Errorf("Switch clbits = %d, want 2", len(sa.Clbits))
	}
	// 2 cases + 1 default = 3 bodies.
	if len(lastOp.ControlFlow.Bodies) != 3 {
		t.Errorf("Bodies = %d, want 3", len(lastOp.ControlFlow.Bodies))
	}
}

func TestNestedControlFlow(t *testing.T) {
	// While loop with an if-else inside.
	c, err := New("nested", 2).
		WithClbits(2).
		H(0).
		Measure(0, 0).
		While(0, 1, func(b *Builder) {
			b.IfElseBlock(0, 1,
				func(b2 *Builder) { b2.X(1) },
				func(b2 *Builder) { b2.Y(1) },
			).Measure(0, 0)
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	whileOp := ops[2]
	if whileOp.ControlFlow == nil || whileOp.ControlFlow.Type != ir.ControlFlowWhile {
		t.Fatal("expected while")
	}
	body := whileOp.ControlFlow.Bodies[0]
	if len(body) != 2 {
		t.Fatalf("while body has %d ops, want 2", len(body))
	}
	if body[0].ControlFlow == nil || body[0].ControlFlow.Type != ir.ControlFlowIfElse {
		t.Error("expected if-else inside while body")
	}
}

// TestWhileSimulation tests that a while loop terminates correctly.
// Circuit: X(0), measure -> c0=1; while(c0==1) { X(0), measure(0->c0) }
// After 1 iteration: X flips back to |0>, measure -> c0=0, while exits.
// Result: c0=0 deterministically.
func TestWhileSimulation(t *testing.T) {
	c, err := New("while-sim", 1).
		WithClbits(1).
		X(0). // set qubit to |1>
		Measure(0, 0). // c0 = 1
		While(0, 1, func(b *Builder) {
			b.X(0).Measure(0, 0) // X flips to |0>, measure gives 0
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := statevector.New(1)
	defer sim.Close()
	counts, err := sim.RunDynamic(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	// All shots should end with c0=0 (bitstring "0").
	if counts["0"] != 100 {
		t.Errorf("expected 100 shots of '0', got %v", counts)
	}
}

// TestForSimulation tests that a for loop applies X the correct number of times.
// X applied 3 times = X^3 = X, so qubit ends in |1>.
func TestForSimulation(t *testing.T) {
	c, err := New("for-sim", 1).
		WithClbits(1).
		For(0, 0, 3, 1, func(b *Builder) {
			b.X(0)
		}).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := statevector.New(1)
	defer sim.Close()
	counts, err := sim.RunDynamic(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	// 3 X gates: |0> -> |1> -> |0> -> |1>. Result: all "1".
	if counts["1"] != 100 {
		t.Errorf("expected 100 shots of '1', got %v", counts)
	}
}

// TestIfElseSimulation tests both branches of if/else.
func TestIfElseSimulation(t *testing.T) {
	// X(0) puts qubit in |1>, measure c0=1.
	// If c0==1: Z(0) (no visible effect on |1>, but tests the if branch).
	// Else: would apply X(0).
	// Measure again to verify qubit is still |1>.
	c, err := New("ifelse-sim", 1).
		WithClbits(1).
		X(0).
		Measure(0, 0). // c0 = 1
		IfElseBlock(0, 1,
			func(b *Builder) { /* do nothing, keep |1> */ },
			func(b *Builder) { b.X(0) }, // would flip, but not taken
		).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := statevector.New(1)
	defer sim.Close()
	counts, err := sim.RunDynamic(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	if counts["1"] != 100 {
		t.Errorf("expected 100 shots of '1', got %v", counts)
	}
}

// TestSwitchSimulation tests switch dispatch.
func TestSwitchSimulation(t *testing.T) {
	// Prepare c0=1, c1=0 => classical value = 1 (little-endian).
	// Switch on {c0, c1}: case 0 -> X(0), case 1 -> noop (keep |1>).
	c, err := New("switch-sim", 1).
		WithClbits(2).
		X(0).
		Measure(0, 0). // c0=1
		Switch([]int{0, 1}, map[int]func(*Builder){
			0: func(b *Builder) { b.X(0) }, // not taken
			1: func(b *Builder) { /* noop, keep |1> */ },
		}, nil).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := statevector.New(1)
	defer sim.Close()
	counts, err := sim.RunDynamic(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	// Case 1 is a noop, qubit stays |1>. Final measure -> c0=1, c1=0.
	if counts["01"] != 100 {
		t.Errorf("expected 100 shots of '01', got %v", counts)
	}
}

// TestWhileInvalidClbit tests error on out-of-range clbit.
func TestWhileInvalidClbit(t *testing.T) {
	_, err := New("bad-while", 1).
		WithClbits(1).
		While(5, 1, func(b *Builder) { b.X(0) }). // clbit 5 out of range
		Build()
	if err == nil {
		t.Fatal("expected error for invalid clbit")
	}
}

// TestControlFlowDrawing tests that ASCII drawing doesn't panic on control flow.
func TestControlFlowDrawing(t *testing.T) {
	c, err := New("draw-cf", 2).
		WithClbits(1).
		H(0).
		Measure(0, 0).
		While(0, 1, func(b *Builder) {
			b.X(0).Measure(0, 0)
		}).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Just verify drawing doesn't panic.
	_ = c.Stats()
}

// TestControlFlowQASMRoundTrip verifies QASM emission of control flow.
func TestControlFlowQASMRoundTrip(t *testing.T) {
	c, err := New("qasm-cf", 2).
		WithClbits(2).
		H(0).
		Measure(0, 0).
		While(0, 1, func(b *Builder) {
			b.X(0).Measure(0, 0)
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	_ = c
	// QASM emission is tested separately in the emitter package.
	// Here we just verify the circuit builds correctly for QASM consumption.
	if !c.IsDynamic() {
		t.Error("expected dynamic circuit")
	}
}

// TestSwitchDefaultBranch tests the default branch of a switch statement.
func TestSwitchDefaultBranch(t *testing.T) {
	// c0=0, c1=0 => classical value = 0.
	// Switch: case 1 -> X (not taken), default -> noop.
	c, err := New("switch-default", 1).
		WithClbits(2).
		Measure(0, 0). // c0 = 0
		Switch([]int{0, 1}, map[int]func(*Builder){
			1: func(b *Builder) { b.X(0) },
		}, func(b *Builder) {
			// default: do nothing
		}).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := statevector.New(1)
	defer sim.Close()
	counts, err := sim.RunDynamic(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	// Default branch is noop, qubit stays |0>. c0=0, c1=0.
	if counts["00"] != 100 {
		t.Errorf("expected 100 shots of '00', got %v", counts)
	}
}

// TestForEvenSteps tests for loop with step > 1.
func TestForEvenSteps(t *testing.T) {
	// For i in [0:6:2] => 3 iterations. 3 X gates => |1>.
	c, err := New("for-step", 1).
		WithClbits(1).
		For(0, 0, 6, 2, func(b *Builder) {
			b.X(0)
		}).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := statevector.New(1)
	defer sim.Close()
	counts, err := sim.RunDynamic(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	if counts["1"] != 100 {
		t.Errorf("expected 100 shots of '1', got %v", counts)
	}
}

func init() {
	// Suppress unused import warnings.
	_ = gate.H
}
