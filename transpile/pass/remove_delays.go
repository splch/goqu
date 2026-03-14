package pass

import (
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/target"
)

// RemoveDelays strips delay pseudo-gates from the circuit.
func RemoveDelays(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
	ops := c.Ops()
	filtered := make([]ir.Operation, 0, len(ops))
	for _, op := range ops {
		if op.Gate != nil && op.Gate.Name() == "delay" {
			continue
		}
		filtered = append(filtered, op)
	}
	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), filtered, c.Metadata()), nil
}
