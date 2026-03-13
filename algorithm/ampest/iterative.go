package ampest

import (
	"context"
	"fmt"
	"math"

	"github.com/splch/goqu/algorithm/grover"
	"github.com/splch/goqu/algorithm/internal/algoutil"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// IterativeConfig specifies parameters for Iterative Amplitude Estimation.
type IterativeConfig struct {
	// StatePrep is the circuit A that prepares the state.
	StatePrep *ir.Circuit
	// Oracle marks good states by flipping their phase.
	Oracle grover.Oracle
	// NumQubits is the number of working qubits.
	NumQubits int
	// MaxIters is the maximum number of doubling rounds. Default: 10.
	MaxIters int
	// ConfLevel is the confidence level (unused in statevector mode,
	// reserved for shot-based implementations). Default: 0.05.
	ConfLevel float64
	// Shots is the number of measurement shots (unused in the current
	// statevector-based implementation). Default: 1024.
	Shots int
}

// IterativeResult holds the output of Iterative Amplitude Estimation.
type IterativeResult struct {
	// Amplitude is the estimated amplitude a.
	Amplitude float64
	// Probability is a^2.
	Probability float64
	// NumIters is the number of refinement rounds performed.
	NumIters int
	// ConfInterval is a conservative [lo, hi] confidence interval on the amplitude.
	ConfInterval [2]float64
}

func (c *IterativeConfig) maxIters() int {
	if c.MaxIters > 0 {
		return c.MaxIters
	}
	return 10
}

// RunIterative executes Iterative Amplitude Estimation.
//
// The algorithm uses statevector simulation to compute the "good" subspace
// probability at increasing Grover powers. For k=0, it runs A alone; for
// k=2^i, it runs A*Q^k. The good-state probability follows
// P_good(k) = sin^2((2k+1)*theta) where a = sin(theta) is the target
// amplitude. Higher Grover powers amplify small differences in theta,
// enabling progressively finer estimates.
//
// Good states are identified by applying the oracle to the statevector
// and detecting which amplitudes change sign.
func RunIterative(ctx context.Context, cfg IterativeConfig) (*IterativeResult, error) {
	if cfg.StatePrep == nil {
		return nil, fmt.Errorf("ampest: state prep circuit is required")
	}
	if cfg.Oracle == nil {
		return nil, fmt.Errorf("ampest: oracle is required")
	}
	if cfg.NumQubits < 1 {
		return nil, fmt.Errorf("ampest: numQubits must be >= 1")
	}

	n := cfg.NumQubits
	maxIters := cfg.maxIters()
	idMap := algoutil.IdentityMap(n)

	// Identify which computational basis states the oracle marks as "good".
	// Apply the oracle to each basis state and check for a phase flip.
	goodMask, err := identifyGoodStates(cfg.Oracle, n)
	if err != nil {
		return nil, fmt.Errorf("ampest: identify good states: %w", err)
	}

	Q, err := buildGroverIterate(cfg.StatePrep, cfg.Oracle, n)
	if err != nil {
		return nil, fmt.Errorf("ampest: grover iterate: %w", err)
	}

	// k=0: run state preparation alone.
	pGood0, err := measureGoodProb(cfg.StatePrep, nil, n, idMap, goodMask)
	if err != nil {
		return nil, fmt.Errorf("ampest: base measurement: %w", err)
	}

	// Initial estimate: P_good(0) = sin^2(theta) => theta = arcsin(sqrt(P_good)).
	theta := math.Asin(clamp(math.Sqrt(clamp(pGood0))))

	// Refine theta using exponentially increasing Grover powers.
	for i := range maxIters {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		k := 1 << i // Grover power: 1, 2, 4, 8, ...
		m := float64(2*k + 1)

		qk, err := ir.Repeat(Q, k)
		if err != nil {
			return nil, fmt.Errorf("ampest: repeat Q^%d: %w", k, err)
		}

		pGood, err := measureGoodProb(cfg.StatePrep, qk, n, idMap, goodMask)
		if err != nil {
			return nil, fmt.Errorf("ampest: measurement iter %d: %w", i, err)
		}

		// P_good(k) = sin^2(m*theta) where m = 2k+1.
		// Solve: m*theta = arcsin(sqrt(P)) + j*pi  (branch A)
		//    or  m*theta = pi - arcsin(sqrt(P)) + j*pi  (branch B)
		// Find the candidate theta closest to our current estimate.
		theta = resolveTheta(pGood, m, theta)
	}

	amp := math.Sin(theta)

	// Conservative confidence interval that shrinks with iterations.
	halfWidth := math.Pi / (2 * float64(int(1)<<maxIters))
	lo := math.Max(0, math.Sin(theta-halfWidth))
	hi := math.Min(1, math.Sin(theta+halfWidth))

	return &IterativeResult{
		Amplitude:    amp,
		Probability:  amp * amp,
		NumIters:     maxIters,
		ConfInterval: [2]float64{lo, hi},
	}, nil
}

// identifyGoodStates determines which computational basis states the oracle
// marks by building a circuit that applies the oracle to a uniform
// superposition and comparing the resulting statevector to the original.
// Returns a boolean slice where true means the state is "good" (phase-flipped).
func identifyGoodStates(oracle grover.Oracle, n int) ([]bool, error) {
	nStates := 1 << n
	qubits := make([]int, n)
	for i := range n {
		qubits[i] = i
	}

	// Build a circuit that puts all qubits in H state (uniform superposition)
	// so all amplitudes are real and positive.
	bBefore := builder.New("before", n)
	for q := range n {
		bBefore.H(q)
	}
	circBefore, err := bBefore.Build()
	if err != nil {
		return nil, err
	}

	// Build same circuit with oracle appended.
	bAfter := builder.New("after", n)
	for q := range n {
		bAfter.H(q)
	}
	oracle(bAfter, qubits)
	circAfter, err := bAfter.Build()
	if err != nil {
		return nil, err
	}

	// Evolve both.
	simBefore := statevector.New(n)
	if err := simBefore.Evolve(circBefore); err != nil {
		return nil, err
	}
	svBefore := simBefore.StateVector()

	simAfter := statevector.New(n)
	if err := simAfter.Evolve(circAfter); err != nil {
		return nil, err
	}
	svAfter := simAfter.StateVector()

	// States where the oracle flipped the phase have opposite sign.
	good := make([]bool, nStates)
	const eps = 1e-10
	for i := range nStates {
		// A phase flip means svAfter[i] ~ -svBefore[i].
		diff := svAfter[i] + svBefore[i] // sum is ~0 if flipped
		if real(diff)*real(diff)+imag(diff)*imag(diff) < eps {
			good[i] = true
		}
	}
	return good, nil
}

// measureGoodProb builds and evolves the circuit StatePrep + extra (if non-nil),
// then returns the total probability of "good" states.
func measureGoodProb(statePrep, extra *ir.Circuit, n int, idMap map[int]int, goodMask []bool) (float64, error) {
	b := builder.New("meas", n)
	b.Compose(statePrep, idMap)
	if extra != nil {
		b.Compose(extra, idMap)
	}
	circ, err := b.Build()
	if err != nil {
		return 0, err
	}

	sim := statevector.New(n)
	if err := sim.Evolve(circ); err != nil {
		return 0, err
	}
	sv := sim.StateVector()

	var pGood float64
	for i, amp := range sv {
		if i < len(goodMask) && goodMask[i] {
			pGood += real(amp)*real(amp) + imag(amp)*imag(amp)
		}
	}
	return pGood, nil
}

// resolveTheta finds the theta in [0, pi/2] that satisfies
// sin^2(m*theta) = pGood and is closest to the current estimate.
//
// sin^2(m*theta) = P means m*theta = arcsin(sqrt(P)) + j*pi
// or m*theta = pi - arcsin(sqrt(P)) + j*pi for integer j.
// We enumerate candidates and pick the one closest to thetaCur.
func resolveTheta(pGood, m, thetaCur float64) float64 {
	base := math.Asin(clamp(math.Sqrt(clamp(pGood))))

	bestTheta := thetaCur
	bestDist := math.MaxFloat64

	// Enumerate branch A: m*theta = base + j*pi
	// and branch B: m*theta = pi - base + j*pi
	// theta must be in [0, pi/2], so m*theta in [0, m*pi/2].
	maxMTheta := m * math.Pi / 2
	for _, branchBase := range []float64{base, math.Pi - base} {
		for j := -1; j <= int(maxMTheta/math.Pi)+1; j++ {
			mTheta := branchBase + float64(j)*math.Pi
			candidate := mTheta / m
			if candidate < -0.01 || candidate > math.Pi/2+0.01 {
				continue
			}
			// Clamp to valid range.
			candidate = math.Max(0, math.Min(math.Pi/2, candidate))
			dist := math.Abs(candidate - thetaCur)
			if dist < bestDist {
				bestDist = dist
				bestTheta = candidate
			}
		}
	}
	return bestTheta
}

// clamp restricts v to [0, 1].
func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
