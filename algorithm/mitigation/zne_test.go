package mitigation_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/mitigation"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/noise"
	"github.com/splch/goqu/sim/pauli"
)

func TestRunZNE_RecoverIdeal(t *testing.T) {
	// Build a 2-qubit Bell circuit: H-CNOT.
	// Observable: Z0⊗Z1. Ideal ⟨Z0Z1⟩ = 1 for |Φ+⟩.
	// Depolarizing noise pushes this toward 0.
	circ, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Observable: Z0 Z1 (ZZ correlation).
	zz := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	hamiltonian, err := pauli.NewPauliSum([]pauli.PauliString{zz})
	if err != nil {
		t.Fatal(err)
	}

	// Compute ideal expectation value (noiseless).
	idealExec := mitigation.StatevectorExecutor(hamiltonian)
	idealVal, err := idealExec(context.Background(), circ)
	if err != nil {
		t.Fatal(err)
	}

	// Create a noisy executor with depolarizing noise.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.02))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.04))
	noisyExec := mitigation.DensityMatrixExecutor(hamiltonian, nm)

	// Get raw noisy value.
	noisyVal, err := noisyExec(context.Background(), circ)
	if err != nil {
		t.Fatal(err)
	}

	// Run ZNE.
	result, err := mitigation.RunZNE(context.Background(), mitigation.ZNEConfig{
		Circuit:      circ,
		Executor:     noisyExec,
		ScaleFactors: []float64{1, 3, 5},
		Extrapolator: mitigation.LinearExtrapolator,
	})
	if err != nil {
		t.Fatal(err)
	}

	// ZNE should bring the value closer to ideal than raw noisy.
	noisyError := math.Abs(noisyVal - idealVal)
	mitigatedError := math.Abs(result.MitigatedValue - idealVal)

	if mitigatedError >= noisyError {
		t.Errorf("ZNE did not improve: ideal=%f, noisy=%f (err=%f), mitigated=%f (err=%f)",
			idealVal, noisyVal, noisyError, result.MitigatedValue, mitigatedError)
	}

	// Verify result fields.
	if len(result.NoisyValues) != 3 {
		t.Errorf("expected 3 noisy values, got %d", len(result.NoisyValues))
	}
	if len(result.ScaleFactors) != 3 {
		t.Errorf("expected 3 scale factors, got %d", len(result.ScaleFactors))
	}
}

func TestRunZNE_Defaults(t *testing.T) {
	circ, err := builder.New("test", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	hamiltonian, err := pauli.NewPauliSum([]pauli.PauliString{
		pauli.ZOn([]int{0}, 1),
	})
	if err != nil {
		t.Fatal(err)
	}

	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.01))
	exec := mitigation.DensityMatrixExecutor(hamiltonian, nm)

	// Use defaults: should not error.
	result, err := mitigation.RunZNE(context.Background(), mitigation.ZNEConfig{
		Circuit:  circ,
		Executor: exec,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Default scale factors are [1, 3, 5].
	if len(result.ScaleFactors) != 3 {
		t.Errorf("default scale factors: got %d, want 3", len(result.ScaleFactors))
	}
}

func TestRunZNE_Errors(t *testing.T) {
	circ, err := builder.New("test", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	exec := mitigation.StatevectorExecutor(pauli.PauliSum{})

	t.Run("nil circuit", func(t *testing.T) {
		_, err := mitigation.RunZNE(context.Background(), mitigation.ZNEConfig{
			Executor: exec,
		})
		if err == nil {
			t.Error("expected error for nil circuit")
		}
	})

	t.Run("nil executor", func(t *testing.T) {
		_, err := mitigation.RunZNE(context.Background(), mitigation.ZNEConfig{
			Circuit: circ,
		})
		if err == nil {
			t.Error("expected error for nil executor")
		}
	})

	t.Run("even scale factor", func(t *testing.T) {
		_, err := mitigation.RunZNE(context.Background(), mitigation.ZNEConfig{
			Circuit:      circ,
			Executor:     exec,
			ScaleFactors: []float64{1, 2, 3},
		})
		if err == nil {
			t.Error("expected error for even scale factor")
		}
	})
}
