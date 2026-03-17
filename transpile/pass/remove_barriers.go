// Package pass provides individual transpilation passes.
package pass

import (
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

// RemoveBarriers strips barrier pseudo-gates from the circuit.
func RemoveBarriers(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
	filtered := make([]ir.Operation, 0, c.NumOps())
	c.RangeOps(func(_ int, op ir.Operation) bool {
		if op.Gate == nil || op.Gate.Name() != "barrier" {
			filtered = append(filtered, op)
		}
		return true
	})
	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), filtered, c.Metadata()), nil
}
