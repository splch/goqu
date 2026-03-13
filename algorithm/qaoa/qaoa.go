// Package qaoa implements the Quantum Approximate Optimization Algorithm.
//
// QAOA solves combinatorial optimization problems by alternating cost
// and mixer layers parameterized by gamma and beta angles.
package qaoa

import (
	"context"
	"fmt"
	"math"

	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// Config specifies the QAOA problem and solver.
type Config struct {
	// CostHamiltonian is the objective to minimize.
	CostHamiltonian pauli.PauliSum
	// MixerHamiltonian is the mixer operator. Nil uses the default X-mixer.
	MixerHamiltonian *pauli.PauliSum
	// Layers is the number of QAOA layers (p). Default: 1.
	Layers int
	// Optimizer is the classical optimization method.
	Optimizer optim.Optimizer
	// InitialParams are the starting [gamma_1..gamma_p, beta_1..beta_p]. Nil means heuristic.
	InitialParams []float64
	// Shots for sampling the best bitstring. Default: 1024.
	Shots int
}

// Result holds QAOA output.
type Result struct {
	OptimalValue  float64
	OptimalParams []float64
	BestBitstring string
	BestCost      float64
	NumIters      int
	Converged     bool
}

func (c *Config) layers() int {
	if c.Layers > 0 {
		return c.Layers
	}
	return 1
}

func (c *Config) shots() int {
	if c.Shots > 0 {
		return c.Shots
	}
	return 1024
}

// Run executes QAOA.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	p := cfg.layers()
	nq := cfg.CostHamiltonian.NumQubits()

	// Build parameterized QAOA circuit.
	gammas := param.NewVector("γ", p)
	betas := param.NewVector("β", p)

	b := builder.New("QAOA", nq)

	// Initial superposition.
	for q := range nq {
		b.H(q)
	}

	// Alternating layers.
	for k := range p {
		// Cost layer: apply exp(-i*gamma_k*C) via Z-terms.
		applyCostLayer(b, cfg.CostHamiltonian, gammas.At(k).Expr(), nq)

		// Mixer layer: apply exp(-i*beta_k*B).
		if cfg.MixerHamiltonian != nil {
			applyMixerFromHamiltonian(b, *cfg.MixerHamiltonian, betas.At(k).Expr(), nq)
		} else {
			// Default X-mixer: RX(2*beta) on each qubit.
			for q := range nq {
				b.SymRX(param.Mul(param.Literal(2), betas.At(k).Expr()), q)
			}
		}
	}

	circ, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("qaoa: circuit build: %w", err)
	}

	paramNames := ir.FreeParameters(circ)

	// Build cost function.
	cost := func(x []float64) float64 {
		bindings := make(map[string]float64, len(paramNames))
		for i, name := range paramNames {
			bindings[name] = x[i]
		}
		bound, err := ir.Bind(circ, bindings)
		if err != nil {
			panic("qaoa: bind: " + err.Error())
		}
		sim := statevector.New(nq)
		if err := sim.Evolve(bound); err != nil {
			panic("qaoa: evolve: " + err.Error())
		}
		return sim.ExpectPauliSum(cfg.CostHamiltonian)
	}

	// Initial parameters.
	x0 := cfg.InitialParams
	if x0 == nil {
		x0 = make([]float64, 2*p)
		for i := range p {
			x0[i] = 0.5           // gamma
			x0[p+i] = math.Pi / 4 // beta
		}
	}

	res, err := cfg.Optimizer.Minimize(ctx, cost, x0, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("qaoa: optimization: %w", err)
	}

	// Sample the optimal circuit.
	bindings := make(map[string]float64, len(paramNames))
	for i, name := range paramNames {
		bindings[name] = res.X[i]
	}
	bound, err := ir.Bind(circ, bindings)
	if err != nil {
		return nil, fmt.Errorf("qaoa: final bind: %w", err)
	}

	measCirc := addMeasurements(bound)
	sim := statevector.New(nq)
	counts, err := sim.Run(measCirc, cfg.shots())
	if err != nil {
		return nil, fmt.Errorf("qaoa: sampling: %w", err)
	}

	bestBS, bestCost := findBestBitstring(counts, cfg.CostHamiltonian)

	return &Result{
		OptimalValue:  res.Fun,
		OptimalParams: res.X,
		BestBitstring: bestBS,
		BestCost:      bestCost,
		NumIters:      res.Iterations,
		Converged:     res.Converged,
	}, nil
}

// applyCostLayer applies the cost unitary exp(-i*gamma*C) for Z-diagonal Hamiltonians.
// For each Z_i term: RZ(2*gamma*coeff, i).
// For each Z_i*Z_j term: RZZ(2*gamma*coeff, i, j).
// Identity terms contribute a global phase (skipped).
func applyCostLayer(b *builder.Builder, h pauli.PauliSum, gamma param.Expr, nq int) {
	for _, term := range h.Terms() {
		coeff := real(term.Coeff())
		if coeff == 0 {
			continue
		}

		// Find which qubits have Z operators.
		var zQubits []int
		allZ := true
		for q := range nq {
			op := term.Op(q)
			if op == pauli.Z {
				zQubits = append(zQubits, q)
			} else if op != pauli.I {
				allZ = false
				break
			}
		}
		if !allZ || len(zQubits) == 0 {
			continue
		}

		angle := param.Mul(param.Literal(2*coeff), gamma)

		switch len(zQubits) {
		case 1:
			b.SymRZ(angle, zQubits[0])
		case 2:
			b.SymRZZ(angle, zQubits[0], zQubits[1])
		}
	}
}

// applyMixerFromHamiltonian applies mixer unitary for X-terms.
func applyMixerFromHamiltonian(b *builder.Builder, h pauli.PauliSum, beta param.Expr, nq int) {
	for _, term := range h.Terms() {
		coeff := real(term.Coeff())
		if coeff == 0 {
			continue
		}
		for q := range nq {
			if term.Op(q) == pauli.X {
				angle := param.Mul(param.Literal(2*coeff), beta)
				b.SymRX(angle, q)
			}
		}
	}
}

// addMeasurements adds MeasureAll to a circuit.
func addMeasurements(c *ir.Circuit) *ir.Circuit {
	b := builder.New(c.Name(), c.NumQubits())
	b.Compose(c, nil)
	b.MeasureAll()
	circ, _ := b.Build()
	return circ
}

// findBestBitstring evaluates the cost Hamiltonian for each measured bitstring.
func findBestBitstring(counts map[string]int, h pauli.PauliSum) (string, float64) {
	bestBS := ""
	bestCost := math.Inf(1)

	for bs := range counts {
		cost := evaluateBitstring(bs, h)
		if cost < bestCost {
			bestCost = cost
			bestBS = bs
		}
	}
	return bestBS, bestCost
}

// evaluateBitstring computes the Hamiltonian value for a computational basis state.
func evaluateBitstring(bs string, h pauli.PauliSum) float64 {
	var total float64
	for _, term := range h.Terms() {
		coeff := real(term.Coeff())
		val := 1.0
		for q := range term.NumQubits() {
			op := term.Op(q)
			if op == pauli.Z {
				// Map '0' -> +1, '1' -> -1.
				// Bitstring convention: leftmost = highest qubit.
				idx := term.NumQubits() - 1 - q
				if idx < len(bs) && bs[idx] == '1' {
					val *= -1
				}
			} else if op != pauli.I {
				// Non-diagonal term: skip (expectation is 0 for basis states).
				val = 0
				break
			}
		}
		total += coeff * val
	}
	return total
}
