package routing

import (
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

func TestDAGFrontLayer(t *testing.T) {
	// H(0) is on q0, CNOT(0,1) is on q0 and q1, X(1) is on q1.
	// qubitOps[0] = [0, 1], qubitOps[1] = [1, 2]
	// predCount: op0=0, op1=1(from q0), op2=1(from q1)
	// Front: [0] only
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.X, Qubits: []int{1}},
	}
	d := newDAG(ops, 2, false)

	front := d.frontLayer()
	if len(front) != 1 || front[0] != 0 {
		t.Fatalf("expected front=[0], got %v", front)
	}

	// After executing op 0, CNOT should become available.
	d.markExecuted(0)
	front = d.frontLayer()
	if len(front) != 1 || front[0] != 1 {
		t.Fatalf("expected front=[1] after executing op 0, got %v", front)
	}

	// After executing CNOT, X should be available.
	d.markExecuted(1)
	front = d.frontLayer()
	if len(front) != 1 || front[0] != 2 {
		t.Fatalf("expected front=[2] after executing op 1, got %v", front)
	}
}

func TestDAGFrontLayerParallel(t *testing.T) {
	// Two independent ops: H(0), X(1) should both be in front layer.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{1}},
	}
	d := newDAG(ops, 2, false)

	front := d.frontLayer()
	if len(front) != 2 {
		t.Fatalf("expected 2 front ops, got %d", len(front))
	}
}

func TestDAGExtendedSet(t *testing.T) {
	// CNOT(0,1), CNOT(1,2), CNOT(2,3)
	// Front: [0], extended should find CNOT(1,2) in first layer, CNOT(2,3) in second.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{1, 2}},
		{Gate: gate.CNOT, Qubits: []int{2, 3}},
	}
	d := newDAG(ops, 4, false)

	front := d.frontLayer()
	ext := d.extendedSet(front, 3)

	if len(ext) < 1 {
		t.Fatalf("expected at least 1 extended layer, got %d", len(ext))
	}
	// First extended layer should contain op 1.
	found := false
	for _, idx := range ext[0] {
		if idx == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected op 1 in first extended layer, got %v", ext[0])
	}
}

func TestDAGMarkExecuted(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{0}},
		{Gate: gate.Y, Qubits: []int{0}},
	}
	d := newDAG(ops, 1, false)

	// Only first op should be available.
	front := d.frontLayer()
	if len(front) != 1 || front[0] != 0 {
		t.Fatalf("expected front=[0], got %v", front)
	}

	d.markExecuted(0)
	if !d.executed[0] {
		t.Fatal("op 0 should be marked executed")
	}

	front = d.frontLayer()
	if len(front) != 1 || front[0] != 1 {
		t.Fatalf("expected front=[1], got %v", front)
	}

	d.markExecuted(1)
	d.markExecuted(2)
	if !d.allExecuted() {
		t.Fatal("expected all ops executed")
	}
}

func TestDAGReverse(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	d := newDAG(ops, 2, true)

	// Reversed: CNOT first, then H.
	if d.ops[0].Gate.Name() != "CNOT" {
		t.Errorf("expected first op to be CNOT in reversed DAG, got %s", d.ops[0].Gate.Name())
	}
	if d.ops[1].Gate.Name() != "H" {
		t.Errorf("expected second op to be H in reversed DAG, got %s", d.ops[1].Gate.Name())
	}
}

func TestDAGPredCountBugFix(t *testing.T) {
	// Regression test: old code had predCount[idx] += k which over-counted.
	// With 3 ops on the same qubit, the 3rd op had predCount=2+1=3 in the
	// old code (because it appeared at k=2 for q0 giving +=2, and potentially
	// on another qubit). The fix ensures each qubit line contributes exactly 1.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},       // op 0: q0
		{Gate: gate.X, Qubits: []int{0}},       // op 1: q0
		{Gate: gate.Y, Qubits: []int{0}},       // op 2: q0
		{Gate: gate.CNOT, Qubits: []int{0, 1}}, // op 3: q0, q1
	}
	d := newDAG(ops, 2, false)

	// op 0: no predecessors
	if d.predCount[0] != 0 {
		t.Errorf("op 0 predCount = %d, want 0", d.predCount[0])
	}
	// op 1: one predecessor (op 0 on q0)
	if d.predCount[1] != 1 {
		t.Errorf("op 1 predCount = %d, want 1", d.predCount[1])
	}
	// op 2: one predecessor (op 1 on q0)
	if d.predCount[2] != 1 {
		t.Errorf("op 2 predCount = %d, want 1", d.predCount[2])
	}
	// op 3: one predecessor (op 2 on q0) - NOT 3 like old code would produce
	if d.predCount[3] != 1 {
		t.Errorf("op 3 predCount = %d, want 1", d.predCount[3])
	}

	// Execute all and verify everything completes.
	d.markExecuted(0)
	d.markExecuted(1)
	d.markExecuted(2)
	front := d.frontLayer()
	if len(front) != 1 || front[0] != 3 {
		t.Fatalf("expected front=[3] after executing 0,1,2, got %v", front)
	}
}
