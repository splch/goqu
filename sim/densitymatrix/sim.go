package densitymatrix

import (
	"fmt"
	"math"
	"math/cmplx"
	"math/rand/v2"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/sim/noise"
)

// parallelThreshold is the minimum number of qubits before enabling parallel kernels.
// At 9 qubits dim=512 and the density matrix has 262K elements; the heavier
// per-element work (row + column passes) justifies a lower threshold than statevector.
const parallelThreshold = 9

// Sim simulates a circuit via density matrix evolution.
type Sim struct {
	numQubits int
	dim       int // 2^numQubits
	rho       []complex128
	noise     *noise.NoiseModel
}

// New creates a simulator initialized to |0...0⟩⟨0...0|.
func New(numQubits int) *Sim {
	if numQubits < 1 || numQubits > 14 {
		panic(fmt.Sprintf("densitymatrix: numQubits %d out of range [1, 14]", numQubits))
	}
	dim := 1 << numQubits
	rho := make([]complex128, dim*dim)
	rho[0] = 1 // |0><0|
	return &Sim{numQubits: numQubits, dim: dim, rho: rho}
}

// WithNoise sets the noise model for the simulation.
func (s *Sim) WithNoise(nm *noise.NoiseModel) *Sim {
	s.noise = nm
	return s
}

// Run executes the circuit and returns measurement counts.
func (s *Sim) Run(c *ir.Circuit, shots int) (map[string]int, error) {
	if err := s.Evolve(c); err != nil {
		return nil, err
	}
	probs := s.diagonalProbs()
	counts := make(map[string]int)
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	for range shots {
		idx := sampleIndex(probs, rng)
		bs := formatBitstring(idx, s.numQubits)
		counts[bs]++
	}
	return counts, nil
}

// Evolve applies all gate operations, resets state to |0><0| first.
func (s *Sim) Evolve(c *ir.Circuit) error {
	if c.NumQubits() != s.numQubits {
		return fmt.Errorf("circuit has %d qubits, simulator has %d", c.NumQubits(), s.numQubits)
	}
	// Reset to |0><0|.
	for i := range s.rho {
		s.rho[i] = 0
	}
	s.rho[0] = 1

	for _, op := range c.Ops() {
		if op.Gate == nil || op.Gate.Name() == "barrier" {
			continue
		}
		m := op.Gate.Matrix()
		if m == nil {
			continue
		}
		switch op.Gate.Qubits() {
		case 1:
			s.applyGate1(op.Qubits[0], m)
		case 2:
			s.applyGate2(op.Qubits[0], op.Qubits[1], m)
		default:
			// Auto-decompose >2 qubit gates into 1-2 qubit operations.
			subOps := decomposeForDensity(op)
			if subOps == nil {
				return fmt.Errorf("densitymatrix: unsupported gate size: %d qubits", op.Gate.Qubits())
			}
			for _, sub := range subOps {
				sm := sub.Gate.Matrix()
				if sm == nil {
					continue
				}
				switch sub.Gate.Qubits() {
				case 1:
					s.applyGate1(sub.Qubits[0], sm)
				case 2:
					s.applyGate2(sub.Qubits[0], sub.Qubits[1], sm)
				default:
					return fmt.Errorf("densitymatrix: decomposition produced %d-qubit gate", sub.Gate.Qubits())
				}
			}
		}
		// Apply noise after gate.
		if s.noise != nil {
			ch := s.noise.Lookup(op.Gate.Name(), op.Qubits)
			if ch != nil {
				s.applyChannel(ch, op.Qubits)
			}
		}
	}
	return nil
}

// applyChannel applies a noise channel ρ' = Σ_k E_k ρ E_k†.
func (s *Sim) applyChannel(ch noise.Channel, qubits []int) {
	kraus := ch.Kraus()
	if len(kraus) == 1 {
		// Single Kraus op: in-place like a gate.
		switch ch.Qubits() {
		case 1:
			s.applyGate1(qubits[0], kraus[0])
		case 2:
			s.applyGate2(qubits[0], qubits[1], kraus[0])
		}
		return
	}

	// Multiple Kraus ops: accumulate into temp buffer.
	n := len(s.rho)
	temp := make([]complex128, n)
	saved := make([]complex128, n)
	copy(saved, s.rho)

	for _, ek := range kraus {
		// Restore rho from saved.
		copy(s.rho, saved)
		// Apply E_k as a gate.
		switch ch.Qubits() {
		case 1:
			s.applyGate1(qubits[0], ek)
		case 2:
			s.applyGate2(qubits[0], qubits[1], ek)
		}
		// Accumulate.
		for i := range n {
			temp[i] += s.rho[i]
		}
	}
	copy(s.rho, temp)
}

// DensityMatrix returns a copy of the current density matrix.
func (s *Sim) DensityMatrix() []complex128 {
	out := make([]complex128, len(s.rho))
	copy(out, s.rho)
	return out
}

// Purity returns Tr(ρ²). Pure states have purity 1, maximally mixed = 1/dim.
func (s *Sim) Purity() float64 {
	dim := s.dim
	var tr complex128
	// Tr(ρ²) = Σ_{ij} ρ_{ij} * ρ_{ji} = Σ_{ij} |ρ_{ij}|²  (since ρ is Hermitian)
	for i := range dim {
		for j := range dim {
			v := s.rho[i*dim+j]
			tr += v * conj(s.rho[j*dim+i])
		}
	}
	return math.Abs(real(tr))
}

// Fidelity computes the fidelity F = ⟨ψ|ρ|ψ⟩ between a pure state |ψ⟩ and ρ.
func (s *Sim) Fidelity(pureState []complex128) float64 {
	if len(pureState) != s.dim {
		panic(fmt.Sprintf("densitymatrix: pure state length %d, expected %d", len(pureState), s.dim))
	}
	dim := s.dim
	var f complex128
	for i := range dim {
		for j := range dim {
			f += conj(pureState[i]) * s.rho[i*dim+j] * pureState[j]
		}
	}
	return math.Abs(real(f))
}

func conj(c complex128) complex128 {
	return complex(real(c), -imag(c))
}

// decomposeForDensity breaks a >2 qubit gate into 1-2 qubit operations.
// Uses the gate's Decompose method, then recursively decomposes until all ops are <=2 qubits.
func decomposeForDensity(op ir.Operation) []ir.Operation {
	applied := op.Gate.Decompose(op.Qubits)
	if applied != nil {
		var result []ir.Operation
		for _, a := range applied {
			sub := ir.Operation{Gate: a.Gate, Qubits: a.Qubits}
			if a.Gate.Qubits() <= 2 {
				result = append(result, sub)
			} else {
				inner := decomposeForDensity(sub)
				if inner == nil {
					return nil
				}
				result = append(result, inner...)
			}
		}
		return result
	}
	// For controlled gates, decompose via the controlled gate interface.
	if cg, ok := op.Gate.(gate.ControlledGate); ok {
		_ = cg
		// Use a simple recursive approach: the controlled kernel for statevector
		// uses direct bit manipulation, but density matrix needs decomposition.
		// Recursively decompose: Controlled(U, n) → smaller controlled gates.
		return decomposeControlledForDensity(op)
	}
	return nil
}

// decomposeControlledForDensity decomposes a controlled gate for density matrix sim.
// Reduces any multi-controlled single-qubit gate to MCX + 1Q gates, then decomposes MCX.
func decomposeControlledForDensity(op ir.Operation) []ir.Operation {
	cg := op.Gate.(gate.ControlledGate)
	nControls := cg.NumControls()
	controls := op.Qubits[:nControls]
	targets := op.Qubits[nControls:]
	inner := cg.Inner()

	if nControls == 1 && inner.Qubits() == 1 {
		// Single-controlled single-qubit: this is a 2-qubit gate.
		return []ir.Operation{op}
	}

	if inner.Qubits() != 1 {
		return nil
	}

	// C^n(X): direct recursive decomposition.
	if inner == gate.X || inner.Name() == "X" {
		return decomposeMCXForDensity(controls, targets[0])
	}

	// General C^n(U): reduce to MCX + single-qubit gates using ABC decomposition.
	// U = e^{iδ} · AXBXC where ABC = I.
	return decomposeGeneralControlledForDensity(inner, controls, targets[0])
}

// decomposeMCXForDensity decomposes C^n(X) recursively for density matrix.
func decomposeMCXForDensity(controls []int, target int) []ir.Operation {
	n := len(controls)
	if n == 1 {
		return []ir.Operation{{Gate: gate.CNOT, Qubits: []int{controls[0], target}}}
	}
	if n == 2 {
		// CCX decomposition.
		c0, c1 := controls[0], controls[1]
		return []ir.Operation{
			{Gate: gate.H, Qubits: []int{target}},
			{Gate: gate.CNOT, Qubits: []int{c1, target}},
			{Gate: gate.Tdg, Qubits: []int{target}},
			{Gate: gate.CNOT, Qubits: []int{c0, target}},
			{Gate: gate.T, Qubits: []int{target}},
			{Gate: gate.CNOT, Qubits: []int{c1, target}},
			{Gate: gate.Tdg, Qubits: []int{target}},
			{Gate: gate.CNOT, Qubits: []int{c0, target}},
			{Gate: gate.T, Qubits: []int{c1}},
			{Gate: gate.T, Qubits: []int{target}},
			{Gate: gate.CNOT, Qubits: []int{c0, c1}},
			{Gate: gate.H, Qubits: []int{target}},
			{Gate: gate.T, Qubits: []int{c0}},
			{Gate: gate.Tdg, Qubits: []int{c1}},
			{Gate: gate.CNOT, Qubits: []int{c0, c1}},
		}
	}

	// Recursive V-gate approach.
	lastCtrl := controls[n-1]
	restCtrls := controls[:n-1]
	var ops []ir.Operation //nolint:prealloc // size depends on recursive decomposition depth

	// C^{n-1}(SX)
	ops = append(ops, decomposeControlled1QForDensity(gate.SX, restCtrls, target)...)
	// CX
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{lastCtrl, target}})
	// C^{n-1}(SX†)
	ops = append(ops, decomposeControlled1QForDensity(gate.SX.Inverse(), restCtrls, target)...)
	// CX
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{lastCtrl, target}})
	// C^{n-1}(S) phase correction
	ops = append(ops, decomposeControlled1QForDensity(gate.S, restCtrls, lastCtrl)...)

	return ops
}

// decomposeControlled1QForDensity decomposes C^n(U) for single-qubit U.
func decomposeControlled1QForDensity(u gate.Gate, controls []int, target int) []ir.Operation {
	if len(controls) == 1 {
		cg := gate.Controlled(u, 1)
		return []ir.Operation{{Gate: cg, Qubits: []int{controls[0], target}}}
	}
	if u == gate.X || u.Name() == "X" {
		return decomposeMCXForDensity(controls, target)
	}
	// General C^n(U): reduce to MCX + 1Q gates.
	return decomposeGeneralControlledForDensity(u, controls, target)
}

// decomposeGeneralControlledForDensity decomposes C^n(U) for general single-qubit U
// by reducing to MCX + single-qubit rotations using the ABC decomposition.
func decomposeGeneralControlledForDensity(u gate.Gate, controls []int, target int) []ir.Operation {
	// U = e^{iδ} · Rz(α) · Ry(β) · Rz(γ)
	// C^n(U) = A(tgt) · MCX(ctrls,tgt) · B(tgt) · MCX(ctrls,tgt) · C(tgt) + phase
	m := u.Matrix()
	// Simple Euler ZYZ decomposition inline.
	det := m[0]*m[3] - m[1]*m[2]
	detPhase := real(cmplx.Log(det)) / 2 // imaginary part
	_ = detPhase
	phase := imag(cmplx.Log(det)) / 2
	factor := cmplx.Exp(complex(0, -phase))
	a := m[0] * factor
	b := m[1] * factor

	absA := cmplx.Abs(a)
	beta := 2 * math.Acos(clamp(absA, 0, 1))

	var alpha, gamma float64
	switch {
	case cmplx.Abs(b) < 1e-10:
		alpha = -2 * cmplx.Phase(a)
		gamma = 0
	case cmplx.Abs(a) < 1e-10:
		alpha = -2 * cmplx.Phase(-b)
		beta = math.Pi
		gamma = 0
	default:
		apg := cmplx.Phase(a)
		amg := cmplx.Phase(-b)
		alpha = -(apg + amg)
		gamma = -(apg - amg)
	}

	var ops []ir.Operation

	// C(tgt) = Rz((γ-α)/2) — applied first in circuit time
	if !dNearZero(gamma - alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ((gamma - alpha) / 2), Qubits: []int{target}})
	}

	// MCX
	ops = append(ops, decomposeMCXForDensity(controls, target)...)

	// B(tgt) = Ry(-β/2) · Rz(-(α+γ)/2); circuit order: Rz then Ry
	if !dNearZero(alpha + gamma) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(-(alpha + gamma) / 2), Qubits: []int{target}})
	}
	if !dNearZero(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(-beta / 2), Qubits: []int{target}})
	}

	// MCX
	ops = append(ops, decomposeMCXForDensity(controls, target)...)

	// A(tgt) = Rz(α) · Ry(β/2); circuit order: Ry then Rz
	if !dNearZero(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(beta / 2), Qubits: []int{target}})
	}
	if !dNearZero(alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(alpha), Qubits: []int{target}})
	}

	// Phase correction.
	if !dNearZero(phase) {
		n := len(controls)
		if n == 1 {
			ops = append(ops, ir.Operation{Gate: gate.Phase(phase), Qubits: []int{controls[0]}})
		} else {
			ops = append(ops, decomposeControlled1QForDensity(gate.Phase(phase), controls[:n-1], controls[n-1])...)
		}
	}

	return ops
}

func dNearZero(x float64) bool {
	return math.Abs(math.Remainder(x, 2*math.Pi)) < 1e-10
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
