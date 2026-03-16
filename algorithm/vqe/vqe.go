// Package vqe implements the Variational Quantum Eigensolver.
//
// VQE finds the ground state energy of a Hamiltonian by variationally
// minimizing ⟨ψ(θ)|H|ψ(θ)⟩ over parameterized circuit parameters θ.
//
// VQE exploits the variational principle: for any trial state |psi(theta)>,
// the expectation value <psi(theta)|H|psi(theta)> is an upper bound on the
// true ground-state energy E0. By optimizing the parameters theta with a
// classical optimizer (such as COBYLA or L-BFGS), VQE finds the lowest
// achievable energy within the ansatz, which approximates E0. The tightness
// of the bound depends on the expressibility of the chosen ansatz.
package vqe

import (
	"context"
	"fmt"

	"github.com/splch/goqu/algorithm/ansatz"
	"github.com/splch/goqu/algorithm/gradient"
	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/pauli"
)

// Config specifies the VQE problem and solver.
type Config struct {
	// Hamiltonian is the observable to minimize.
	Hamiltonian pauli.PauliSum
	// Ansatz is the parameterized circuit template.
	Ansatz ansatz.Ansatz
	// Optimizer is the classical optimization method.
	Optimizer optim.Optimizer
	// Gradient is the gradient function. Nil means gradient-free.
	Gradient optim.GradientFunc
	// InitialParams are the starting parameters. Nil means zeros.
	InitialParams []float64
}

// Result holds VQE output.
type Result struct {
	Energy        float64
	OptimalParams []float64
	NumIters      int
	NumEvals      int
	Converged     bool
	History       []float64
}

// Run executes VQE.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	circ, err := cfg.Ansatz.Circuit()
	if err != nil {
		return nil, fmt.Errorf("vqe: ansatz circuit: %w", err)
	}

	paramNames := ir.FreeParameters(circ)
	if len(paramNames) == 0 {
		return nil, fmt.Errorf("vqe: ansatz has no free parameters")
	}

	cost := gradient.CostFunc(circ, cfg.Hamiltonian, paramNames)

	// Track energy history.
	var history []float64
	wrappedCost := optim.ObjectiveFunc(func(x []float64) float64 {
		v := cost(x)
		history = append(history, v)
		return v
	})

	x0 := cfg.InitialParams
	if x0 == nil {
		x0 = make([]float64, len(paramNames))
	}

	res, err := cfg.Optimizer.Minimize(ctx, wrappedCost, x0, cfg.Gradient, nil)
	if err != nil {
		return nil, fmt.Errorf("vqe: optimization: %w", err)
	}

	return &Result{
		Energy:        res.Fun,
		OptimalParams: res.X,
		NumIters:      res.Iterations,
		NumEvals:      res.FuncEvals,
		Converged:     res.Converged,
		History:       history,
	}, nil
}
