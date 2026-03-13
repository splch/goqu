package mitigation

import (
	"fmt"
	"math"
)

// Extrapolator selects the extrapolation method for ZNE.
type Extrapolator int

const (
	// LinearExtrapolator fits y = a + bx and returns a (the y-intercept at x=0).
	LinearExtrapolator Extrapolator = iota
	// PolynomialExtrapolator fits a polynomial of degree len(points)-1 and evaluates at x=0.
	PolynomialExtrapolator
	// ExponentialExtrapolator fits y = a + b·exp(cx) and returns a.
	ExponentialExtrapolator
)

// Extrapolate estimates the zero-noise value from (scaleFactors, values) pairs.
// Both slices must have the same length (≥2).
func Extrapolate(scaleFactors, values []float64, method Extrapolator) (float64, error) {
	n := len(scaleFactors)
	if n != len(values) {
		return 0, fmt.Errorf("mitigation.Extrapolate: length mismatch: %d scale factors, %d values", n, len(values))
	}
	if n < 2 {
		return 0, fmt.Errorf("mitigation.Extrapolate: need at least 2 points, got %d", n)
	}

	switch method {
	case LinearExtrapolator:
		return extrapolateLinear(scaleFactors, values)
	case PolynomialExtrapolator:
		return extrapolatePolynomial(scaleFactors, values)
	case ExponentialExtrapolator:
		return extrapolateExponential(scaleFactors, values)
	default:
		return 0, fmt.Errorf("mitigation.Extrapolate: unknown method %d", method)
	}
}

// extrapolateLinear fits y = a + bx via least-squares normal equations.
func extrapolateLinear(x, y []float64) (float64, error) {
	n := float64(len(x))
	var sx, sy, sxx, sxy float64
	for i := range x {
		sx += x[i]
		sy += y[i]
		sxx += x[i] * x[i]
		sxy += x[i] * y[i]
	}
	det := n*sxx - sx*sx
	if math.Abs(det) < 1e-15 {
		return 0, fmt.Errorf("mitigation.Extrapolate: singular linear system (all scale factors equal?)")
	}
	a := (sxx*sy - sx*sxy) / det
	return a, nil
}

// extrapolatePolynomial fits a polynomial of degree n-1 through n points
// using Gaussian elimination on the Vandermonde system, then evaluates at x=0.
func extrapolatePolynomial(x, y []float64) (float64, error) {
	n := len(x)

	// Build augmented Vandermonde matrix [V | y].
	// V[i][j] = x[i]^j, so evaluating at x=0 gives just coefficients[0].
	aug := make([][]float64, n)
	for i := range n {
		aug[i] = make([]float64, n+1)
		val := 1.0
		for j := range n {
			aug[i][j] = val
			val *= x[i]
		}
		aug[i][n] = y[i]
	}

	// Gaussian elimination with partial pivoting.
	for col := range n {
		// Find pivot.
		maxVal := math.Abs(aug[col][col])
		maxRow := col
		for row := col + 1; row < n; row++ {
			if v := math.Abs(aug[row][col]); v > maxVal {
				maxVal = v
				maxRow = row
			}
		}
		if maxVal < 1e-15 {
			return 0, fmt.Errorf("mitigation.Extrapolate: singular Vandermonde system")
		}
		aug[col], aug[maxRow] = aug[maxRow], aug[col]

		// Eliminate below.
		for row := col + 1; row < n; row++ {
			factor := aug[row][col] / aug[col][col]
			for j := col; j <= n; j++ {
				aug[row][j] -= factor * aug[col][j]
			}
		}
	}

	// Back-substitution.
	coeffs := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		coeffs[i] = aug[i][n]
		for j := i + 1; j < n; j++ {
			coeffs[i] -= aug[i][j] * coeffs[j]
		}
		coeffs[i] /= aug[i][i]
	}

	// P(0) = coeffs[0] since P(x) = c0 + c1*x + c2*x^2 + ...
	return coeffs[0], nil
}

// extrapolateExponential fits y = a + b·exp(c·x) and returns a + b (the value at x=0).
// Uses Richardson-like approach: estimate asymptote from the data,
// then linearize log(y - a) = log(b) + c·x.
//
// The asymptote formula assumes the first three x-values are evenly spaced,
// which holds for the default ZNE scale factors (1, 3, 5, ...). For
// non-uniform spacing, the estimate degrades and linear extrapolation
// may be more reliable.
func extrapolateExponential(x, y []float64) (float64, error) {
	n := len(x)
	if n < 3 {
		return 0, fmt.Errorf("mitigation.Extrapolate: exponential fit needs at least 3 points, got %d", n)
	}

	// Estimate asymptote 'a' using the first three points.
	// Valid when x[0], x[1], x[2] are evenly spaced.
	// For y_i = a + b*exp(c*x_i), if points are evenly spaced in x:
	// a ≈ (y0*y2 - y1^2) / (y0 + y2 - 2*y1)
	// We use the first three points since ZNE scale factors are typically
	// evenly spaced (1, 3, 5, ...).
	y0, y1, y2 := y[0], y[1], y[2]

	denom := y0 + y2 - 2*y1
	var a float64
	if math.Abs(denom) > 1e-10 {
		a = (y0*y2 - y1*y1) / denom
	} else {
		// Fallback: use the minimum value as asymptote estimate (works when
		// the expectation value decays toward a limit).
		a = y[0]
		for _, v := range y[1:] {
			if v < a {
				a = v
			}
		}
	}

	// Linearize: log(y_i - a) = log(b) + c*x_i.
	// Shift a slightly to ensure y_i - a > 0.
	minShifted := math.Inf(1)
	for _, v := range y {
		if d := v - a; d < minShifted {
			minShifted = d
		}
	}
	if minShifted <= 0 {
		a -= math.Abs(minShifted) + 1e-10
	}

	// Linear regression on (x, log(y - a)).
	var sx, slog, sxx, sxlog float64
	nf := float64(n)
	for i := range x {
		li := math.Log(y[i] - a)
		sx += x[i]
		slog += li
		sxx += x[i] * x[i]
		sxlog += x[i] * li
	}
	det := nf*sxx - sx*sx
	if math.Abs(det) < 1e-15 {
		// Degenerate: fall back to linear extrapolation.
		return extrapolateLinear(x, y)
	}
	logb := (sxx*slog - sx*sxlog) / det
	c := (nf*sxlog - sx*slog) / det

	// a + b*exp(c*0) = a + b.
	b := math.Exp(logb)
	_ = c // c is used in the fit but not needed for the zero-noise value
	return a + b, nil
}
