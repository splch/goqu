package decompose_test

import (
	"testing"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/transpile/decompose"
)

func TestDecomposeMultiControlledMCX3(t *testing.T) {
	// C3-X should decompose recursively.
	cg := gate.MCX(3).(gate.ControlledGate)
	ops := decompose.DecomposeMultiControlled(cg, []int{0, 1, 2, 3})
	if ops == nil {
		t.Fatal("DecomposeMultiControlled returned nil for C3-X")
	}
	// All resulting ops should be <=3 qubit gates (CCX is allowed in first pass).
	for _, op := range ops {
		if op.Gate.Qubits() > 3 {
			t.Errorf("decomposed op %s has %d qubits", op.Gate.Name(), op.Gate.Qubits())
		}
	}
	t.Logf("C3-X decomposed into %d ops", len(ops))
}

func TestDecomposeMultiControlledMCZ2(t *testing.T) {
	// C2-Z: Controlled(Z, 2) returns a *controlled since CZ is only n=1.
	cg := gate.MCZ(2).(gate.ControlledGate)
	ops := decompose.DecomposeMultiControlled(cg, []int{0, 1, 2})
	if ops == nil {
		t.Fatal("DecomposeMultiControlled returned nil for C2-Z")
	}
	if len(ops) == 0 {
		t.Error("decomposition produced zero ops")
	}
	t.Logf("C2-Z decomposed into %d ops", len(ops))
}

func TestDecomposeMultiControlledH2(t *testing.T) {
	// C2-H: 2-controlled Hadamard.
	cg := gate.Controlled(gate.H, 2).(gate.ControlledGate)
	ops := decompose.DecomposeMultiControlled(cg, []int{0, 1, 2})
	if ops == nil {
		t.Fatal("DecomposeMultiControlled returned nil for C2-H")
	}
	if len(ops) == 0 {
		t.Error("decomposition produced zero ops")
	}
	t.Logf("C2-H decomposed into %d ops", len(ops))
}

func TestDecomposeMultiControlledGateCount(t *testing.T) {
	// C3-X should produce a reasonable number of gates.
	cg := gate.MCX(3).(gate.ControlledGate)
	ops := decompose.DecomposeMultiControlled(cg, []int{0, 1, 2, 3})
	if len(ops) > 200 {
		t.Errorf("C3-X produced %d ops, expected < 200", len(ops))
	}
}
