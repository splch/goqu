package optim_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/optim"
)

// rosenbrock is (1-x)^2 + 100*(y-x^2)^2, minimum at (1,1).
func rosenbrock(x []float64) float64 {
	return (1-x[0])*(1-x[0]) + 100*(x[1]-x[0]*x[0])*(x[1]-x[0]*x[0])
}

func rosenbrockGrad(x []float64) []float64 {
	dx := -2*(1-x[0]) + 200*(x[1]-x[0]*x[0])*(-2*x[0])
	dy := 200 * (x[1] - x[0]*x[0])
	return []float64{dx, dy}
}

// sphere is sum(x^2), minimum at origin.
func sphere(x []float64) float64 {
	var s float64
	for _, v := range x {
		s += v * v
	}
	return s
}

func sphereGrad(x []float64) []float64 {
	g := make([]float64, len(x))
	for i, v := range x {
		g[i] = 2 * v
	}
	return g
}

func TestNelderMead_Rosenbrock(t *testing.T) {
	nm := &optim.NelderMead{}
	res, err := nm.Minimize(context.Background(), rosenbrock, []float64{-1, -1}, nil,
		&optim.Options{MaxIter: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Converged {
		t.Fatalf("did not converge: %s", res.Message)
	}
	if math.Abs(res.X[0]-1) > 1e-3 || math.Abs(res.X[1]-1) > 1e-3 {
		t.Errorf("expected (1,1), got (%f,%f)", res.X[0], res.X[1])
	}
}

func TestNelderMead_Sphere(t *testing.T) {
	nm := &optim.NelderMead{}
	res, err := nm.Minimize(context.Background(), sphere, []float64{3, -4, 5}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(res.Fun) > 1e-6 {
		t.Errorf("expected ~0, got %e", res.Fun)
	}
}

func TestSPSA_Sphere(t *testing.T) {
	sp := &optim.SPSA{}
	res, err := sp.Minimize(context.Background(), sphere, []float64{3, -4}, nil,
		&optim.Options{MaxIter: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if res.Fun > 0.1 {
		t.Errorf("expected near 0, got %f", res.Fun)
	}
}

func TestAdam_Rosenbrock(t *testing.T) {
	adam := &optim.Adam{LR: 0.01}
	res, err := adam.Minimize(context.Background(), rosenbrock, []float64{-1, -1},
		rosenbrockGrad, &optim.Options{MaxIter: 20000, FunTol: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Fun > 0.01 {
		t.Errorf("expected near 0, got %f", res.Fun)
	}
}

func TestAdam_RequiresGradient(t *testing.T) {
	adam := &optim.Adam{}
	_, err := adam.Minimize(context.Background(), sphere, []float64{1}, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil gradient")
	}
}

func TestLBFGS_Rosenbrock(t *testing.T) {
	lb := &optim.LBFGS{}
	res, err := lb.Minimize(context.Background(), rosenbrock, []float64{-1, -1},
		rosenbrockGrad, &optim.Options{MaxIter: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Converged {
		t.Fatalf("did not converge: %s", res.Message)
	}
	if math.Abs(res.X[0]-1) > 1e-3 || math.Abs(res.X[1]-1) > 1e-3 {
		t.Errorf("expected (1,1), got (%f,%f)", res.X[0], res.X[1])
	}
}

func TestLBFGS_RequiresGradient(t *testing.T) {
	lb := &optim.LBFGS{}
	_, err := lb.Minimize(context.Background(), sphere, []float64{1}, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil gradient")
	}
}

func TestCallback_Stops(t *testing.T) {
	nm := &optim.NelderMead{}
	iters := 0
	res, err := nm.Minimize(context.Background(), sphere, []float64{5, 5}, nil,
		&optim.Options{
			MaxIter: 10000,
			Callback: func(iter int, _ []float64, _ float64) error {
				iters = iter
				if iter >= 3 {
					return context.Canceled
				}
				return nil
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	if iters < 3 {
		t.Errorf("callback did not run enough, iters=%d", iters)
	}
	if res.Message != "stopped by callback" {
		t.Errorf("unexpected message: %s", res.Message)
	}
}

func TestContext_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	nm := &optim.NelderMead{}
	_, err := nm.Minimize(ctx, sphere, []float64{5, 5}, nil, nil)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestOptimizer_Names(t *testing.T) {
	tests := []struct {
		opt  optim.Optimizer
		name string
	}{
		{&optim.NelderMead{}, "Nelder-Mead"},
		{&optim.SPSA{}, "SPSA"},
		{&optim.Adam{}, "Adam"},
		{&optim.LBFGS{}, "L-BFGS"},
	}
	for _, tt := range tests {
		if got := tt.opt.Name(); got != tt.name {
			t.Errorf("got %q, want %q", got, tt.name)
		}
	}
}
