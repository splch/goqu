package grover_test

import (
	"context"
	"testing"

	"github.com/splch/goqu/algorithm/grover"
)

func TestGrover_3Qubit(t *testing.T) {
	// Search for state |101⟩ = 5 on 3 qubits.
	oracle := grover.PhaseOracle([]int{5}, 3)
	cfg := grover.Config{
		NumQubits: 3,
		Oracle:    oracle,
		Shots:     2048,
	}

	res, err := grover.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// The top result should be "101" (bitstring with qubit 0 = MSB reversed convention).
	// Convention: leftmost = highest qubit, so state 5 = 101 binary:
	// q2=1, q1=0, q0=1 -> bitstring "101"
	if res.TopResult != "101" {
		t.Errorf("top result = %q, want \"101\"", res.TopResult)
	}

	// Check that the marked state has the majority of counts.
	total := 0
	for _, v := range res.Counts {
		total += v
	}
	if res.Counts["101"] < total/2 {
		t.Errorf("expected |101⟩ to dominate, counts=%v", res.Counts)
	}
}

func TestGrover_2Qubit(t *testing.T) {
	// Search for |11⟩ = 3 on 2 qubits.
	oracle := grover.PhaseOracle([]int{3}, 2)
	cfg := grover.Config{
		NumQubits: 2,
		Oracle:    oracle,
		Shots:     1024,
	}

	res, err := grover.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if res.TopResult != "11" {
		t.Errorf("top result = %q, want \"11\"", res.TopResult)
	}
}

func TestGrover_MultipleSolutions(t *testing.T) {
	// 2 solutions out of 8 states.
	oracle := grover.PhaseOracle([]int{2, 5}, 3)
	cfg := grover.Config{
		NumQubits:    3,
		Oracle:       oracle,
		NumSolutions: 2,
		Shots:        2048,
	}

	res, err := grover.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// One of the two solutions should be the top result.
	if res.TopResult != "010" && res.TopResult != "101" {
		t.Errorf("top result = %q, expected \"010\" or \"101\"", res.TopResult)
	}
}

func TestGrover_BooleanOracle(t *testing.T) {
	// Search for x where x%3 == 0 in 3-qubit space (0, 3, 6).
	oracle := grover.BooleanOracle(func(x int) bool { return x%3 == 0 && x > 0 }, 3)
	cfg := grover.Config{
		NumQubits:    3,
		Oracle:       oracle,
		NumSolutions: 2,
		Shots:        2048,
	}

	res, err := grover.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should find 3 (011) or 6 (110).
	if res.TopResult != "011" && res.TopResult != "110" {
		t.Errorf("top result = %q, expected \"011\" or \"110\"", res.TopResult)
	}
}

func TestGrover_Errors(t *testing.T) {
	_, err := grover.Run(context.Background(), grover.Config{NumQubits: 0})
	if err == nil {
		t.Error("expected error for 0 qubits")
	}

	_, err = grover.Run(context.Background(), grover.Config{NumQubits: 2})
	if err == nil {
		t.Error("expected error for nil oracle")
	}
}
