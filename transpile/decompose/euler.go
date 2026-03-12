package decompose

import (
	"math"
	"math/cmplx"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/internal/mathutil"
)

// EulerBasis selects the Euler decomposition convention.
type EulerBasis int

const (
	BasisZYZ EulerBasis = iota // RZ · RY · RZ (default, Quantinuum-native)
	BasisZXZ                   // RZ · RX · RZ
	BasisZSX                   // IBM-native: {RZ, SX, X}
	BasisXYX                   // RX · RY · RX
	BasisXZX                   // RX · RZ · RX
	BasisU3                    // U3(θ,φ,λ) single gate
)

// BasisForTarget selects the optimal Euler convention for a target's basis gates.
func BasisForTarget(basisGates []string) EulerBasis {
	basis := make(map[string]bool, len(basisGates))
	for _, b := range basisGates {
		basis[b] = true
	}
	if basis["*"] {
		return BasisZYZ
	}
	if basis["SX"] && basis["RZ"] {
		return BasisZSX
	}
	if basis["RX"] && basis["RZ"] {
		return BasisZXZ
	}
	if basis["U3"] {
		return BasisU3
	}
	if basis["RX"] && basis["RY"] {
		return BasisXYX
	}
	return BasisZYZ
}

// EulerDecomposeForBasis decomposes a single-qubit gate using the given Euler convention.
func EulerDecomposeForBasis(g gate.Gate, qubit int, basis EulerBasis) []ir.Operation {
	if g.Qubits() != 1 {
		return nil
	}
	switch basis {
	case BasisZSX:
		return eulerZSX(g.Matrix(), qubit)
	case BasisZXZ:
		return eulerZXZ(g.Matrix(), qubit)
	case BasisXYX:
		return eulerXYX(g.Matrix(), qubit)
	case BasisXZX:
		return eulerXZX(g.Matrix(), qubit)
	case BasisU3:
		return eulerU3(g.Matrix(), qubit)
	default:
		return EulerDecompose(g, qubit)
	}
}

// EulerZYZ decomposes a 2×2 unitary U into Rz(alpha)·Ry(beta)·Rz(gamma)
// plus a global phase: U = e^{i·phase} · Rz(alpha) · Ry(beta) · Rz(gamma).
func EulerZYZ(m []complex128) (alpha, beta, gamma, phase float64) {
	// Normalize to SU(2).
	det := Det2x2(m)
	detPhase := cmplx.Phase(det) / 2
	factor := cmplx.Exp(complex(0, -detPhase))
	// su2 = m * e^{-i*detPhase}
	a := m[0] * factor // cos(beta/2) * e^{i(alpha+gamma)/2}
	b := m[1] * factor // -sin(beta/2) * e^{i(alpha-gamma)/2}

	absB := cmplx.Abs(b)
	beta = 2 * math.Acos(clamp(cmplx.Abs(a), 0, 1))

	switch {
	case absB < 1e-10:
		// Near identity: Rz(alpha+gamma). Assign all to alpha.
		// a = e^{-i(α+γ)/2}, so Phase(a) = -(α+γ)/2, thus α = -2·Phase(a).
		alpha = -2 * cmplx.Phase(a)
		beta = 0
		gamma = 0
	case cmplx.Abs(a) < 1e-10:
		// beta ≈ π: cos(β/2) ≈ 0, so use b.
		// b = -sin(β/2)·e^{-i(α-γ)/2}, Phase(-b) = -(α-γ)/2
		alpha = -2 * cmplx.Phase(-b)
		beta = math.Pi
		gamma = 0
	default:
		// General case.
		// a = cos(β/2) · e^{-i(α+γ)/2}  →  Phase(a) = -(α+γ)/2
		// -b = sin(β/2) · e^{-i(α-γ)/2}  →  Phase(-b) = -(α-γ)/2
		apg := cmplx.Phase(a)  // -(alpha+gamma)/2
		amg := cmplx.Phase(-b) // -(alpha-gamma)/2
		alpha = -(apg + amg)
		gamma = -(apg - amg)
	}

	phase = detPhase
	return
}

// EulerDecompose returns the gate sequence for a single-qubit gate on the given qubit,
// decomposed as Rz(alpha)·Ry(beta)·Rz(gamma). Skips identity rotations.
func EulerDecompose(g gate.Gate, qubit int) []ir.Operation {
	if g.Qubits() != 1 {
		return nil
	}
	alpha, beta, gamma, _ := EulerZYZ(g.Matrix())

	var ops []ir.Operation
	if !mathutil.NearZeroMod2Pi(gamma) {
		ops = append(ops, ir.Operation{
			Gate:   gate.RZ(mathutil.NormalizeAngle(gamma)),
			Qubits: []int{qubit},
		})
	}
	if !mathutil.NearZeroMod2Pi(beta) {
		ops = append(ops, ir.Operation{
			Gate:   gate.RY(mathutil.NormalizeAngle(beta)),
			Qubits: []int{qubit},
		})
	}
	if !mathutil.NearZeroMod2Pi(alpha) {
		ops = append(ops, ir.Operation{
			Gate:   gate.RZ(mathutil.NormalizeAngle(alpha)),
			Qubits: []int{qubit},
		})
	}

	if len(ops) == 0 {
		// Gate is effectively identity; no operations needed.
		return nil
	}
	return ops
}

// clamp restricts v to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// EulerZXZ decomposes a 2×2 unitary U into Rz(alpha)·Rx(beta)·Rz(gamma)
// plus a global phase: U = e^{i·phase} · Rz(alpha) · Rx(beta) · Rz(gamma).
func EulerZXZ(m []complex128) (alpha, beta, gamma, phase float64) {
	det := Det2x2(m)
	detPhase := cmplx.Phase(det) / 2
	factor := cmplx.Exp(complex(0, -detPhase))
	a := m[0] * factor // cos(beta/2) * e^{-i(alpha+gamma)/2}
	b := m[1] * factor // -i*sin(beta/2) * e^{-i(alpha-gamma)/2}

	beta = 2 * math.Acos(clamp(cmplx.Abs(a), 0, 1))

	switch {
	case cmplx.Abs(b) < 1e-10:
		alpha = -2 * cmplx.Phase(a)
		beta = 0
		gamma = 0
	case cmplx.Abs(a) < 1e-10:
		// beta ≈ π: b = -i·sin(β/2)·e^{-i(α-γ)/2}, so i·b = sin(β/2)·e^{-i(α-γ)/2}.
		alpha = -2 * cmplx.Phase(1i*b)
		beta = math.Pi
		gamma = 0
	default:
		// a = cos(β/2) · e^{-i(α+γ)/2}  →  Phase(a) = -(α+γ)/2
		// b = -i·sin(β/2) · e^{-i(α-γ)/2}, so i·b = sin(β/2)·e^{-i(α-γ)/2}
		// Phase(i·b) = -(α-γ)/2
		apg := cmplx.Phase(a)      // -(alpha+gamma)/2
		amg := cmplx.Phase(1i * b) // -(alpha-gamma)/2
		alpha = -(apg + amg)
		gamma = -(apg - amg)
	}

	phase = detPhase
	return
}

// eulerZXZ decomposes a 2×2 unitary into RZ·RX·RZ operations.
func eulerZXZ(m []complex128, qubit int) []ir.Operation {
	if IsIdentity(m, 2, 1e-10) {
		return nil
	}
	alpha, beta, gamma, _ := EulerZXZ(m)

	var ops []ir.Operation
	if !mathutil.NearZeroMod2Pi(gamma) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(mathutil.NormalizeAngle(gamma)), Qubits: []int{qubit}})
	}
	if !mathutil.NearZeroMod2Pi(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RX(mathutil.NormalizeAngle(beta)), Qubits: []int{qubit}})
	}
	if !mathutil.NearZeroMod2Pi(alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(mathutil.NormalizeAngle(alpha)), Qubits: []int{qubit}})
	}
	if len(ops) == 0 {
		return nil
	}
	return ops
}

// eulerZSX decomposes a 2×2 unitary into IBM-native {RZ, SX, X} operations.
// Uses ZYZ angles then converts with special-case reductions:
//
//	Case 1: beta ≈ 0       → RZ(alpha+gamma)                           [1 gate]
//	Case 2: beta ≈ π/2     → RZ(gamma-π/2), SX, RZ(alpha+π/2)         [3 gates]
//	Case 3: beta ≈ π       → RZ(gamma-π), X, RZ(alpha)                 [3 gates]
//	Case 4: general         → RZ(gamma), SX, RZ(beta+π), SX, RZ(alpha+π)  [5 gates]
func eulerZSX(m []complex128, qubit int) []ir.Operation {
	if IsIdentity(m, 2, 1e-10) {
		return nil
	}
	alpha, beta, gamma, _ := EulerZYZ(m)

	var ops []ir.Operation
	addRZ := func(angle float64) {
		a := mathutil.NormalizeAngle(angle)
		if !mathutil.NearZeroMod2Pi(a) {
			ops = append(ops, ir.Operation{Gate: gate.RZ(a), Qubits: []int{qubit}})
		}
	}

	switch {
	case mathutil.NearZeroMod2Pi(beta):
		// Case 1: diagonal unitary → single RZ
		addRZ(alpha + gamma)

	case math.Abs(beta-math.Pi/2) < 1e-8:
		// Case 2: beta ≈ π/2
		addRZ(gamma - math.Pi/2)
		ops = append(ops, ir.Operation{Gate: gate.SX, Qubits: []int{qubit}})
		addRZ(alpha + math.Pi/2)

	case math.Abs(beta-math.Pi) < 1e-8:
		// Case 3: beta ≈ π → X-like rotation
		addRZ(gamma - math.Pi)
		ops = append(ops, ir.Operation{Gate: gate.X, Qubits: []int{qubit}})
		addRZ(alpha)

	default:
		// Case 4: general → 5-gate decomposition
		// U = RZ(alpha) · RY(beta) · RZ(gamma)
		//   = RZ(alpha+π) · SX · RZ(beta+π) · SX · RZ(gamma)
		// using identity: RY(β) = RZ(π/2)·RX(β)·RZ(-π/2)
		//                       = RZ(π)·SX·RZ(β+π)·SX·RZ(0)
		addRZ(gamma)
		ops = append(ops, ir.Operation{Gate: gate.SX, Qubits: []int{qubit}})
		addRZ(beta + math.Pi)
		ops = append(ops, ir.Operation{Gate: gate.SX, Qubits: []int{qubit}})
		addRZ(alpha + math.Pi)
	}

	if len(ops) == 0 {
		return nil
	}
	return ops
}

// EulerXYX decomposes a 2×2 unitary U into Rx(alpha)·Ry(beta)·Rx(gamma)
// plus a global phase. Uses conjugation: XYX angles of U = ZYZ angles of Ry(-π/2)·U·Ry(π/2).
func EulerXYX(m []complex128) (alpha, beta, gamma, phase float64) {
	ry := ryMat(math.Pi / 2)
	ryInv := ryMat(-math.Pi / 2)
	mp := matMul2(ryInv, matMul2(m, ry))
	return EulerZYZ(mp)
}

// EulerXZX decomposes a 2×2 unitary U into Rx(alpha)·Rz(beta)·Rx(gamma)
// plus a global phase. Uses conjugation: XZX angles of U = ZXZ angles of H·U·H.
func EulerXZX(m []complex128) (alpha, beta, gamma, phase float64) {
	h := hMat()
	mp := matMul2(h, matMul2(m, h))
	return EulerZXZ(mp)
}

// eulerXYX decomposes a 2×2 unitary into RX·RY·RX operations.
func eulerXYX(m []complex128, qubit int) []ir.Operation {
	if IsIdentity(m, 2, 1e-10) {
		return nil
	}
	alpha, beta, gamma, _ := EulerXYX(m)

	var ops []ir.Operation
	if !mathutil.NearZeroMod2Pi(gamma) {
		ops = append(ops, ir.Operation{Gate: gate.RX(mathutil.NormalizeAngle(gamma)), Qubits: []int{qubit}})
	}
	if !mathutil.NearZeroMod2Pi(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RY(mathutil.NormalizeAngle(beta)), Qubits: []int{qubit}})
	}
	if !mathutil.NearZeroMod2Pi(alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RX(mathutil.NormalizeAngle(alpha)), Qubits: []int{qubit}})
	}
	if len(ops) == 0 {
		return nil
	}
	return ops
}

// eulerXZX decomposes a 2×2 unitary into RX·RZ·RX operations.
func eulerXZX(m []complex128, qubit int) []ir.Operation {
	if IsIdentity(m, 2, 1e-10) {
		return nil
	}
	alpha, beta, gamma, _ := EulerXZX(m)

	var ops []ir.Operation
	if !mathutil.NearZeroMod2Pi(gamma) {
		ops = append(ops, ir.Operation{Gate: gate.RX(mathutil.NormalizeAngle(gamma)), Qubits: []int{qubit}})
	}
	if !mathutil.NearZeroMod2Pi(beta) {
		ops = append(ops, ir.Operation{Gate: gate.RZ(mathutil.NormalizeAngle(beta)), Qubits: []int{qubit}})
	}
	if !mathutil.NearZeroMod2Pi(alpha) {
		ops = append(ops, ir.Operation{Gate: gate.RX(mathutil.NormalizeAngle(alpha)), Qubits: []int{qubit}})
	}
	if len(ops) == 0 {
		return nil
	}
	return ops
}

// eulerU3 decomposes a 2×2 unitary into a single U3 gate (or Phase for diagonal).
func eulerU3(m []complex128, qubit int) []ir.Operation {
	if IsIdentity(m, 2, 1e-10) {
		return nil
	}
	alpha, beta, gamma, _ := EulerZYZ(m)
	if mathutil.NearZeroMod2Pi(beta) {
		angle := mathutil.NormalizeAngle(alpha + gamma)
		if mathutil.NearZeroMod2Pi(angle) {
			return nil
		}
		return []ir.Operation{{Gate: gate.Phase(angle), Qubits: []int{qubit}}}
	}
	return []ir.Operation{{Gate: gate.U3(
		mathutil.NormalizeAngle(beta),
		mathutil.NormalizeAngle(alpha),
		mathutil.NormalizeAngle(gamma),
	), Qubits: []int{qubit}}}
}
