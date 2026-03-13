package ansatz_test

import (
	"testing"

	"github.com/splch/goqu/algorithm/ansatz"
	"github.com/splch/goqu/circuit/ir"
)

func TestRealAmplitudes(t *testing.T) {
	tests := []struct {
		name      string
		nQubits   int
		reps      int
		ent       ansatz.Entanglement
		wantParam int
	}{
		{"2q-1rep-linear", 2, 1, ansatz.Linear, 4},
		{"3q-2rep-full", 3, 2, ansatz.Full, 9},
		{"2q-1rep-circular", 2, 1, ansatz.Circular, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ra := ansatz.NewRealAmplitudes(tt.nQubits, tt.reps, tt.ent)
			if ra.NumParams() != tt.wantParam {
				t.Errorf("NumParams() = %d, want %d", ra.NumParams(), tt.wantParam)
			}
			circ, err := ra.Circuit()
			if err != nil {
				t.Fatal(err)
			}
			if circ.NumQubits() != tt.nQubits {
				t.Errorf("NumQubits() = %d, want %d", circ.NumQubits(), tt.nQubits)
			}
			params := ir.FreeParameters(circ)
			if len(params) != tt.wantParam {
				t.Errorf("FreeParameters() = %d, want %d", len(params), tt.wantParam)
			}
		})
	}
}

func TestEfficientSU2(t *testing.T) {
	tests := []struct {
		name      string
		nQubits   int
		reps      int
		ent       ansatz.Entanglement
		wantParam int
	}{
		{"2q-1rep-linear", 2, 1, ansatz.Linear, 8},
		{"3q-2rep-full", 3, 2, ansatz.Full, 18},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := ansatz.NewEfficientSU2(tt.nQubits, tt.reps, tt.ent)
			if es.NumParams() != tt.wantParam {
				t.Errorf("NumParams() = %d, want %d", es.NumParams(), tt.wantParam)
			}
			circ, err := es.Circuit()
			if err != nil {
				t.Fatal(err)
			}
			if circ.NumQubits() != tt.nQubits {
				t.Errorf("NumQubits() = %d, want %d", circ.NumQubits(), tt.nQubits)
			}
			params := ir.FreeParameters(circ)
			if len(params) != tt.wantParam {
				t.Errorf("FreeParameters() = %d, want %d", len(params), tt.wantParam)
			}
		})
	}
}

func TestAnsatzInterface(t *testing.T) {
	// Verify both types satisfy the Ansatz interface.
	var _ ansatz.Ansatz = ansatz.NewRealAmplitudes(2, 1, ansatz.Linear)
	var _ ansatz.Ansatz = ansatz.NewEfficientSU2(2, 1, ansatz.Linear)
}
