package pass

import (
	"math"
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

func TestRemoveIdentity_ZeroAngle(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.RZ(0), Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := ir.New("ri", 1, 0, ops, nil)
	out, err := RemoveIdentity(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	if out.NumOps() != 1 {
		t.Errorf("expected 1 op, got %d", out.NumOps())
	}
}

func TestRemoveIdentity_TwoPi(t *testing.T) {
	// RZ(2*pi) = -I (global phase), not identity. RemoveIdentity only removes
	// exact identity matrices, so this gate is preserved.
	ops := []ir.Operation{
		{Gate: gate.RZ(2 * math.Pi), Qubits: []int{0}},
	}
	c := ir.New("ri", 1, 0, ops, nil)
	out, err := RemoveIdentity(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	if out != c {
		t.Error("expected same circuit (RZ(2pi) = -I is global phase, not identity)")
	}
}

func TestRemoveIdentity_NonIdentityPreserved(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.RZ(0.5), Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("ri", 2, 0, ops, nil)
	out, err := RemoveIdentity(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	if out != c {
		t.Error("expected same circuit pointer (no changes)")
	}
}

func TestRemoveIdentity_MeasurementPreserved(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.RZ(0), Qubits: []int{0}},
		{Gate: nil, Qubits: []int{0}, Clbits: []int{0}}, // measurement
	}
	c := ir.New("ri", 1, 1, ops, nil)
	out, err := RemoveIdentity(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	if out.NumOps() != 1 {
		t.Errorf("expected 1 op (measurement), got %d", out.NumOps())
	}
}

func TestRemoveIdentity_EmptyCircuit(t *testing.T) {
	c := ir.New("ri", 1, 0, nil, nil)
	out, err := RemoveIdentity(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	if out != c {
		t.Error("expected same circuit pointer")
	}
}
