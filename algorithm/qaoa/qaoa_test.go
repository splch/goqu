package qaoa_test

import (
	"context"
	"testing"

	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/algorithm/qaoa"
)

func TestMaxCutHamiltonian(t *testing.T) {
	// Triangle graph: 3 edges, 3 qubits.
	edges := [][2]int{{0, 1}, {1, 2}, {0, 2}}
	h, err := qaoa.MaxCutHamiltonian(edges, 3)
	if err != nil {
		t.Fatal(err)
	}
	if h.NumQubits() != 3 {
		t.Errorf("NumQubits() = %d, want 3", h.NumQubits())
	}
	// 3 edges * 2 terms each = 6 terms.
	if len(h.Terms()) != 6 {
		t.Errorf("len(Terms()) = %d, want 6", len(h.Terms()))
	}
}

func TestQAOA_TriangleMaxCut(t *testing.T) {
	// Triangle graph: optimal cut = 2 (any 2-partition cuts 2 edges).
	edges := [][2]int{{0, 1}, {1, 2}, {0, 2}}
	h, err := qaoa.MaxCutHamiltonian(edges, 3)
	if err != nil {
		t.Fatal(err)
	}

	cfg := qaoa.Config{
		CostHamiltonian: h,
		Layers:          2,
		Optimizer:       &optim.NelderMead{},
		Shots:           2048,
	}

	res, err := qaoa.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// MaxCut Hamiltonian is negated, so optimal value should be <= -1.5.
	// For a triangle, max cut = 2, so min cost = -2.
	if res.OptimalValue > -1.0 {
		t.Errorf("QAOA cost = %f, expected <= -1.0", res.OptimalValue)
	}

	// Best bitstring should cut at least 1 edge.
	if res.BestCost > -1.0 {
		t.Errorf("best bitstring cost = %f, expected <= -1.0", res.BestCost)
	}
}

func TestMaxCutHamiltonian_Errors(t *testing.T) {
	_, err := qaoa.MaxCutHamiltonian(nil, 3)
	if err == nil {
		t.Error("expected error for empty edges")
	}
	_, err = qaoa.MaxCutHamiltonian([][2]int{{0, 1}}, 1)
	if err == nil {
		t.Error("expected error for numQubits < 2")
	}
}
