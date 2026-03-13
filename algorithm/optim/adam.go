package optim

import (
	"context"
	"fmt"
	"math"
)

// Adam implements the Adam optimizer (gradient-based).
type Adam struct {
	// LR is the learning rate. Default: 0.001.
	LR float64
	// Beta1 is the exponential decay rate for first moment. Default: 0.9.
	Beta1 float64
	// Beta2 is the exponential decay rate for second moment. Default: 0.999.
	Beta2 float64
	// Epsilon prevents division by zero. Default: 1e-8.
	Epsilon float64
}

func (a *Adam) Name() string { return "Adam" }

func (a *Adam) lr() float64 {
	if a.LR > 0 {
		return a.LR
	}
	return 0.001
}

func (a *Adam) beta1() float64 {
	if a.Beta1 > 0 {
		return a.Beta1
	}
	return 0.9
}

func (a *Adam) beta2() float64 {
	if a.Beta2 > 0 {
		return a.Beta2
	}
	return 0.999
}

func (a *Adam) epsilon() float64 {
	if a.Epsilon > 0 {
		return a.Epsilon
	}
	return 1e-8
}

func (a *Adam) Minimize(ctx context.Context, f ObjectiveFunc, x0 []float64,
	grad GradientFunc, opts *Options) (Result, error) {

	n := len(x0)
	if n == 0 {
		return Result{}, fmt.Errorf("adam: x0 must be non-empty")
	}
	if grad == nil {
		return Result{}, fmt.Errorf("adam: gradient function required")
	}

	maxIter := opts.maxIter()
	gradTol := opts.gradTol()
	funTol := opts.funTol()
	cb := opts.callback()

	lr := a.lr()
	b1 := a.beta1()
	b2 := a.beta2()
	eps := a.epsilon()

	x := copyVec(x0)
	m := make([]float64, n) // first moment
	v := make([]float64, n) // second moment

	fEvals := 0
	gEvals := 0

	fx := f(x)
	fEvals++
	prevFx := fx

	var converged bool
	var iter int

	for iter = 0; iter < maxIter; iter++ {
		if err := ctx.Err(); err != nil {
			return Result{X: copyVec(x), Fun: fx, Iterations: iter,
				FuncEvals: fEvals, GradEvals: gEvals, Message: "cancelled"}, err
		}

		g := grad(x)
		gEvals++

		// Check gradient convergence.
		if gradTol > 0 {
			gNorm := vecNorm(g)
			if gNorm < gradTol {
				converged = true
				break
			}
		}

		t := float64(iter + 1)
		for j := range n {
			m[j] = b1*m[j] + (1-b1)*g[j]
			v[j] = b2*v[j] + (1-b2)*g[j]*g[j]
			mHat := m[j] / (1 - math.Pow(b1, t))
			vHat := v[j] / (1 - math.Pow(b2, t))
			x[j] -= lr * mHat / (math.Sqrt(vHat) + eps)
		}

		fx = f(x)
		fEvals++

		if math.Abs(fx-prevFx) < funTol {
			converged = true
			iter++
			break
		}
		prevFx = fx

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

func vecNorm(g []float64) float64 {
	var sum float64
	for _, v := range g {
		sum += v * v
	}
	return math.Sqrt(sum)
}
