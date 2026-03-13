package mitigation_test

import (
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/mitigation"
)

func TestExtrapolateLinear(t *testing.T) {
	// y = 1 - 0.1*x, true zero-noise value = 1.0
	x := []float64{1, 3, 5}
	y := []float64{0.9, 0.7, 0.5}

	got, err := mitigation.Extrapolate(x, y, mitigation.LinearExtrapolator)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-1.0) > 1e-10 {
		t.Errorf("linear extrapolation = %f, want 1.0", got)
	}
}

func TestExtrapolateLinear_TwoPoints(t *testing.T) {
	x := []float64{1, 3}
	y := []float64{0.8, 0.6}

	got, err := mitigation.Extrapolate(x, y, mitigation.LinearExtrapolator)
	if err != nil {
		t.Fatal(err)
	}
	// slope = (0.6-0.8)/(3-1) = -0.1, intercept = 0.8 - (-0.1)*1 = 0.9
	if math.Abs(got-0.9) > 1e-10 {
		t.Errorf("linear extrapolation = %f, want 0.9", got)
	}
}

func TestExtrapolatePolynomial(t *testing.T) {
	// y = 1 - 0.05*x^2, zero-noise value = 1.0
	x := []float64{1, 3, 5}
	y := make([]float64, 3)
	for i, xi := range x {
		y[i] = 1 - 0.05*xi*xi
	}

	got, err := mitigation.Extrapolate(x, y, mitigation.PolynomialExtrapolator)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-1.0) > 1e-8 {
		t.Errorf("polynomial extrapolation = %f, want 1.0", got)
	}
}

func TestExtrapolateExponential(t *testing.T) {
	// y = 0.2 + 0.8*exp(-0.3*x), zero-noise value = 0.2 + 0.8 = 1.0
	a, b, c := 0.2, 0.8, -0.3
	x := []float64{1, 3, 5, 7}
	y := make([]float64, len(x))
	for i, xi := range x {
		y[i] = a + b*math.Exp(c*xi)
	}

	got, err := mitigation.Extrapolate(x, y, mitigation.ExponentialExtrapolator)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-1.0) > 0.05 {
		t.Errorf("exponential extrapolation = %f, want ~1.0", got)
	}
}

func TestExtrapolate_Errors(t *testing.T) {
	tests := []struct {
		name string
		x, y []float64
		m    mitigation.Extrapolator
	}{
		{"length mismatch", []float64{1, 2}, []float64{1}, mitigation.LinearExtrapolator},
		{"too few points", []float64{1}, []float64{1}, mitigation.LinearExtrapolator},
		{"exp too few", []float64{1, 2}, []float64{1, 2}, mitigation.ExponentialExtrapolator},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mitigation.Extrapolate(tt.x, tt.y, tt.m)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}
