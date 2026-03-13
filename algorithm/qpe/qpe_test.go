package qpe_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/qpe"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
)

func TestQPE_TGate(t *testing.T) {
	// T gate eigenvalue: e^{2πi * 1/8}, so phase = 1/8 = 0.125.
	// T|1⟩ = e^{iπ/4}|1⟩ = e^{2πi * 1/8}|1⟩
	// Need to prepare |1⟩ as the eigenstate.
	eigenState, err := builder.New("eigenstate", 1).X(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	cfg := qpe.Config{
		Unitary:      gate.T,
		NumPhaseBits: 3,
		EigenState:   eigenState,
		Shots:        2048,
	}

	res, err := qpe.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(res.Phase-0.125) > 0.01 {
		t.Errorf("phase = %f, want 0.125", res.Phase)
	}
}

func TestQPE_SGate(t *testing.T) {
	// S gate: e^{2πi * 1/4}, so phase = 0.25.
	// S|1⟩ = e^{iπ/2}|1⟩ = e^{2πi * 0.25}|1⟩
	eigenState, err := builder.New("eigenstate", 1).X(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	cfg := qpe.Config{
		Unitary:      gate.S,
		NumPhaseBits: 3,
		EigenState:   eigenState,
		Shots:        1024,
	}

	res, err := qpe.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(res.Phase-0.25) > 0.01 {
		t.Errorf("phase = %f, want 0.25", res.Phase)
	}
}

func TestQPE_ZGate(t *testing.T) {
	// Z gate: e^{2πi * 1/2}, so phase = 0.5.
	// Z|1⟩ = -|1⟩ = e^{iπ}|1⟩ = e^{2πi * 0.5}|1⟩
	eigenState, err := builder.New("eigenstate", 1).X(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	cfg := qpe.Config{
		Unitary:      gate.Z,
		NumPhaseBits: 3,
		EigenState:   eigenState,
		Shots:        1024,
	}

	res, err := qpe.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(res.Phase-0.5) > 0.01 {
		t.Errorf("phase = %f, want 0.5", res.Phase)
	}
}

func TestQFT(t *testing.T) {
	circ, err := qpe.QFT(3)
	if err != nil {
		t.Fatal(err)
	}
	if circ.NumQubits() != 3 {
		t.Errorf("NumQubits() = %d, want 3", circ.NumQubits())
	}
}

func TestInverseQFT(t *testing.T) {
	circ, err := qpe.InverseQFT(3)
	if err != nil {
		t.Fatal(err)
	}
	if circ.NumQubits() != 3 {
		t.Errorf("NumQubits() = %d, want 3", circ.NumQubits())
	}
}

func TestQPE_Errors(t *testing.T) {
	_, err := qpe.Run(context.Background(), qpe.Config{NumPhaseBits: 3})
	if err == nil {
		t.Error("expected error for nil unitary")
	}

	_, err = qpe.Run(context.Background(), qpe.Config{Unitary: gate.T, NumPhaseBits: 0})
	if err == nil {
		t.Error("expected error for 0 phase bits")
	}
}
