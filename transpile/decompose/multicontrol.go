package decompose

import (
	"math"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
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
		return decomposeSingleControlled(u, controls[0], target)
	}

	if n == 2 && isXGate(u) {
		return decomposeCCX(controls[0], controls[1], target)
	}

	// For C^n(X) with n >= 3: recursive V-gate approach.
	if isXGate(u) {
		return decomposeMCX(controls, target)
	}

	// General C^n(U): reduce to C^n(X) + single-qubit gates.
	return decomposeGeneralControlled(u, controls, target)
}

// decomposeSingleControlled decomposes C(U) for a single-qubit U into CX + 1Q gates.
// Uses the standard decomposition: C(U) where U = e^{iα} · A · X · B · X · C, ABC = I.
// C(U) = C(tgt) · CX(ctrl,tgt) · B(tgt) · CX(ctrl,tgt) · A(tgt) · Phase(α)(ctrl)
func decomposeSingleControlled(u gate.Gate, control, target int) []ir.Operation {
	// Check for known controlled gates that are already primitive.
	if isXGate(u) {
		return []ir.Operation{{Gate: gate.CNOT, Qubits: []int{control, target}}}
	}
	if isZGate(u) {
		return []ir.Operation{{Gate: gate.CZ, Qubits: []int{control, target}}}
	}
	if isYGate(u) {
		return []ir.Operation{{Gate: gate.CY, Qubits: []int{control, target}}}
	}

	// Standard CU decomposition (Nielsen & Chuang, Theorem 4.3).
	// U = e^{iδ} · Rz(α) · Ry(β) · Rz(γ)  (Euler ZYZ)
	// Define: A = Rz(α) · Ry(β/2), B = Ry(-β/2) · Rz(-(α+γ)/2), C = Rz((γ-α)/2)
	// Then ABC = I and U = e^{iδ} · AXBXC.
	// Matrix form: C(U) = Phase(δ)(ctrl) · A(tgt) · CX · B(tgt) · CX · C(tgt)
	// Circuit time order (left→right): C, CX, B, CX, A, Phase
	alpha, beta, gamma, phase := EulerZYZ(u.Matrix())

	var ops []ir.Operation

	// C(tgt) = Rz((γ-α)/2) — applied first in circuit time
	if !nearZero(gamma - alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ((gamma - alpha) / 2), Qubits: []int{target}})
	}

	// CX(ctrl, tgt)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{control, target}})

	// B(tgt) = Ry(-β/2) · Rz(-(α+γ)/2); circuit order: Rz then Ry
	if !nearZero(alpha + gamma) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(-(alpha + gamma) / 2), Qubits: []int{target}})
	}
	if !nearZero(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(-beta / 2), Qubits: []int{target}})
	}

	// CX(ctrl, tgt)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{control, target}})

	// A(tgt) = Rz(α) · Ry(β/2); circuit order: Ry then Rz
	if !nearZero(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(beta / 2), Qubits: []int{target}})
	}
	if !nearZero(alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(alpha), Qubits: []int{target}})
	}

	// Phase(δ)(ctrl) for global phase correction.
	if !nearZero(phase) {
		ops = append(ops, ir.Operation{Gate: gate.Phase(phase), Qubits: []int{control}})
	}

	return ops
}

// decomposeCCX decomposes a Toffoli (CCX) gate into 6 CNOTs + single-qubit
// gates (H, T, Tdg). This is the standard Toffoli decomposition from
// Nielsen & Chuang, Figure 4.9.
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

// decomposeMCX decomposes C^n(X) for n >= 3 using the recursive V-gate
// approach from Barenco et al. (1995).
// V = SX (sqrt of X), V† = SX†.
// C^n(X) = C^{n-1}(V) · CX(last_ctrl, target) · C^{n-1}(V†) · CX(last_ctrl, target) · C^{n-1}(S)
// Each recursion level reduces the control count by one, producing O(n^2) CNOT
// gates total for an n-controlled X gate without ancilla qubits.
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

	var ops []ir.Operation //nolint:prealloc // size depends on recursive decomposition depth

	// C^{n-1}(V) on restCtrls -> target
	ops = append(ops, decomposeControlled1Q(v, restCtrls, target)...)

	// CX(lastCtrl, target)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{lastCtrl, target}})

	// C^{n-1}(V†) on restCtrls -> target
	ops = append(ops, decomposeControlled1Q(vdg, restCtrls, target)...)

	// CX(lastCtrl, target)
	ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{lastCtrl, target}})

	// C^{n-1}(S) on restCtrls -> lastCtrl to fix phase.
	ops = append(ops, decomposeControlled1Q(gate.S, restCtrls, lastCtrl)...)

	return ops
}

// decomposeGeneralControlled decomposes C^n(U) for general single-qubit U with n >= 2.
// Reduces to C^n(X) + single-qubit gates using:
// U = e^{iδ} · AXBXC where ABC = I (Euler decomposition).
// C^n(U) = Phase(δ)(ctrls) · A(tgt) · MCX(ctrls,tgt) · B(tgt) · MCX(ctrls,tgt) · C(tgt)
// Circuit time order (left→right): C, MCX, B, MCX, A, Phase + phase correction on controls.
func decomposeGeneralControlled(u gate.Gate, controls []int, target int) []ir.Operation {
	alpha, beta, gamma, phase := EulerZYZ(u.Matrix())

	var ops []ir.Operation

	// C(tgt) = Rz((γ-α)/2) — applied first in circuit time
	if !nearZero(gamma - alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ((gamma - alpha) / 2), Qubits: []int{target}})
	}

	// C^n(X) on all controls -> target
	ops = append(ops, decomposeMCX(controls, target)...)

	// B(tgt) = Ry(-β/2) · Rz(-(α+γ)/2); circuit order: Rz then Ry
	if !nearZero(alpha + gamma) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(-(alpha + gamma) / 2), Qubits: []int{target}})
	}
	if !nearZero(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(-beta / 2), Qubits: []int{target}})
	}

	// C^n(X) on all controls -> target
	ops = append(ops, decomposeMCX(controls, target)...)

	// A(tgt) = Rz(α) · Ry(β/2); circuit order: Ry then Rz
	if !nearZero(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(beta / 2), Qubits: []int{target}})
	}
	if !nearZero(alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(alpha), Qubits: []int{target}})
	}

	// Phase correction: C^{n-1}(Phase(δ)) on controls[:-1] -> controls[-1].
	if !nearZero(phase) {
		n := len(controls)
		if n == 1 {
			ops = append(ops, ir.Operation{Gate: gate.Phase(phase), Qubits: []int{controls[0]}})
		} else {
			ops = append(ops, decomposeControlled1Q(gate.Phase(phase), controls[:n-1], controls[n-1])...)
		}
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
