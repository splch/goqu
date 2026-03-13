package decompose

import (
	"math"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/internal/mathutil"
)

// DecomposeByRule returns a basis-gate decomposition for known gates.
// Returns nil if no rule applies for the given basis set.
func DecomposeByRule(g gate.Gate, qubits []int, basisGates []string) []ir.Operation {
	basis := make(map[string]bool, len(basisGates))
	for _, b := range basisGates {
		basis[b] = true
	}

	if basis["CX"] || basis["CNOT"] {
		return decomposeToCX(g, qubits, basis)
	}
	if basis["MS"] {
		return decomposeToIonQ(g, qubits, basis)
	}
	return nil
}

// decomposeToCX decomposes known gates into CX + single-qubit basis.
func decomposeToCX(g gate.Gate, qubits []int, basis map[string]bool) []ir.Operation {
	// Check for multi-controlled gates first.
	if cg, ok := g.(gate.ControlledGate); ok {
		return DecomposeMultiControlled(cg, qubits)
	}
	switch g.Qubits() {
	case 1:
		return decompose1qToCXBasis(g, qubits, basis)
	case 2:
		return decompose2qToCX(g, qubits)
	case 3:
		return decompose3qToCX(g, qubits)
	}
	return nil
}

// decompose1qToCXBasis decomposes single-qubit gates to IBM-style basis {RZ, SX, X}.
func decompose1qToCXBasis(g gate.Gate, qubits []int, basis map[string]bool) []ir.Operation {
	q := qubits[0]

	switch g {
	case gate.H:
		// H = RZ(π/2)·SX·RZ(π/2)
		if basis["SX"] && basis["RZ"] {
			return []ir.Operation{
				{Gate: gate.RZ(math.Pi / 2), Qubits: []int{q}},
				{Gate: gate.SX, Qubits: []int{q}},
				{Gate: gate.RZ(math.Pi / 2), Qubits: []int{q}},
			}
		}
	case gate.S:
		if basis["RZ"] {
			return []ir.Operation{{Gate: gate.RZ(math.Pi / 2), Qubits: []int{q}}}
		}
	case gate.Sdg:
		if basis["RZ"] {
			return []ir.Operation{{Gate: gate.RZ(-math.Pi / 2), Qubits: []int{q}}}
		}
	case gate.T:
		if basis["RZ"] {
			return []ir.Operation{{Gate: gate.RZ(math.Pi / 4), Qubits: []int{q}}}
		}
	case gate.Tdg:
		if basis["RZ"] {
			return []ir.Operation{{Gate: gate.RZ(-math.Pi / 4), Qubits: []int{q}}}
		}
	case gate.Y:
		if basis["X"] && basis["RZ"] {
			return []ir.Operation{
				{Gate: gate.RZ(math.Pi), Qubits: []int{q}},
				{Gate: gate.X, Qubits: []int{q}},
			}
		}
	case gate.Z:
		if basis["RZ"] {
			return []ir.Operation{{Gate: gate.RZ(math.Pi), Qubits: []int{q}}}
		}
	case gate.I:
		// Identity: no ops needed.
		return []ir.Operation{}
	}

	// Parameterized single-qubit gates: decompose via Euler if not in basis.
	params := g.Params()
	if params == nil {
		return nil
	}

	name := mathutil.StripParamsAndDagger(g.Name())
	switch name {
	case "RX":
		// RX(θ) = H·RZ(θ)·H = RZ(π/2)·SX·RZ(θ+π)·SX·RZ(π/2)
		if basis["RZ"] && basis["SX"] {
			theta := params[0]
			return []ir.Operation{
				{Gate: gate.RZ(math.Pi / 2), Qubits: []int{q}},
				{Gate: gate.SX, Qubits: []int{q}},
				{Gate: gate.RZ(theta + math.Pi), Qubits: []int{q}},
				{Gate: gate.SX, Qubits: []int{q}},
				{Gate: gate.RZ(math.Pi / 2), Qubits: []int{q}},
			}
		}
	case "RY":
		// RY(θ) = SX·RZ(θ)·SX†  or via Euler
		if basis["RZ"] && basis["SX"] {
			theta := params[0]
			return []ir.Operation{
				{Gate: gate.SX, Qubits: []int{q}},
				{Gate: gate.RZ(theta), Qubits: []int{q}},
				{Gate: gate.SX.Inverse(), Qubits: []int{q}},
			}
		}
	case "P":
		// Phase(φ) = RZ(φ) up to global phase.
		if basis["RZ"] {
			return []ir.Operation{{Gate: gate.RZ(params[0]), Qubits: []int{q}}}
		}
	case "U3":
		// U3(θ,φ,λ) = RZ(φ+π)·SX·RZ(θ+π)·SX·RZ(λ) for IBM basis
		if basis["RZ"] && basis["SX"] {
			theta, phi, lambda := params[0], params[1], params[2]
			return []ir.Operation{
				{Gate: gate.RZ(lambda), Qubits: []int{q}},
				{Gate: gate.SX, Qubits: []int{q}},
				{Gate: gate.RZ(theta + math.Pi), Qubits: []int{q}},
				{Gate: gate.SX, Qubits: []int{q}},
				{Gate: gate.RZ(phi + math.Pi), Qubits: []int{q}},
			}
		}
	}

	return nil
}

// decompose2qToCX decomposes known 2-qubit gates to CX + 1-qubit gates.
func decompose2qToCX(g gate.Gate, qubits []int) []ir.Operation {
	q0, q1 := qubits[0], qubits[1]

	switch g {
	case gate.SWAP:
		// SWAP = CX(0,1)·CX(1,0)·CX(0,1)
		return []ir.Operation{
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.CNOT, Qubits: []int{q1, q0}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		}
	case gate.CZ:
		// CZ = H(target)·CX·H(target)
		return []ir.Operation{
			{Gate: gate.H, Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.H, Qubits: []int{q1}},
		}
	case gate.CY:
		// CY = Sdg(target)·CX·S(target)
		return []ir.Operation{
			{Gate: gate.Sdg, Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.S, Qubits: []int{q1}},
		}
	}

	// Parameterized 2-qubit gates.
	params := g.Params()
	if params == nil {
		return nil
	}
	name := mathutil.StripParamsAndDagger(g.Name())
	switch name {
	case "CP":
		// CP(φ) = RZ(φ/2)(q0)·CX(q0,q1)·RZ(-φ/2)(q1)·CX(q0,q1)·RZ(φ/2)(q1)
		phi := params[0]
		return []ir.Operation{
			{Gate: gate.RZ(phi / 2), Qubits: []int{q0}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.RZ(-phi / 2), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.RZ(phi / 2), Qubits: []int{q1}},
		}
	case "CRZ":
		// CRZ(θ) = RZ(θ/2)(q1)·CX(q0,q1)·RZ(-θ/2)(q1)·CX(q0,q1)
		theta := params[0]
		return []ir.Operation{
			{Gate: gate.RZ(theta / 2), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.RZ(-theta / 2), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		}
	case "CRX":
		// CRX(θ) = H(q1)·CRZ(θ)·H(q1)
		theta := params[0]
		return []ir.Operation{
			{Gate: gate.H, Qubits: []int{q1}},
			{Gate: gate.RZ(theta / 2), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.RZ(-theta / 2), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.H, Qubits: []int{q1}},
		}
	case "CRY":
		// CRY(θ) = RY(θ/2)(q1)·CX(q0,q1)·RY(-θ/2)(q1)·CX(q0,q1)
		theta := params[0]
		return []ir.Operation{
			{Gate: gate.RY(theta / 2), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.RY(-theta / 2), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		}
	case "RZZ":
		// RZZ(θ) = CX(q0,q1)·RZ(θ,q1)·CX(q0,q1)
		theta := params[0]
		return []ir.Operation{
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.RZ(theta), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		}
	case "RXX":
		// RXX(θ) = H(q0)·H(q1)·CX(q0,q1)·RZ(θ,q1)·CX(q0,q1)·H(q0)·H(q1)
		theta := params[0]
		return []ir.Operation{
			{Gate: gate.H, Qubits: []int{q0}},
			{Gate: gate.H, Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.RZ(theta), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.H, Qubits: []int{q0}},
			{Gate: gate.H, Qubits: []int{q1}},
		}
	case "RYY":
		// RYY(θ) = RX(π/2,q0)·RX(π/2,q1)·CX(q0,q1)·RZ(θ,q1)·CX(q0,q1)·RX(-π/2,q0)·RX(-π/2,q1)
		theta := params[0]
		return []ir.Operation{
			{Gate: gate.RX(math.Pi / 2), Qubits: []int{q0}},
			{Gate: gate.RX(math.Pi / 2), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.RZ(theta), Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.RX(-math.Pi / 2), Qubits: []int{q0}},
			{Gate: gate.RX(-math.Pi / 2), Qubits: []int{q1}},
		}
	}
	return nil
}

// decompose3qToCX decomposes known 3-qubit gates to CX + 1-qubit gates.
func decompose3qToCX(g gate.Gate, qubits []int) []ir.Operation {
	q0, q1, q2 := qubits[0], qubits[1], qubits[2]

	switch g {
	case gate.CCX:
		// Toffoli decomposition into 6 CX + single-qubit gates.
		return []ir.Operation{
			{Gate: gate.H, Qubits: []int{q2}},
			{Gate: gate.CNOT, Qubits: []int{q1, q2}},
			{Gate: gate.Tdg, Qubits: []int{q2}},
			{Gate: gate.CNOT, Qubits: []int{q0, q2}},
			{Gate: gate.T, Qubits: []int{q2}},
			{Gate: gate.CNOT, Qubits: []int{q1, q2}},
			{Gate: gate.Tdg, Qubits: []int{q2}},
			{Gate: gate.CNOT, Qubits: []int{q0, q2}},
			{Gate: gate.T, Qubits: []int{q1}},
			{Gate: gate.T, Qubits: []int{q2}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
			{Gate: gate.H, Qubits: []int{q2}},
			{Gate: gate.T, Qubits: []int{q0}},
			{Gate: gate.Tdg, Qubits: []int{q1}},
			{Gate: gate.CNOT, Qubits: []int{q0, q1}},
		}
	case gate.CSWAP:
		// Fredkin = CX(q2,q1)·CCX(q0,q1,q2)·CX(q2,q1)
		ccxOps := decompose3qToCX(gate.CCX, []int{q0, q1, q2})
		ops := make([]ir.Operation, 0, 1+len(ccxOps)+1)
		ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{q2, q1}})
		ops = append(ops, ccxOps...)
		ops = append(ops, ir.Operation{Gate: gate.CNOT, Qubits: []int{q2, q1}})
		return ops
	}
	return nil
}

// decomposeToIonQ decomposes gates to {GPI, GPI2, MS} basis.
func decomposeToIonQ(g gate.Gate, qubits []int, _ map[string]bool) []ir.Operation {
	switch g.Qubits() {
	case 1:
		return decompose1qToIonQ(g, qubits)
	case 2:
		return decompose2qToIonQ(g, qubits)
	case 3:
		return decompose3qToIonQ(g, qubits)
	}
	return nil
}

// decompose1qToIonQ decomposes single-qubit gates to GPI/GPI2 sequences.
func decompose1qToIonQ(g gate.Gate, qubits []int) []ir.Operation {
	q := qubits[0]

	// GPI2(φ) is a π/2 rotation about axis at angle φ in XY plane.
	// For IonQ native: single-qubit gates are combinations of GPI and GPI2.

	switch g {
	case gate.I:
		return []ir.Operation{}
	case gate.X:
		// X = GPI(0)
		return []ir.Operation{{Gate: gate.GPI(0), Qubits: []int{q}}}
	case gate.Y:
		// Y = GPI(π/2)
		return []ir.Operation{{Gate: gate.GPI(math.Pi / 2), Qubits: []int{q}}}
	case gate.Z:
		// Z = GPI(0)·GPI(π/2) (up to global phase)
		// Or: Z = GPI2(0)·GPI2(0) (more efficient)
		return []ir.Operation{
			{Gate: gate.GPI(0), Qubits: []int{q}},
			{Gate: gate.GPI(math.Pi / 2), Qubits: []int{q}},
		}
	case gate.H:
		// H = GPI(0)·GPI2(π/2)  (GPI2 is √X-like rotation)
		return []ir.Operation{
			{Gate: gate.GPI(0), Qubits: []int{q}},
			{Gate: gate.GPI2(math.Pi / 2), Qubits: []int{q}},
		}
	}

	// For other single-qubit gates, use Euler ZYZ decomposition then
	// convert each RZ/RY to GPI/GPI2 sequences.
	return euler1qToIonQ(g, q)
}

// euler1qToIonQ converts a single-qubit gate via Euler angles to IonQ natives.
func euler1qToIonQ(g gate.Gate, q int) []ir.Operation {
	alpha, beta, gamma, _ := EulerZYZ(g.Matrix())
	var ops []ir.Operation

	// RZ(γ) → virtual Z rotation: GPI(γ/2)·GPI(0)
	if !mathutil.NearZeroMod2Pi(gamma) {
		ops = append(ops, rzToIonQ(gamma, q)...)
	}
	// RY(β) → GPI2(π/2)·RZ(β)·GPI2(-π/2) → GPI2(π/2)·GPI(β/2)·GPI(0)·GPI2(-π/2)
	if !mathutil.NearZeroMod2Pi(beta) {
		ops = append(ops,
			ir.Operation{Gate: gate.GPI2(0), Qubits: []int{q}},
		)
		ops = append(ops, rzToIonQ(beta, q)...)
		ops = append(ops,
			ir.Operation{Gate: gate.GPI2(math.Pi), Qubits: []int{q}},
		)
	}
	// RZ(α)
	if !mathutil.NearZeroMod2Pi(alpha) {
		ops = append(ops, rzToIonQ(alpha, q)...)
	}
	return ops
}

// rzToIonQ converts RZ(θ) to IonQ native gates.
// RZ(θ) = GPI(θ/(2π))·GPI(0) up to global phase.
// More precisely: use GPI2 pair: GPI2(φ+π)·GPI2(φ) = RZ(2φ+π)
// So RZ(θ) = GPI2((θ-π)/2 + π)·GPI2((θ-π)/2) ... this gets complex.
// Simplest correct: GPI(θ/2)·GPI(0) = [[0,e^{-iθ/2}],[e^{iθ/2},0]]·[[0,1],[1,0]]
//
//	= [[e^{-iθ/2},0],[0,e^{iθ/2}]] which is exactly RZ(θ)!
func rzToIonQ(theta float64, q int) []ir.Operation {
	return []ir.Operation{
		{Gate: gate.GPI(theta / 2), Qubits: []int{q}},
		{Gate: gate.GPI(0), Qubits: []int{q}},
	}
}

// decompose2qToIonQ decomposes 2-qubit gates to MS-based sequences.
func decompose2qToIonQ(g gate.Gate, qubits []int) []ir.Operation {
	q0, q1 := qubits[0], qubits[1]

	switch g {
	case gate.CNOT:
		// CNOT = GPI2(q1,π/2)·MS(0,0)·GPI(q0,0)·GPI2(q0,-π/2)·GPI2(q1,-π/2)
		// Simplified standard IonQ CNOT decomposition:
		return []ir.Operation{
			{Gate: gate.GPI2(-math.Pi / 2), Qubits: []int{q0}},
			{Gate: gate.MS(0, 0), Qubits: []int{q0, q1}},
			{Gate: gate.GPI(0), Qubits: []int{q0}},
			{Gate: gate.GPI2(-math.Pi / 2), Qubits: []int{q0}},
			{Gate: gate.GPI2(-math.Pi / 2), Qubits: []int{q1}},
		}
	case gate.CZ:
		// CZ = (1q gates)·MS·(1q gates)
		// Decompose as CNOT with Hadamard wrapper.
		cnotOps := decompose2qToIonQ(gate.CNOT, qubits)
		var ops []ir.Operation
		ops = append(ops, decompose1qToIonQ(gate.H, []int{q1})...)
		ops = append(ops, cnotOps...)
		ops = append(ops, decompose1qToIonQ(gate.H, []int{q1})...)
		return ops
	case gate.SWAP:
		// SWAP = 3 CNOTs
		cnot01 := decompose2qToIonQ(gate.CNOT, []int{q0, q1})
		cnot10 := decompose2qToIonQ(gate.CNOT, []int{q1, q0})
		ops := make([]ir.Operation, 0, len(cnot01)+len(cnot10)+len(cnot01))
		ops = append(ops, cnot01...)
		ops = append(ops, cnot10...)
		ops = append(ops, cnot01...)
		return ops
	}

	// For other 2-qubit gates, decompose to CNOT first, then CNOT to IonQ.
	cxOps := decompose2qToCX(g, qubits)
	if cxOps == nil {
		return nil
	}
	return expandOpsToIonQ(cxOps)
}

// decompose3qToIonQ decomposes 3-qubit gates via CX decomposition then to IonQ.
func decompose3qToIonQ(g gate.Gate, qubits []int) []ir.Operation {
	cxOps := decompose3qToCX(g, qubits)
	if cxOps == nil {
		return nil
	}
	return expandOpsToIonQ(cxOps)
}

// expandOpsToIonQ recursively expands CX-basis ops to IonQ native gates.
func expandOpsToIonQ(ops []ir.Operation) []ir.Operation {
	var result []ir.Operation
	ionqBasis := map[string]bool{"GPI": true, "GPI2": true, "MS": true}
	for _, op := range ops {
		if op.Gate == nil {
			result = append(result, op)
			continue
		}
		name := op.Gate.Name()
		// Check if already IonQ native.
		bn := mathutil.StripParamsAndDagger(op.Gate.Name())
		if ionqBasis[bn] {
			result = append(result, op)
			continue
		}
		// Decompose further.
		switch {
		case name == "CNOT" || name == "CX":
			result = append(result, decompose2qToIonQ(gate.CNOT, op.Qubits)...)
		case op.Gate.Qubits() == 1:
			sub := decompose1qToIonQ(op.Gate, op.Qubits)
			if sub != nil {
				result = append(result, sub...)
			} else {
				result = append(result, op)
			}
		default:
			result = append(result, op)
		}
	}
	return result
}
