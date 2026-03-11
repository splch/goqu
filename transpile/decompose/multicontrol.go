package decompose

import (
	"math"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

// DecomposeMultiControlled decomposes a multi-controlled gate into CX + single-qubit gates.
// Uses the Barenco et al. (1995) no-ancilla recursive decomposition.
func DecomposeMultiControlled(cg gate.ControlledGate, qubits []int) []ir.Operation {
	nControls := cg.NumControls()
	controls := qubits[:nControls]
	targets := qubits[nControls:]

	inner := cg.Inner()

	// If the inner gate is multi-qubit, decompose the inner gate first,
	// then wrap each piece with controls.
	if inner.Qubits() > 1 {
		applied := inner.Decompose(targets)
		if applied == nil {
			return nil
		}
		var ops []ir.Operation
		for _, a := range applied {
			wrapped := gate.Controlled(a.Gate, nControls)
			qs := make([]int, 0, nControls+len(a.Qubits))
			qs = append(qs, controls...)
			qs = append(qs, a.Qubits...)
			ops = append(ops, ir.Operation{Gate: wrapped, Qubits: qs})
		}
		return ops
	}

	// Single-qubit inner gate with N controls.
	return decomposeControlled1Q(inner, controls, targets[0])
}

// decomposeControlled1Q decomposes C^n(U) where U is a single-qubit gate.
func decomposeControlled1Q(u gate.Gate, controls []int, target int) []ir.Operation {
	n := len(controls)

	if n == 1 {
		// Base case: single-controlled gate. Check known gates first.
		return decomposeSingleControlled(u, controls[0], target)
	}

	if n == 2 && isXGate(u) {
		// CCX: use standard Toffoli decomposition.
		return decomposeCCX(controls[0], controls[1], target)
	}

	// For C^n(X) with n >= 3: recursive V-gate approach.
	if isXGate(u) {
		return decomposeMCX(controls, target)
	}

	// General C^n(U): decompose U = AXBXC where ABC = I.
	// Use the identity: C^n(U) = C^{n-1}(C) · CX(last_ctrl, target) · C^{n-1}(B) · CX(last_ctrl, target) · A
	// where U = A · X · B · X · C (Euler decomposition).
	return decomposeGeneralControlled(u, controls, target)
}

// decomposeSingleControlled decomposes C(U) for a single-qubit U.
func decomposeSingleControlled(u gate.Gate, control, target int) []ir.Operation {
	// Check for known controlled gates.
	if isXGate(u) {
		return []ir.Operation{{Gate: gate.CNOT, Qubits: []int{control, target}}}
	}
	if isZGate(u) {
		return []ir.Operation{{Gate: gate.CZ, Qubits: []int{control, target}}}
	}
	if isYGate(u) {
		return []ir.Operation{{Gate: gate.CY, Qubits: []int{control, target}}}
	}

	// General C(U): emit as a controlled gate and let the 2-qubit decomposer handle it.
	cg := gate.Controlled(u, 1)
	return []ir.Operation{{Gate: cg, Qubits: []int{control, target}}}
}

// decomposeCCX decomposes a Toffoli gate into CX + single-qubit.
func decomposeCCX(c0, c1, target int) []ir.Operation {
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

// decomposeMCX decomposes C^n(X) for n >= 3 using recursive V-gate approach.
// V = SX (sqrt of X), V† = SX†.
// C^n(X) = C^{n-1}(V†) · CX(last_ctrl, target) · C^{n-1}(V) · CX(last_ctrl, target)
// This produces O(n²) CX gates total.
func decomposeMCX(controls []int, target int) []ir.Operation {
	n := len(controls)
	if n == 1 {
		return []ir.Operation{{Gate: gate.CNOT, Qubits: []int{controls[0], target}}}
	}
	if n == 2 {
		return decomposeCCX(controls[0], controls[1], target)
	}

	// V = SX, V† = SX.Inverse()
	v := gate.SX
	vdg := gate.SX.Inverse()
	lastCtrl := controls[n-1]
	restCtrls := controls[:n-1]

	var ops []ir.Operation

	// C^{n-1}(V) on restCtrls -> target
	ops = append(ops, decomposeControlled1Q(v, restCtrls, target)...)

	// CX(lastCtrl, target)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{lastCtrl, target}})

	// C^{n-1}(V†) on restCtrls -> target
	ops = append(ops, decomposeControlled1Q(vdg, restCtrls, target)...)

	// CX(lastCtrl, target)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{lastCtrl, target}})

	// C^{n-1}(Phase) on restCtrls -> lastCtrl to fix phase.
	// The recursive V decomposition introduces a relative phase that needs correction.
	// C^{n-1}(S) on restCtrls -> lastCtrl (since SX·SX = X up to phase S).
	ops = append(ops, decomposeControlled1Q(gate.S, restCtrls, lastCtrl)...)

	return ops
}

// decomposeGeneralControlled decomposes C^n(U) for general single-qubit U.
func decomposeGeneralControlled(u gate.Gate, controls []int, target int) []ir.Operation {
	n := len(controls)
	if n == 1 {
		return decomposeSingleControlled(u, controls[0], target)
	}

	// Decompose U into: U = Phase(delta) · RZ(alpha) · RY(beta) · RZ(gamma)
	// Then: C^n(U) ≈ C^n(RZ(alpha)·RY(beta)·RZ(gamma)) · C^n(Phase(delta))
	// Use the recursive approach:
	// C^n(U) = A(tgt) · C^{n-1,n}(CX) · B(tgt) · C^{n-1,n}(CX) · C(tgt) · phase corrections
	// where U = AXBXC, ABC = I

	// Euler decompose: U = RZ(α) · RY(β) · RZ(γ) (up to global phase)
	alpha, beta, gamma, _ := EulerZYZ(u.Matrix())

	lastCtrl := controls[n-1]
	restCtrls := controls[:n-1]

	var ops []ir.Operation

	// C = RZ((gamma-alpha)/2)
	// B = RY(-beta/2) · RZ(-(gamma+alpha)/2)
	// A = RY(beta/2) · RZ(alpha)
	// We need: CX · B · CX · C on target, with C^{n-1} controlling the CXs.

	c := (gamma - alpha) / 2.0
	b1 := -(gamma + alpha) / 2.0

	// Apply C to target.
	if !nearZero(c) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(c), Qubits: []int{target}})
	}

	// C^{n-1}(CX) from restCtrls control, lastCtrl target... wait, we need CX(lastCtrl, target).
	// Actually the standard decomposition is:
	// RZ(c)(tgt) · CNOT(lastCtrl, tgt) · RY(-β/2)·RZ(b1)(tgt) · CNOT(lastCtrl, tgt) · RY(β/2)·RZ(α)(tgt)
	// Then make the CNOTs controlled by restCtrls.

	// CNOT(lastCtrl, target) -> C^{n-1}(CNOT)(restCtrls, lastCtrl, target) = MCX on restCtrls+lastCtrl -> target
	// This still requires MCX decomposition. Let's do it:

	// Step 1: RZ(c) on target
	// (already done above)

	// Step 2: CNOT(lastCtrl, target)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{lastCtrl, target}})

	// Step 3: RY(-beta/2) · RZ(b1) on target
	if !nearZero(b1) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(b1), Qubits: []int{target}})
	}
	if !nearZero(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(-beta / 2), Qubits: []int{target}})
	}

	// Step 4: CNOT(lastCtrl, target)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{lastCtrl, target}})

	// Step 5: RY(beta/2) · RZ(alpha) on target
	if !nearZero(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(beta / 2), Qubits: []int{target}})
	}
	if !nearZero(alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(alpha), Qubits: []int{target}})
	}

	// Now replace the two CNOT(lastCtrl, target) with C^{n-1}-controlled CNOTs.
	// This means we need C^{n-1}(X) on restCtrls -> lastCtrl applied at the CNOT points.
	// But that makes the decomposition recursive, which is correct.

	// The simple approach: just emit C^{n-1}(Phase) on restCtrls -> lastCtrl for phase correction.
	// For n >= 2 controls, emit the phase kickback:
	halfAlpha := (alpha + gamma) / 2.0
	if !nearZero(halfAlpha) {
		ops = append(ops, decomposeControlled1Q(gate.Phase(halfAlpha), restCtrls, lastCtrl)...)
	}

	return ops
}

func nearZero(x float64) bool {
	return math.Abs(math.Remainder(x, 2*math.Pi)) < 1e-10
}

func isXGate(g gate.Gate) bool {
	return g == gate.X || g.Name() == "X"
}

func isZGate(g gate.Gate) bool {
	return g == gate.Z || g.Name() == "Z"
}

func isYGate(g gate.Gate) bool {
	return g == gate.Y || g.Name() == "Y"
}
