package pass

import (
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/transpile/decompose"
	"github.com/splch/goqu/transpile/target"
)

// RemoveIdentity removes gates whose unitary matrix is the identity within
// numerical tolerance. This catches zero-angle rotations (RZ(0), RY(2*pi))
// and any gate that evaluates to identity after merging.
func RemoveIdentity(c *ir.Circuit, _ target.Target) (*ir.Circuit, error) {
	ops := c.Ops()
	var result []ir.Operation
	changed := false
	for _, op := range ops {
		if op.Gate == nil || op.Condition != nil || op.ControlFlow != nil {
			result = append(result, op)
			continue
		}
		name := op.Gate.Name()
		if name == "barrier" || name == "delay" || name == "reset" {
			result = append(result, op)
			continue
		}
		m := op.Gate.Matrix()
		if m == nil {
			result = append(result, op)
			continue
		}
		dim := 1 << op.Gate.Qubits()
		if decompose.IsIdentity(m, dim, 1e-10) {
			changed = true
			continue
		}
		result = append(result, op)
	}
	if !changed {
		return c, nil
	}
	return ir.New(c.Name(), c.NumQubits(), c.NumClbits(), result, c.Metadata()), nil
}
