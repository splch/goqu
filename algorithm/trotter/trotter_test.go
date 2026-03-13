package trotter_test

import (
	"context"
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/goqu/algorithm/trotter"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// svDist computes the Euclidean distance between two state vectors.
func svDist(a, b []complex128) float64 {
	var d float64
	for i := range a {
		d += cmplx.Abs(a[i]-b[i]) * cmplx.Abs(a[i]-b[i])
	}
	return math.Sqrt(d)
}

// TestSingleZ verifies that H = Z, time = pi/4 matches the analytic result.
// e^{-i*(pi/4)*Z} on |0> = [e^{-i*pi/4}, 0].
func TestSingleZ(t *testing.T) {
	ctx := context.Background()

	// H = 1.0 * Z
	zTerm := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z}, 1)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zTerm})
	if err != nil {
		t.Fatal(err)
	}

	sv, err := trotter.Evolve(ctx, trotter.Config{
		Hamiltonian: ham,
		Time:        math.Pi / 4,
		Steps:       1,
		Order:       trotter.First,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Direct computation: RZ(pi/2) on |0>.
	// RZ(theta) = diag(e^{-i*theta/2}, e^{i*theta/2}), so RZ(pi/2)|0> = [e^{-i*pi/4}, 0].
	// e^{-i*theta*Z} = diag(e^{-i*theta}, e^{i*theta}), so e^{-i*(pi/4)*Z}|0> = [e^{-i*pi/4}, 0].
	// applyPauliExp applies exp(-i*angle*Z) via RZ(2*angle), so angle = 1.0 * pi/4.
	// RZ(2*pi/4) = RZ(pi/2) => |0> -> [e^{-i*pi/4}, 0].
	expected := []complex128{cmplx.Exp(-1i * math.Pi / 4), 0}

	for i, got := range sv {
		if cmplx.Abs(got-expected[i]) > 1e-6 {
			t.Errorf("amplitude[%d]: got %v, want %v", i, got, expected[i])
		}
	}
}

// TestSingleX verifies that H = X, time = pi/4 on |0> gives the analytic result.
// e^{-i*(pi/4)*X}|0> = [cos(pi/4), -i*sin(pi/4)] = [1/sqrt(2), -i/sqrt(2)].
func TestSingleX(t *testing.T) {
	ctx := context.Background()

	xTerm := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.X}, 1)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{xTerm})
	if err != nil {
		t.Fatal(err)
	}

	sv, err := trotter.Evolve(ctx, trotter.Config{
		Hamiltonian: ham,
		Time:        math.Pi / 4,
		Steps:       1,
		Order:       trotter.First,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	s := 1.0 / math.Sqrt(2)
	expected := []complex128{complex(s, 0), complex(0, -s)}

	for i, got := range sv {
		if cmplx.Abs(got-expected[i]) > 1e-6 {
			t.Errorf("amplitude[%d]: got %v, want %v", i, got, expected[i])
		}
	}
}

// TestConvergence verifies that increasing Trotter steps reduces the
// approximation error for H = 0.5*X + 0.5*Z, time = 1.0.
func TestConvergence(t *testing.T) {
	ctx := context.Background()

	xTerm := pauli.NewPauliString(0.5, map[int]pauli.Pauli{0: pauli.X}, 1)
	zTerm := pauli.NewPauliString(0.5, map[int]pauli.Pauli{0: pauli.Z}, 1)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{xTerm, zTerm})
	if err != nil {
		t.Fatal(err)
	}

	evolveSteps := func(steps int) []complex128 {
		sv, err := trotter.Evolve(ctx, trotter.Config{
			Hamiltonian: ham,
			Time:        1.0,
			Steps:       steps,
			Order:       trotter.First,
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return sv
	}

	sv1 := evolveSteps(1)
	sv10 := evolveSteps(10)
	sv100 := evolveSteps(100)
	svExact := evolveSteps(1000) // treat as "exact"

	d1 := svDist(sv1, svExact)
	d10 := svDist(sv10, svExact)
	d100 := svDist(sv100, svExact)

	if d1 <= d10 {
		t.Errorf("expected dist(1 step) > dist(10 steps): %v <= %v", d1, d10)
	}
	if d10 <= d100 {
		t.Errorf("expected dist(10 steps) > dist(100 steps): %v <= %v", d10, d100)
	}

	t.Logf("distances: steps=1 %.6e, steps=10 %.6e, steps=100 %.6e", d1, d10, d100)
}

// TestSecondOrder verifies that second-order Trotter is more accurate
// than first-order for the same number of steps.
func TestSecondOrder(t *testing.T) {
	ctx := context.Background()

	xTerm := pauli.NewPauliString(0.5, map[int]pauli.Pauli{0: pauli.X}, 1)
	zTerm := pauli.NewPauliString(0.5, map[int]pauli.Pauli{0: pauli.Z}, 1)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{xTerm, zTerm})
	if err != nil {
		t.Fatal(err)
	}

	// Use a reference with very high step count as "exact".
	svExact, err := trotter.Evolve(ctx, trotter.Config{
		Hamiltonian: ham,
		Time:        1.0,
		Steps:       1000,
		Order:       trotter.First,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	steps := 5

	svFirst, err := trotter.Evolve(ctx, trotter.Config{
		Hamiltonian: ham,
		Time:        1.0,
		Steps:       steps,
		Order:       trotter.First,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	svSecond, err := trotter.Evolve(ctx, trotter.Config{
		Hamiltonian: ham,
		Time:        1.0,
		Steps:       steps,
		Order:       trotter.Second,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	dFirst := svDist(svFirst, svExact)
	dSecond := svDist(svSecond, svExact)

	if dSecond >= dFirst {
		t.Errorf("expected second-order to be more accurate: dist_first=%v, dist_second=%v", dFirst, dSecond)
	}
	t.Logf("first-order dist: %.6e, second-order dist: %.6e", dFirst, dSecond)
}

// TestEvolveWithInitialCircuit verifies that an initial state preparation
// circuit is correctly composed before the Trotter circuit.
func TestEvolveWithInitialCircuit(t *testing.T) {
	ctx := context.Background()

	// H = Z, time = pi/4.
	zTerm := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z}, 1)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zTerm})
	if err != nil {
		t.Fatal(err)
	}

	// Initial circuit: X gate to prepare |1>.
	ib := builder.New("init", 1)
	ib.X(0)
	initCirc, err := ib.Build()
	if err != nil {
		t.Fatal(err)
	}

	sv, err := trotter.Evolve(ctx, trotter.Config{
		Hamiltonian: ham,
		Time:        math.Pi / 4,
		Steps:       1,
	}, initCirc)
	if err != nil {
		t.Fatal(err)
	}

	// e^{-i*(pi/4)*Z}|1> = [0, e^{i*pi/4}].
	expected := []complex128{0, cmplx.Exp(1i * math.Pi / 4)}

	for i, got := range sv {
		if cmplx.Abs(got-expected[i]) > 1e-6 {
			t.Errorf("amplitude[%d]: got %v, want %v", i, got, expected[i])
		}
	}
}

// TestEmptyHamiltonian verifies that an empty Hamiltonian returns an error.
func TestEmptyHamiltonian(t *testing.T) {
	ctx := context.Background()
	_, err := trotter.Run(ctx, trotter.Config{
		Time:  1.0,
		Steps: 1,
	})
	if err == nil {
		t.Fatal("expected error for empty Hamiltonian")
	}
}

// TestTwoQubitHamiltonian verifies Trotter simulation with a two-qubit ZZ term.
func TestTwoQubitHamiltonian(t *testing.T) {
	ctx := context.Background()

	// H = ZZ on 2 qubits.
	zzTerm := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zzTerm})
	if err != nil {
		t.Fatal(err)
	}

	sv, err := trotter.Evolve(ctx, trotter.Config{
		Hamiltonian: ham,
		Time:        math.Pi / 4,
		Steps:       1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// e^{-i*(pi/4)*ZZ}|00> = e^{-i*pi/4}|00> since ZZ|00> = +1*|00>.
	// State should be [e^{-i*pi/4}, 0, 0, 0].
	expected := []complex128{cmplx.Exp(-1i * math.Pi / 4), 0, 0, 0}

	for i, got := range sv {
		if cmplx.Abs(got-expected[i]) > 1e-6 {
			t.Errorf("amplitude[%d]: got %v, want %v", i, got, expected[i])
		}
	}
}

// TestRunReturnsCircuit verifies that Run returns a valid circuit and metadata.
func TestRunReturnsCircuit(t *testing.T) {
	ctx := context.Background()

	xTerm := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.X}, 1)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{xTerm})
	if err != nil {
		t.Fatal(err)
	}

	res, err := trotter.Run(ctx, trotter.Config{
		Hamiltonian: ham,
		Time:        1.0,
		Steps:       3,
		Order:       trotter.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Circuit == nil {
		t.Fatal("expected non-nil circuit")
	}
	if res.Steps != 3 {
		t.Errorf("expected Steps=3, got %d", res.Steps)
	}
	if res.Order != trotter.Second {
		t.Errorf("expected Order=Second, got %d", res.Order)
	}
	if res.Circuit.NumQubits() != 1 {
		t.Errorf("expected 1 qubit, got %d", res.Circuit.NumQubits())
	}

	// Verify that evolving the returned circuit with a simulator produces
	// the same result as calling Evolve directly.
	sim := statevector.New(1)
	if err := sim.Evolve(res.Circuit); err != nil {
		t.Fatal(err)
	}
	svFromRun := sim.StateVector()

	svFromEvolve, err := trotter.Evolve(ctx, trotter.Config{
		Hamiltonian: ham,
		Time:        1.0,
		Steps:       3,
		Order:       trotter.Second,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if svDist(svFromRun, svFromEvolve) > 1e-10 {
		t.Errorf("Run circuit and Evolve produced different results")
	}
}
