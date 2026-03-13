package vqd_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/ansatz"
	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/algorithm/vqd"
	"github.com/splch/goqu/sim/pauli"
)

func TestVQD_SingleQubitZ(t *testing.T) {
	// H = Z has eigenvalues -1 (|1>) and +1 (|0>).
	h, err := pauli.NewPauliSum([]pauli.PauliString{
		pauli.ZOn([]int{0}, 1),
	})
	if err != nil {
		t.Fatal(err)
	}

	ans := ansatz.NewRealAmplitudes(1, 0, ansatz.Linear)
	cfg := vqd.Config{
		Hamiltonian: h,
		Ansatz:      ans,
		Optimizer:   &optim.NelderMead{InitialStep: 1.0},
		NumStates:   2,
		BetaPenalty: 5.0,
	}

	res, err := vqd.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Energies) != 2 {
		t.Fatalf("expected 2 energies, got %d", len(res.Energies))
	}

	const tol = 0.15
	if math.Abs(res.Energies[0]-(-1.0)) > tol {
		t.Errorf("ground state energy = %f, expected ~-1.0", res.Energies[0])
	}
	if math.Abs(res.Energies[1]-1.0) > tol {
		t.Errorf("excited state energy = %f, expected ~+1.0", res.Energies[1])
	}
}

func TestVQD_H2(t *testing.T) {
	// Simplified 2-qubit H2 Hamiltonian at equilibrium bond length.
	// H = -1.0523*II + 0.3979*IZ - 0.3979*ZI - 0.0112*ZZ + 0.1809*XX
	// Exact ground state energy ~ -1.137
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
	cfg := vqd.Config{
		Hamiltonian: h,
		Ansatz:      ans,
		Optimizer:   &optim.NelderMead{},
		NumStates:   2,
	}

	res, err := vqd.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Energies) != 2 {
		t.Fatalf("expected 2 energies, got %d", len(res.Energies))
	}

	// Ground state should be close to the exact value of -1.137.
	if res.Energies[0] > -1.05 {
		t.Errorf("ground state energy = %f, expected <= -1.05", res.Energies[0])
	}

	// Excited state must be higher than ground state.
	if res.Energies[1] <= res.Energies[0] {
		t.Errorf("excited state energy %f should be > ground state energy %f",
			res.Energies[1], res.Energies[0])
	}
}
