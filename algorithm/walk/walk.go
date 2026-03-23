// Package walk implements discrete-time quantum walks on a line.
//
// A discrete-time quantum walk uses a coin (internal) degree of freedom
// and a position (external) degree of freedom. At each step, a coin
// operator (Hadamard) is applied, then a conditional shift moves the
// walker left or right depending on the coin state. The resulting
// probability distribution is markedly different from a classical
// random walk: it spreads ballistically (linear in steps) rather than
// diffusively (square root of steps).
package walk

import (
	"context"
	"fmt"
	"math"
	"math/cmplx"
)

// Config specifies the quantum walk parameters.
type Config struct {
	// Steps is the number of discrete time steps to simulate.
	Steps int
}

// Result holds the quantum walk output.
type Result struct {
	// Classical is the probability distribution for a symmetric classical
	// random walk. Length is 2*Steps+1, where index i corresponds to
	// position (i - Steps).
	Classical []float64
	// Quantum is the probability distribution for a discrete-time quantum
	// walk using a Hadamard coin. Same indexing as Classical.
	Quantum []float64
}

// Run executes a discrete-time quantum walk on a line.
//
// The quantum walk uses a Hadamard coin and starts in state |R>|0>,
// meaning the walker begins at position 0 with the coin in the |R>
// (right-moving) state.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.Steps < 0 {
		return nil, fmt.Errorf("walk: steps must be >= 0, got %d", cfg.Steps)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	size := 2*cfg.Steps + 1
	classical := classicalWalk(cfg.Steps, size)
	quantum := quantumWalk(cfg.Steps, size)

	return &Result{
		Classical: classical,
		Quantum:   quantum,
	}, nil
}

// classicalWalk computes the probability distribution for a symmetric
// classical random walk on a line after the given number of steps.
//
// At each step, the walker moves left or right with equal probability 1/2.
// After n steps, the probability of being at position k (where k has the
// same parity as n) is C(n, (n+k)/2) / 2^n.
func classicalWalk(steps, size int) []float64 {
	dist := make([]float64, size)
	if steps == 0 {
		dist[steps] = 1.0 // position 0 maps to index steps
		return dist
	}

	// Use Pascal's triangle to compute binomial coefficients stably.
	// P(position = k) = C(n, (n+k)/2) * (1/2)^n
	// Only positions with same parity as n are reachable.
	binom := make([]float64, steps+1)
	binom[0] = 1.0
	for i := 1; i <= steps; i++ {
		// Update in-place from right to left.
		for j := i; j >= 1; j-- {
			binom[j] = binom[j] + binom[j-1]
		}
	}

	scale := math.Pow(0.5, float64(steps))
	for k := -steps; k <= steps; k++ {
		// Position k is reachable only if k and steps have the same parity.
		if (steps+k)%2 != 0 {
			continue
		}
		idx := (steps + k) / 2 // number of right steps
		dist[k+steps] = binom[idx] * scale
	}
	return dist
}

// quantumWalk computes the probability distribution for a discrete-time
// quantum walk on a line using a Hadamard coin.
//
// State representation: the walker has a 2D coin space (|L>, |R>) and
// an integer position. The full state is a vector of (left, right)
// amplitudes at each position.
//
// The Hadamard coin operator is:
//
//	H = (1/sqrt(2)) * [[1, 1], [1, -1]]
//
// The shift operator moves |L> one position left and |R> one position right:
//
//	S|L>|x> = |L>|x-1>,  S|R>|x> = |R>|x+1>
//
// Initial state: |R>|0> (coin in right state, position 0).
//
// Boundary-amplitude note: the position grid spans [-steps, +steps], which
// is large enough to contain every reachable site after `steps` time steps
// (the walker moves at most one position per step). Therefore the boundary
// conditions (pos > 0 and pos < size-1 guards in the shift) are never
// triggered for reachable amplitudes, and no probability is lost for the
// |R>|0> initial state.
func quantumWalk(steps, size int) []float64 {
	if steps == 0 {
		dist := make([]float64, size)
		dist[steps] = 1.0
		return dist
	}

	inv := 1.0 / math.Sqrt2

	// Amplitudes: left[i] and right[i] are the amplitudes for the
	// |L> and |R> coin states at position (i - steps).
	left := make([]complex128, size)
	right := make([]complex128, size)

	// Initial state: |R> at position 0 (index = steps).
	right[steps] = 1.0

	for range steps {
		newLeft := make([]complex128, size)
		newRight := make([]complex128, size)

		for pos := range size {
			l := left[pos]
			r := right[pos]

			// Apply Hadamard coin: H|L> = (|L> + |R>)/sqrt(2)
			//                      H|R> = (|L> - |R>)/sqrt(2)
			coinL := complex(inv, 0) * (l + r)
			coinR := complex(inv, 0) * (l - r)

			// Apply shift: |L> moves left (pos-1), |R> moves right (pos+1).
			if pos > 0 {
				newLeft[pos-1] += coinL
			}
			if pos < size-1 {
				newRight[pos+1] += coinR
			}
		}

		left = newLeft
		right = newRight
	}

	// Compute probability distribution.
	dist := make([]float64, size)
	for i := range size {
		pL := cmplx.Abs(left[i])
		pR := cmplx.Abs(right[i])
		dist[i] = pL*pL + pR*pR
	}
	return dist
}
