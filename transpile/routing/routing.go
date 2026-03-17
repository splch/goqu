package routing

import (
	"math"
	"math/rand/v2"
	"runtime"
	"sync"

	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

// Options configures the SABRE routing algorithm.
type Options struct {
	Trials                int     // number of random initial layouts to try (default 20)
	BidirectionalIters    int     // forward+backward iterations per trial (default 4)
	Seed                  *uint64 // random seed; nil = non-deterministic
	Parallelism           int     // max concurrent trials (default GOMAXPROCS)
	DecayDelta            float64 // decay increment per SWAP (default 0.001)
	ExtendedSetDepth      int     // BFS layers for lookahead (default 3)
	ExtendedSetWeight     float64 // geometric weight for extended set layers (default 0.5)
	ReleaseValveThreshold int     // SWAPs before release valve fires (default 10*numQubits, -1 disables)
}

func (o Options) withDefaults(numQubits int) Options {
	if o.Trials <= 0 {
		o.Trials = 20
	}
	if o.BidirectionalIters <= 0 {
		o.BidirectionalIters = 4
	}
	if o.Parallelism <= 0 {
		o.Parallelism = runtime.GOMAXPROCS(0)
	}
	if o.DecayDelta <= 0 {
		o.DecayDelta = 0.001
	}
	if o.ExtendedSetDepth <= 0 {
		o.ExtendedSetDepth = 3
	}
	if o.ExtendedSetWeight <= 0 {
		o.ExtendedSetWeight = 0.5
	}
	if o.ReleaseValveThreshold == 0 {
		o.ReleaseValveThreshold = 10 * numQubits
	}
	return o
}

// Route inserts SWAP gates to satisfy target connectivity constraints.
// Uses the SABRE algorithm with production defaults (20 trials, bidirectional iteration).
// Returns the circuit unchanged for all-to-all targets.
func Route(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
	return RouteWithOptions(c, t, Options{})
}

// RouteWithOptions inserts SWAP gates using configurable SABRE parameters.
func RouteWithOptions(c *ir.Circuit, t target.Target, opts Options) (*ir.Circuit, error) {
	if t.Connectivity == nil {
		return c, nil
	}

	dist := t.DistanceMatrix()
	adj := t.AdjacencyMap()
	n := c.NumQubits()
	ops := c.Ops()

	// Quick check: if no 2-qubit gates, just return as-is.
	has2Q := false
	for _, op := range ops {
		if op.Gate != nil && op.Gate.Qubits() >= 2 {
			has2Q = true
			break
		}
	}
	if !has2Q {
		return c, nil
	}

	opts = opts.withDefaults(n)

	// Determine RNG source for each trial.
	var baseSeed uint64
	if opts.Seed != nil {
		baseSeed = *opts.Seed
	} else {
		// Use a random seed from the global source.
		baseSeed = rand.Uint64()
	}

	type trialResult struct {
		ops   []ir.Operation
		swaps int
	}

	results := make([]trialResult, opts.Trials)

	// Run trials in parallel with a semaphore.
	sem := make(chan struct{}, opts.Parallelism)
	var wg sync.WaitGroup

	for trial := range opts.Trials {
		wg.Add(1)
		sem <- struct{}{}
		go func(t int) {
			defer wg.Done()
			defer func() { <-sem }()

			// Each trial gets a deterministic RNG derived from baseSeed + trial index.
			rng := rand.New(rand.NewPCG(baseSeed+uint64(t), uint64(t)))

			bestOps, bestSwaps := runTrial(ops, n, dist, adj, opts, rng)
			results[t] = trialResult{ops: bestOps, swaps: bestSwaps}
		}(trial)
	}
	wg.Wait()

	// Pick the trial with fewest SWAPs.
	bestIdx := 0
	bestSwaps := math.MaxInt
	for i, r := range results {
		if r.swaps < bestSwaps {
			bestSwaps = r.swaps
			bestIdx = i
		}
	}

	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), results[bestIdx].ops, c.Metadata()), nil
}

// runTrial runs one full bidirectional SABRE trial.
//
// Bidirectional iteration (Li et al., arXiv:1809.02573, Section IV-B) improves
// routing quality by alternating forward and backward passes over the circuit.
// Each forward pass routes the circuit front-to-back, producing a final layout.
// The subsequent backward pass routes the reversed circuit starting from that
// final layout, effectively refining the initial layout for the next forward
// pass. Over multiple iterations, the initial layout converges toward one that
// naturally reduces SWAP count. Both forward and backward results are kept,
// and the best (fewest SWAPs) across all passes is returned.
func runTrial(ops []ir.Operation, n int, dist [][]int, adj map[int][]int,
	opts Options, rng *rand.Rand) ([]ir.Operation, int) {

	layout := RandomLayout(n, len(dist), rng)

	var bestOps []ir.Operation
	bestSwaps := math.MaxInt

	for iter := range opts.BidirectionalIters {
		_ = iter

		// Forward pass.
		fwdDAG := newDAG(ops, n, false)
		fwdOps, fwdLayout, fwdSwaps := sabrePass(fwdDAG, dist, adj, layout, opts, rng)
		if fwdSwaps < bestSwaps {
			bestSwaps = fwdSwaps
			bestOps = fwdOps
		}

		// Backward pass: use forward's final layout.
		bwdDAG := newDAG(ops, n, true)
		bwdOps, bwdLayout, bwdSwaps := sabrePass(bwdDAG, dist, adj, fwdLayout, opts, rng)
		if bwdSwaps < bestSwaps {
			bestSwaps = bwdSwaps
			// Reverse backward ops to get forward order.
			for i, j := 0, len(bwdOps)-1; i < j; i, j = i+1, j-1 {
				bwdOps[i], bwdOps[j] = bwdOps[j], bwdOps[i]
			}
			bestOps = bwdOps
		}

		// Use backward's final layout for next iteration (layout convergence).
		layout = bwdLayout
	}

	return bestOps, bestSwaps
}

// countSwaps counts SWAP gates in an operation list.
func countSwaps(ops []ir.Operation) int {
	count := 0
	for _, op := range ops {
		if op.Gate != nil && op.Gate.Name() == "SWAP" {
			count++
		}
	}
	return count
}
