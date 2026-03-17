package pass

import (
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
	"github.com/splch/goqu/transpile/verify"
)

func TestConsolidate2Q_DoubleInverse(t *testing.T) {
	// CNOT * CNOT = I — should be removed entirely.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("dbl", 2, 0, ops, nil)
	out, err := Consolidate2QBlocks(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	if out.NumOps() != 0 {
		t.Errorf("expected 0 ops (identity block), got %d", out.NumOps())
	}
}

func TestConsolidate2Q_MixedBlock(t *testing.T) {
	// CX, H(0), CX — block of 3 ops on qubits 0,1.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("mix", 2, 0, ops, nil)
	out, err := Consolidate2QBlocks(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	// The combined unitary should be re-synthesized.
	// Verify equivalence.
	eq, eqErr := verify.EquivalentOnZero(c, out, 1e-6)
	if eqErr != nil {
		t.Fatal(eqErr)
	}
	if !eq {
		t.Error("consolidated circuit not equivalent to original")
	}
}

func TestConsolidate2Q_DifferentPairNoMerge(t *testing.T) {
	// CX(0,1) then CX(0,2) — different pairs, first block has only 1 op.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 2}},
	}
	c := ir.New("diff", 3, 0, ops, nil)
	out, err := Consolidate2QBlocks(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	if out != c {
		t.Error("expected same circuit pointer (no blocks to consolidate)")
	}
}

func TestConsolidate2Q_BlockEndsMeasurement(t *testing.T) {
	// CX(0,1), measurement on qubit 0, CX(0,1) — block should not span measurement.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: nil, Qubits: []int{0}, Clbits: []int{0}}, // measurement
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("meas", 2, 1, ops, nil)
	out, err := Consolidate2QBlocks(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	// Each CX is in its own block (single op) — no consolidation.
	if out != c {
		t.Error("expected same circuit (no multi-op blocks)")
	}
}

func TestConsolidate2Q_IBMBasis(t *testing.T) {
	// Two CX gates with target IBM Eagle basis.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("ibm", 2, 0, ops, nil)
	out, err := Consolidate2QBlocks(c, target.IBMEagle)
	if err != nil {
		t.Fatal(err)
	}
	if out.NumOps() != 0 {
		t.Errorf("expected 0 ops, got %d", out.NumOps())
	}
}

func TestConsolidate2Q_PreservesThirdQubit(t *testing.T) {
	// CX(0,1), H(2), CX(0,1) — H(2) is on a different qubit, should not break block.
	ops := []ir.Operation{
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.H, Qubits: []int{2}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	c := ir.New("third", 3, 0, ops, nil)
	out, err := Consolidate2QBlocks(c, target.Simulator)
	if err != nil {
		t.Fatal(err)
	}
	// CX*CX = I, H(2) remains.
	eq, eqErr := verify.EquivalentOnZero(c, out, 1e-6)
	if eqErr != nil {
		t.Fatal(eqErr)
	}
	if !eq {
		t.Error("consolidated circuit not equivalent to original")
	}
	// Should have just the H(2).
	if out.NumOps() != 1 {
		t.Errorf("expected 1 op (H on qubit 2), got %d", out.NumOps())
	}
}
