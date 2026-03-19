package textbook

import (
	"context"
	"fmt"
	"math/bits"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// SimonOracle applies Simon's oracle to the circuit.
// It receives the builder, input qubit indices [0..n-1],
// and output qubit indices [n..2n-1].
type SimonOracle func(b *builder.Builder, inputs, outputs []int)

// SimonConfig specifies Simon's problem.
type SimonConfig struct {
	// NumQubits is the number of bits in the domain (n).
	NumQubits int
	// Oracle implements f: {0,1}^n -> {0,1}^n with f(x) = f(x XOR s).
	Oracle SimonOracle
	// MaxRounds is the maximum number of sampling rounds. Default: 10*NumQubits.
	MaxRounds int
	// Shots is the number of measurement shots per round. Default: 1024.
	Shots int
}

// SimonResult holds Simon's algorithm output.
type SimonResult struct {
	Circuit   *ir.Circuit // last round's circuit
	Period    int         // recovered period s
	Equations []int       // collected y values
}

func (c *SimonConfig) maxRounds() int {
	if c.MaxRounds > 0 {
		return c.MaxRounds
	}
	return 10 * c.NumQubits
}

func (c *SimonConfig) shots() int {
	if c.Shots > 0 {
		return c.Shots
	}
	return 1024
}

// Simon runs Simon's algorithm to find the period s of a function
// f: {0,1}^n -> {0,1}^n satisfying f(x) = f(x XOR s).
//
// The algorithm uses 2n qubits (n input + n output). Each round
// produces a value y such that y·s = 0 mod 2. After collecting
// n-1 linearly independent equations, the period is recovered
// by Gaussian elimination over GF(2).
func Simon(ctx context.Context, cfg SimonConfig) (*SimonResult, error) {
	n := cfg.NumQubits
	if n < 1 {
		return nil, fmt.Errorf("simon: numQubits must be >= 1")
	}
	if cfg.Oracle == nil {
		return nil, fmt.Errorf("simon: oracle is required")
	}

	inputs := make([]int, n)
	outputs := make([]int, n)
	for i := range n {
		inputs[i] = i
		outputs[i] = n + i
	}

	var equations []int
	var lastCirc *ir.Circuit

	for range cfg.maxRounds() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		nTotal := 2 * n
		b := builder.New("Simon", nTotal)

		// Step 1: Hadamard on all input qubits.
		for q := range n {
			b.H(q)
		}

		// Step 2: Apply oracle.
		cfg.Oracle(b, inputs, outputs)

		// Step 3: Hadamard on all input qubits.
		for q := range n {
			b.H(q)
		}

		// Step 4: Measure input qubits only.
		b.WithClbits(n)
		for q := range n {
			b.Measure(q, q)
		}

		circ, err := b.Build()
		if err != nil {
			return nil, fmt.Errorf("simon: circuit build: %w", err)
		}
		lastCirc = circ

		sim := statevector.New(nTotal)
		counts, err := sim.Run(circ, cfg.shots())
		if err != nil {
			return nil, fmt.Errorf("simon: simulation: %w", err)
		}

		// Extract input register and collect distinct non-zero y values.
		inputCounts := extractInputCounts(counts, n)
		for bs := range inputCounts {
			y := parseSecret(bs)
			if y == 0 {
				continue
			}
			// Add if linearly independent of existing equations.
			if isLinearlyIndependent(equations, y) {
				equations = append(equations, y)
			}
		}

		// We need n-1 linearly independent equations to solve for s.
		if len(equations) >= n-1 {
			break
		}
	}

	// Solve the system y·s = 0 mod 2 for s.
	period := gf2Solve(equations, n)

	return &SimonResult{
		Circuit:   lastCirc,
		Period:    period,
		Equations: equations,
	}, nil
}

// isLinearlyIndependent checks whether y is linearly independent
// of the existing equations over GF(2).
func isLinearlyIndependent(equations []int, y int) bool {
	// Reduce y using the existing equations.
	reduced := y
	for _, eq := range equations {
		msb := bits.Len(uint(eq)) - 1
		if reduced&(1<<msb) != 0 {
			reduced ^= eq
		}
	}
	return reduced != 0
}

// gf2Solve solves the system of equations y·s = 0 mod 2 using
// Gaussian elimination over GF(2). Returns the non-trivial
// solution s, or 0 if only the trivial solution exists.
func gf2Solve(equations []int, n int) int {
	if len(equations) == 0 {
		return 0
	}

	// Build the matrix: each equation is a row of n bits.
	nEq := len(equations)
	matrix := make([]int, nEq)
	copy(matrix, equations)

	// Forward elimination: row echelon form.
	pivotRow := 0
	pivotCols := make([]int, n) // pivotCols[col] = row with pivot, -1 if free
	for i := range n {
		pivotCols[i] = -1
	}

	for col := n - 1; col >= 0; col-- {
		// Find a row with a 1 in this column.
		found := -1
		for row := pivotRow; row < nEq; row++ {
			if matrix[row]&(1<<col) != 0 {
				found = row
				break
			}
		}
		if found == -1 {
			continue // free variable
		}

		// Swap to pivot position.
		matrix[pivotRow], matrix[found] = matrix[found], matrix[pivotRow]
		pivotCols[col] = pivotRow

		// Eliminate this column from all other rows.
		for row := range nEq {
			if row != pivotRow && matrix[row]&(1<<col) != 0 {
				matrix[row] ^= matrix[pivotRow]
			}
		}
		pivotRow++
	}

	// Back-substitution: find a free variable and set it to 1.
	// The solution s has y·s = 0 for all collected y.
	s := 0
	for col := range n {
		if pivotCols[col] == -1 {
			// Free variable - set it to 1 and propagate.
			s |= 1 << col
			for otherCol := range n {
				if pivotCols[otherCol] >= 0 {
					row := pivotCols[otherCol]
					if matrix[row]&(1<<col) != 0 {
						s |= 1 << otherCol
					}
				}
			}
			break // one free variable gives one non-trivial solution
		}
	}

	return s
}

// TwoToOneOracle creates a Simon oracle for the given secret period.
// It implements a 2-to-1 function f with f(x) = f(x XOR secret).
//
// Construction:
//  1. Copy inputs to outputs via CNOT: output = x.
//  2. Let h be the highest set bit of secret.
//  3. For each bit j set in secret, apply CNOT(input[h], output[j]).
//
// This gives f(x) = x when x_h = 0, and f(x) = x XOR secret when x_h = 1,
// ensuring f(x) = f(x XOR secret) for all x.
func TwoToOneOracle(secret, numQubits int) SimonOracle {
	return func(b *builder.Builder, inputs, outputs []int) {
		// Step 1: Copy inputs to outputs.
		for i := range len(inputs) {
			b.CNOT(inputs[i], outputs[i])
		}

		if secret == 0 {
			return // 1-to-1 function, no additional operations
		}

		// Step 2: Find the highest set bit of secret.
		h := bits.Len(uint(secret)) - 1

		// Step 3: For each bit j set in secret, CNOT(input[h], output[j]).
		for j := range numQubits {
			if secret&(1<<j) != 0 {
				b.CNOT(inputs[h], outputs[j])
			}
		}
	}
}
