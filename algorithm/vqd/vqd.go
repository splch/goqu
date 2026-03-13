// Package vqd implements the Variational Quantum Deflation algorithm.
//
// VQD finds multiple eigenvalues of a Hamiltonian by sequentially
// optimizing excited states. For each state k, it minimizes
// E(theta) + beta * sum_{j<k} |<psi_j|psi_k>|^2, where the overlap
// penalty deflates previously found eigenstates.
package vqd

import (
	"context"
	"fmt"
	"math"
	"math/cmplx"

	"github.com/splch/goqu/algorithm/ansatz"
	"github.com/splch/goqu/algorithm/gradient"
	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// Config specifies the VQD problem and solver.
type Config struct {
	// Hamiltonian is the observable whose eigenvalues are sought.
	Hamiltonian pauli.PauliSum
	// Ansatz is the parameterized circuit template.
	Ansatz ansatz.Ansatz
	// Optimizer is the classical optimization method.
	Optimizer optim.Optimizer
	// Gradient is the gradient function. Nil means gradient-free.
	Gradient optim.GradientFunc
	// InitialParams are the starting parameters. Nil means zeros.
	InitialParams []float64
	// NumStates is the number of eigenstates to find. Default: 2.
	NumStates int
	// BetaPenalty is the overlap penalty weight. Zero means auto (2*|E_0|).
	BetaPenalty float64
}

// Result holds VQD output.
type Result struct {
	// Energies are the eigenvalues found, one per state.
	Energies []float64
	// OptimalParams are the optimal parameters for each state.
	OptimalParams [][]float64
	// NumIters is the number of optimizer iterations for each state.
	NumIters []int
	// Converged indicates whether the optimizer converged for each state.
	Converged []bool
}

// Run executes the VQD algorithm.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	circ, err := cfg.Ansatz.Circuit()
	if err != nil {
		return nil, fmt.Errorf("vqd: ansatz circuit: %w", err)
	}

	paramNames := ir.FreeParameters(circ)
	if len(paramNames) == 0 {
		return nil, fmt.Errorf("vqd: ansatz has no free parameters")
	}

	numStates := cfg.NumStates
	if numStates <= 0 {
		numStates = 2
	}

	beta := cfg.BetaPenalty

	result := &Result{
		Energies:      make([]float64, numStates),
		OptimalParams: make([][]float64, numStates),
		NumIters:      make([]int, numStates),
		Converged:     make([]bool, numStates),
	}

	// Store the statevectors for previously converged states.
	prevStates := make([][]complex128, 0, numStates)

	energyCost := gradient.CostFunc(circ, cfg.Hamiltonian, paramNames)

	for k := range numStates {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var costFunc optim.ObjectiveFunc
		if k == 0 {
			// Ground state: plain energy minimization.
			costFunc = energyCost
		} else {
			// Excited state k: energy + beta * sum of overlaps with previous states.
			currentBeta := beta
			previousSVs := make([][]complex128, len(prevStates))
			copy(previousSVs, prevStates)

			costFunc = optim.ObjectiveFunc(func(x []float64) float64 {
				energy := energyCost(x)

				// Compute statevector for current parameters.
				sv := evalStateVector(circ, paramNames, x)

				// Sum overlaps with all previously found states.
				overlapSum := 0.0
				for _, prevSV := range previousSVs {
					overlapSum += overlap(prevSV, sv)
				}

				return energy + currentBeta*overlapSum
			})
		}

		x0 := cfg.InitialParams
		if x0 == nil {
			x0 = make([]float64, len(paramNames))
		} else {
			// Copy to avoid mutating the caller's slice across iterations.
			tmp := make([]float64, len(x0))
			copy(tmp, x0)
			x0 = tmp
		}

		res, err := cfg.Optimizer.Minimize(ctx, costFunc, x0, cfg.Gradient, nil)
		if err != nil {
			return nil, fmt.Errorf("vqd: optimization for state %d: %w", k, err)
		}

		result.Energies[k] = energyCost(res.X)
		result.OptimalParams[k] = res.X
		result.NumIters[k] = res.Iterations
		result.Converged[k] = res.Converged

		// Store the converged statevector for overlap penalties.
		prevStates = append(prevStates, evalStateVector(circ, paramNames, res.X))

		// Auto-set beta after ground state if not provided.
		if k == 0 && beta == 0 {
			beta = 2 * math.Abs(result.Energies[0])
			if beta < 1.0 {
				beta = 1.0
			}
		}
	}

	return result, nil
}

// evalStateVector binds parameters to the circuit and returns the resulting statevector.
func evalStateVector(circ *ir.Circuit, paramNames []string, x []float64) []complex128 {
	bindings := make(map[string]float64, len(paramNames))
	for i, name := range paramNames {
		bindings[name] = x[i]
	}
	bound, err := ir.Bind(circ, bindings)
	if err != nil {
		panic("vqd: bind failed: " + err.Error())
	}
	sim := statevector.New(bound.NumQubits())
	if err := sim.Evolve(bound); err != nil {
		panic("vqd: evolve failed: " + err.Error())
	}
	return sim.StateVector()
}

// overlap computes |<a|b>|^2, the squared modulus of the inner product.
func overlap(a, b []complex128) float64 {
	var dot complex128
	for i := range a {
		dot += cmplx.Conj(a[i]) * b[i]
	}
	return real(dot)*real(dot) + imag(dot)*imag(dot)
}
