package textbook

import (
	"context"
	"fmt"
	"strings"

	"github.com/splch/goqu/algorithm/internal/algoutil"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// DJOracle applies a Deutsch-Jozsa oracle to the circuit.
// It receives the builder, input qubit indices, and the ancilla qubit index.
type DJOracle func(b *builder.Builder, inputs []int, ancilla int)

// DJConfig specifies the Deutsch-Jozsa problem.
type DJConfig struct {
	// NumQubits is the number of input qubits.
	NumQubits int
	// Oracle is the black-box function to test.
	Oracle DJOracle
	// Shots is the number of measurement shots. Default: 1024.
	Shots int
}

// DJResult holds the Deutsch-Jozsa output.
type DJResult struct {
	Circuit    *ir.Circuit
	Counts     map[string]int
	IsConstant bool
}

func (c *DJConfig) shots() int {
	if c.Shots > 0 {
		return c.Shots
	}
	return 1024
}

// DeutschJozsa runs the Deutsch-Jozsa algorithm to determine whether
// a boolean function is constant (same output for all inputs) or
// balanced (returns 0 for half and 1 for the other half).
//
// The algorithm uses n+1 qubits (n input + 1 ancilla) and determines
// the answer in a single query.
func DeutschJozsa(ctx context.Context, cfg DJConfig) (*DJResult, error) {
	n := cfg.NumQubits
	if n < 1 {
		return nil, fmt.Errorf("deutsch-jozsa: numQubits must be >= 1")
	}
	if cfg.Oracle == nil {
		return nil, fmt.Errorf("deutsch-jozsa: oracle is required")
	}

	nTotal := n + 1 // n input qubits + 1 ancilla
	b := builder.New("DeutschJozsa", nTotal)

	// Step 1: Prepare ancilla (qubit n) in |-> = H|1>.
	b.X(n)
	b.H(n)

	// Step 2: Hadamard on all input qubits.
	for q := range n {
		b.H(q)
	}

	// Step 3: Apply oracle.
	inputs := make([]int, n)
	for i := range n {
		inputs[i] = i
	}
	cfg.Oracle(b, inputs, n)

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
		return nil, fmt.Errorf("deutsch-jozsa: circuit build: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sim := statevector.New(nTotal)
	counts, err := sim.Run(circ, cfg.shots())
	if err != nil {
		return nil, fmt.Errorf("deutsch-jozsa: simulation: %w", err)
	}

	// Extract input register bits from full bitstring.
	inputCounts := extractInputCounts(counts, n)
	top := algoutil.TopKey(inputCounts, 0)

	// The function is constant if the measurement is all zeros
	// with high probability (> 90% of shots).
	allZeros := strings.Repeat("0", n)
	isConstant := top == allZeros && inputCounts[allZeros] > cfg.shots()*9/10

	return &DJResult{
		Circuit:    circ,
		Counts:     inputCounts,
		IsConstant: isConstant,
	}, nil
}

// ConstantOracle returns a Deutsch-Jozsa oracle for a constant function.
// value=0: f(x)=0 for all x (does nothing).
// value=1: f(x)=1 for all x (applies X to the ancilla).
func ConstantOracle(value int) DJOracle {
	return func(b *builder.Builder, inputs []int, ancilla int) {
		if value == 1 {
			b.X(ancilla)
		}
	}
}

// BalancedOracle returns a Deutsch-Jozsa oracle for a balanced function.
// The function computes f(x) = popcount(x AND mask) mod 2.
// For each bit i set in mask, a CNOT(input[i], ancilla) is applied.
func BalancedOracle(mask int) DJOracle {
	return func(b *builder.Builder, inputs []int, ancilla int) {
		for i, q := range inputs {
			if mask&(1<<i) != 0 {
				b.CNOT(q, ancilla)
			}
		}
	}
}
