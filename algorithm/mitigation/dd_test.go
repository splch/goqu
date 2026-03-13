package mitigation_test

import (
	"testing"

	"github.com/splch/goqu/algorithm/mitigation"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

func TestInsertDD_PreservesExpectation(t *testing.T) {
	// Build a circuit with idle periods: H(0), CNOT(0,1), H(0).
	// Qubit 1 is idle during the first H and last H.
	circ, err := builder.New("test", 2).
		H(0).
		CNOT(0, 1).
		H(0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Ideal expectation.
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

	for _, seq := range []mitigation.DDSequence{mitigation.DDXX, mitigation.DDXY4} {
		ddCirc, err := mitigation.InsertDD(mitigation.DDConfig{
			Circuit:  circ,
			Sequence: seq,
		})
		if err != nil {
			t.Fatal(err)
		}

		sim2 := statevector.New(2)
		if err := sim2.Evolve(ddCirc); err != nil {
			t.Fatal(err)
		}
		ddVal := sim2.ExpectPauliSum(ham)

		// DD should preserve the ideal expectation (XX and XY4 are identity-equivalent).
		if abs(ddVal-idealVal) > 1e-10 {
			t.Errorf("DD sequence %d changed expectation: ideal=%f, dd=%f", seq, idealVal, ddVal)
		}
	}
}

func TestInsertDD_IncreasesGateCount(t *testing.T) {
	// Build a circuit with a clear idle period.
	circ, err := builder.New("test", 3).
		H(0).
		CNOT(0, 1).
		H(0).
		CNOT(0, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ddCirc, err := mitigation.InsertDD(mitigation.DDConfig{
		Circuit:  circ,
		Sequence: mitigation.DDXX,
	})
	if err != nil {
		t.Fatal(err)
	}

	origCount := len(circ.Ops())
	ddCount := len(ddCirc.Ops())

	if ddCount <= origCount {
		t.Errorf("expected DD to add gates: original=%d, dd=%d", origCount, ddCount)
	}
}

func TestInsertDD_NoIdlePeriod(t *testing.T) {
	// Single-qubit circuit with no idle periods.
	circ, err := builder.New("test", 1).
		H(0).
		X(0).
		H(0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ddCirc, err := mitigation.InsertDD(mitigation.DDConfig{
		Circuit:  circ,
		Sequence: mitigation.DDXX,
	})
	if err != nil {
		t.Fatal(err)
	}

	// No idle periods → gate count should be the same.
	if len(ddCirc.Ops()) != len(circ.Ops()) {
		t.Errorf("expected no DD insertion: original=%d, dd=%d", len(circ.Ops()), len(ddCirc.Ops()))
	}
}

func TestInsertDD_NilCircuit(t *testing.T) {
	_, err := mitigation.InsertDD(mitigation.DDConfig{})
	if err == nil {
		t.Error("expected error for nil circuit")
	}
}

func TestInsertDD_EmptyCircuit(t *testing.T) {
	circ, err := builder.New("empty", 2).Build()
	if err != nil {
		t.Fatal(err)
	}

	ddCirc, err := mitigation.InsertDD(mitigation.DDConfig{
		Circuit: circ,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(ddCirc.Ops()) != 0 {
		t.Errorf("expected empty circuit, got %d ops", len(ddCirc.Ops()))
	}
}

func TestInsertDD_XY4Sequence(t *testing.T) {
	// Build a circuit with enough idle time for XY4 (4 gates).
	// 6 layers of idle for qubit 2 while qubit 0,1 are busy.
	circ, err := builder.New("test", 3).
		H(0).
		X(0).
		H(0).
		X(0).
		H(0).
		CNOT(0, 1).
		CNOT(0, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ddCirc, err := mitigation.InsertDD(mitigation.DDConfig{
		Circuit:  circ,
		Sequence: mitigation.DDXY4,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should have added XY4 gates.
	if len(ddCirc.Ops()) <= len(circ.Ops()) {
		t.Errorf("expected XY4 DD insertion: original=%d, dd=%d", len(circ.Ops()), len(ddCirc.Ops()))
	}

	// Verify expectation is preserved.
	z2 := pauli.NewPauliString(1, map[int]pauli.Pauli{2: pauli.Z}, 3)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{z2})
	if err != nil {
		t.Fatal(err)
	}

	sim1 := statevector.New(3)
	if err := sim1.Evolve(circ); err != nil {
		t.Fatal(err)
	}
	idealVal := sim1.ExpectPauliSum(ham)

	sim2 := statevector.New(3)
	if err := sim2.Evolve(ddCirc); err != nil {
		t.Fatal(err)
	}
	ddVal := sim2.ExpectPauliSum(ham)

	if abs(ddVal-idealVal) > 1e-10 {
		t.Errorf("XY4 changed expectation: ideal=%f, dd=%f", idealVal, ddVal)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
