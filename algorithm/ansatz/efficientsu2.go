package ansatz

import (
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
)

// EfficientSU2 is an ansatz with RY+RZ rotation layers and CNOT entanglement.
// The circuit has 2*numQubits*(reps+1) parameters.
type EfficientSU2 struct {
	numQubits int
	reps      int
	ent       Entanglement
	vec       *param.Vector
}

// NewEfficientSU2 creates an EfficientSU2 ansatz.
func NewEfficientSU2(numQubits, reps int, ent Entanglement) *EfficientSU2 {
	nParams := 2 * numQubits * (reps + 1)
	return &EfficientSU2{
		numQubits: numQubits,
		reps:      reps,
		ent:       ent,
		vec:       param.NewVector("θ", nParams),
	}
}

func (es *EfficientSU2) NumParams() int             { return es.vec.Size() }
func (es *EfficientSU2) ParamVector() *param.Vector { return es.vec }

func (es *EfficientSU2) Circuit() (*ir.Circuit, error) {
	b := builder.New("EfficientSU2", es.numQubits)
	idx := 0

	for rep := range es.reps + 1 {
		// RY + RZ rotation layer.
		for q := range es.numQubits {
			b.SymRY(es.vec.At(idx).Expr(), q)
			idx++
			b.SymRZ(es.vec.At(idx).Expr(), q)
			idx++
		}
		// Entanglement layer (skip after last rotation).
		if rep < es.reps {
			applyEntanglement(b, es.numQubits, es.ent)
		}
	}

	return b.Build()
}
