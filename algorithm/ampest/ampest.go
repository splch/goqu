// Package ampest implements Quantum Amplitude Estimation.
//
// Standard AE uses Quantum Phase Estimation on the Grover iterate
// Q = A S_0 A^dag S_f to estimate the amplitude a = sin(pi * theta)
// of the "good" subspace prepared by a state-preparation circuit A.
package ampest

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

// Config specifies parameters for standard Amplitude Estimation.
type Config struct {
	// StatePrep is the circuit A that prepares the state whose amplitude
	// we want to estimate. It acts on NumQubits qubits.
	StatePrep *ir.Circuit
	// Oracle marks the "good" states by flipping their phase.
	Oracle grover.Oracle
	// NumQubits is the number of working qubits used by StatePrep.
	NumQubits int
	// NumPhaseBits is the number of ancilla qubits controlling precision.
	NumPhaseBits int
	// Shots is the number of measurement shots. Default: 1024.
	Shots int
}

// Result holds the output of standard Amplitude Estimation.
type Result struct {
	// Circuit is the full QPE-on-Grover circuit that was executed.
	Circuit *ir.Circuit
	// Amplitude is the estimated amplitude a = sin(pi * theta).
	Amplitude float64
	// Probability is a^2.
	Probability float64
	// Phase is the raw phase theta from QPE.
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

// Run executes standard Quantum Amplitude Estimation.
//
// The algorithm builds the Grover iterate Q = A * S_0 * A^dag * S_f,
// applies QPE to estimate the phase theta of Q, then computes
// amplitude a = sin(pi * theta).
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.StatePrep == nil {
		return nil, fmt.Errorf("ampest: state prep circuit is required")
	}
	if cfg.Oracle == nil {
		return nil, fmt.Errorf("ampest: oracle is required")
	}
	if cfg.NumQubits < 1 {
		return nil, fmt.Errorf("ampest: numQubits must be >= 1")
	}
	if cfg.NumPhaseBits < 1 {
		return nil, fmt.Errorf("ampest: numPhaseBits must be >= 1")
	}

	n := cfg.NumQubits
	nPhase := cfg.NumPhaseBits
	nTotal := nPhase + n

	// Build the Grover iterate Q on the working register.
	groverQ, err := buildGroverIterate(cfg.StatePrep, cfg.Oracle, n)
	if err != nil {
		return nil, fmt.Errorf("ampest: grover iterate: %w", err)
	}

	// Build the full QPE circuit: phase register [0..nPhase-1],
	// target register [nPhase..nTotal-1].
	b := builder.New("AmpEst", nTotal)

	// Prepare the state A|0> on the target register.
	targetMap := make(map[int]int, n)
	for i := range n {
		targetMap[i] = nPhase + i
	}
	b.Compose(cfg.StatePrep, targetMap)

	// Hadamard on all phase qubits.
	for q := range nPhase {
		b.H(q)
	}

	// Controlled-Q^(2^k) for each phase qubit k.
	for k := range nPhase {
		power := 1 << (nPhase - 1 - k)

		// Build Q^power as a circuit.
		qPow, err := ir.Repeat(groverQ, power)
		if err != nil {
			return nil, fmt.Errorf("ampest: repeat Q^%d: %w", power, err)
		}

		// Apply each gate from Q^power as controlled by phase qubit k.
		for _, op := range qPow.Ops() {
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

	// Inverse QFT on the phase register.
	qpe.ApplyInverseQFT(b, nPhase)

	// Measure only the phase register.
	b.WithClbits(nPhase)
	for q := range nPhase {
		b.Measure(q, q)
	}

	circ, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("ampest: circuit build: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Simulate and collect measurements.
	sim := statevector.New(nTotal)
	counts, err := sim.Run(circ, cfg.shots())
	if err != nil {
		return nil, fmt.Errorf("ampest: simulation: %w", err)
	}

	// Extract phase from the most common measurement bitstring.
	topBS := algoutil.TopKey(counts, nPhase)
	phase := algoutil.BitstringToPhase(topBS, nPhase)

	// Convert phase to amplitude.
	//
	// The Grover iterate Q = A*S_0*A^dag*S_f (without a global minus sign)
	// has eigenvalues e^{i(pi +/- 2*theta_a)} where sin(theta_a) = a.
	// QPE measures phi where eigenvalue = e^{2*pi*i*phi}, so:
	//   phi = 1/2 +/- theta_a/pi
	// Therefore theta_a = pi * |phi - 1/2| and a = sin(theta_a).
	amp := math.Sin(math.Pi * math.Abs(phase-0.5))

	return &Result{
		Circuit:     circ,
		Amplitude:   amp,
		Probability: amp * amp,
		Phase:       phase,
		Counts:      counts,
	}, nil
}

// buildGroverIterate constructs the Grover iterate Q = A * S_0 * A^dag * S_f
// as a circuit on n qubits.
func buildGroverIterate(statePrep *ir.Circuit, oracle grover.Oracle, n int) (*ir.Circuit, error) {
	b := builder.New("Q", n)
	qubits := make([]int, n)
	for i := range n {
		qubits[i] = i
	}

	// S_f: oracle marks good states by phase flip.
	oracle(b, qubits)

	// A^dag: inverse of state preparation.
	idMap := algoutil.IdentityMap(n)
	b.ComposeInverse(statePrep, idMap)

	// S_0: reflection about |0>: 2|0><0| - I.
	// Implemented as X^n, MCZ, X^n which flips the phase of |0...0>.
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

	// A: state preparation.
	b.Compose(statePrep, idMap)

	return b.Build()
}


