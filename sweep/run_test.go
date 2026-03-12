package sweep_test

import (
	"context"
	"math"
	"sync/atomic"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/circuit/param"
	"github.com/splch/qgo/observe"
	"github.com/splch/qgo/sim/noise"
	"github.com/splch/qgo/sweep"
)

func buildRYCircuit(t *testing.T) *ir.Circuit {
	t.Helper()
	theta := param.New("theta")
	c, err := builder.New("ry", 1).
		SymRY(theta.Expr(), 0).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRunSim_Basic(t *testing.T) {
	c := buildRYCircuit(t)
	sw := sweep.Linspace{Key: "theta", Start: 0, Stop: math.Pi, Count: 5}
	results, err := sweep.RunSim(context.Background(), c, 1000, sw)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("len(results) = %d, want 5", len(results))
	}
	// theta=0 → always |0>, theta=pi → always |1>.
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if results[0].Counts["1"] > 10 {
		t.Errorf("theta=0: got %d |1> counts, expected ~0", results[0].Counts["1"])
	}
	if results[4].Err != nil {
		t.Fatal(results[4].Err)
	}
	if results[4].Counts["0"] > 10 {
		t.Errorf("theta=pi: got %d |0> counts, expected ~0", results[4].Counts["0"])
	}
}

func TestRunSim_MultiParam(t *testing.T) {
	theta := param.New("theta")
	phi := param.New("phi")
	c, err := builder.New("multi", 2).
		SymRY(theta.Expr(), 0).
		SymRY(phi.Expr(), 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sw := sweep.Product(
		sweep.Linspace{Key: "theta", Start: 0, Stop: math.Pi, Count: 3},
		sweep.Linspace{Key: "phi", Start: 0, Stop: math.Pi, Count: 2},
	)
	results, err := sweep.RunSim(context.Background(), c, 100, sw)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 6 {
		t.Errorf("len(results) = %d, want 6", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result[%d]: %v", i, r.Err)
		}
		if r.Index != i {
			t.Errorf("result[%d].Index = %d", i, r.Index)
		}
	}
}

func TestRunSim_UnboundParam(t *testing.T) {
	c := buildRYCircuit(t)
	// Sweep doesn't cover "theta".
	sw := sweep.NewPoints("wrong_name", []float64{1.0})
	_, err := sweep.RunSim(context.Background(), c, 100, sw)
	if err == nil {
		t.Fatal("expected error for unbound parameter")
	}
}

func TestRunSim_Hooks(t *testing.T) {
	c := buildRYCircuit(t)
	sw := sweep.Linspace{Key: "theta", Start: 0, Stop: math.Pi, Count: 3}

	var sweepCalls atomic.Int32
	var simCalls atomic.Int32
	hooks := &observe.Hooks{
		WrapSweep: func(ctx context.Context, info observe.SweepInfo) (context.Context, func(error)) {
			sweepCalls.Add(1)
			if info.NumPoints != 3 {
				t.Errorf("SweepInfo.NumPoints = %d, want 3", info.NumPoints)
			}
			return ctx, func(error) {}
		},
		WrapSim: func(ctx context.Context, info observe.SimInfo) (context.Context, func(error)) {
			simCalls.Add(1)
			return ctx, func(error) {}
		},
	}
	ctx := observe.WithHooks(context.Background(), hooks)

	results, err := sweep.RunSim(ctx, c, 100, sw)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if sweepCalls.Load() != 1 {
		t.Errorf("WrapSweep called %d times, want 1", sweepCalls.Load())
	}
	if simCalls.Load() != 3 {
		t.Errorf("WrapSim called %d times, want 3", simCalls.Load())
	}
}

func TestRunDensitySim_Basic(t *testing.T) {
	c := buildRYCircuit(t)
	sw := sweep.Linspace{Key: "theta", Start: 0, Stop: math.Pi, Count: 3}
	results, err := sweep.RunDensitySim(context.Background(), c, 1000, sw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result[%d]: %v", i, r.Err)
		}
	}
	// theta=0 → all |0>.
	if results[0].Counts["1"] > 10 {
		t.Errorf("theta=0: got %d |1> counts, expected ~0", results[0].Counts["1"])
	}
}

func TestRunDensitySim_WithNoise(t *testing.T) {
	c := buildRYCircuit(t)
	sw := sweep.NewPoints("theta", []float64{math.Pi})

	nm := noise.New()
	nm.AddDefaultError(1, noise.BitFlip(0.1))

	results, err := sweep.RunDensitySim(context.Background(), c, 1000, sw, nm)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	// With bit flip noise, theta=pi should have some |0> counts.
	if results[0].Counts["0"] == 0 {
		t.Error("expected some |0> counts with bit flip noise")
	}
}

func TestRunSim_NoFreeParams(t *testing.T) {
	// A circuit with no free parameters should work with any sweep.
	c, err := builder.New("fixed", 1).
		X(0).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sw := sweep.NewPoints("ignored", []float64{1.0})
	results, err := sweep.RunSim(context.Background(), c, 100, sw)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Counts["1"] != 100 {
		t.Errorf("expected all |1>, got %v", results[0].Counts)
	}
}
