package sweep

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/observe"
	"github.com/splch/qgo/sim/densitymatrix"
	"github.com/splch/qgo/sim/noise"
	"github.com/splch/qgo/sim/statevector"
)

// PointResult holds the outcome for a single sweep point.
type PointResult struct {
	Index    int
	Bindings map[string]float64
	Counts   map[string]int
	Err      error
}

// RunSim executes a parameterized circuit across all sweep points using
// statevector simulation. Each point gets a fresh simulator instance.
// Results are returned in sweep order.
func RunSim(ctx context.Context, c *ir.Circuit, shots int, sw Sweep) ([]PointResult, error) {
	bindings := sw.Resolve()
	if err := validateParams(c, sw); err != nil {
		return nil, err
	}

	nq := c.NumQubits()

	// Fire WrapSweep hook.
	hooks := observe.FromContext(ctx)
	if hooks != nil && hooks.WrapSweep != nil {
		var done func(error)
		ctx, done = hooks.WrapSweep(ctx, observe.SweepInfo{
			NumPoints: len(bindings),
			NumParams: len(sw.Params()),
			Shots:     shots,
			NumQubits: nq,
		})
		defer func() { done(nil) }()
	}

	results := make([]PointResult, len(bindings))
	nWorkers := min(runtime.GOMAXPROCS(0), len(bindings))
	if nWorkers < 1 {
		nWorkers = 1
	}

	work := make(chan int, len(bindings))
	for i := range bindings {
		work <- i
	}
	close(work)

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	for range nWorkers {
		go func() {
			defer wg.Done()
			for i := range work {
				results[i] = runStatevectorPoint(ctx, c, nq, shots, i, bindings[i])
			}
		}()
	}
	wg.Wait()

	return results, nil
}

func runStatevectorPoint(ctx context.Context, c *ir.Circuit, nq, shots, idx int, bind map[string]float64) PointResult {
	bound, err := ir.Bind(c, bind)
	if err != nil {
		return PointResult{Index: idx, Bindings: bind, Err: err}
	}

	// Fire per-point WrapSim hook.
	hooks := observe.FromContext(ctx)
	if hooks != nil && hooks.WrapSim != nil {
		_, done := hooks.WrapSim(ctx, observe.SimInfo{
			NumQubits: nq,
			GateCount: len(bound.Ops()),
			Shots:     shots,
		})
		defer func() { done(nil) }()
	}

	sim := statevector.New(nq)
	counts, err := sim.Run(bound, shots)
	return PointResult{Index: idx, Bindings: bind, Counts: counts, Err: err}
}

// RunDensitySim executes a parameterized circuit across all sweep points using
// density matrix simulation with an optional noise model.
// Results are returned in sweep order.
func RunDensitySim(ctx context.Context, c *ir.Circuit, shots int, sw Sweep, nm *noise.NoiseModel) ([]PointResult, error) {
	bindings := sw.Resolve()
	if err := validateParams(c, sw); err != nil {
		return nil, err
	}

	nq := c.NumQubits()

	// Fire WrapSweep hook.
	hooks := observe.FromContext(ctx)
	if hooks != nil && hooks.WrapSweep != nil {
		var done func(error)
		ctx, done = hooks.WrapSweep(ctx, observe.SweepInfo{
			NumPoints: len(bindings),
			NumParams: len(sw.Params()),
			Shots:     shots,
			NumQubits: nq,
		})
		defer func() { done(nil) }()
	}

	results := make([]PointResult, len(bindings))
	nWorkers := min(runtime.GOMAXPROCS(0), len(bindings))
	if nWorkers < 1 {
		nWorkers = 1
	}

	work := make(chan int, len(bindings))
	for i := range bindings {
		work <- i
	}
	close(work)

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	for range nWorkers {
		go func() {
			defer wg.Done()
			for i := range work {
				results[i] = runDensityPoint(ctx, c, nq, shots, i, bindings[i], nm)
			}
		}()
	}
	wg.Wait()

	return results, nil
}

func runDensityPoint(ctx context.Context, c *ir.Circuit, nq, shots, idx int, bind map[string]float64, nm *noise.NoiseModel) PointResult {
	bound, err := ir.Bind(c, bind)
	if err != nil {
		return PointResult{Index: idx, Bindings: bind, Err: err}
	}

	hooks := observe.FromContext(ctx)
	if hooks != nil && hooks.WrapSim != nil {
		_, done := hooks.WrapSim(ctx, observe.SimInfo{
			NumQubits: nq,
			GateCount: len(bound.Ops()),
			Shots:     shots,
		})
		defer func() { done(nil) }()
	}

	sim := densitymatrix.New(nq)
	if nm != nil {
		sim.WithNoise(nm)
	}
	counts, err := sim.Run(bound, shots)
	return PointResult{Index: idx, Bindings: bind, Counts: counts, Err: err}
}

// validateParams checks that all free parameters in the circuit are covered by the sweep.
func validateParams(c *ir.Circuit, sw Sweep) error {
	free := ir.FreeParameters(c)
	if len(free) == 0 {
		return nil
	}
	sweepParams := make(map[string]bool, len(sw.Params()))
	for _, p := range sw.Params() {
		sweepParams[p] = true
	}
	for _, f := range free {
		if !sweepParams[f] {
			return fmt.Errorf("sweep: circuit has free parameter %q not covered by sweep", f)
		}
	}
	return nil
}
