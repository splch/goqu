// Package gradient provides quantum gradient computation methods.
//
// [CostFunc] creates an [optim.ObjectiveFunc] from a parameterized circuit
// and Hamiltonian using statevector simulation.
//
// [ParameterShift] and [FiniteDifference] compute gradients of an
// objective function, suitable for use as [optim.GradientFunc].
package gradient

import (
	"math"

	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// CostFunc creates an ObjectiveFunc that evaluates the expectation value
// ⟨ψ(θ)|H|ψ(θ)⟩ using statevector simulation. Each call binds the
// parameter vector to paramNames and runs a fresh simulation.
//
// The returned function is goroutine-safe because each call creates
// its own simulator instance.
func CostFunc(circuit *ir.Circuit, hamiltonian pauli.PauliSum,
	paramNames []string) optim.ObjectiveFunc {

	return func(x []float64) float64 {
		bindings := make(map[string]float64, len(paramNames))
		for i, name := range paramNames {
			bindings[name] = x[i]
		}
		bound, err := ir.Bind(circuit, bindings)
		if err != nil {
			panic("gradient.CostFunc: bind failed: " + err.Error())
		}
		sim := statevector.New(bound.NumQubits())
		if err := sim.Evolve(bound); err != nil {
			panic("gradient.CostFunc: evolve failed: " + err.Error())
		}
		return sim.ExpectPauliSum(hamiltonian)
	}
}

// ParameterShift returns exact gradients via the parameter-shift rule.
// For each parameter, it evaluates f(x + π/2*e_i) and f(x - π/2*e_i),
// computing the gradient as [f(x+s) - f(x-s)] / 2.
// This requires 2*N function evaluations per call (N = number of parameters).
func ParameterShift(f optim.ObjectiveFunc) optim.GradientFunc {
	return func(x []float64) []float64 {
		n := len(x)
		grad := make([]float64, n)
		xp := make([]float64, n)
		xm := make([]float64, n)

		for i := range n {
			copy(xp, x)
			copy(xm, x)
			shift := 0.5 * math.Pi
			xp[i] += shift
			xm[i] -= shift
			grad[i] = (f(xp) - f(xm)) / 2
		}
		return grad
	}
}

// FiniteDifference returns approximate gradients via central differences
// with step size h.
func FiniteDifference(f optim.ObjectiveFunc, h float64) optim.GradientFunc {
	return func(x []float64) []float64 {
		n := len(x)
		grad := make([]float64, n)
		xp := make([]float64, n)
		xm := make([]float64, n)

		for i := range n {
			copy(xp, x)
			copy(xm, x)
			xp[i] += h
			xm[i] -= h
			grad[i] = (f(xp) - f(xm)) / (2 * h)
		}
		return grad
	}
}
