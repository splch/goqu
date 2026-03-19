package ir

import (
	"testing"

	"github.com/splch/goqu/circuit/gate"
)

func TestNew_BasicProperties(t *testing.T) {
	ops := []Operation{{Gate: gate.H, Qubits: []int{0}}}
	c := New("test", 2, 1, ops, map[string]string{"k": "v"})
	if c.Name() != "test" {
		t.Errorf("Name() = %q, want %q", c.Name(), "test")
	}
	if c.NumQubits() != 2 {
		t.Errorf("NumQubits() = %d, want 2", c.NumQubits())
	}
	if c.NumClbits() != 1 {
		t.Errorf("NumClbits() = %d, want 1", c.NumClbits())
	}
}

func TestNew_NilOps(t *testing.T) {
	c := New("empty", 1, 0, nil, nil)
	ops := c.Ops()
	if ops == nil {
		t.Error("Ops() returned nil, want empty slice")
	}
	if len(ops) != 0 {
		t.Errorf("len(Ops()) = %d, want 0", len(ops))
	}
}

func TestNew_NilMetadata(t *testing.T) {
	c := New("test", 1, 0, nil, nil)
	if c.Metadata() != nil {
		t.Errorf("Metadata() = %v, want nil", c.Metadata())
	}
}

func TestOps_Immutability(t *testing.T) {
	ops := []Operation{{Gate: gate.H, Qubits: []int{0}}}
	c := New("test", 1, 0, ops, nil)

	// Modify the returned slice.
	returned := c.Ops()
	returned[0] = Operation{Gate: gate.X, Qubits: []int{0}}

	// Original should be unchanged.
	if c.Ops()[0].Gate.Name() != "H" {
		t.Error("modifying returned Ops() affected the circuit")
	}
}

func TestMetadata_Immutability(t *testing.T) {
	c := New("test", 1, 0, nil, map[string]string{"k": "v"})

	// Modify the returned map.
	md := c.Metadata()
	md["k"] = "changed"
	md["new"] = "added"

	// Original should be unchanged.
	if c.Metadata()["k"] != "v" {
		t.Error("modifying returned Metadata() affected the circuit")
	}
	if _, ok := c.Metadata()["new"]; ok {
		t.Error("adding to returned Metadata() affected the circuit")
	}
}

func TestStats_EmptyCircuit(t *testing.T) {
	c := New("empty", 2, 0, nil, nil)
	s := c.Stats()
	if s.GateCount != 0 || s.Depth != 0 || s.TwoQubitGates != 0 || s.Params != 0 {
		t.Errorf("Stats() = %+v, want all zeros", s)
	}
}

func TestStats_BellCircuit(t *testing.T) {
	ops := []Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := New("bell", 2, 0, ops, nil)
	s := c.Stats()
	if s.GateCount != 2 {
		t.Errorf("GateCount = %d, want 2", s.GateCount)
	}
	if s.Depth != 2 {
		t.Errorf("Depth = %d, want 2", s.Depth)
	}
	if s.TwoQubitGates != 1 {
		t.Errorf("TwoQubitGates = %d, want 1", s.TwoQubitGates)
	}
	if s.Params != 0 {
		t.Errorf("Params = %d, want 0", s.Params)
	}
}

func TestStats_AllMeasurements(t *testing.T) {
	// Measurement ops have Gate=nil.
	ops := []Operation{
		{Gate: nil, Qubits: []int{0}, Clbits: []int{0}},
		{Gate: nil, Qubits: []int{1}, Clbits: []int{1}},
	}
	c := New("meas", 2, 2, ops, nil)
	s := c.Stats()
	if s.TwoQubitGates != 0 {
		t.Errorf("TwoQubitGates = %d, want 0", s.TwoQubitGates)
	}
}

func TestStats_ParameterizedGates(t *testing.T) {
	ops := []Operation{
		{Gate: gate.RZ(0.5), Qubits: []int{0}},
		{Gate: gate.RX(1.0), Qubits: []int{1}},
	}
	c := New("param", 2, 0, ops, nil)
	s := c.Stats()
	if s.Params != 2 {
		t.Errorf("Params = %d, want 2", s.Params)
	}
}

func TestDepth_AllParallel(t *testing.T) {
	ops := []Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.H, Qubits: []int{1}},
		{Gate: gate.H, Qubits: []int{2}},
	}
	c := New("par", 3, 0, ops, nil)
	if c.Stats().Depth != 1 {
		t.Errorf("Depth = %d, want 1", c.Stats().Depth)
	}
}

func TestDepth_Sequential(t *testing.T) {
	ops := []Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.X, Qubits: []int{0}},
		{Gate: gate.Y, Qubits: []int{0}},
	}
	c := New("seq", 1, 0, ops, nil)
	if c.Stats().Depth != 3 {
		t.Errorf("Depth = %d, want 3", c.Stats().Depth)
	}
}

func TestDepth_OutOfBoundsQubits(t *testing.T) {
	// Op referencing qubit >= numQubits - should not panic.
	ops := []Operation{
		{Gate: gate.H, Qubits: []int{5}},
	}
	c := New("oob", 2, 0, ops, nil)
	// Just verify no panic; depth result is undefined for out-of-bounds.
	_ = c.Stats()
}

func TestBind_NoSymbolicGates(t *testing.T) {
	ops := []Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := New("fixed", 2, 0, ops, nil)
	bound, err := Bind(c, map[string]float64{"theta": 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if len(bound.Ops()) != 2 {
		t.Errorf("len(Ops) = %d, want 2", len(bound.Ops()))
	}
}

func TestFreeParameters_Empty(t *testing.T) {
	ops := []Operation{
		{Gate: gate.H, Qubits: []int{0}},
	}
	c := New("fixed", 1, 0, ops, nil)
	params := FreeParameters(c)
	if len(params) != 0 {
		t.Errorf("FreeParameters = %v, want empty", params)
	}
}
