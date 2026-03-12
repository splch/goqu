package ir_test

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/sim/statevector"
)

func TestComposeIdentityMapping(t *testing.T) {
	c1, _ := builder.New("c1", 2).H(0).CNOT(0, 1).Build()
	c2, _ := builder.New("c2", 2).X(0).Z(1).Build()

	result, err := ir.Compose(c1, c2, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ops := result.Ops()
	if len(ops) != 4 { // H, CNOT, X, Z
		t.Fatalf("len(Ops()) = %d, want 4", len(ops))
	}
	if ops[2].Gate.Name() != "X" {
		t.Errorf("ops[2].Gate.Name() = %q, want X", ops[2].Gate.Name())
	}
	if ops[3].Gate.Name() != "Z" {
		t.Errorf("ops[3].Gate.Name() = %q, want Z", ops[3].Gate.Name())
	}
	if result.NumQubits() != 2 {
		t.Errorf("NumQubits() = %d, want 2", result.NumQubits())
	}
}

func TestComposeExplicitMapping(t *testing.T) {
	c1, _ := builder.New("c1", 4).H(0).Build()
	c2, _ := builder.New("c2", 2).CNOT(0, 1).Build()

	// Map c2's qubits 0,1 → c1's qubits 2,3.
	qMap := map[int]int{0: 2, 1: 3}
	result, err := ir.Compose(c1, c2, qMap, nil)
	if err != nil {
		t.Fatal(err)
	}
	ops := result.Ops()
	if len(ops) != 2 {
		t.Fatalf("len(Ops()) = %d, want 2", len(ops))
	}
	if ops[1].Qubits[0] != 2 || ops[1].Qubits[1] != 3 {
		t.Errorf("CNOT qubits = %v, want [2,3]", ops[1].Qubits)
	}
}

func TestComposeExpandsDimensions(t *testing.T) {
	c1, _ := builder.New("c1", 2).H(0).Build()
	c2, _ := builder.New("c2", 1).X(0).Build()

	// Map c2's qubit 0 → qubit 4 (beyond c1's range).
	qMap := map[int]int{0: 4}
	result, err := ir.Compose(c1, c2, qMap, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.NumQubits() != 5 {
		t.Errorf("NumQubits() = %d, want 5", result.NumQubits())
	}
}

func TestComposeMismatchedSizesError(t *testing.T) {
	c1, _ := builder.New("c1", 2).H(0).Build()
	c2, _ := builder.New("c2", 3).X(0).Build()

	_, err := ir.Compose(c1, c2, nil, nil)
	if err == nil {
		t.Fatal("expected error when c2 has more qubits than c1 with nil map")
	}
}

func TestComposeMissingMappingError(t *testing.T) {
	c1, _ := builder.New("c1", 4).H(0).Build()
	c2, _ := builder.New("c2", 2).CNOT(0, 1).Build()

	// Only map qubit 0, missing qubit 1.
	qMap := map[int]int{0: 2}
	_, err := ir.Compose(c1, c2, qMap, nil)
	if err == nil {
		t.Fatal("expected error for incomplete qubit mapping")
	}
}

func TestComposeWithMeasurements(t *testing.T) {
	c1, _ := builder.New("c1", 2).WithClbits(2).H(0).Build()
	c2, _ := builder.New("c2", 2).WithClbits(2).H(0).MeasureAll().Build()

	result, err := ir.Compose(c1, c2, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// c1: H, c2: H + 2 measurements = 4 ops.
	ops := result.Ops()
	if len(ops) != 4 {
		t.Fatalf("len(Ops()) = %d, want 4", len(ops))
	}
	// Last two should be measurements (nil Gate).
	if ops[2].Gate != nil {
		t.Errorf("ops[2] should be measurement (nil Gate)")
	}
	if ops[3].Gate != nil {
		t.Errorf("ops[3] should be measurement (nil Gate)")
	}
}

func TestComposeWithCondition(t *testing.T) {
	c1, _ := builder.New("c1", 2).WithClbits(2).H(0).Build()
	// Build c2 with a conditioned gate.
	c2, _ := builder.New("c2", 2).WithClbits(2).If(0, 1, gate.X, 1).Build()

	// Remap clbit 0 → 1.
	clMap := map[int]int{0: 1}
	result, err := ir.Compose(c1, c2, nil, clMap)
	if err != nil {
		t.Fatal(err)
	}
	ops := result.Ops()
	if len(ops) != 2 {
		t.Fatalf("len(Ops()) = %d, want 2", len(ops))
	}
	if ops[1].Condition == nil {
		t.Fatal("ops[1] should have Condition")
	}
	if ops[1].Condition.Clbit != 1 {
		t.Errorf("Condition.Clbit = %d, want 1", ops[1].Condition.Clbit)
	}
	// Register should be cleared after remap.
	if ops[1].Condition.Register != "" {
		t.Errorf("Condition.Register = %q, want empty", ops[1].Condition.Register)
	}
}

func TestComposeEmptyCircuits(t *testing.T) {
	c1, _ := builder.New("c1", 2).Build()
	c2, _ := builder.New("c2", 2).Build()

	result, err := ir.Compose(c1, c2, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ops()) != 0 {
		t.Errorf("len(Ops()) = %d, want 0", len(result.Ops()))
	}
}

func TestTensor(t *testing.T) {
	// Two Bell-prep circuits tensored → 4 qubits.
	c1, _ := builder.New("bell1", 2).H(0).CNOT(0, 1).Build()
	c2, _ := builder.New("bell2", 2).H(0).CNOT(0, 1).Build()

	result := ir.Tensor(c1, c2)
	if result.NumQubits() != 4 {
		t.Errorf("NumQubits() = %d, want 4", result.NumQubits())
	}
	ops := result.Ops()
	if len(ops) != 4 {
		t.Fatalf("len(Ops()) = %d, want 4", len(ops))
	}
	// c2's H should be on qubit 2.
	if ops[2].Gate.Name() != "H" || ops[2].Qubits[0] != 2 {
		t.Errorf("ops[2] = %v on qubit %v, want H on [2]", ops[2].Gate.Name(), ops[2].Qubits)
	}
	// c2's CNOT should be on qubits 2,3.
	if ops[3].Qubits[0] != 2 || ops[3].Qubits[1] != 3 {
		t.Errorf("CNOT qubits = %v, want [2,3]", ops[3].Qubits)
	}

	if result.Name() != "bell1⊗bell2" {
		t.Errorf("Name() = %q, want %q", result.Name(), "bell1⊗bell2")
	}
}

func TestTensorWithClbits(t *testing.T) {
	c1, _ := builder.New("c1", 2).WithClbits(1).H(0).Measure(0, 0).Build()
	c2, _ := builder.New("c2", 2).WithClbits(1).H(0).Measure(0, 0).Build()

	result := ir.Tensor(c1, c2)
	if result.NumClbits() != 2 {
		t.Errorf("NumClbits() = %d, want 2", result.NumClbits())
	}
	ops := result.Ops()
	// c2's measurement clbit should be shifted.
	lastOp := ops[len(ops)-1]
	if lastOp.Clbits[0] != 1 {
		t.Errorf("c2 measurement clbit = %d, want 1", lastOp.Clbits[0])
	}
}

func TestTensorWithCondition(t *testing.T) {
	c1, _ := builder.New("c1", 1).WithClbits(1).H(0).Build()
	c2, _ := builder.New("c2", 1).WithClbits(1).If(0, 1, gate.X, 0).Build()

	result := ir.Tensor(c1, c2)
	ops := result.Ops()
	if ops[1].Condition == nil {
		t.Fatal("ops[1] should have Condition")
	}
	if ops[1].Condition.Clbit != 1 {
		t.Errorf("Condition.Clbit = %d, want 1", ops[1].Condition.Clbit)
	}
}

func TestInverse(t *testing.T) {
	c, _ := builder.New("test", 2).H(0).CNOT(0, 1).T(0).Build()

	inv := ir.Inverse(c)
	ops := inv.Ops()
	if len(ops) != 3 {
		t.Fatalf("len(Ops()) = %d, want 3", len(ops))
	}
	// Order should be reversed: T†, CNOT†, H†.
	// T† = Tdg, CNOT† = CNOT (self-inverse), H† = H (self-inverse).
	if ops[0].Qubits[0] != 0 { // T† on qubit 0
		t.Errorf("ops[0].Qubits = %v, want [0]", ops[0].Qubits)
	}
	if ops[1].Gate.Name() != "CNOT" {
		t.Errorf("ops[1].Gate.Name() = %q, want CNOT", ops[1].Gate.Name())
	}
	if ops[2].Gate.Name() != "H" {
		t.Errorf("ops[2].Gate.Name() = %q, want H", ops[2].Gate.Name())
	}

	if inv.Name() != "test†" {
		t.Errorf("Name() = %q, want %q", inv.Name(), "test†")
	}
}

func TestInverseDropsMeasurementsResetsBarriers(t *testing.T) {
	c, _ := builder.New("test", 2).
		WithClbits(2).
		H(0).
		Barrier(0, 1).
		Reset(0).
		Measure(0, 0).
		Build()

	inv := ir.Inverse(c)
	ops := inv.Ops()
	// Only H† should remain.
	if len(ops) != 1 {
		t.Fatalf("len(Ops()) = %d, want 1", len(ops))
	}
	if ops[0].Gate.Name() != "H" {
		t.Errorf("ops[0].Gate.Name() = %q, want H", ops[0].Gate.Name())
	}
}

func TestInverseEmpty(t *testing.T) {
	c, _ := builder.New("empty", 2).Build()
	inv := ir.Inverse(c)
	if len(inv.Ops()) != 0 {
		t.Errorf("len(Ops()) = %d, want 0", len(inv.Ops()))
	}
}

func TestRepeat(t *testing.T) {
	c, _ := builder.New("layer", 2).H(0).CNOT(0, 1).Build()

	r, err := ir.Repeat(c, 3)
	if err != nil {
		t.Fatal(err)
	}
	ops := r.Ops()
	if len(ops) != 6 { // 2 ops × 3
		t.Fatalf("len(Ops()) = %d, want 6", len(ops))
	}
	// Verify pattern repeats.
	for i := 0; i < 3; i++ {
		if ops[i*2].Gate.Name() != "H" {
			t.Errorf("ops[%d].Gate.Name() = %q, want H", i*2, ops[i*2].Gate.Name())
		}
		if ops[i*2+1].Gate.Name() != "CNOT" {
			t.Errorf("ops[%d].Gate.Name() = %q, want CNOT", i*2+1, ops[i*2+1].Gate.Name())
		}
	}
	if r.Name() != "layer×3" {
		t.Errorf("Name() = %q, want %q", r.Name(), "layer×3")
	}
}

func TestRepeatOne(t *testing.T) {
	c, _ := builder.New("single", 1).X(0).Build()
	r, err := ir.Repeat(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Ops()) != 1 {
		t.Errorf("len(Ops()) = %d, want 1", len(r.Ops()))
	}
}

func TestRepeatInvalidN(t *testing.T) {
	c, _ := builder.New("test", 1).X(0).Build()
	_, err := ir.Repeat(c, 0)
	if err == nil {
		t.Fatal("expected error for n=0")
	}
	_, err = ir.Repeat(c, -1)
	if err == nil {
		t.Fatal("expected error for n=-1")
	}
}

// Integration test: circuit + inverse should act as identity.
func TestComposeWithInverseIsIdentity(t *testing.T) {
	// Build a non-trivial circuit.
	c, _ := builder.New("qft-like", 2).
		H(0).
		Phase(math.Pi/4, 1).
		CNOT(0, 1).
		RZ(math.Pi/3, 0).
		Build()

	inv := ir.Inverse(c)
	identity, err := ir.Compose(c, inv, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate: result should be |00⟩.
	sim := statevector.New(2)
	if err := sim.Evolve(identity); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	// |00⟩ = sv[0] ≈ 1, all others ≈ 0.
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

func TestTensorStats(t *testing.T) {
	c1, _ := builder.New("a", 2).H(0).CNOT(0, 1).Build()
	c2, _ := builder.New("b", 1).X(0).Build()

	result := ir.Tensor(c1, c2)
	stats := result.Stats()
	if stats.GateCount != 3 {
		t.Errorf("GateCount = %d, want 3", stats.GateCount)
	}
	if stats.TwoQubitGates != 1 {
		t.Errorf("TwoQubitGates = %d, want 1", stats.TwoQubitGates)
	}
}
