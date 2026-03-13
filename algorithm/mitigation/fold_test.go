package mitigation_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/mitigation"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

func TestFoldCircuit_UnitaryGateCount(t *testing.T) {
	// 3-gate circuit folded at scale 3: should have 3 + 2*(3-1)/2 * 3... wait
	// C → C C† C means original + inverse + original = 3*original gates
	// Scale factor s: gate count = s * original
	circ, err := builder.New("test", 2).
		H(0).
		CNOT(0, 1).
		H(1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	origCount := circ.Stats().GateCount

	tests := []struct {
		scale    int
		wantMult int
	}{
		{1, 1},
		{3, 3},
		{5, 5},
	}

	for _, tt := range tests {
		folded, err := mitigation.FoldCircuit(circ, tt.scale, mitigation.UnitaryFolding)
		if err != nil {
			t.Fatalf("scale %d: %v", tt.scale, err)
		}
		got := folded.Stats().GateCount
		want := origCount * tt.wantMult
		if got != want {
			t.Errorf("scale %d: gate count = %d, want %d", tt.scale, got, want)
		}
	}
}

func TestFoldCircuit_LocalFoldingGateCount(t *testing.T) {
	circ, err := builder.New("test", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	origCount := circ.Stats().GateCount

	tests := []struct {
		scale    int
		wantMult int
	}{
		{1, 1},
		{3, 3},
		{5, 5},
	}

	for _, tt := range tests {
		folded, err := mitigation.FoldCircuit(circ, tt.scale, mitigation.LocalFolding)
		if err != nil {
			t.Fatalf("scale %d: %v", tt.scale, err)
		}
		got := folded.Stats().GateCount
		want := origCount * tt.wantMult
		if got != want {
			t.Errorf("scale %d: gate count = %d, want %d", tt.scale, got, want)
		}
	}
}

func TestFoldCircuit_PreservesUnitary(t *testing.T) {
	// Folding should not change the logical unitary.
	// Under ideal simulation, folded circuits should give the same expectation value.
	circ, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	hamiltonian, err := pauli.NewPauliSum([]pauli.PauliString{
		pauli.ZOn([]int{0}, 2),
	})
	if err != nil {
		t.Fatal(err)
	}

	exec := mitigation.StatevectorExecutor(hamiltonian)
	ctx := context.Background()

	idealVal, err := exec(ctx, circ)
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []mitigation.ScaleMethod{mitigation.UnitaryFolding, mitigation.LocalFolding} {
		for _, scale := range []int{3, 5} {
			folded, err := mitigation.FoldCircuit(circ, scale, method)
			if err != nil {
				t.Fatalf("method %d, scale %d: %v", method, scale, err)
			}

			sim := statevector.New(2)
			if err := sim.Evolve(folded); err != nil {
				t.Fatalf("method %d, scale %d: evolve: %v", method, scale, err)
			}
			got := sim.ExpectPauliSum(hamiltonian)

			if math.Abs(got-idealVal) > 1e-10 {
				t.Errorf("method %d, scale %d: expectation = %f, want %f", method, scale, got, idealVal)
			}
		}
	}
}

func TestFoldCircuit_InvalidScaleFactor(t *testing.T) {
	circ, err := builder.New("test", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	tests := []int{0, -1, 2, 4}
	for _, sf := range tests {
		_, err := mitigation.FoldCircuit(circ, sf, mitigation.UnitaryFolding)
		if err == nil {
			t.Errorf("scale %d: expected error", sf)
		}
	}
}
