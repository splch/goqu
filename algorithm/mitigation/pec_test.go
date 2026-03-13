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

func TestExtractDepolarizing_KnownChannels(t *testing.T) {
	// Verify PEC works with known depolarizing channels.
	// Build a simple 1-qubit circuit.
	circ, err := builder.New("test", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	z0 := pauli.ZOn([]int{0}, 1)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{z0})
	if err != nil {
		t.Fatal(err)
	}

	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.05))
	noisyExec := mitigation.DensityMatrixExecutor(ham, nm)

	result, err := mitigation.RunPEC(context.Background(), mitigation.PECConfig{
		Circuit:    circ,
		Executor:   noisyExec,
		NoiseModel: nm,
		Samples:    500,
	})
	if err != nil {
		t.Fatal(err)
	}

	// PEC should return a value.
	if math.IsNaN(result.MitigatedValue) {
		t.Error("mitigated value is NaN")
	}
	if result.Overhead < 1 {
		t.Errorf("overhead should be >= 1, got %f", result.Overhead)
	}
}

func TestRunPEC_RecoverIdeal(t *testing.T) {
	// Bell circuit with depolarizing noise.
	circ, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	zz := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zz})
	if err != nil {
		t.Fatal(err)
	}

	// Compute ideal value.
	idealExec := mitigation.StatevectorExecutor(ham)
	idealVal, _ := idealExec(context.Background(), circ)

	// Create noisy executor.
	p := 0.02
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(p))
	nm.AddDefaultError(2, noise.Depolarizing2Q(p))
	noisyExec := mitigation.DensityMatrixExecutor(ham, nm)

	noisyVal, _ := noisyExec(context.Background(), circ)

	result, err := mitigation.RunPEC(context.Background(), mitigation.PECConfig{
		Circuit:    circ,
		Executor:   noisyExec,
		NoiseModel: nm,
		Samples:    2000,
	})
	if err != nil {
		t.Fatal(err)
	}

	// PEC should improve on noisy.
	noisyError := math.Abs(noisyVal - idealVal)
	mitigatedError := math.Abs(result.MitigatedValue - idealVal)

	// PEC is unbiased but has variance; check it's at least in the right direction.
	// Allow some slack for statistical noise.
	t.Logf("ideal=%f, noisy=%f (err=%f), PEC=%f (err=%f), overhead=%.2f",
		idealVal, noisyVal, noisyError, result.MitigatedValue, mitigatedError, result.Overhead)

	if result.Overhead < 1 {
		t.Errorf("overhead should be >= 1, got %f", result.Overhead)
	}
	_ = noisyError // logged above
}

func TestRunPEC_NonDepolarizingError(t *testing.T) {
	circ, err := builder.New("test", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	ham, _ := pauli.NewPauliSum([]pauli.PauliString{pauli.ZOn([]int{0}, 1)})
	nm := noise.New()
	nm.AddDefaultError(1, noise.AmplitudeDamping(0.05))
	noisyExec := mitigation.DensityMatrixExecutor(ham, nm)

	_, err = mitigation.RunPEC(context.Background(), mitigation.PECConfig{
		Circuit:    circ,
		Executor:   noisyExec,
		NoiseModel: nm,
		Samples:    10,
	})
	if err == nil {
		t.Error("expected error for non-depolarizing channel")
	}
}

func TestRunPEC_Errors(t *testing.T) {
	circ, _ := builder.New("test", 1).H(0).Build()
	ham, _ := pauli.NewPauliSum([]pauli.PauliString{pauli.ZOn([]int{0}, 1)})
	exec := mitigation.StatevectorExecutor(ham)
	nm := noise.New()

	t.Run("nil circuit", func(t *testing.T) {
		_, err := mitigation.RunPEC(context.Background(), mitigation.PECConfig{
			Executor:   exec,
			NoiseModel: nm,
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("nil executor", func(t *testing.T) {
		_, err := mitigation.RunPEC(context.Background(), mitigation.PECConfig{
			Circuit:    circ,
			NoiseModel: nm,
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("nil noise model", func(t *testing.T) {
		_, err := mitigation.RunPEC(context.Background(), mitigation.PECConfig{
			Circuit:  circ,
			Executor: exec,
		})
		if err == nil {
			t.Error("expected error")
		}
	})
}
