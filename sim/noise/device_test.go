package noise

import (
	"testing"

	"github.com/splch/qgo/transpile/target"
)

func TestFromTargetSimple(t *testing.T) {
	tgt := target.Target{
		Name:       "test",
		NumQubits:  5,
		BasisGates: []string{"CX", "RZ", "SX", "X"},
		GateFidelities: map[string]float64{
			"CX": 0.99,
			"SX": 0.999,
			"X":  0.9999,
			"RZ": 1.0,
		},
	}

	m := FromTargetSimple(tgt)

	// CX has fidelity 0.99 → should produce a 2-qubit channel
	ch := m.Lookup("CX", []int{0, 1})
	if ch == nil {
		t.Fatal("expected noise channel for CX")
	}
	if ch.Qubits() != 2 {
		t.Errorf("CX channel: expected 2 qubits, got %d", ch.Qubits())
	}

	// SX has fidelity 0.999 → should produce a 1-qubit channel
	ch = m.Lookup("SX", []int{0})
	if ch == nil {
		t.Fatal("expected noise channel for SX")
	}
	if ch.Qubits() != 1 {
		t.Errorf("SX channel: expected 1 qubit, got %d", ch.Qubits())
	}

	// X has fidelity 0.9999 → should produce a 1-qubit channel
	ch = m.Lookup("X", []int{0})
	if ch == nil {
		t.Fatal("expected noise channel for X")
	}
	if ch.Qubits() != 1 {
		t.Errorf("X channel: expected 1 qubit, got %d", ch.Qubits())
	}

	// RZ has fidelity 1.0 → no channel
	ch = m.Lookup("RZ", []int{0})
	if ch != nil {
		t.Error("expected nil channel for RZ (perfect fidelity)")
	}
}

func TestFromTarget_PerfectFidelity(t *testing.T) {
	tgt := target.Target{
		Name:       "perfect",
		NumQubits:  3,
		BasisGates: []string{"CX", "H", "T"},
		GateFidelities: map[string]float64{
			"CX": 1.0,
			"H":  1.0,
			"T":  1.0,
		},
	}

	m := FromTarget(tgt, nil)

	// All gates have perfect fidelity → no noise channels
	if ch := m.Lookup("CX", []int{0, 1}); ch != nil {
		t.Error("expected nil channel for perfect CX")
	}
	if ch := m.Lookup("H", []int{0}); ch != nil {
		t.Error("expected nil channel for perfect H")
	}
	if ch := m.Lookup("T", []int{0}); ch != nil {
		t.Error("expected nil channel for perfect T")
	}
}

func TestFromTarget_WithCalibration(t *testing.T) {
	tgt := target.Target{
		Name:       "cal-test",
		NumQubits:  3,
		BasisGates: []string{"CX", "SX", "RZ"},
		GateFidelities: map[string]float64{
			"CX": 0.98,
			"SX": 0.999,
			"RZ": 1.0,
		},
	}

	cal := &CalibrationData{
		GateTimes: map[string]float64{
			"CX": 300,
			"SX": 50,
		},
		T1: map[int]float64{
			0: 100000,
			1: 80000,
		},
		T2: map[int]float64{
			0: 60000,
			1: 50000,
		},
		ReadoutErrors: map[int][2]float64{
			0: {0.02, 0.03},
			1: {0.01, 0.04},
		},
	}

	m := FromTarget(tgt, cal)

	// Gate-level depolarizing channels should exist
	if ch := m.Lookup("CX", []int{2, 3}); ch == nil {
		t.Error("expected gate-level CX channel")
	} else if ch.Qubits() != 2 {
		t.Errorf("CX channel: expected 2 qubits, got %d", ch.Qubits())
	}

	if ch := m.Lookup("SX", []int{2}); ch == nil {
		t.Error("expected gate-level SX channel")
	} else if ch.Qubits() != 1 {
		t.Errorf("SX channel: expected 1 qubit, got %d", ch.Qubits())
	}

	// Qubit-specific thermal relaxation should exist for CX on qubit 0 and qubit 1
	ch := m.Lookup("CX", []int{0})
	if ch == nil {
		t.Error("expected qubit-specific CX channel for qubit 0")
	} else {
		checkKrausComplete(t, ch)
	}

	ch = m.Lookup("SX", []int{1})
	if ch == nil {
		t.Error("expected qubit-specific SX channel for qubit 1")
	} else {
		checkKrausComplete(t, ch)
	}

	// Readout errors
	re0 := m.ReadoutFor(0)
	if re0 == nil {
		t.Fatal("expected readout error for qubit 0")
	}
	if re0.P01 != 0.02 || re0.P10 != 0.03 {
		t.Errorf("qubit 0 readout: got P01=%f, P10=%f, want 0.02, 0.03", re0.P01, re0.P10)
	}

	re1 := m.ReadoutFor(1)
	if re1 == nil {
		t.Fatal("expected readout error for qubit 1")
	}
	if re1.P01 != 0.01 || re1.P10 != 0.04 {
		t.Errorf("qubit 1 readout: got P01=%f, P10=%f, want 0.01, 0.04", re1.P01, re1.P10)
	}

	// No readout error for qubit 2
	if re := m.ReadoutFor(2); re != nil {
		t.Error("expected nil readout error for qubit 2")
	}
}

func TestFromTarget_2QGates(t *testing.T) {
	tgt := target.Target{
		Name:       "2q-test",
		NumQubits:  4,
		BasisGates: []string{"CX", "CZ", "SWAP", "MS", "ECR"},
		GateFidelities: map[string]float64{
			"CX":   0.99,
			"CZ":   0.995,
			"SWAP": 0.98,
			"MS":   0.99,
			"ECR":  0.985,
		},
	}

	m := FromTargetSimple(tgt)

	for _, gateName := range []string{"CX", "CZ", "SWAP", "MS", "ECR"} {
		ch := m.Lookup(gateName, []int{0, 1})
		if ch == nil {
			t.Errorf("expected noise channel for %s", gateName)
			continue
		}
		if ch.Qubits() != 2 {
			t.Errorf("%s channel: expected 2 qubits, got %d", gateName, ch.Qubits())
		}
		checkKrausComplete(t, ch)
	}
}

func TestFromTargetSimple_NoFidelities(t *testing.T) {
	tgt := target.Target{
		Name:       "no-fidelity",
		NumQubits:  5,
		BasisGates: []string{"CX", "RZ", "SX", "X"},
		// GateFidelities is nil
	}

	m := FromTargetSimple(tgt)

	// No fidelity data → no noise channels
	if ch := m.Lookup("CX", []int{0, 1}); ch != nil {
		t.Error("expected nil for CX with no fidelities")
	}
	if ch := m.Lookup("SX", []int{0}); ch != nil {
		t.Error("expected nil for SX with no fidelities")
	}
}

func TestIsGate2Q(t *testing.T) {
	twoQ := []string{"CX", "CZ", "CNOT", "SWAP", "CY", "CP", "CRZ", "CRX", "CRY",
		"RXX", "RYY", "RZZ", "MS", "ECR"}
	for _, name := range twoQ {
		if !isGate2Q(name) {
			t.Errorf("expected isGate2Q(%q) = true", name)
		}
	}

	oneQ := []string{"H", "X", "Y", "Z", "S", "T", "RX", "RY", "RZ", "SX", "ID", "I", "U3"}
	for _, name := range oneQ {
		if isGate2Q(name) {
			t.Errorf("expected isGate2Q(%q) = false", name)
		}
	}
}
