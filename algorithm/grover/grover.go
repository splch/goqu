// Package grover implements Grover's quantum search algorithm.
//
// The algorithm amplifies the probability of marked states using an
// oracle and a diffusion operator. For N states with M solutions,
// the optimal number of iterations is approximately π/4 * √(N/M).
package grover

import (
	"context"
	"fmt"
	"math"

	"github.com/splch/goqu/algorithm/internal/algoutil"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// Oracle marks target states by flipping their phase.
// It receives a builder and the working qubit indices.
type Oracle func(b *builder.Builder, qubits []int)

// Config specifies the Grover search parameters.
type Config struct {
	// NumQubits is the number of search qubits.
	NumQubits int
	// Oracle marks the target states.
	Oracle Oracle
	// NumIters is the number of Grover iterations.
	// 0 means optimal: floor(π/4 * √(2^n / M)).
	NumIters int
	// NumSolutions is the number of marked states (M). Default: 1.
	NumSolutions int
	// Shots is the number of measurement shots. Default: 1024.
	Shots int
}

// Result holds Grover search output.
type Result struct {
	Circuit   *ir.Circuit
	Counts    map[string]int
	TopResult string
	NumIters  int
}

func (c *Config) numSolutions() int {
	if c.NumSolutions > 0 {
		return c.NumSolutions
	}
	return 1
}

func (c *Config) shots() int {
	if c.Shots > 0 {
		return c.Shots
	}
	return 1024
}

// Run executes Grover's search algorithm.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.NumQubits < 1 {
		return nil, fmt.Errorf("grover: numQubits must be >= 1")
	}
	if cfg.Oracle == nil {
		return nil, fmt.Errorf("grover: oracle is required")
	}

	n := cfg.NumQubits
	m := cfg.numSolutions()
	nIter := cfg.NumIters
	if nIter <= 0 {
		// Optimal iteration count.
		nIter = max(1, int(math.Floor(math.Pi/4*math.Sqrt(float64(int(1)<<n)/float64(m)))))
	}

	b := builder.New("Grover", n)

	// Initial superposition.
	for q := range n {
		b.H(q)
	}

	qubits := make([]int, n)
	for i := range n {
		qubits[i] = i
	}

	// Grover iterations.
	for range nIter {
		// Oracle.
		cfg.Oracle(b, qubits)

		// Diffusion operator: 2|s⟩⟨s| - I
		// = H^⊗n · (2|0⟩⟨0| - I) · H^⊗n
		for q := range n {
			b.H(q)
		}
		for q := range n {
			b.X(q)
		}

		// Multi-controlled Z: phase flip on |11...1⟩.
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
	}

	b.MeasureAll()

	circ, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("grover: circuit build: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sim := statevector.New(n)
	counts, err := sim.Run(circ, cfg.shots())
	if err != nil {
		return nil, fmt.Errorf("grover: simulation: %w", err)
	}

	top := algoutil.TopKey(counts, 0)

	return &Result{
		Circuit:   circ,
		Counts:    counts,
		TopResult: top,
		NumIters:  nIter,
	}, nil
}

// PhaseOracle creates an oracle that marks specific target states by index.
// Each target is an integer representing a computational basis state.
func PhaseOracle(targets []int, numQubits int) Oracle {
	return func(b *builder.Builder, qubits []int) {
		for _, target := range targets {
			// Apply X gates to set up the target pattern.
			for i, q := range qubits {
				if target&(1<<i) == 0 {
					b.X(q)
				}
			}
			// Multi-controlled Z.
			if len(qubits) == 1 {
				b.Z(qubits[0])
			} else {
				controls := make([]int, len(qubits)-1)
				copy(controls, qubits[:len(qubits)-1])
				b.MCZ(controls, qubits[len(qubits)-1])
			}
			// Undo X gates.
			for i, q := range qubits {
				if target&(1<<i) == 0 {
					b.X(q)
				}
			}
		}
	}
}

// BooleanOracle creates an oracle from a classical boolean function.
// The function f takes a bit pattern (as int) and returns true for marked states.
// An ancilla qubit is added automatically.
func BooleanOracle(f func(int) bool, numQubits int) Oracle {
	// Use phase kickback with an ancilla in |-⟩ state.
	// For simplicity, decompose into PhaseOracle by enumerating.
	var targets []int
	for i := range 1 << numQubits {
		if f(i) {
			targets = append(targets, i)
		}
	}
	return PhaseOracle(targets, numQubits)
}
