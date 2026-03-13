// Package counting implements Quantum Approximate Counting.
//
// The algorithm combines Grover's search operator with Quantum Phase
// Estimation to estimate the number of solutions M in a search space
// of size N = 2^n. The Grover iterate Q has eigenvalues e^{±2iθ}
// where sin²(θ) = M/N, so QPE on Q yields θ from which M is recovered.
package counting

import (
	"context"
	"fmt"
	"math"

	"github.com/splch/goqu/algorithm/grover"
	"github.com/splch/goqu/algorithm/internal/algoutil"
	"github.com/splch/goqu/algorithm/qpe"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// Config specifies the Quantum Approximate Counting parameters.
type Config struct {
	// NumQubits is the number of search space qubits.
	NumQubits int
	// Oracle marks the target states.
	Oracle grover.Oracle
	// NumPhaseBits is the number of precision bits for phase estimation.
	NumPhaseBits int
	// Shots is the number of measurement shots. Default: 1024.
	Shots int
}

// Result holds the counting output.
type Result struct {
	// Circuit is the full quantum circuit used.
	Circuit *ir.Circuit
	// Count is the estimated number of solutions.
	Count float64
	// Phase is the raw phase θ extracted from QPE.
	Phase float64
	// Counts is the measurement histogram.
	Counts map[string]int
}

func (c *Config) shots() int {
	if c.Shots > 0 {
		return c.Shots
	}
	return 1024
}

// Run executes Quantum Approximate Counting.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.NumQubits < 1 {
		return nil, fmt.Errorf("counting: numQubits must be >= 1")
	}
	if cfg.NumPhaseBits < 1 {
		return nil, fmt.Errorf("counting: numPhaseBits must be >= 1")
	}
	if cfg.Oracle == nil {
		return nil, fmt.Errorf("counting: oracle is required")
	}

	n := cfg.NumQubits
	nPhase := cfg.NumPhaseBits
	nTotal := nPhase + n

	// Build the Grover iterate circuit on n qubits.
	groverCirc, err := buildGroverIterate(cfg.Oracle, n)
	if err != nil {
		return nil, fmt.Errorf("counting: grover iterate: %w", err)
	}

	b := builder.New("Counting", nTotal)

	// Prepare target register in equal superposition.
	for q := nPhase; q < nTotal; q++ {
		b.H(q)
	}

	// Hadamard on all phase qubits.
	for q := range nPhase {
		b.H(q)
	}

	// Controlled-Q^(2^k) applications.
	for k := range nPhase {
		power := 1 << (nPhase - 1 - k)

		// Build Q^power circuit.
		qPow, err := ir.Repeat(groverCirc, power)
		if err != nil {
			return nil, fmt.Errorf("counting: repeat: %w", err)
		}

		// Apply each gate from Q^power as controlled with phase qubit k.
		for _, op := range qPow.Ops() {
			if op.Gate == nil || op.Gate.Name() == "barrier" {
				continue
			}
			// Remap target qubits into the target register.
			targetQubits := make([]int, len(op.Qubits))
			for i, q := range op.Qubits {
				targetQubits[i] = nPhase + q
			}
			b.Ctrl(op.Gate, []int{k}, targetQubits...)
		}
	}

	// Inverse QFT on phase register.
	qpe.ApplyInverseQFT(b, nPhase)

	// Measure phase qubits only.
	b.WithClbits(nPhase)
	for q := range nPhase {
		b.Measure(q, q)
	}

	circ, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("counting: circuit build: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sim := statevector.New(nTotal)
	counts, err := sim.Run(circ, cfg.shots())
	if err != nil {
		return nil, fmt.Errorf("counting: simulation: %w", err)
	}

	// Extract phase from the most common measurement.
	topBS := algoutil.TopKey(counts, nPhase)
	phi := algoutil.BitstringToPhase(topBS, nPhase)

	// The diffusion operator as implemented is (I - 2|s⟩⟨s|) rather than
	// (2|s⟩⟨s| - I), which shifts the Grover eigenvalue by a factor of -1.
	// This adds 0.5 to the measured phase, so the Grover angle θ satisfies
	// φ = 0.5 ± θ/π. Recover θ and compute Count = N · sin²(θ).
	theta := math.Pi * math.Abs(phi-0.5)

	bigN := float64(int(1) << n)
	count := bigN * math.Sin(theta) * math.Sin(theta)

	return &Result{
		Circuit: circ,
		Count:   count,
		Phase:   phi,
		Counts:  counts,
	}, nil
}

// buildGroverIterate constructs a single Grover iterate Q = D · O
// (oracle followed by diffusion) on n qubits.
func buildGroverIterate(oracle grover.Oracle, n int) (*ir.Circuit, error) {
	b := builder.New("Q", n)
	qubits := make([]int, n)
	for i := range n {
		qubits[i] = i
	}

	// Oracle.
	oracle(b, qubits)

	// Diffusion: H X MCZ X H
	for q := range n {
		b.H(q)
	}
	for q := range n {
		b.X(q)
	}
	if n == 1 {
		b.Z(0)
	} else {
		controls := make([]int, n-1)
		for i := range n - 1 {
			controls[i] = i
		}
		b.MCZ(controls, n-1)
	}
	for q := range n {
		b.X(q)
	}
	for q := range n {
		b.H(q)
	}

	return b.Build()
}

