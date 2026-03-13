package mitigation

import (
	"context"

	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/densitymatrix"
	"github.com/splch/goqu/sim/noise"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// Executor evaluates a circuit and returns an expectation value.
// Implementations must be safe for concurrent use.
type Executor func(ctx context.Context, circuit *ir.Circuit) (float64, error)

// StatevectorExecutor returns an Executor that computes ⟨ψ|H|ψ⟩ using
// ideal (noiseless) statevector simulation. Each call creates a fresh
// simulator for goroutine safety.
func StatevectorExecutor(hamiltonian pauli.PauliSum) Executor {
	return func(_ context.Context, circuit *ir.Circuit) (float64, error) {
		sim := statevector.New(circuit.NumQubits())
		if err := sim.Evolve(circuit); err != nil {
			return 0, err
		}
		return sim.ExpectPauliSum(hamiltonian), nil
	}
}

// DensityMatrixExecutor returns an Executor that computes Tr(ρH) using
// density matrix simulation with a noise model. Each call creates a fresh
// simulator for goroutine safety.
func DensityMatrixExecutor(hamiltonian pauli.PauliSum, nm *noise.NoiseModel) Executor {
	return func(_ context.Context, circuit *ir.Circuit) (float64, error) {
		sim := densitymatrix.New(circuit.NumQubits()).WithNoise(nm)
		if err := sim.Evolve(circuit); err != nil {
			return 0, err
		}
		return sim.ExpectPauliSum(hamiltonian), nil
	}
}
