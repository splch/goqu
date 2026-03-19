package pass

import (
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/internal/mathutil"
	"github.com/splch/goqu/transpile/target"
)

// CommuteThroughCNOT commutes single-qubit gates through CNOT gates
// to enable further cancellation and merging.
//
// Rules:
//   - Z/RZ/S/T/Phase commute through CNOT control
//   - X/RX commute through CNOT target
//
// Direction: push single-qubit gates forward (later in the circuit)
// past CNOTs when they commute, to bring same-type gates together.
func CommuteThroughCNOT(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
	ops := c.Ops()

	changed := true
	for changed {
		changed = false
		for i := 0; i < len(ops)-1; i++ {
			a := ops[i]
			b := ops[i+1]

			// Only Case: single-qubit gate followed by CNOT - push gate forward.
			if a.Gate == nil || a.Gate.Qubits() != 1 || b.Gate == nil || !isCNOT(b.Gate) {
				continue
			}

			q := a.Qubits[0]
			ctrl, tgt := b.Qubits[0], b.Qubits[1]

			canCommute := false
			if q == ctrl && commutesWithControl(a.Gate) {
				canCommute = true
			} else if q == tgt && commutesWithTarget(a.Gate) {
				canCommute = true
			}

			if canCommute {
				ops[i], ops[i+1] = ops[i+1], ops[i]
				changed = true
				i++ // skip the just-swapped pair to avoid re-swapping
			}
		}
	}

	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), ops, c.Metadata()), nil
}

func isCNOT(g gate.Gate) bool {
	return g == gate.CNOT || g.Name() == "CX"
}

// commutesWithControl: Z-type gates commute through CNOT control.
func commutesWithControl(g gate.Gate) bool {
	switch g {
	case gate.Z, gate.S, gate.Sdg, gate.T, gate.Tdg:
		return true
	}
	name := mathutil.StripParams(g.Name())
	return name == "RZ" || name == "P"
}

// commutesWithTarget: X-type gates commute through CNOT target.
func commutesWithTarget(g gate.Gate) bool {
	if g == gate.X {
		return true
	}
	name := mathutil.StripParams(g.Name())
	return name == "RX"
}
