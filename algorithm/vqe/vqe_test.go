package vqe_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/ansatz"
	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/algorithm/vqe"
	"github.com/splch/goqu/sim/pauli"
)

func TestVQE_H2(t *testing.T) {
	// Simplified 2-qubit H2 Hamiltonian at equilibrium bond length.
	// H = -1.0523*II + 0.3979*IZ - 0.3979*ZI - 0.0112*ZZ + 0.1809*XX
	// Exact ground state energy ≈ -1.137
	nq := 2
	terms := []pauli.PauliString{
		pauli.NewPauliString(-1.0523, nil, nq),
		pauli.NewPauliString(0.3979, map[int]pauli.Pauli{1: pauli.Z}, nq),
		pauli.NewPauliString(-0.3979, map[int]pauli.Pauli{0: pauli.Z}, nq),
		pauli.NewPauliString(-0.0112, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, nq),
		pauli.NewPauliString(0.1809, map[int]pauli.Pauli{0: pauli.X, 1: pauli.X}, nq),
	}
	h, err := pauli.NewPauliSum(terms)
	if err != nil {
		t.Fatal(err)
	}

	ans := ansatz.NewRealAmplitudes(2, 2, ansatz.Linear)
	cfg := vqe.Config{
		Hamiltonian: h,
		Ansatz:      ans,
		Optimizer:   &optim.NelderMead{},
	}

	res, err := vqe.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// The exact ground state energy is approximately -1.137.
	// We allow tolerance for the variational bound.
	if res.Energy > -1.10 {
		t.Errorf("VQE energy = %f, expected <= -1.10", res.Energy)
	}
	if len(res.History) == 0 {
		t.Error("expected non-empty history")
	}
}

func TestVQE_SingleQubit(t *testing.T) {
	// H = Z, minimum eigenvalue = -1 (|1⟩ state).
	h, err := pauli.NewPauliSum([]pauli.PauliString{
		pauli.ZOn([]int{0}, 1),
	})
	if err != nil {
		t.Fatal(err)
	}

	ans := ansatz.NewRealAmplitudes(1, 0, ansatz.Linear)
	cfg := vqe.Config{
		Hamiltonian: h,
		Ansatz:      ans,
		Optimizer:   &optim.NelderMead{},
	}

	res, err := vqe.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(res.Energy-(-1.0)) > 0.01 {
		t.Errorf("VQE energy = %f, expected ~-1.0", res.Energy)
	}
}
