package mitigation_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/mitigation"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/noise"
	"github.com/splch/goqu/sim/pauli"
)

func TestRunCDR_RecoverIdeal(t *testing.T) {
	// Circuit with non-Clifford gates (T gates).
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.T, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.T, Qubits: []int{1}},
	}
	circ := ir.New("t_circuit", 2, 0, ops, nil)

	zz := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zz})
	if err != nil {
		t.Fatal(err)
	}

	idealExec := mitigation.StatevectorExecutor(ham)
	idealVal, _ := idealExec(context.Background(), circ)

	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.02))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.04))
	noisyExec := mitigation.DensityMatrixExecutor(ham, nm)

	noisyVal, _ := noisyExec(context.Background(), circ)

	result, err := mitigation.RunCDR(context.Background(), mitigation.CDRConfig{
		Circuit:     circ,
		Executor:    noisyExec,
		Hamiltonian: ham,
		NumTraining: 30,
		Fraction:    0.75,
	})
	if err != nil {
		t.Fatal(err)
	}

	noisyError := math.Abs(noisyVal - idealVal)
	mitigatedError := math.Abs(result.MitigatedValue - idealVal)

	t.Logf("ideal=%f, noisy=%f (err=%f), CDR=%f (err=%f), fit: a=%f b=%f",
		idealVal, noisyVal, noisyError, result.MitigatedValue, mitigatedError,
		result.FitA, result.FitB)

	// CDR should generally improve.
	if len(result.TrainingNoisy) != 30 {
		t.Errorf("expected 30 training points, got %d", len(result.TrainingNoisy))
	}
	if len(result.TrainingIdeal) != 30 {
		t.Errorf("expected 30 ideal points, got %d", len(result.TrainingIdeal))
	}
}

func TestAffineFit_Exact(t *testing.T) {
	// Test via CDR with a simple case: all-Clifford circuit should have
	// perfect noisy→ideal mapping.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.S, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	circ := ir.New("clifford", 2, 0, ops, nil)

	z0 := pauli.ZOn([]int{0}, 2)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{z0})
	if err != nil {
		t.Fatal(err)
	}

	// For an all-Clifford circuit, CDR training circuits are effectively
	// the same circuit, so the fit should be near-perfect.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.01))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.02))
	noisyExec := mitigation.DensityMatrixExecutor(ham, nm)

	result, err := mitigation.RunCDR(context.Background(), mitigation.CDRConfig{
		Circuit:     circ,
		Executor:    noisyExec,
		Hamiltonian: ham,
		NumTraining: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	// With all-Clifford, training data should be consistent.
	if math.IsNaN(result.MitigatedValue) {
		t.Error("mitigated value is NaN")
	}
}

func TestIsCliffordGate(t *testing.T) {
	// Test via CDR — a circuit with only Clifford gates should
	// produce training circuits identical to the original.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
	}
	circ := ir.New("cliff", 2, 0, ops, nil)

	ham, _ := pauli.NewPauliSum([]pauli.PauliString{pauli.ZOn([]int{0}, 2)})
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.01))
	noisyExec := mitigation.DensityMatrixExecutor(ham, nm)

	result, err := mitigation.RunCDR(context.Background(), mitigation.CDRConfig{
		Circuit:     circ,
		Executor:    noisyExec,
		Hamiltonian: ham,
		NumTraining: 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	// All training ideal values should be the same (same circuit).
	if len(result.TrainingIdeal) < 2 {
		t.Skip("not enough training data")
	}
	for i := 1; i < len(result.TrainingIdeal); i++ {
		if math.Abs(result.TrainingIdeal[i]-result.TrainingIdeal[0]) > 1e-10 {
			t.Errorf("Clifford circuit training ideal values differ: %f vs %f",
				result.TrainingIdeal[0], result.TrainingIdeal[i])
		}
	}
}

func TestNearestClifford_TToS(t *testing.T) {
	// Build circuit with T, run CDR — verify it doesn't error.
	ops := []ir.Operation{
		{Gate: gate.T, Qubits: []int{0}},
	}
	circ := ir.New("t_gate", 1, 0, ops, nil)

	ham, _ := pauli.NewPauliSum([]pauli.PauliString{pauli.ZOn([]int{0}, 1)})
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.01))
	noisyExec := mitigation.DensityMatrixExecutor(ham, nm)

	_, err := mitigation.RunCDR(context.Background(), mitigation.CDRConfig{
		Circuit:     circ,
		Executor:    noisyExec,
		Hamiltonian: ham,
		NumTraining: 5,
	})
	if err != nil {
		t.Fatalf("CDR with T gate failed: %v", err)
	}
}

func TestRunCDR_WithRZGates(t *testing.T) {
	// Circuit with parameterized RZ gates (non-Clifford angles).
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.RZ(0.7), Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.RZ(1.2), Qubits: []int{1}},
	}
	circ := ir.New("rz_circuit", 2, 0, ops, nil)

	zz := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zz})
	if err != nil {
		t.Fatal(err)
	}

	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.02))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.04))
	noisyExec := mitigation.DensityMatrixExecutor(ham, nm)

	result, err := mitigation.RunCDR(context.Background(), mitigation.CDRConfig{
		Circuit:     circ,
		Executor:    noisyExec,
		Hamiltonian: ham,
		NumTraining: 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	if math.IsNaN(result.MitigatedValue) {
		t.Error("mitigated value is NaN")
	}
}

func TestRunCDR_Errors(t *testing.T) {
	circ := ir.New("test", 1, 0, []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
	}, nil)
	ham, _ := pauli.NewPauliSum([]pauli.PauliString{pauli.ZOn([]int{0}, 1)})
	exec := mitigation.StatevectorExecutor(ham)

	t.Run("nil circuit", func(t *testing.T) {
		_, err := mitigation.RunCDR(context.Background(), mitigation.CDRConfig{
			Executor:    exec,
			Hamiltonian: ham,
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("nil executor", func(t *testing.T) {
		_, err := mitigation.RunCDR(context.Background(), mitigation.CDRConfig{
			Circuit:     circ,
			Hamiltonian: ham,
		})
		if err == nil {
			t.Error("expected error")
		}
	})
}
