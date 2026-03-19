// Package textbook implements textbook quantum algorithms:
// Bernstein-Vazirani, Deutsch-Jozsa, and Simon's algorithm.
package textbook

import (
	"context"
	"fmt"
	"strconv"

	"github.com/splch/goqu/algorithm/internal/algoutil"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// BVConfig specifies the Bernstein-Vazirani problem.
type BVConfig struct {
	// Secret is the hidden bitstring s. The oracle computes f(x) = s·x mod 2.
	Secret int
	// NumQubits is the number of input qubits.
	NumQubits int
	// Shots is the number of measurement shots. Default: 1024.
	Shots int
}

// BVResult holds the Bernstein-Vazirani output.
type BVResult struct {
	Circuit *ir.Circuit
	Counts  map[string]int
	Secret  int
}

func (c *BVConfig) shots() int {
	if c.Shots > 0 {
		return c.Shots
	}
	return 1024
}

// BernsteinVazirani runs the Bernstein-Vazirani algorithm to recover
// a secret string s from the oracle f(x) = s·x mod 2.
//
// The algorithm uses n+1 qubits (n input + 1 ancilla) and determines
// the secret in a single query.
func BernsteinVazirani(ctx context.Context, cfg BVConfig) (*BVResult, error) {
	n := cfg.NumQubits
	if n < 1 {
		return nil, fmt.Errorf("bernstein-vazirani: numQubits must be >= 1")
	}
	if cfg.Secret < 0 || cfg.Secret >= (1<<n) {
		return nil, fmt.Errorf("bernstein-vazirani: secret %d out of range [0, %d)", cfg.Secret, 1<<n)
	}

	nTotal := n + 1 // n input qubits + 1 ancilla
	b := builder.New("BernsteinVazirani", nTotal)

	// Step 1: Prepare ancilla (qubit n) in |-> = H|1>.
	b.X(n)
	b.H(n)

	// Step 2: Hadamard on all input qubits.
	for q := range n {
		b.H(q)
	}

	// Step 3: Oracle - for each bit i set in secret, CNOT(i, ancilla).
	for i := range n {
		if cfg.Secret&(1<<i) != 0 {
			b.CNOT(i, n)
		}
	}

	// Step 4: Hadamard on all input qubits.
	for q := range n {
		b.H(q)
	}

	// Step 5: Measure input qubits only.
	b.WithClbits(n)
	for q := range n {
		b.Measure(q, q)
	}

	circ, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("bernstein-vazirani: circuit build: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sim := statevector.New(nTotal)
	counts, err := sim.Run(circ, cfg.shots())
	if err != nil {
		return nil, fmt.Errorf("bernstein-vazirani: simulation: %w", err)
	}

	// Extract the input register from the full bitstring.
	// The full bitstring has length nTotal; the rightmost n characters
	// correspond to input qubits 0..n-1.
	inputCounts := extractInputCounts(counts, n)
	top := algoutil.TopKey(inputCounts, 0)
	secret := parseSecret(top)

	return &BVResult{
		Circuit: circ,
		Counts:  inputCounts,
		Secret:  secret,
	}, nil
}

// extractInputCounts reduces full-length bitstrings to the last n characters
// (corresponding to the lowest-indexed input qubits) and aggregates counts.
func extractInputCounts(counts map[string]int, n int) map[string]int {
	result := make(map[string]int)
	for k, v := range counts {
		inputBS := k
		if len(k) > n {
			inputBS = k[len(k)-n:]
		}
		result[inputBS] += v
	}
	return result
}

// parseSecret converts an MSB-first bitstring to an integer.
// For n qubits, the string "101" means q2=1, q1=0, q0=1 = 5.
func parseSecret(bs string) int {
	val, _ := strconv.ParseInt(bs, 2, 64)
	return int(val)
}
