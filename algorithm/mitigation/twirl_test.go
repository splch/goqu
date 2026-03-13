package mitigation_test

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/splch/goqu/algorithm/mitigation"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

func TestTwirlCircuit_PreservesUnitary(t *testing.T) {
	// A twirled circuit should produce the same expectation value under
	// ideal (noiseless) simulation as the original circuit.
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

	sim1 := statevector.New(2)
	if err := sim1.Evolve(circ); err != nil {
		t.Fatal(err)
	}
	idealVal := sim1.ExpectPauliSum(ham)

	// Check multiple random twirls.
	rng := rand.New(rand.NewSource(42))
	for i := range 20 {
		twirled, err := mitigation.TwirlCircuit(circ, rng)
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}

		sim2 := statevector.New(2)
		if err := sim2.Evolve(twirled); err != nil {
			t.Fatalf("sample %d: evolve: %v", i, err)
		}
		twirlVal := sim2.ExpectPauliSum(ham)

		if math.Abs(twirlVal-idealVal) > 1e-10 {
			t.Errorf("sample %d: twirl changed expectation: ideal=%f, twirled=%f", i, idealVal, twirlVal)
		}
	}
}

func TestTwirlCircuit_CZPreservesUnitary(t *testing.T) {
	// Test with CZ gate.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.CZ, Qubits: []int{0, 1}},
	}
	circ := ir.New("cz_test", 2, 0, ops, nil)

	zz := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zz})
	if err != nil {
		t.Fatal(err)
	}

	sim1 := statevector.New(2)
	if err := sim1.Evolve(circ); err != nil {
		t.Fatal(err)
	}
	idealVal := sim1.ExpectPauliSum(ham)

	rng := rand.New(rand.NewSource(99))
	for i := range 20 {
		twirled, err := mitigation.TwirlCircuit(circ, rng)
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}

		sim2 := statevector.New(2)
		if err := sim2.Evolve(twirled); err != nil {
			t.Fatalf("sample %d: evolve: %v", i, err)
		}
		twirlVal := sim2.ExpectPauliSum(ham)

		if math.Abs(twirlVal-idealVal) > 1e-10 {
			t.Errorf("sample %d: CZ twirl changed expectation: ideal=%f, twirled=%f", i, idealVal, twirlVal)
		}
	}
}

func TestTwirlCircuit_UnsupportedGate(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.SWAP, Qubits: []int{0, 1}},
	}
	circ := ir.New("swap_test", 2, 0, ops, nil)

	rng := rand.New(rand.NewSource(0))
	_, err := mitigation.TwirlCircuit(circ, rng)
	if err == nil {
		t.Error("expected error for unsupported 2Q gate SWAP")
	}
}

func TestRunTwirl_IdealRecovery(t *testing.T) {
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

	idealExec := mitigation.StatevectorExecutor(ham)

	result, err := mitigation.RunTwirl(context.Background(), mitigation.TwirlConfig{
		Circuit:  circ,
		Executor: idealExec,
		Samples:  50,
	})
	if err != nil {
		t.Fatal(err)
	}

	// With ideal executor, mitigated value should equal ideal.
	idealVal, _ := idealExec(context.Background(), circ)
	if math.Abs(result.MitigatedValue-idealVal) > 1e-10 {
		t.Errorf("ideal twirl: expected=%f, got=%f", idealVal, result.MitigatedValue)
	}

	if len(result.RawValues) != 50 {
		t.Errorf("expected 50 raw values, got %d", len(result.RawValues))
	}
}

func TestRunTwirl_Errors(t *testing.T) {
	circ, _ := builder.New("test", 2).H(0).CNOT(0, 1).Build()
	exec := mitigation.StatevectorExecutor(pauli.PauliSum{})

	t.Run("nil circuit", func(t *testing.T) {
		_, err := mitigation.RunTwirl(context.Background(), mitigation.TwirlConfig{
			Executor: exec,
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("nil executor", func(t *testing.T) {
		_, err := mitigation.RunTwirl(context.Background(), mitigation.TwirlConfig{
			Circuit: circ,
		})
		if err == nil {
			t.Error("expected error")
		}
	})
}
