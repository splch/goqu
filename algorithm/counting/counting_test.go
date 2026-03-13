package counting_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/counting"
	"github.com/splch/goqu/algorithm/grover"
)

func TestCounting_OneSolution(t *testing.T) {
	// 3 qubits, 1 marked state (state 5 = |101⟩) → count ≈ 1.0
	oracle := grover.PhaseOracle([]int{5}, 3)
	cfg := counting.Config{
		NumQubits:    3,
		Oracle:       oracle,
		NumPhaseBits: 4,
		Shots:        2048,
	}
	res, err := counting.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(res.Count-1.0) > 1.5 {
		t.Errorf("count = %f, want ~1.0", res.Count)
	}
}

func TestCounting_TwoSolutions(t *testing.T) {
	// 3 qubits, 2 marked states → count ≈ 2.0
	oracle := grover.PhaseOracle([]int{2, 5}, 3)
	cfg := counting.Config{
		NumQubits:    3,
		Oracle:       oracle,
		NumPhaseBits: 4,
		Shots:        2048,
	}
	res, err := counting.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(res.Count-2.0) > 1.5 {
		t.Errorf("count = %f, want ~2.0", res.Count)
	}
}

func TestCounting_Errors(t *testing.T) {
	oracle := grover.PhaseOracle([]int{0}, 2)

	_, err := counting.Run(context.Background(), counting.Config{
		NumQubits:    0,
		Oracle:       oracle,
		NumPhaseBits: 3,
	})
	if err == nil {
		t.Error("expected error for 0 qubits")
	}

	_, err = counting.Run(context.Background(), counting.Config{
		NumQubits:    2,
		Oracle:       oracle,
		NumPhaseBits: 0,
	})
	if err == nil {
		t.Error("expected error for 0 phase bits")
	}

	_, err = counting.Run(context.Background(), counting.Config{
		NumQubits:    2,
		Oracle:       nil,
		NumPhaseBits: 3,
	})
	if err == nil {
		t.Error("expected error for nil oracle")
	}
}
