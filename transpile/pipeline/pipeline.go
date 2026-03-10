// Package pipeline provides pre-built transpilation pipelines.
package pipeline

import (
	"context"

	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/observe"
	"github.com/splch/qgo/transpile"
	"github.com/splch/qgo/transpile/pass"
	"github.com/splch/qgo/transpile/routing"
	"github.com/splch/qgo/transpile/target"
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
func passesForLevel(level Level) []namedPass {
	switch level {
	case LevelNone:
		return []namedPass{
			{"remove_barriers", pass.RemoveBarriers},
			{"decompose_to_target", pass.DecomposeToTarget},
			{"validate_target", pass.ValidateTarget},
		}
	case LevelBasic:
		return []namedPass{
			{"remove_barriers", pass.RemoveBarriers},
			{"route", routeIfNeeded},
			{"decompose_to_target", pass.DecomposeToTarget},
			{"cancel_adjacent", pass.CancelAdjacent},
			{"merge_rotations", pass.MergeRotations},
			{"cancel_adjacent", pass.CancelAdjacent},
			{"validate_target", pass.ValidateTarget},
		}
	case LevelFull:
		return []namedPass{
			{"remove_barriers", pass.RemoveBarriers},
			{"route", routeIfNeeded},
			{"decompose_to_target", pass.DecomposeToTarget},
			{"cancel_adjacent", pass.CancelAdjacent},
			{"merge_rotations", pass.MergeRotations},
			{"commute", pass.CommuteThroughCNOT},
			{"cancel_adjacent", pass.CancelAdjacent},
			{"merge_rotations", pass.MergeRotations},
			{"parallelize", pass.ParallelizeOps},
			{"validate_target", pass.ValidateTarget},
		}
	default:
		return passesForLevel(LevelBasic)
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
// Weights: 10·TwoQubitGates + Depth + 0.1·GateCount.
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
