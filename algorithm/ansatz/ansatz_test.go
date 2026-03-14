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

func TestBasicEntanglerLayers(t *testing.T) {
	tests := []struct {
		name      string
		nQubits   int
		layers    int
		wantParam int
	}{
		{"1q-1layer", 1, 1, 1},
		{"2q-1layer", 2, 1, 2},
		{"3q-2layer", 3, 2, 6},
		{"4q-3layer", 4, 3, 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be := ansatz.NewBasicEntanglerLayers(tt.nQubits, tt.layers)
			if be.NumParams() != tt.wantParam {
				t.Errorf("NumParams() = %d, want %d", be.NumParams(), tt.wantParam)
			}
			circ, err := be.Circuit()
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

func TestStronglyEntanglingLayers(t *testing.T) {
	tests := []struct {
		name      string
		nQubits   int
		layers    int
		wantParam int
	}{
		{"1q-1layer", 1, 1, 3},
		{"2q-1layer", 2, 1, 6},
		{"2q-2layer", 2, 2, 12},
		{"3q-2layer", 3, 2, 18},
		{"4q-3layer", 4, 3, 36},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			se := ansatz.NewStronglyEntanglingLayers(tt.nQubits, tt.layers)
			if se.NumParams() != tt.wantParam {
				t.Errorf("NumParams() = %d, want %d", se.NumParams(), tt.wantParam)
			}
			circ, err := se.Circuit()
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

func TestUCCSD(t *testing.T) {
	tests := []struct {
		name       string
		nQubits    int
		nElectrons int
		wantParam  int
	}{
		// 2 occ * 2 virt = 4 singles, C(2,2)*C(2,2) = 1 double = 5
		{"4q-2e", 4, 2, 5},
		// 2 occ * 4 virt = 8 singles, C(2,2)*C(4,2) = 1*6 = 6 doubles = 14
		{"6q-2e", 6, 2, 14},
		// 1 occ * 1 virt = 1 single, C(1,2)*C(1,2) = 0 doubles = 1
		{"2q-1e", 2, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := ansatz.NewUCCSD(tt.nQubits, tt.nElectrons)
			if u.NumParams() != tt.wantParam {
				t.Errorf("NumParams() = %d, want %d", u.NumParams(), tt.wantParam)
			}
			circ, err := u.Circuit()
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
	// Verify all types satisfy the Ansatz interface.
	var _ ansatz.Ansatz = ansatz.NewRealAmplitudes(2, 1, ansatz.Linear)
	var _ ansatz.Ansatz = ansatz.NewEfficientSU2(2, 1, ansatz.Linear)
	var _ ansatz.Ansatz = ansatz.NewBasicEntanglerLayers(2, 1)
	var _ ansatz.Ansatz = ansatz.NewStronglyEntanglingLayers(2, 1)
	var _ ansatz.Ansatz = ansatz.NewUCCSD(4, 2)
}
