package optim

import (
	"context"
	"fmt"
	"math"
	"sort"
)

// NelderMead implements the Nelder-Mead simplex algorithm (gradient-free).
type NelderMead struct {
	// InitialStep controls the initial simplex size. Default: 0.05.
	InitialStep float64
}

func (nm *NelderMead) Name() string { return "Nelder-Mead" }

func (nm *NelderMead) initialStep() float64 {
	if nm.InitialStep > 0 {
		return nm.InitialStep
	}
	return 0.05
}

func (nm *NelderMead) Minimize(ctx context.Context, f ObjectiveFunc, x0 []float64,
	_ GradientFunc, opts *Options) (Result, error) {

	n := len(x0)
	if n == 0 {
		return Result{}, fmt.Errorf("nelder-mead: x0 must be non-empty")
	}

	maxIter := opts.maxIter()
	funTol := opts.funTol()
	xTol := opts.xTol()
	cb := opts.callback()

	const (
		alpha = 1.0 // reflection
		gamma = 2.0 // expansion
		rho   = 0.5 // contraction
		sigma = 0.5 // shrink
	)

	// Initialize simplex: n+1 vertices.
	type vertex struct {
		x []float64
		f float64
	}
	simplex := make([]vertex, n+1)
	evals := 0

	simplex[0] = vertex{x: copyVec(x0), f: f(x0)}
	evals++
	step := nm.initialStep()
	for i := range n {
		xi := copyVec(x0)
		if xi[i] == 0 {
			xi[i] = step
		} else {
			xi[i] += step * xi[i]
		}
		simplex[i+1] = vertex{x: xi, f: f(xi)}
		evals++
	}

	sortSimplex := func() {
		sort.Slice(simplex, func(i, j int) bool {
			return simplex[i].f < simplex[j].f
		})
	}

	centroid := make([]float64, n)

	computeCentroid := func() {
		for j := range n {
			centroid[j] = 0
		}
		for i := range n { // exclude worst
			for j := range n {
				centroid[j] += simplex[i].x[j]
			}
		}
		inv := 1.0 / float64(n)
		for j := range n {
			centroid[j] *= inv
		}
	}

	reflectPoint := func(factor float64) []float64 {
		r := make([]float64, n)
		for j := range n {
			r[j] = centroid[j] + factor*(centroid[j]-simplex[n].x[j])
		}
		return r
	}

	sortSimplex()

	var converged bool
	var iter int

	for iter = 0; iter < maxIter; iter++ {
		if err := ctx.Err(); err != nil {
			return Result{X: copyVec(simplex[0].x), Fun: simplex[0].f,
				Iterations: iter, FuncEvals: evals, Message: "cancelled"}, err
		}

		// Convergence: check spread of function values.
		fRange := math.Abs(simplex[n].f - simplex[0].f)
		if fRange < funTol {
			converged = true
			break
		}

		// Convergence: check spread of simplex vertices.
		maxDist := 0.0
		for i := 1; i <= n; i++ {
			d := vecDist(simplex[i].x, simplex[0].x)
			if d > maxDist {
				maxDist = d
			}
		}
		if maxDist < xTol {
			converged = true
			break
		}

		computeCentroid()

		// Reflect.
		xr := reflectPoint(alpha)
		fr := f(xr)
		evals++

		switch {
		case fr < simplex[n-1].f && fr >= simplex[0].f:
			// Accept reflection.
			simplex[n] = vertex{x: xr, f: fr}
		case fr < simplex[0].f:
			// Expand.
			xe := reflectPoint(gamma)
			fe := f(xe)
			evals++
			if fe < fr {
				simplex[n] = vertex{x: xe, f: fe}
			} else {
				simplex[n] = vertex{x: xr, f: fr}
			}
		default:
			// Contract.
			if fr < simplex[n].f {
				// Outside contraction.
				xc := make([]float64, n)
				for j := range n {
					xc[j] = centroid[j] + rho*(xr[j]-centroid[j])
				}
				fc := f(xc)
				evals++
				if fc <= fr {
					simplex[n] = vertex{x: xc, f: fc}
				} else {
					for i := 1; i <= n; i++ {
						for j := range n {
							simplex[i].x[j] = simplex[0].x[j] + sigma*(simplex[i].x[j]-simplex[0].x[j])
						}
						simplex[i].f = f(simplex[i].x)
						evals++
					}
				}
			} else {
				// Inside contraction.
				xc := make([]float64, n)
				for j := range n {
					xc[j] = centroid[j] - rho*(centroid[j]-simplex[n].x[j])
				}
				fc := f(xc)
				evals++
				if fc < simplex[n].f {
					simplex[n] = vertex{x: xc, f: fc}
				} else {
					for i := 1; i <= n; i++ {
						for j := range n {
							simplex[i].x[j] = simplex[0].x[j] + sigma*(simplex[i].x[j]-simplex[0].x[j])
						}
						simplex[i].f = f(simplex[i].x)
						evals++
					}
				}
			}
		}

		sortSimplex()

		if cb != nil {
			if err := cb(iter, simplex[0].x, simplex[0].f); err != nil {
				return Result{X: copyVec(simplex[0].x), Fun: simplex[0].f,
					Iterations: iter + 1, FuncEvals: evals, Message: "stopped by callback"}, nil
			}
		}
	}

	msg := "maximum iterations reached"
	if converged {
		msg = "converged"
	}
	return Result{
		X:          copyVec(simplex[0].x),
		Fun:        simplex[0].f,
		Iterations: iter,
		FuncEvals:  evals,
		Converged:  converged,
		Message:    msg,
	}, nil
}

func copyVec(x []float64) []float64 {
	out := make([]float64, len(x))
	copy(out, x)
	return out
}

func vecDist(a, b []float64) float64 {
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}
