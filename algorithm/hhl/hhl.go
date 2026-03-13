// Package hhl implements the Harrow-Hassidim-Lloyd algorithm for solving
// linear systems of equations Ax = b on a quantum computer.
//
// Given a Hermitian matrix A (expressed as a PauliSum Hamiltonian) and a
// state preparation circuit for |b⟩, HHL estimates x = A⁻¹b encoded in
// the amplitudes of the output quantum state.
//
// The algorithm uses three registers:
//   - 1 ancilla qubit for eigenvalue inversion flagging
//   - NumPhaseBits phase qubits for quantum phase estimation
//   - NumQubits system qubits representing the vector space
package hhl

import (
	"context"
	"fmt"
	"math"

	"github.com/splch/goqu/algorithm/qpe"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// Config specifies the HHL problem.
type Config struct {
	// Matrix is the Hermitian matrix A expressed as a Pauli Hamiltonian.
	Matrix pauli.PauliSum
	// RHS is the state preparation circuit for |b⟩.
	RHS *ir.Circuit
	// NumPhaseBits is the number of ancilla qubits for phase estimation precision.
	NumPhaseBits int
	// NumQubits is the number of system qubits (dimension of A is 2^NumQubits).
	NumQubits int
	// Shots is the number of measurement shots. Default: 1024.
	Shots int
}

// Result holds HHL output.
type Result struct {
	// Circuit is the full HHL circuit.
	Circuit *ir.Circuit
	// StateVector is the output state of the system register,
	// post-selected on ancilla = |1⟩ and phase register = |0⟩.
	StateVector []complex128
	// Success is the probability of the post-selection succeeding
	// (ancilla = |1⟩ and phase register = |0⟩).
	Success float64
}

// Run executes the HHL algorithm.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.NumQubits < 1 {
		return nil, fmt.Errorf("hhl: numQubits must be >= 1")
	}
	if cfg.NumPhaseBits < 1 {
		return nil, fmt.Errorf("hhl: numPhaseBits must be >= 1")
	}
	if cfg.RHS == nil {
		return nil, fmt.Errorf("hhl: RHS circuit required")
	}

	nAnc := 1
	nPhase := cfg.NumPhaseBits
	nSys := cfg.NumQubits
	nTotal := nAnc + nPhase + nSys

	ancilla := 0
	phaseStart := nAnc
	sysStart := nAnc + nPhase

	b := builder.New("HHL", nTotal)

	// Step 1: Prepare |b⟩ on system register.
	sysMap := make(map[int]int, nSys)
	for i := range nSys {
		sysMap[i] = sysStart + i
	}
	b.Compose(cfg.RHS, sysMap)

	// Step 2: QPE forward — Hadamard on phase register.
	for q := range nPhase {
		b.H(phaseStart + q)
	}

	// Controlled Hamiltonian evolution: controlled-e^{i*A*2^k*t₀}.
	// Convention: QPE estimates φ where U|ψ⟩ = e^{2πiφ}|ψ⟩.
	// For U = e^{iAt}, eigenvalue e^{iλt} = e^{2πi(λt/(2π))}.
	// So φ = λt/(2π), and λ = 2πφ/t.
	// With t₀ = 1, eigenvalue register reads j where λ ≈ 2πj/2^nPhase.
	t0 := 1.0
	trotterSteps := 1

	// Compute the identity coefficient: sum of real(coeff) for identity terms.
	// Identity terms contribute a global phase e^{i*identCoeff*t} to the
	// evolution operator. Under controlled application this becomes a relative
	// phase on the control qubit, implemented as a Phase gate.
	identCoeff := identityCoefficient(cfg.Matrix)

	for k := range nPhase {
		power := 1 << (nPhase - 1 - k)
		t := t0 * float64(power)

		// Build evolution circuit for time t (non-identity terms only).
		evB := builder.New("evo", nSys)
		applyHamiltonianEvolution(evB, cfg.Matrix, t, 0, max(1, trotterSteps*power))
		evCirc, err := evB.Build()
		if err != nil {
			return nil, fmt.Errorf("hhl: evolution circuit: %w", err)
		}

		// Apply controlled version with phase qubit k as control.
		controlQ := phaseStart + k
		for _, op := range evCirc.Ops() {
			if op.Gate == nil || op.Gate.Name() == "barrier" {
				continue
			}
			targetQubits := make([]int, len(op.Qubits))
			for i, q := range op.Qubits {
				targetQubits[i] = sysStart + q
			}
			b.Ctrl(op.Gate, []int{controlQ}, targetQubits...)
		}

		// Apply identity contribution as a phase gate on the control qubit.
		// controlled-e^{i*c*I*t} = Phase(c*t) on control qubit.
		if identCoeff != 0 {
			b.Phase(identCoeff*t, controlQ)
		}
	}

	// Inverse QFT on phase register.
	iqft, err := qpe.InverseQFT(nPhase)
	if err != nil {
		return nil, fmt.Errorf("hhl: inverse QFT: %w", err)
	}
	phaseMap := make(map[int]int, nPhase)
	for i := range nPhase {
		phaseMap[i] = phaseStart + i
	}
	b.Compose(iqft, phaseMap)

	// Step 3: Controlled rotation on ancilla.
	// For each phase register basis state |j⟩, the estimated eigenvalue is
	// λ_j = 2π*j / 2^nPhase.
	// Apply RY(2*arcsin(C/λ_j)) on ancilla, controlled by phase register = |j⟩.
	// C is the smallest representable eigenvalue (j=1).
	nPhaseDim := 1 << nPhase
	C := 2 * math.Pi / float64(nPhaseDim)

	for j := 1; j < nPhaseDim; j++ {
		lambda := 2 * math.Pi * float64(j) / float64(nPhaseDim)
		ratio := C / lambda
		if ratio > 1 {
			ratio = 1
		}
		if ratio < -1 {
			ratio = -1
		}
		angle := 2 * math.Asin(ratio)

		if math.Abs(angle) < 1e-10 {
			continue
		}

		// Set up phase register to match |j⟩: X on bits where j has a 0.
		for bit := range nPhase {
			if j&(1<<bit) == 0 {
				b.X(phaseStart + bit)
			}
		}

		// Multi-controlled RY on ancilla.
		phaseControls := make([]int, nPhase)
		for i := range nPhase {
			phaseControls[i] = phaseStart + i
		}
		b.Ctrl(gate.RY(angle), phaseControls, ancilla)

		// Undo X gates.
		for bit := range nPhase {
			if j&(1<<bit) == 0 {
				b.X(phaseStart + bit)
			}
		}
	}

	// Step 4: Inverse QPE to uncompute phase register.
	// Forward QFT on phase register.
	qftCirc, err := qpe.QFT(nPhase)
	if err != nil {
		return nil, fmt.Errorf("hhl: QFT: %w", err)
	}
	b.Compose(qftCirc, phaseMap)

	// Undo controlled evolutions in reverse order.
	for k := nPhase - 1; k >= 0; k-- {
		power := 1 << (nPhase - 1 - k)
		t := t0 * float64(power)

		// Undo identity phase contribution (inverse = negate the angle).
		if identCoeff != 0 {
			b.Phase(-identCoeff*t, phaseStart+k)
		}

		evB := builder.New("evo", nSys)
		applyHamiltonianEvolution(evB, cfg.Matrix, t, 0, max(1, trotterSteps*power))
		evCirc, err := evB.Build()
		if err != nil {
			return nil, fmt.Errorf("hhl: inverse evolution: %w", err)
		}

		// Apply controlled-INVERSE with phase qubit k as control.
		controlQ := phaseStart + k
		ops := evCirc.Ops()
		for i := len(ops) - 1; i >= 0; i-- {
			op := ops[i]
			if op.Gate == nil || op.Gate.Name() == "barrier" {
				continue
			}
			inv := op.Gate.Inverse()
			targetQubits := make([]int, len(op.Qubits))
			for j, q := range op.Qubits {
				targetQubits[j] = sysStart + q
			}
			b.Ctrl(inv, []int{controlQ}, targetQubits...)
		}
	}

	// Undo Hadamards on phase register.
	for q := range nPhase {
		b.H(phaseStart + q)
	}

	// Build circuit.
	circ, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("hhl: circuit build: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Simulate.
	sim := statevector.New(nTotal)
	if err := sim.Evolve(circ); err != nil {
		return nil, fmt.Errorf("hhl: simulation: %w", err)
	}
	fullSV := sim.StateVector()

	// Post-select on ancilla = |1⟩ and phase register = |0⟩.
	var successProb float64
	sysStates := make([]complex128, 1<<nSys)

	for i, amp := range fullSV {
		// Ancilla is qubit 0: check bit 0 is set (|1⟩).
		if i&1 == 0 {
			continue
		}

		// Check phase register is |0⟩ (all phase qubits are 0).
		phaseZero := true
		for pq := range nPhase {
			if i&(1<<(phaseStart+pq)) != 0 {
				phaseZero = false
				break
			}
		}
		if !phaseZero {
			continue
		}

		// Extract system register index.
		sysIdx := 0
		for sq := range nSys {
			if i&(1<<(sysStart+sq)) != 0 {
				sysIdx |= 1 << sq
			}
		}

		prob := real(amp)*real(amp) + imag(amp)*imag(amp)
		successProb += prob
		sysStates[sysIdx] += amp
	}

	// Normalize system states.
	if successProb > 1e-10 {
		norm := math.Sqrt(successProb)
		for i := range sysStates {
			sysStates[i] /= complex(norm, 0)
		}
	}

	return &Result{
		Circuit:     circ,
		StateVector: sysStates,
		Success:     successProb,
	}, nil
}

// identityCoefficient returns the sum of real coefficients for all identity
// terms in the Hamiltonian. These contribute a global phase e^{i*c*t} that
// must be tracked for controlled evolution.
func identityCoefficient(h pauli.PauliSum) float64 {
	var c float64
	for _, term := range h.Terms() {
		if term.IsIdentity() {
			c += real(term.Coeff())
		}
	}
	return c
}

// applyHamiltonianEvolution applies e^{iHt} using first-order Trotter decomposition.
func applyHamiltonianEvolution(b *builder.Builder, h pauli.PauliSum, t float64, systemStart int, steps int) {
	dt := t / float64(steps)
	for range steps {
		for _, term := range h.Terms() {
			if term.IsIdentity() {
				continue
			}
			applyPauliRotation(b, term, dt, systemStart)
		}
	}
}

// applyPauliRotation applies e^{i * coeff * P * angle} for a single Pauli string term.
func applyPauliRotation(b *builder.Builder, ps pauli.PauliString, angle float64, offset int) {
	coeff := real(ps.Coeff())
	nq := ps.NumQubits()

	// Find non-identity qubit positions.
	var nonI []int
	for q := range nq {
		if ps.Op(q) != pauli.I {
			nonI = append(nonI, q)
		}
	}
	if len(nonI) == 0 {
		return
	}

	// Basis change: rotate X→Z, Y→Z.
	for _, q := range nonI {
		switch ps.Op(q) {
		case pauli.X:
			b.H(offset + q)
		case pauli.Y:
			b.RX(math.Pi/2, offset+q)
		}
	}

	// CNOT cascade to compute parity into the last qubit.
	for i := 0; i < len(nonI)-1; i++ {
		b.CNOT(offset+nonI[i], offset+nonI[i+1])
	}

	// RZ rotation on the last qubit.
	// We want e^{i*coeff*P*angle}, and the RZ convention is e^{-i*theta/2*Z},
	// so the RZ angle is -2*coeff*angle.
	b.RZ(-2*coeff*angle, offset+nonI[len(nonI)-1])

	// Undo CNOT cascade.
	for i := len(nonI) - 2; i >= 0; i-- {
		b.CNOT(offset+nonI[i], offset+nonI[i+1])
	}

	// Undo basis change.
	for _, q := range nonI {
		switch ps.Op(q) {
		case pauli.X:
			b.H(offset + q)
		case pauli.Y:
			b.RX(-math.Pi/2, offset+q)
		}
	}
}
