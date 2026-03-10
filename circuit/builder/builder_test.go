package builder

import (
	"math"
	"testing"

	"github.com/splch/qgo/circuit/gate"
)

func TestBellCircuit(t *testing.T) {
	c, err := New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if c.Name() != "bell" {
		t.Errorf("Name() = %q, want %q", c.Name(), "bell")
	}
	if c.NumQubits() != 2 {
		t.Errorf("NumQubits() = %d, want 2", c.NumQubits())
	}
	if c.NumClbits() != 2 {
		t.Errorf("NumClbits() = %d, want 2", c.NumClbits())
	}
	// H + CNOT + 2 measurements = 4 ops
	if len(c.Ops()) != 4 {
		t.Errorf("len(Ops()) = %d, want 4", len(c.Ops()))
	}
}

func TestGHZCircuit(t *testing.T) {
	c, err := New("ghz-4", 4).
		H(0).
		CNOT(0, 1).
		CNOT(1, 2).
		CNOT(2, 3).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}
	stats := c.Stats()
	if stats.GateCount != 8 { // 1 H + 3 CNOT + 4 M
		t.Errorf("GateCount = %d, want 8", stats.GateCount)
	}
	if stats.TwoQubitGates != 3 {
		t.Errorf("TwoQubitGates = %d, want 3", stats.TwoQubitGates)
	}
}

func TestQubitOutOfRange(t *testing.T) {
	_, err := New("bad", 2).H(2).Build()
	if err == nil {
		t.Fatal("expected error for out-of-range qubit")
	}

	_, err = New("bad", 2).H(-1).Build()
	if err == nil {
		t.Fatal("expected error for negative qubit")
	}
}

func TestGateQubitMismatch(t *testing.T) {
	// nil gate
	_, err := New("bad", 3).Apply(nil, 0).Build()
	if err == nil {
		t.Fatal("expected error for nil gate")
	}

	// CNOT needs 2 qubits, provide 3
	_, err = New("bad", 4).
		Apply(gate.CNOT, 0, 1, 2).
		Build()
	if err == nil {
		t.Fatal("expected error for wrong number of qubits")
	}
}

func TestMeasureClbitRange(t *testing.T) {
	_, err := New("bad", 2).
		WithClbits(1).
		Measure(0, 0).
		Measure(1, 1). // clbit 1 out of range
		Build()
	if err == nil {
		t.Fatal("expected error for out-of-range classical bit")
	}
}

func TestParameterizedGatesInBuilder(t *testing.T) {
	c, err := New("param", 1).
		WithClbits(1).
		RX(math.Pi/4, 0).
		RY(math.Pi/3, 0).
		RZ(math.Pi/6, 0).
		Phase(math.Pi/4, 0).
		U3(math.Pi/4, math.Pi/3, math.Pi/6, 0).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	stats := c.Stats()
	if stats.GateCount != 6 { // 5 gates + 1 measurement
		t.Errorf("GateCount = %d, want 6", stats.GateCount)
	}
}

func TestBarrier(t *testing.T) {
	c, err := New("barrier", 3).
		H(0).
		Barrier(0, 1, 2).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Ops()) != 3 {
		t.Errorf("len(Ops()) = %d, want 3", len(c.Ops()))
	}
	if c.Ops()[1].Gate.Name() != "barrier" {
		t.Errorf("Ops()[1].Gate.Name() = %q, want %q", c.Ops()[1].Gate.Name(), "barrier")
	}
}

func TestStats(t *testing.T) {
	c, err := New("stats", 3).
		H(0).
		CNOT(0, 1).
		CNOT(1, 2).
		RZ(1.0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	stats := c.Stats()
	if stats.GateCount != 4 {
		t.Errorf("GateCount = %d, want 4", stats.GateCount)
	}
	if stats.TwoQubitGates != 2 {
		t.Errorf("TwoQubitGates = %d, want 2", stats.TwoQubitGates)
	}
	// H(0) @ depth 1, CNOT(0,1) @ depth 2, CNOT(1,2) @ depth 3, RZ(0) @ depth 3 (parallel)
	if stats.Depth != 3 {
		t.Errorf("Depth = %d, want 3", stats.Depth)
	}
	if stats.Params != 1 {
		t.Errorf("Params = %d, want 1", stats.Params)
	}
}

func TestMetadata(t *testing.T) {
	c, err := New("meta", 1).
		SetMetadata("author", "test").
		H(0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if c.Metadata()["author"] != "test" {
		t.Errorf("Metadata[author] = %q, want %q", c.Metadata()["author"], "test")
	}
}
