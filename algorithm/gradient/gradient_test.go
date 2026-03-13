package gradient_test

import (
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/gradient"
	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
	"github.com/splch/goqu/sim/pauli"
)

func TestCostFunc_RYZ(t *testing.T) {
	// Single RY(theta) on qubit 0, measure Z expectation.
	// E(theta) = cos(theta).
	p := param.New("theta")
	circ, err := builder.New("test", 1).
		SymRY(p.Expr(), 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Hamiltonian: Z on qubit 0.
	h, err := pauli.NewPauliSum([]pauli.PauliString{
		pauli.ZOn([]int{0}, 1),
	})
	if err != nil {
		t.Fatal(err)
	}

	names := ir.FreeParameters(circ)
	cost := gradient.CostFunc(circ, h, names)

	tests := []struct {
		theta float64
		want  float64
	}{
		{0, 1.0},
		{math.Pi / 2, 0.0},
		{math.Pi, -1.0},
	}
	for _, tt := range tests {
		got := cost([]float64{tt.theta})
		if math.Abs(got-tt.want) > 1e-10 {
			t.Errorf("CostFunc(%f) = %f, want %f", tt.theta, got, tt.want)
		}
	}
}

func TestParameterShift_RYZ(t *testing.T) {
	// Gradient of cos(theta) is -sin(theta).
	p := param.New("theta")
	circ, err := builder.New("test", 1).
		SymRY(p.Expr(), 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	h, err := pauli.NewPauliSum([]pauli.PauliString{
		pauli.ZOn([]int{0}, 1),
	})
	if err != nil {
		t.Fatal(err)
	}

	names := ir.FreeParameters(circ)
	cost := gradient.CostFunc(circ, h, names)
	gradFn := gradient.ParameterShift(cost)

	tests := []struct {
		theta float64
		want  float64 // -sin(theta)
	}{
		{0, 0.0},
		{math.Pi / 4, -math.Sin(math.Pi / 4)},
		{math.Pi / 2, -1.0},
	}
	for _, tt := range tests {
		g := gradFn([]float64{tt.theta})
		if math.Abs(g[0]-tt.want) > 1e-6 {
			t.Errorf("grad(%f) = %f, want %f", tt.theta, g[0], tt.want)
		}
	}
}

func TestFiniteDifference(t *testing.T) {
	// f(x) = x^2, gradient = 2x.
	f := optim.ObjectiveFunc(func(x []float64) float64 { return x[0] * x[0] })
	gradFn := gradient.FiniteDifference(f, 1e-5)

	tests := []struct {
		x    float64
		want float64
	}{
		{0, 0},
		{1, 2},
		{-3, -6},
	}
	for _, tt := range tests {
		g := gradFn([]float64{tt.x})
		if math.Abs(g[0]-tt.want) > 1e-4 {
			t.Errorf("grad(%f) = %f, want %f", tt.x, g[0], tt.want)
		}
	}
}
