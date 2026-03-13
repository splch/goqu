package optim

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
)

// SPSA implements Simultaneous Perturbation Stochastic Approximation.
// Only 2 objective evaluations per iteration regardless of dimension.
type SPSA struct {
	// A, a, c are SPSA hyperparameters following Spall's notation.
	// Defaults: A=100, a=0.16, c=0.1, alpha=0.602, gamma=0.101.
	A     float64
	LR    float64 // a
	C     float64 // c
	Alpha float64 // learning rate decay exponent
	Gamma float64 // perturbation decay exponent
}

func (s *SPSA) Name() string { return "SPSA" }

func (s *SPSA) a() float64 {
	if s.LR > 0 {
		return s.LR
	}
	return 0.16
}

func (s *SPSA) bigA() float64 {
	if s.A > 0 {
		return s.A
	}
	return 100
}

func (s *SPSA) c0() float64 {
	if s.C > 0 {
		return s.C
	}
	return 0.1
}

func (s *SPSA) alpha() float64 {
	if s.Alpha > 0 {
		return s.Alpha
	}
	return 0.602
}

func (s *SPSA) gamma() float64 {
	if s.Gamma > 0 {
		return s.Gamma
	}
	return 0.101
}

func (s *SPSA) Minimize(ctx context.Context, f ObjectiveFunc, x0 []float64,
	_ GradientFunc, opts *Options) (Result, error) {

	n := len(x0)
	if n == 0 {
		return Result{}, fmt.Errorf("spsa: x0 must be non-empty")
	}

	maxIter := opts.maxIter()
	funTol := opts.funTol()
	cb := opts.callback()

	a := s.a()
	bigA := s.bigA()
	c0 := s.c0()
	alphaExp := s.alpha()
	gammaExp := s.gamma()

	x := copyVec(x0)
	bestX := copyVec(x0)
	bestF := f(x0)
	evals := 1
	prevF := bestF

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

	var converged bool
	var iter int

	for iter = 0; iter < maxIter; iter++ {
		if err := ctx.Err(); err != nil {
			return Result{X: bestX, Fun: bestF, Iterations: iter,
				FuncEvals: evals, Message: "cancelled"}, err
		}

		k := float64(iter + 1)
		ak := a / math.Pow(k+bigA, alphaExp)
		ck := c0 / math.Pow(k, gammaExp)

		// Random perturbation: Rademacher (+1/-1).
		delta := make([]float64, n)
		for j := range n {
			if rng.Float64() < 0.5 {
				delta[j] = 1
			} else {
				delta[j] = -1
			}
		}

		// Perturbed evaluations (standard SPSA: 2 evals per iteration).
		xPlus := make([]float64, n)
		xMinus := make([]float64, n)
		for j := range n {
			xPlus[j] = x[j] + ck*delta[j]
			xMinus[j] = x[j] - ck*delta[j]
		}
		fPlus := f(xPlus)
		fMinus := f(xMinus)
		evals += 2

		// Approximate gradient and update.
		for j := range n {
			gj := (fPlus - fMinus) / (2 * ck * delta[j])
			x[j] -= ak * gj
		}

		// Track best from the perturbed evaluations.
		fCur := min(fPlus, fMinus)
		if fPlus < fMinus {
			if fPlus < bestF {
				bestF = fPlus
				copy(bestX, xPlus)
			}
		} else {
			if fMinus < bestF {
				bestF = fMinus
				copy(bestX, xMinus)
			}
		}

		if math.Abs(fCur-prevF) < funTol {
			converged = true
			iter++
			break
		}
		prevF = fCur

		if cb != nil {
			if err := cb(iter, x, fCur); err != nil {
				return Result{X: bestX, Fun: bestF, Iterations: iter + 1,
					FuncEvals: evals, Message: "stopped by callback"}, nil
			}
		}
	}

	msg := "maximum iterations reached"
	if converged {
		msg = "converged"
	}
	return Result{
		X:          bestX,
		Fun:        bestF,
		Iterations: iter,
		FuncEvals:  evals,
		Converged:  converged,
		Message:    msg,
	}, nil
}
