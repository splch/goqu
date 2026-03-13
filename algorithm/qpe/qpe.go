// Package qpe implements Quantum Phase Estimation.
//
// Given a unitary U and an eigenstate |ψ⟩ with U|ψ⟩ = e^{2πiφ}|ψ⟩,
// QPE estimates the phase φ using a register of ancilla qubits
// and the inverse Quantum Fourier Transform.
package qpe

import (
	"context"
	"fmt"
	"math"

	"github.com/splch/goqu/algorithm/internal/algoutil"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// Config specifies the QPE problem.
type Config struct {
	// Unitary is the gate whose eigenvalue phase to estimate.
	Unitary gate.Gate
	// NumPhaseBits is the number of ancilla qubits for precision.
	NumPhaseBits int
	// EigenState is a preparation circuit for the target eigenstate.
	// If nil, the target register is left in |0...0⟩.
	EigenState *ir.Circuit
	// Shots is the number of measurement shots. Default: 1024.
	Shots int
}

// Result holds QPE output.
type Result struct {
	Circuit      *ir.Circuit
	Counts       map[string]int
	Phase        float64
	PhaseRegBits int
}

func (c *Config) shots() int {
	if c.Shots > 0 {
		return c.Shots
	}
	return 1024
}

// Run executes Quantum Phase Estimation.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.Unitary == nil {
		return nil, fmt.Errorf("qpe: unitary is required")
	}
	if cfg.NumPhaseBits < 1 {
		return nil, fmt.Errorf("qpe: numPhaseBits must be >= 1")
	}

	nTarget := cfg.Unitary.Qubits()
	nPhase := cfg.NumPhaseBits
	nTotal := nPhase + nTarget

	b := builder.New("QPE", nTotal)

	// Prepare eigenstate on target register.
	if cfg.EigenState != nil {
		qMap := make(map[int]int, nTarget)
		for i := range nTarget {
			qMap[i] = nPhase + i
		}
		b.Compose(cfg.EigenState, qMap)
	}

	// Hadamard on phase register.
	for q := range nPhase {
		b.H(q)
	}

	// Controlled-U^(2^k) applications.
	for k := range nPhase {
		power := 1 << (nPhase - 1 - k)

		// Build U^power circuit.
		uCirc := singleGateCircuit(cfg.Unitary, nTarget)
		uPow, err := ir.Repeat(uCirc, power)
		if err != nil {
			return nil, fmt.Errorf("qpe: repeat: %w", err)
		}

		// Apply controlled-U^power with phase qubit k as control.
		targets := make([]int, nTarget)
		for i := range nTarget {
			targets[i] = nPhase + i
		}

		// Apply each gate from uPow as a controlled version.
		for _, op := range uPow.Ops() {
			if op.Gate == nil || op.Gate.Name() == "barrier" {
				continue
			}
			// Remap target qubits.
			targetQubits := make([]int, len(op.Qubits))
			for i, q := range op.Qubits {
				targetQubits[i] = nPhase + q
			}
			b.Ctrl(op.Gate, []int{k}, targetQubits...)
		}
	}

	// Inverse QFT on phase register.
	applyInverseQFT(b, nPhase)

	// Measure phase register.
	b.WithClbits(nPhase)
	for q := range nPhase {
		b.Measure(q, q)
	}

	circ, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("qpe: circuit build: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sim := statevector.New(nTotal)
	counts, err := sim.Run(circ, cfg.shots())
	if err != nil {
		return nil, fmt.Errorf("qpe: simulation: %w", err)
	}

	// Extract phase from the most common measurement.
	topBS := algoutil.TopKey(counts, nPhase)
	phase := algoutil.BitstringToPhase(topBS, nPhase)

	return &Result{
		Circuit:      circ,
		Counts:       counts,
		Phase:        phase,
		PhaseRegBits: nPhase,
	}, nil
}

// QFT builds a Quantum Fourier Transform circuit on numQubits.
func QFT(numQubits int) (*ir.Circuit, error) {
	if numQubits < 1 {
		return nil, fmt.Errorf("qpe: QFT numQubits must be >= 1")
	}
	b := builder.New("QFT", numQubits)
	applyQFT(b, numQubits)
	return b.Build()
}

// InverseQFT builds an inverse QFT circuit on numQubits.
func InverseQFT(numQubits int) (*ir.Circuit, error) {
	if numQubits < 1 {
		return nil, fmt.Errorf("qpe: InverseQFT numQubits must be >= 1")
	}
	b := builder.New("QFT†", numQubits)
	applyInverseQFT(b, numQubits)
	return b.Build()
}

// ApplyQFT applies the QFT in-place on qubits [0..n-1] of the builder.
func ApplyQFT(b *builder.Builder, n int) { applyQFT(b, n) }

// ApplyInverseQFT applies the inverse QFT in-place on qubits [0..n-1].
func ApplyInverseQFT(b *builder.Builder, n int) { applyInverseQFT(b, n) }

func applyQFT(b *builder.Builder, n int) {
	for i := range n {
		b.H(i)
		for j := i + 1; j < n; j++ {
			angle := math.Pi / float64(int(1)<<(j-i))
			b.Apply(gate.CP(angle), i, j)
		}
	}
	// Swap to match standard QFT output ordering.
	for i := range n / 2 {
		b.SWAP(i, n-1-i)
	}
}

func applyInverseQFT(b *builder.Builder, n int) {
	// Reverse of QFT: swaps first, then reversed rotations.
	for i := range n / 2 {
		b.SWAP(i, n-1-i)
	}
	for i := n - 1; i >= 0; i-- {
		for j := n - 1; j > i; j-- {
			angle := -math.Pi / float64(int(1)<<(j-i))
			b.Apply(gate.CP(angle), i, j)
		}
		b.H(i)
	}
}

// singleGateCircuit wraps a gate into a minimal circuit.
func singleGateCircuit(g gate.Gate, nQubits int) *ir.Circuit {
	qubits := make([]int, nQubits)
	for i := range nQubits {
		qubits[i] = i
	}
	return ir.New("U", nQubits, 0, []ir.Operation{
		{Gate: g, Qubits: qubits},
	}, nil)
}
