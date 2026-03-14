package pass

import (
	"fmt"

	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile"
	"github.com/splch/goqu/transpile/target"
)

// ValidateTarget checks that all gates are in the target basis, all 2-qubit
// pairs are connected, and circuit depth is within limits. Returns the
// circuit unchanged on success, or an error on the first violation.
func ValidateTarget(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
	if c.NumQubits() > t.NumQubits && t.NumQubits > 0 {
		return nil, fmt.Errorf("validate: circuit has %d qubits, target %q supports %d",
			c.NumQubits(), t.Name, t.NumQubits)
	}

	for i, op := range c.Ops() {
		if op.Gate == nil {
			continue // measurement or other non-gate op
		}
		name := op.Gate.Name()
		if name == "barrier" || name == "delay" {
			continue
		}

		// Check basis gate.
		bname := transpile.BasisName(op.Gate)
		if !t.HasBasisGate(bname) {
			return nil, fmt.Errorf("validate: op %d gate %q (basis %q) not in target %q basis %v",
				i, name, bname, t.Name, t.BasisGates)
		}

		// Check connectivity for multi-qubit gates.
		if len(op.Qubits) >= 2 && t.Connectivity != nil {
			q0, q1 := op.Qubits[0], op.Qubits[1]
			if !t.IsConnected(q0, q1) {
				return nil, fmt.Errorf("validate: op %d gate %q on qubits (%d,%d) not connected in target %q",
					i, name, q0, q1, t.Name)
			}
		}
	}

	// Check depth limit.
	if t.MaxCircuitDepth > 0 {
		depth := c.Stats().Depth
		if depth > t.MaxCircuitDepth {
			return nil, fmt.Errorf("validate: circuit depth %d exceeds target %q limit %d",
				depth, t.Name, t.MaxCircuitDepth)
		}
	}

	return c, nil
}
