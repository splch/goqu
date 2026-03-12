package noise

import (
	"math"
	"testing"
)

func TestCustom_ValidChannel(t *testing.T) {
	// Identity channel: single Kraus op = I
	ch, err := Custom("identity", 1, [][]complex128{
		{1, 0, 0, 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Name() != "identity" {
		t.Errorf("Name = %q, want identity", ch.Name())
	}
	if ch.Qubits() != 1 {
		t.Errorf("Qubits = %d, want 1", ch.Qubits())
	}
	if len(ch.Kraus()) != 1 {
		t.Errorf("len(Kraus) = %d, want 1", len(ch.Kraus()))
	}
}

func TestCustom_BitFlipChannel(t *testing.T) {
	// Manually construct bit-flip: sqrt(1-p)*I, sqrt(p)*X
	p := 0.1
	s0 := complex(math.Sqrt(1-p), 0)
	sp := complex(math.Sqrt(p), 0)
	ch, err := Custom("manual_bitflip", 1, [][]complex128{
		{s0, 0, 0, s0},
		{0, sp, sp, 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	checkKrausComplete(t, ch)
}

func TestCustom_TwoQubitChannel(t *testing.T) {
	// Identity channel on 2 qubits: single 4x4 identity
	kraus := make([]complex128, 16)
	for i := range 4 {
		kraus[i*4+i] = 1
	}
	ch, err := Custom("identity2q", 2, [][]complex128{kraus})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Qubits() != 2 {
		t.Errorf("Qubits = %d, want 2", ch.Qubits())
	}
}

func TestCustom_NotTracePreserving(t *testing.T) {
	// A channel where sum E_k† E_k != I
	_, err := Custom("bad", 1, [][]complex128{
		{1, 0, 0, 1}, // I
		{1, 0, 0, 1}, // I again — sum = 2*I, not I
	})
	if err == nil {
		t.Error("expected error for non-TP channel")
	}
}

func TestCustom_ScaledDown(t *testing.T) {
	// sqrt(0.5)*I, sqrt(0.5)*X — this IS trace-preserving
	s := complex(math.Sqrt(0.5), 0)
	ch, err := Custom("half_flip", 1, [][]complex128{
		{s, 0, 0, s}, // sqrt(0.5)*I
		{0, s, s, 0}, // sqrt(0.5)*X
	})
	if err != nil {
		t.Fatal(err)
	}
	checkKrausComplete(t, ch)
}

func TestCustom_WrongDimension(t *testing.T) {
	// 1-qubit channel but 3-element matrix
	_, err := Custom("bad_dim", 1, [][]complex128{
		{1, 0, 0},
	})
	if err == nil {
		t.Error("expected error for wrong dimension")
	}
}

func TestCustom_EmptyKraus(t *testing.T) {
	_, err := Custom("empty", 1, nil)
	if err == nil {
		t.Error("expected error for empty Kraus list")
	}
}

func TestCustom_ZeroQubits(t *testing.T) {
	_, err := Custom("zero", 0, [][]complex128{{1}})
	if err == nil {
		t.Error("expected error for nQubits=0")
	}
}

func TestCustom_DefensiveCopy(t *testing.T) {
	op := []complex128{1, 0, 0, 1}
	ch, err := Custom("copy_test", 1, [][]complex128{op})
	if err != nil {
		t.Fatal(err)
	}
	// Mutate original — channel should be unaffected
	op[0] = 42
	if ch.Kraus()[0][0] != 1 {
		t.Error("Custom should defensively copy Kraus operators")
	}
}

func TestMustCustom_Valid(t *testing.T) {
	ch := MustCustom("identity", 1, [][]complex128{{1, 0, 0, 1}})
	if ch.Name() != "identity" {
		t.Errorf("Name = %q, want identity", ch.Name())
	}
}

func TestMustCustom_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustCustom should panic on invalid channel")
		}
	}()
	MustCustom("bad", 1, [][]complex128{
		{1, 0, 0, 1},
		{1, 0, 0, 1},
	})
}
