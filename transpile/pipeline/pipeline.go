// Package pipeline provides pre-built transpilation pipelines.
package pipeline

import (
	"context"

	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/observe"
	"github.com/splch/goqu/transpile"
	"github.com/splch/goqu/transpile/pass"
	"github.com/splch/goqu/transpile/routing"
	"github.com/splch/goqu/transpile/target"
)

// Level controls the optimization aggressiveness.
type Level int

const (
	LevelNone     Level = 0 // decompose only
	LevelBasic    Level = 1 // + cancel + merge
	LevelFull     Level = 2 // + commute + parallelize
	LevelParallel Level = 3 // multi-strategy, pick best
)

type namedPass struct {
	name string
	fn   transpile.Pass
}

// passesForLevel returns the ordered named passes for a given optimization level.
//
// The pipeline follows the industry-standard quantum compilation flow:
//
//  1. INIT        — decompose >2Q gates so SABRE can route all 2Q interactions
//  2. PRE-ROUTE   — cancel/merge to shrink the circuit before routing (fewer SWAPs)
//  3. ROUTE       — SABRE qubit routing (insert SWAPs for connectivity)
//  4. TRANSLATE   — decompose to target basis + fix gate direction
//  5. OPTIMIZE    — iterative cancel/merge/commute/consolidate until fixed-point
//  6. VALIDATE    — verify basis, connectivity, and depth constraints
func passesForLevel(level Level) []namedPass {
	switch level {
	case LevelNone:
		return []namedPass{
			{"remove_barriers", pass.RemoveBarriers},
			{"decompose_multi_qubit", pass.DecomposeMultiQubit},
			{"route", routeIfNeeded},
			{"decompose_to_target", pass.DecomposeToTarget},
			{"fix_direction", pass.FixDirection},
			{"decompose_to_target", pass.DecomposeToTarget},
			{"validate_target", pass.ValidateTarget},
		}
	case LevelBasic:
		return []namedPass{
			// Init.
			{"remove_barriers", pass.RemoveBarriers},
			{"decompose_multi_qubit", pass.DecomposeMultiQubit},
			// Pre-routing optimization.
			{"cancel_adjacent", pass.CancelAdjacent},
			{"merge_rotations", pass.MergeRotations},
			{"remove_identity", pass.RemoveIdentity},
			// Routing.
			{"route", routeIfNeeded},
			// Target translation.
			{"decompose_to_target", pass.DecomposeToTarget},
			{"fix_direction", pass.FixDirection},
			{"decompose_to_target", pass.DecomposeToTarget},
			// Post-routing optimization (iterative).
			fixedPointLoop("optimize_loop", 10,
				namedPass{"cancel_adjacent", pass.CancelAdjacent},
				namedPass{"merge_rotations", pass.MergeRotations},
				namedPass{"remove_identity", pass.RemoveIdentity},
			),
			{"validate_target", pass.ValidateTarget},
		}
	case LevelFull:
		return []namedPass{
			// Init.
			{"remove_barriers", pass.RemoveBarriers},
			{"decompose_multi_qubit", pass.DecomposeMultiQubit},
			// Pre-routing optimization.
			{"cancel_adjacent", pass.CancelAdjacent},
			{"merge_rotations", pass.MergeRotations},
			{"remove_identity", pass.RemoveIdentity},
			// Routing.
			{"route", routeIfNeeded},
			// Target translation.
			{"decompose_to_target", pass.DecomposeToTarget},
			{"fix_direction", pass.FixDirection},
			{"decompose_to_target", pass.DecomposeToTarget},
			// Post-routing optimization with 2Q consolidation (iterative).
			fixedPointLoop("optimize_loop", 10,
				namedPass{"consolidate_2q", pass.Consolidate2QBlocks},
				namedPass{"decompose_to_target", pass.DecomposeToTarget},
				namedPass{"fix_direction", pass.FixDirection},
				namedPass{"decompose_to_target", pass.DecomposeToTarget},
				namedPass{"cancel_adjacent", pass.CancelAdjacent},
				namedPass{"merge_rotations", pass.MergeRotations},
				namedPass{"commute", pass.CommuteThroughCNOT},
				namedPass{"cancel_adjacent", pass.CancelAdjacent},
				namedPass{"merge_rotations", pass.MergeRotations},
				namedPass{"remove_identity", pass.RemoveIdentity},
			),
			{"parallelize", pass.ParallelizeOps},
			{"validate_target", pass.ValidateTarget},
		}
	default:
		return passesForLevel(LevelBasic)
	}
}

// fixedPointLoop wraps passes and runs them iteratively until circuit
// statistics (gate count, depth, 2Q gates) stabilize or maxIter is reached.
func fixedPointLoop(name string, maxIter int, passes ...namedPass) namedPass {
	inner := make([]transpile.Pass, len(passes))
	for i, p := range passes {
		inner[i] = p.fn
	}
	pipeline := transpile.Pipeline(inner...)

	return namedPass{
		name: name,
		fn: func(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
			for range maxIter {
				prev := c.Stats()
				var err error
				c, err = pipeline(c, t)
				if err != nil {
					return nil, err
				}
				cur := c.Stats()
				if cur.GateCount == prev.GateCount &&
					cur.Depth == prev.Depth &&
					cur.TwoQubitGates == prev.TwoQubitGates {
					break
				}
			}
			return c, nil
		},
	}
}

// DefaultPipeline returns a transpilation pass for the given optimization level.
func DefaultPipeline(level Level) transpile.Pass {
	if level == LevelParallel {
		return func(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
			strategies := []transpile.Pass{
				DefaultPipeline(LevelBasic),
				DefaultPipeline(LevelFull),
			}
			return OptimizeParallel(c, t, strategies, DefaultCost)
		}
	}
	np := passesForLevel(level)
	fns := make([]transpile.Pass, len(np))
	for i, p := range np {
		fns[i] = p.fn
	}
	return transpile.Pipeline(fns...)
}

// Run executes a transpilation pipeline with observability hooks from the context.
// It fires WrapTranspile around the full pipeline and WrapPass around each pass.
func Run(ctx context.Context, c *ir.Circuit, t target.Target, level Level) (*ir.Circuit, error) {
	hooks := observe.FromContext(ctx)
	inInfo := circuitInfo(c)

	if level == LevelParallel {
		return runParallel(ctx, c, t, hooks, inInfo)
	}

	var transpileDone func(observe.CircuitInfo, error)
	if hooks != nil && hooks.WrapTranspile != nil {
		ctx, transpileDone = hooks.WrapTranspile(ctx, int(level), inInfo)
	}

	result, err := runPasses(ctx, c, t, passesForLevel(level), hooks)

	if transpileDone != nil {
		out := observe.CircuitInfo{}
		if err == nil && result != nil {
			out = circuitInfo(result)
		}
		transpileDone(out, err)
	}
	return result, err
}

func runPasses(ctx context.Context, c *ir.Circuit, t target.Target, passes []namedPass, hooks *observe.Hooks) (*ir.Circuit, error) {
	result := c
	for _, np := range passes {
		passIn := circuitInfo(result)

		var passDone func(observe.CircuitInfo, error)
		if hooks != nil && hooks.WrapPass != nil {
			ctx, passDone = hooks.WrapPass(ctx, np.name, passIn)
		}

		out, err := np.fn(result, t)

		if passDone != nil {
			passOut := observe.CircuitInfo{}
			if err == nil && out != nil {
				passOut = circuitInfo(out)
			}
			passDone(passOut, err)
		}

		if err != nil {
			return nil, err
		}
		result = out
	}
	return result, nil
}

func runParallel(ctx context.Context, c *ir.Circuit, t target.Target, hooks *observe.Hooks, inInfo observe.CircuitInfo) (*ir.Circuit, error) {
	var transpileDone func(observe.CircuitInfo, error)
	if hooks != nil && hooks.WrapTranspile != nil {
		ctx, transpileDone = hooks.WrapTranspile(ctx, int(LevelParallel), inInfo)
	}

	strategies := []transpile.Pass{
		func(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
			return Run(ctx, c, t, LevelBasic)
		},
		func(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
			return Run(ctx, c, t, LevelFull)
		},
	}
	result, err := OptimizeParallel(c, t, strategies, DefaultCost)

	if transpileDone != nil {
		out := observe.CircuitInfo{}
		if err == nil && result != nil {
			out = circuitInfo(result)
		}
		transpileDone(out, err)
	}
	return result, err
}

// DefaultCost scores a circuit: lower is better.
// Two-qubit gates dominate error budget (10× weight), depth determines execution
// time (1×), and total gate count is a minor tiebreaker (0.1×).
func DefaultCost(c *ir.Circuit) float64 {
	s := c.Stats()
	return 10*float64(s.TwoQubitGates) + float64(s.Depth) + 0.1*float64(s.GateCount)
}

// OptimizeParallel runs multiple strategies and returns the lowest-cost result.
func OptimizeParallel(c *ir.Circuit, t target.Target, strategies []transpile.Pass, cost func(*ir.Circuit) float64) (*ir.Circuit, error) {
	type result struct {
		circuit *ir.Circuit
		cost    float64
		err     error
	}

	ch := make(chan result, len(strategies))
	for _, s := range strategies {
		go func(p transpile.Pass) {
			out, err := p(c, t)
			if err != nil {
				ch <- result{err: err}
				return
			}
			ch <- result{circuit: out, cost: cost(out)}
		}(s)
	}

	var best *ir.Circuit
	bestCost := 0.0
	var firstErr error

	for range strategies {
		r := <-ch
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		if best == nil || r.cost < bestCost {
			best = r.circuit
			bestCost = r.cost
		}
	}

	if best == nil {
		return nil, firstErr
	}
	return best, nil
}

// routeIfNeeded applies SABRE routing only when the target has constrained connectivity.
func routeIfNeeded(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
	if t.Connectivity == nil {
		return c, nil
	}
	return routing.Route(c, t)
}

func circuitInfo(c *ir.Circuit) observe.CircuitInfo {
	s := c.Stats()
	return observe.CircuitInfo{
		Name:          c.Name(),
		NumQubits:     c.NumQubits(),
		GateCount:     s.GateCount,
		TwoQubitGates: s.TwoQubitGates,
		Depth:         s.Depth,
		Params:        s.Params,
	}
}
