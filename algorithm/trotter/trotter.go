// Package trotter implements Trotter-Suzuki Hamiltonian simulation.
//
// Given a Hamiltonian H = sum_k c_k P_k expressed as a PauliSum, the
// algorithm approximates the time-evolution operator e^{-iHt} by
// decomposing it into a product of single-term exponentials.
//
// First-order (Lie-Trotter):
//
//	e^{-iHt} ~ [prod_k e^{-i c_k P_k dt/steps}]^steps
//
// Second-order (Suzuki-Trotter):
//
//	e^{-iHt} ~ [prod_k e^{-i c_k P_k dt/(2*steps)} * prod_k' e^{-i c_k' P_k' dt/(2*steps)}]^steps
//
// where the second product runs in reverse order.
package trotter

import (
	"context"
	"fmt"
	"math"

	"github.com/splch/goqu/algorithm/internal/algoutil"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// Order specifies the Trotter decomposition order.
type Order int

const (
	// First is the first-order Lie-Trotter decomposition.
	First Order = 1
	// Second is the second-order symmetric Suzuki-Trotter decomposition.
	Second Order = 2
)

// Config specifies the Trotter simulation parameters.
type Config struct {
	// Hamiltonian is the operator to simulate, expressed as a sum of Pauli strings.
	Hamiltonian pauli.PauliSum
	// Time is the total evolution time t in e^{-iHt}.
	Time float64
	// Steps is the number of Trotter steps (higher = more accurate). Default: 1.
	Steps int
	// Order is the Trotter decomposition order. Default: First.
	Order Order
}

// Result holds the output of a Trotter simulation circuit construction.
type Result struct {
	// Circuit is the compiled Trotter circuit.
	Circuit *ir.Circuit
	// Steps is the number of Trotter steps used.
	Steps int
	// Order is the Trotter decomposition order used.
	Order Order
}

// Run builds a Trotter circuit for the given Hamiltonian simulation and returns it.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg = applyDefaults(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}

	nq := cfg.Hamiltonian.NumQubits()
	b := builder.New("trotter", nq)

	dt := cfg.Time
	terms := cfg.Hamiltonian.Terms()

	for range cfg.Steps {
		switch cfg.Order {
		case First:
			for _, ps := range terms {
				applyPauliExp(b, ps, real(ps.Coeff())*dt/float64(cfg.Steps))
			}
		case Second:
			halfDt := dt / (2.0 * float64(cfg.Steps))
			// Forward half-step.
			for _, ps := range terms {
				applyPauliExp(b, ps, real(ps.Coeff())*halfDt)
			}
			// Backward half-step.
			for i := len(terms) - 1; i >= 0; i-- {
				applyPauliExp(b, terms[i], real(terms[i].Coeff())*halfDt)
			}
		}
	}

	circ, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("trotter: build circuit: %w", err)
	}
	return &Result{Circuit: circ, Steps: cfg.Steps, Order: cfg.Order}, nil
}

// Evolve builds the Trotter circuit, optionally prepends an initial state
// preparation circuit, evolves the combined circuit using a statevector
// simulator, and returns the final state vector.
func Evolve(ctx context.Context, cfg Config, initial *ir.Circuit) ([]complex128, error) {
	res, err := Run(ctx, cfg)
	if err != nil {
		return nil, err
	}

	nq := cfg.Hamiltonian.NumQubits()
	var fullCirc *ir.Circuit

	if initial != nil {
		fb := builder.New("trotter-evolve", nq)
		qm := algoutil.IdentityMap(nq)
		fb.Compose(initial, qm)
		fb.Compose(res.Circuit, qm)
		fullCirc, err = fb.Build()
		if err != nil {
			return nil, fmt.Errorf("trotter: compose circuits: %w", err)
		}
	} else {
		fullCirc = res.Circuit
	}

	sim := statevector.New(nq)
	if err := sim.Evolve(fullCirc); err != nil {
		return nil, fmt.Errorf("trotter: evolve: %w", err)
	}
	return sim.StateVector(), nil
}

// applyPauliExp applies exp(-i * angle * P) where P is a Pauli string
// (ignoring its coefficient, which the caller has already folded into angle).
//
// The decomposition uses the standard basis-change / CNOT-cascade / RZ pattern:
//  1. Basis change: X -> H, Y -> RX(pi/2), Z -> nothing.
//  2. CNOT cascade to compute parity on the last non-identity qubit.
//  3. RZ(2*angle) on the parity qubit.
//  4. Undo CNOT cascade.
//  5. Undo basis change.
func applyPauliExp(b *builder.Builder, ps pauli.PauliString, angle float64) {
	if ps.IsIdentity() {
		// Pure identity term contributes only a global phase; skip.
		return
	}

	nq := ps.NumQubits()

	// Collect non-identity qubit positions.
	var nonI []int
	for q := range nq {
		if ps.Op(q) != pauli.I {
			nonI = append(nonI, q)
		}
	}
	if len(nonI) == 0 {
		return
	}

	// Step 1: Basis change.
	for _, q := range nonI {
		switch ps.Op(q) {
		case pauli.X:
			b.H(q)
		case pauli.Y:
			b.RX(math.Pi/2, q)
		}
		// Z requires no basis change.
	}

	// Step 2: CNOT cascade to accumulate parity.
	for i := 0; i < len(nonI)-1; i++ {
		b.CNOT(nonI[i], nonI[i+1])
	}

	// Step 3: RZ rotation on the parity qubit.
	b.RZ(2*angle, nonI[len(nonI)-1])

	// Step 4: Undo CNOT cascade (reverse order).
	for i := len(nonI) - 2; i >= 0; i-- {
		b.CNOT(nonI[i], nonI[i+1])
	}

	// Step 5: Undo basis change.
	for _, q := range nonI {
		switch ps.Op(q) {
		case pauli.X:
			b.H(q)
		case pauli.Y:
			b.RX(-math.Pi/2, q)
		}
	}
}

// applyDefaults fills in zero-valued fields with sensible defaults.
func applyDefaults(cfg Config) Config {
	if cfg.Steps <= 0 {
		cfg.Steps = 1
	}
	if cfg.Order == 0 {
		cfg.Order = First
	}
	return cfg
}

// validate checks that the configuration is usable.
func validate(cfg Config) error {
	if len(cfg.Hamiltonian.Terms()) == 0 {
		return fmt.Errorf("trotter: Hamiltonian has no terms")
	}
	if cfg.Steps < 1 {
		return fmt.Errorf("trotter: Steps must be >= 1, got %d", cfg.Steps)
	}
	if cfg.Order != First && cfg.Order != Second {
		return fmt.Errorf("trotter: unsupported Order %d (use First or Second)", cfg.Order)
	}
	return nil
}
