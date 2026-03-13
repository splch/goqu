package optim

import (
	"context"
	"fmt"
	"math"
)

// LBFGS implements the Limited-memory BFGS quasi-Newton method (gradient-based).
type LBFGS struct {
	// Memory is the number of correction pairs stored. Default: 10.
	Memory int
}

func (l *LBFGS) Name() string { return "L-BFGS" }

func (l *LBFGS) memory() int {
	if l.Memory > 0 {
		return l.Memory
	}
	return 10
}

// lbfgsPair stores a correction pair for the L-BFGS two-loop recursion.
type lbfgsPair struct {
	s []float64 // x_{k+1} - x_k
	y []float64 // g_{k+1} - g_k
	r float64   // 1 / (y^T s)
}

func (l *LBFGS) Minimize(ctx context.Context, f ObjectiveFunc, x0 []float64,
	grad GradientFunc, opts *Options) (Result, error) {

	n := len(x0)
	if n == 0 {
		return Result{}, fmt.Errorf("lbfgs: x0 must be non-empty")
	}
	if grad == nil {
		return Result{}, fmt.Errorf("lbfgs: gradient function required")
	}

	maxIter := opts.maxIter()
	gradTol := opts.gradTol()
	funTol := opts.funTol()
	cb := opts.callback()
	mem := l.memory()

	x := copyVec(x0)
	fx := f(x)
	g := grad(x)
	fEvals := 1
	gEvals := 1

	history := make([]lbfgsPair, 0, mem)

	var converged bool
	var iter int

	for iter = 0; iter < maxIter; iter++ {
		if err := ctx.Err(); err != nil {
			return Result{X: copyVec(x), Fun: fx, Iterations: iter,
				FuncEvals: fEvals, GradEvals: gEvals, Message: "cancelled"}, err
		}

		gNorm := vecNorm(g)
		if gradTol > 0 && gNorm < gradTol {
			converged = true
			break
		}

		// Compute search direction via two-loop recursion.
		d := lbfgsTwoLoop(g, history)

		// Line search (backtracking with Armijo condition).
		alpha := 1.0
		dg := dot(d, g)
		if dg >= 0 {
			// Not a descent direction; reset to steepest descent.
			for j := range n {
				d[j] = -g[j]
			}
			dg = dot(d, g)
			history = history[:0]
		}

		prevFx := fx
		const c1 = 1e-4
		const shrink = 0.5
		xNew := make([]float64, n)
		for range 30 {
			for j := range n {
				xNew[j] = x[j] + alpha*d[j]
			}
			fNew := f(xNew)
			fEvals++
			if fNew <= fx+c1*alpha*dg {
				fx = fNew
				break
			}
			alpha *= shrink
		}

		// Update history.
		s := make([]float64, n)
		gNew := grad(xNew)
		gEvals++
		y := make([]float64, n)
		for j := range n {
			s[j] = xNew[j] - x[j]
			y[j] = gNew[j] - g[j]
		}

		ys := dot(y, s)
		if ys > 1e-10 {
			if len(history) >= mem {
				history = history[1:]
			}
			history = append(history, lbfgsPair{s: s, y: y, r: 1.0 / ys})
		}

		copy(x, xNew)
		copy(g, gNew)

		if math.Abs(fx-prevFx) < funTol {
			converged = true
			iter++
			break
		}

		if cb != nil {
			if err := cb(iter, x, fx); err != nil {
				return Result{X: copyVec(x), Fun: fx, Iterations: iter + 1,
					FuncEvals: fEvals, GradEvals: gEvals, Message: "stopped by callback"}, nil
			}
		}
	}

	msg := "maximum iterations reached"
	if converged {
		msg = "converged"
	}
	return Result{
		X:          copyVec(x),
		Fun:        fx,
		Iterations: iter,
		FuncEvals:  fEvals,
		GradEvals:  gEvals,
		Converged:  converged,
		Message:    msg,
	}, nil
}

// lbfgsTwoLoop computes the search direction d = -H*g using the L-BFGS two-loop recursion.
func lbfgsTwoLoop(g []float64, history []lbfgsPair) []float64 {
	n := len(g)
	q := copyVec(g)

	m := len(history)
	alphas := make([]float64, m)

	// First loop (reverse).
	for i := m - 1; i >= 0; i-- {
		alphas[i] = history[i].r * dot(history[i].s, q)
		for j := range n {
			q[j] -= alphas[i] * history[i].y[j]
		}
	}

	// Initial Hessian approximation: H0 = gamma * I.
	d := make([]float64, n)
	if m > 0 {
		last := history[m-1]
		gamma := dot(last.s, last.y) / dot(last.y, last.y)
		for j := range n {
			d[j] = gamma * q[j]
		}
	} else {
		copy(d, q)
	}

	// Second loop (forward).
	for i := range m {
		beta := history[i].r * dot(history[i].y, d)
		for j := range n {
			d[j] += (alphas[i] - beta) * history[i].s[j]
		}
	}

	// Negate for descent direction.
	for j := range n {
		d[j] = -d[j]
	}
	return d
}

func dot(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}
