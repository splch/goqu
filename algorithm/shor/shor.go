// Package shor implements Shor's quantum factoring algorithm.
//
// Given a composite integer N, the algorithm finds a non-trivial factor
// by using quantum phase estimation to determine the period of modular
// exponentiation. For small N (suitable for simulation), it builds the
// full order-finding circuit and extracts factors classically.
package shor

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"

	"github.com/splch/goqu/algorithm/internal/algoutil"
	"github.com/splch/goqu/algorithm/qpe"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// Config specifies the Shor factoring parameters.
type Config struct {
	// N is the number to factor.
	N int
	// NumPhaseBits is the precision bits for phase estimation.
	// 0 means auto: 2*ceil(log2(N)).
	NumPhaseBits int
	// Shots is the number of measurement shots. Default: 1024.
	Shots int
	// MaxAttempts is the maximum number of random base attempts. Default: 10.
	MaxAttempts int
}

// Result holds the factoring output.
type Result struct {
	Factors  [2]int
	Period   int
	Base     int
	Attempts int
	Circuit  *ir.Circuit
}

func (c *Config) shots() int {
	if c.Shots > 0 {
		return c.Shots
	}
	return 1024
}

func (c *Config) maxAttempts() int {
	if c.MaxAttempts > 0 {
		return c.MaxAttempts
	}
	return 10
}

// Run executes Shor's factoring algorithm.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	N := cfg.N

	// Classical pre-checks.
	if N < 2 {
		return nil, fmt.Errorf("shor: N must be >= 2, got %d", N)
	}
	if isPrime(N) {
		return nil, fmt.Errorf("shor: N=%d is prime", N)
	}
	if isPrimePower(N) {
		return nil, fmt.Errorf("shor: N=%d is a prime power", N)
	}
	if N%2 == 0 {
		return &Result{Factors: [2]int{2, N / 2}, Attempts: 0}, nil
	}

	nTarget := max(1, int(math.Ceil(math.Log2(float64(N)))))
	nPhase := cfg.NumPhaseBits
	if nPhase <= 0 {
		nPhase = 2 * nTarget
	}

	maxAttempts := cfg.maxAttempts()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Pick random base a in [2, N-1].
		a := rand.IntN(N-2) + 2

		// Check if gcd(a, N) > 1 (lucky factor).
		g := gcd(a, N)
		if g > 1 {
			return &Result{
				Factors:  [2]int{g, N / g},
				Base:     a,
				Attempts: attempt,
			}, nil
		}

		// Build quantum order-finding circuit.
		circ, err := buildOrderFindingCircuit(a, N, nPhase, nTarget)
		if err != nil {
			continue
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Run the circuit.
		nTotal := nPhase + nTarget
		sim := statevector.New(nTotal)
		counts, err := sim.Run(circ, cfg.shots())
		if err != nil {
			continue
		}

		// Try all measurement outcomes, weighted by frequency.
		type outcome struct {
			bs    string
			count int
		}
		var outcomes []outcome
		for bs, cnt := range counts {
			outcomes = append(outcomes, outcome{bs, cnt})
		}
		// Sort by count descending (simple selection for small maps).
		for i := range outcomes {
			for j := i + 1; j < len(outcomes); j++ {
				if outcomes[j].count > outcomes[i].count {
					outcomes[i], outcomes[j] = outcomes[j], outcomes[i]
				}
			}
		}

		for _, out := range outcomes {
			// Extract phase register bits (last nPhase chars).
			phaseBS := out.bs
			if len(phaseBS) > nPhase {
				phaseBS = phaseBS[len(phaseBS)-nPhase:]
			}

			// Convert bitstring to phase value.
			phase := algoutil.BitstringToPhase(phaseBS, nPhase)
			if phase < 1e-10 {
				continue
			}

			// Use continued fraction to find period.
			r := continuedFraction(phase, N)
			if r <= 0 || r%2 != 0 {
				continue
			}

			// Check gcd(a^(r/2) +/- 1, N).
			halfPow := modPow(a, r/2, N)

			for _, delta := range []int{-1, 1} {
				candidate := halfPow + delta
				if candidate <= 0 {
					continue
				}
				g := gcd(candidate, N)
				if g > 1 && g < N {
					return &Result{
						Factors:  [2]int{g, N / g},
						Period:   r,
						Base:     a,
						Attempts: attempt,
						Circuit:  circ,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("shor: failed to factor %d after %d attempts", N, maxAttempts)
}

// buildOrderFindingCircuit constructs the quantum subroutine of Shor's
// algorithm: Quantum Phase Estimation (QPE) applied to the modular
// exponentiation unitary U|y> = |a*y mod N>. The eigenvalues of U are
// exp(2*pi*i*s/r) for s = 0,...,r-1, where r is the period of a^x mod N.
// QPE estimates the phase s/r into the nPhase-bit phase register. The
// period r is then extracted classically via continued fraction expansion
// of the measured phase value.
func buildOrderFindingCircuit(a, modulus, nPhase, nTarget int) (*ir.Circuit, error) {
	nTotal := nPhase + nTarget

	b := builder.New("Shor", nTotal)

	// Prepare target register with |1> (set least significant target qubit).
	b.X(nPhase)

	// Hadamard on phase register.
	for q := range nPhase {
		b.H(q)
	}

	// Controlled modular exponentiation.
	for k := range nPhase {
		power := 1 << (nPhase - 1 - k)
		modCirc := modExpCircuit(a, power, modulus, nTarget)

		// Apply controlled version: phase qubit k controls the mod-exp circuit.
		for _, op := range modCirc.Ops() {
			if op.Gate == nil || op.Gate.Name() == "barrier" {
				continue
			}
			targetQubits := make([]int, len(op.Qubits))
			for i, q := range op.Qubits {
				targetQubits[i] = nPhase + q
			}
			b.Ctrl(op.Gate, []int{k}, targetQubits...)
		}
	}

	// Inverse QFT on phase register.
	qpe.ApplyInverseQFT(b, nPhase)

	// Measure phase register.
	b.WithClbits(nPhase)
	for q := range nPhase {
		b.Measure(q, q)
	}

	return b.Build()
}

// continuedFraction extracts the period from a phase measurement.
// phase = s/r where s is some integer. We use continued fraction
// expansion to find the best rational approximation with denominator <= maxDenom.
func continuedFraction(measured float64, maxDenom int) int {
	if measured < 1e-10 {
		return 0
	}
	h0, h1 := 0, 1
	k0, k1 := 1, 0
	x := measured
	for range 50 {
		a := int(math.Floor(x))
		h2 := a*h1 + h0
		k2 := a*k1 + k0
		if k2 > maxDenom {
			break
		}
		h0, h1 = h1, h2
		k0, k1 = k1, k2
		rem := x - float64(a)
		if rem < 1e-10 {
			break
		}
		x = 1.0 / rem
	}
	if k1 <= 0 {
		return 0
	}
	return k1
}

// gcd returns the greatest common divisor of a and b.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// modPow computes base^exp mod mod using binary exponentiation.
func modPow(base, exp, mod int) int {
	result := 1
	base %= mod
	for exp > 0 {
		if exp%2 == 1 {
			result = result * base % mod
		}
		exp /= 2
		base = base * base % mod
	}
	return result
}

// isPrime checks whether n is prime using trial division.
func isPrime(n int) bool {
	if n < 2 {
		return false
	}
	if n < 4 {
		return true
	}
	if n%2 == 0 || n%3 == 0 {
		return false
	}
	for i := 5; i*i <= n; i += 6 {
		if n%i == 0 || n%(i+2) == 0 {
			return false
		}
	}
	return true
}

// isPrimePower checks whether n is a perfect power of a prime (p^k, k >= 2).
func isPrimePower(n int) bool {
	if n < 2 {
		return false
	}
	bn := big.NewInt(int64(n))
	// Check if n = b^k for any b >= 2, k >= 2.
	for k := 2; k <= int(math.Log2(float64(n))); k++ {
		// Binary search for b such that b^k = n.
		lo, hi := 2, n
		for lo <= hi {
			mid := lo + (hi-lo)/2
			power := intPow(mid, k)
			if power == n {
				return true
			}
			if power < n && power > 0 {
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
	}
	// Also check via big.Int sqrt for k=2 specifically.
	root := new(big.Int).Sqrt(bn)
	if new(big.Int).Mul(root, root).Cmp(bn) == 0 && isPrime(int(root.Int64())) {
		return true
	}
	return false
}

// intPow computes base^exp as an integer. Returns -1 on overflow.
func intPow(base, exp int) int {
	result := 1
	for range exp {
		result *= base
		if result > 1<<50 {
			return -1 // overflow sentinel
		}
	}
	return result
}
