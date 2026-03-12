package builder

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/sim/statevector"
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

func TestUnitaryBuilderMethod(t *testing.T) {
	s2 := 1.0 / math.Sqrt2
	hMatrix := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}

	// Build circuit using the Unitary builder method.
	c, err := New("unitary-test", 1).
		Unitary("myH", hMatrix, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Ops()) != 1 {
		t.Fatalf("len(Ops()) = %d, want 1", len(c.Ops()))
	}
	if c.Ops()[0].Gate.Name() != "myH" {
		t.Errorf("gate name = %q, want %q", c.Ops()[0].Gate.Name(), "myH")
	}
}

func TestUnitaryBuilderInvalidMatrix(t *testing.T) {
	_, err := New("bad-unitary", 1).
		Unitary("bad", []complex128{1, 1, 1, 1}, 0).
		Build()
	if err == nil {
		t.Fatal("expected error for non-unitary matrix")
	}
}

func TestUnitaryEndToEnd(t *testing.T) {
	// Create a custom unitary matching the H gate, apply it, and verify
	// the simulator produces the same statevector as the built-in H gate.
	s2 := 1.0 / math.Sqrt2
	hMatrix := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}

	// Circuit with custom unitary H.
	cCustom, err := New("custom-h", 1).
		Unitary("myH", hMatrix, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Circuit with built-in H.
	cBuiltin, err := New("builtin-h", 1).
		H(0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Evolve both and compare statevectors.
	simCustom := statevector.New(1)
	if err := simCustom.Evolve(cCustom); err != nil {
		t.Fatal(err)
	}

	simBuiltin := statevector.New(1)
	if err := simBuiltin.Evolve(cBuiltin); err != nil {
		t.Fatal(err)
	}

	svCustom := simCustom.StateVector()
	svBuiltin := simBuiltin.StateVector()

	const eps = 1e-14
	for i := range svCustom {
		if cmplx.Abs(svCustom[i]-svBuiltin[i]) > eps {
			t.Errorf("StateVector[%d]: custom=%v, builtin=%v", i, svCustom[i], svBuiltin[i])
		}
	}
}

func TestBuilderCompose(t *testing.T) {
	// Build a sub-circuit: H(0) CNOT(0,1).
	sub, err := New("sub", 2).H(0).CNOT(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}

	// Compose into a 4-qubit circuit, mapping sub's 0→2, 1→3.
	c, err := New("main", 4).
		X(0).
		Compose(sub, map[int]int{0: 2, 1: 3}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ops := c.Ops()
	if len(ops) != 3 { // X + H + CNOT
		t.Fatalf("len(Ops()) = %d, want 3", len(ops))
	}
	if ops[1].Gate.Name() != "H" || ops[1].Qubits[0] != 2 {
		t.Errorf("ops[1] = %s on %v, want H on [2]", ops[1].Gate.Name(), ops[1].Qubits)
	}
	if ops[2].Qubits[0] != 2 || ops[2].Qubits[1] != 3 {
		t.Errorf("CNOT qubits = %v, want [2,3]", ops[2].Qubits)
	}
}

func TestBuilderComposeIdentity(t *testing.T) {
	sub, err := New("sub", 2).H(0).X(1).Build()
	if err != nil {
		t.Fatal(err)
	}

	// nil qubitMap = identity.
	c, err := New("main", 2).
		Compose(sub, nil).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	ops := c.Ops()
	if len(ops) != 2 {
		t.Fatalf("len(Ops()) = %d, want 2", len(ops))
	}
	if ops[0].Gate.Name() != "H" || ops[0].Qubits[0] != 0 {
		t.Errorf("ops[0] = %s on %v, want H on [0]", ops[0].Gate.Name(), ops[0].Qubits)
	}
}

func TestBuilderComposeOutOfRange(t *testing.T) {
	sub, err := New("sub", 3).X(2).Build()
	if err != nil {
		t.Fatal(err)
	}

	// sub has qubit 2, builder only has 2 qubits (0,1) → error.
	_, err = New("main", 2).
		Compose(sub, nil).
		Build()
	if err == nil {
		t.Fatal("expected error for out-of-range qubit")
	}
}

func TestBuilderComposeInverse(t *testing.T) {
	// Build a sub-circuit, then compose its inverse for uncomputation.
	sub, err := New("sub", 2).H(0).CNOT(0, 1).RZ(math.Pi/4, 0).Build()
	if err != nil {
		t.Fatal(err)
	}

	c, err := New("uncomp", 2).
		Compose(sub, nil).
		ComposeInverse(sub, nil).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Simulate: should return to |00⟩.
	sim := statevector.New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	const eps = 1e-10
	if cmplx.Abs(sv[0]-1) > eps {
		t.Errorf("|00⟩ amplitude = %v, want ≈1", sv[0])
	}
	for i := 1; i < len(sv); i++ {
		if cmplx.Abs(sv[i]) > eps {
			t.Errorf("|%02b⟩ amplitude = %v, want ≈0", i, sv[i])
		}
	}
}

func TestBuilderComposeInverseSkipsNonUnitary(t *testing.T) {
	sub, err := New("sub", 2).
		WithClbits(2).
		H(0).
		Barrier(0, 1).
		Reset(0).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	c, err := New("main", 2).
		WithClbits(2).
		ComposeInverse(sub, nil).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Only H† should remain.
	ops := c.Ops()
	if len(ops) != 1 {
		t.Fatalf("len(Ops()) = %d, want 1", len(ops))
	}
	if ops[0].Gate.Name() != "H" {
		t.Errorf("ops[0].Gate.Name() = %q, want H", ops[0].Gate.Name())
	}
}
