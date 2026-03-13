// Package optim provides classical optimization algorithms.
//
// The [Optimizer] interface accepts a generic [ObjectiveFunc] and optional
// [GradientFunc], making the package usable for any minimization problem.
// Gradient-free methods (Nelder-Mead, SPSA) ignore the gradient argument;
// gradient-based methods (Adam, L-BFGS) require it.
package optim

import "context"

// ObjectiveFunc evaluates cost at a parameter point. Must be goroutine-safe.
type ObjectiveFunc func(x []float64) float64

// GradientFunc computes the gradient at a parameter point.
type GradientFunc func(x []float64) []float64

// Callback is called after each iteration. Return non-nil error to stop.
type Callback func(iter int, x []float64, fun float64) error

// Options controls optimization behavior. A nil *Options uses defaults.
type Options struct {
	MaxIter  int      // Default: 1000
	FunTol   float64  // Default: 1e-8
	XTol     float64  // Default: 1e-8
	GradTol  float64  // Default: 0 (disabled)
	Callback Callback // Optional per-iteration callback
}

func (o *Options) maxIter() int {
	if o == nil || o.MaxIter <= 0 {
		return 1000
	}
	return o.MaxIter
}

func (o *Options) funTol() float64 {
	if o == nil || o.FunTol <= 0 {
		return 1e-8
	}
	return o.FunTol
}

func (o *Options) xTol() float64 {
	if o == nil || o.XTol <= 0 {
		return 1e-8
	}
	return o.XTol
}

func (o *Options) gradTol() float64 {
	if o == nil {
		return 0
	}
	return o.GradTol
}

func (o *Options) callback() Callback {
	if o == nil {
		return nil
	}
	return o.Callback
}

// Result holds the outcome of an optimization run.
type Result struct {
	X          []float64 // Optimal parameters
	Fun        float64   // Objective value at X
	Iterations int       // Number of iterations performed
	FuncEvals  int       // Number of objective evaluations
	GradEvals  int       // Number of gradient evaluations
	Converged  bool      // Whether the optimizer converged
	Message    string    // Human-readable status
}

// Optimizer is a classical optimization algorithm.
type Optimizer interface {
	// Minimize finds the minimum of f starting from x0.
	// grad may be nil for gradient-free methods.
	// opts may be nil for defaults.
	Minimize(ctx context.Context, f ObjectiveFunc, x0 []float64,
		grad GradientFunc, opts *Options) (Result, error)

	// Name returns the optimizer name.
	Name() string
}
