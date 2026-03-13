package ampest_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/ampest"
	"github.com/splch/goqu/algorithm/grover"
	"github.com/splch/goqu/circuit/builder"
)

// TestAE_TwoQubit runs standard AE on an equal superposition of 4 states
// with one marked state (|11>). The expected amplitude is 0.5.
func TestAE_TwoQubit(t *testing.T) {
	// Prepare |+>|+> (equal superposition of 4 states).
	prep, err := builder.New("prep", 2).H(0).H(1).Build()
	if err != nil {
		t.Fatal(err)
	}

	// Mark state |11> = 3.
	oracle := grover.PhaseOracle([]int{3}, 2)

	cfg := ampest.Config{
		StatePrep:    prep,
		Oracle:       oracle,
		NumQubits:    2,
		NumPhaseBits: 4,
		Shots:        2048,
	}

	res, err := ampest.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Amplitude should be approximately 0.5 (1 good out of 4 states,
	// each with amplitude 1/2).
	if math.Abs(res.Amplitude-0.5) > 0.15 {
		t.Errorf("amplitude = %f, want ~0.5", res.Amplitude)
	}

	if res.Probability < 0 || res.Probability > 1 {
		t.Errorf("probability = %f, want value in [0, 1]", res.Probability)
	}

	if res.Circuit == nil {
		t.Error("circuit is nil")
	}

	if len(res.Counts) == 0 {
		t.Error("counts map is empty")
	}
}

// TestAE_SingleQubit runs standard AE on a single qubit with H state prep.
func TestAE_SingleQubit(t *testing.T) {
	prep, err := builder.New("prep", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	// Mark state |1> = 1.
	oracle := grover.PhaseOracle([]int{1}, 1)

	cfg := ampest.Config{
		StatePrep:    prep,
		Oracle:       oracle,
		NumQubits:    1,
		NumPhaseBits: 3,
		Shots:        1024,
	}

	res, err := ampest.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// H|0> = (|0>+|1>)/sqrt(2), amplitude of |1> is 1/sqrt(2) ~ 0.707.
	if math.Abs(res.Amplitude-math.Sqrt(0.5)) > 0.2 {
		t.Errorf("amplitude = %f, want ~%f", res.Amplitude, math.Sqrt(0.5))
	}
}

// TestIterativeAE runs iterative AE on the same 2-qubit problem and
// checks that it converges to a similar estimate.
func TestIterativeAE(t *testing.T) {
	prep, err := builder.New("prep", 2).H(0).H(1).Build()
	if err != nil {
		t.Fatal(err)
	}

	oracle := grover.PhaseOracle([]int{3}, 2)

	cfg := ampest.IterativeConfig{
		StatePrep: prep,
		Oracle:    oracle,
		NumQubits: 2,
		MaxIters:  5,
		Shots:     1024,
	}

	res, err := ampest.RunIterative(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(res.Amplitude-0.5) > 0.2 {
		t.Errorf("amplitude = %f, want ~0.5", res.Amplitude)
	}

	if res.NumIters != 5 {
		t.Errorf("numIters = %d, want 5", res.NumIters)
	}

	if res.ConfInterval[0] > res.ConfInterval[1] {
		t.Errorf("confidence interval [%f, %f] is invalid", res.ConfInterval[0], res.ConfInterval[1])
	}
}

// TestIterativeAE_SingleQubit tests iterative AE on a single-qubit problem.
func TestIterativeAE_SingleQubit(t *testing.T) {
	prep, err := builder.New("prep", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	oracle := grover.PhaseOracle([]int{1}, 1)

	cfg := ampest.IterativeConfig{
		StatePrep: prep,
		Oracle:    oracle,
		NumQubits: 1,
		MaxIters:  4,
	}

	res, err := ampest.RunIterative(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Amplitude of |1> in H|0> is 1/sqrt(2) ~ 0.707.
	if math.Abs(res.Amplitude-math.Sqrt(0.5)) > 0.25 {
		t.Errorf("amplitude = %f, want ~%f", res.Amplitude, math.Sqrt(0.5))
	}
}

// TestAE_Errors verifies that invalid configurations produce errors.
func TestAE_Errors(t *testing.T) {
	oracle := grover.PhaseOracle([]int{3}, 2)
	prep, _ := builder.New("prep", 2).H(0).H(1).Build()

	tests := []struct {
		name string
		cfg  ampest.Config
	}{
		{
			name: "nil state prep",
			cfg: ampest.Config{
				Oracle:       oracle,
				NumQubits:    2,
				NumPhaseBits: 3,
			},
		},
		{
			name: "nil oracle",
			cfg: ampest.Config{
				StatePrep:    prep,
				NumQubits:    2,
				NumPhaseBits: 3,
			},
		},
		{
			name: "zero qubits",
			cfg: ampest.Config{
				StatePrep:    prep,
				Oracle:       oracle,
				NumQubits:    0,
				NumPhaseBits: 3,
			},
		},
		{
			name: "zero phase bits",
			cfg: ampest.Config{
				StatePrep:    prep,
				Oracle:       oracle,
				NumQubits:    2,
				NumPhaseBits: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ampest.Run(context.Background(), tt.cfg)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// TestIterativeAE_Errors verifies that invalid iterative configurations
// produce errors.
func TestIterativeAE_Errors(t *testing.T) {
	oracle := grover.PhaseOracle([]int{3}, 2)
	prep, _ := builder.New("prep", 2).H(0).H(1).Build()

	tests := []struct {
		name string
		cfg  ampest.IterativeConfig
	}{
		{
			name: "nil state prep",
			cfg: ampest.IterativeConfig{
				Oracle:    oracle,
				NumQubits: 2,
			},
		},
		{
			name: "nil oracle",
			cfg: ampest.IterativeConfig{
				StatePrep: prep,
				NumQubits: 2,
			},
		},
		{
			name: "zero qubits",
			cfg: ampest.IterativeConfig{
				StatePrep: prep,
				Oracle:    oracle,
				NumQubits: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ampest.RunIterative(context.Background(), tt.cfg)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// TestAE_DefaultShots verifies that default shots are used when not specified.
func TestAE_DefaultShots(t *testing.T) {
	prep, err := builder.New("prep", 2).H(0).H(1).Build()
	if err != nil {
		t.Fatal(err)
	}

	oracle := grover.PhaseOracle([]int{3}, 2)

	cfg := ampest.Config{
		StatePrep:    prep,
		Oracle:       oracle,
		NumQubits:    2,
		NumPhaseBits: 3,
		// Shots omitted: should default to 1024.
	}

	res, err := ampest.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	total := 0
	for _, v := range res.Counts {
		total += v
	}
	if total != 1024 {
		t.Errorf("total shots = %d, want 1024 (default)", total)
	}
}
