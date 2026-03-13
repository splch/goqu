package ansatz

import (
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
)

// RealAmplitudes is an ansatz with RY rotation layers and CNOT entanglement.
// The circuit has numQubits*(reps+1) parameters.
type RealAmplitudes struct {
	numQubits int
	reps      int
	ent       Entanglement
	vec       *param.Vector
}

// NewRealAmplitudes creates a RealAmplitudes ansatz.
func NewRealAmplitudes(numQubits, reps int, ent Entanglement) *RealAmplitudes {
	nParams := numQubits * (reps + 1)
	return &RealAmplitudes{
		numQubits: numQubits,
		reps:      reps,
		ent:       ent,
		vec:       param.NewVector("θ", nParams),
	}
}

func (ra *RealAmplitudes) NumParams() int             { return ra.vec.Size() }
func (ra *RealAmplitudes) ParamVector() *param.Vector { return ra.vec }

func (ra *RealAmplitudes) Circuit() (*ir.Circuit, error) {
	b := builder.New("RealAmplitudes", ra.numQubits)
	idx := 0

	for rep := range ra.reps + 1 {
		// Rotation layer.
		for q := range ra.numQubits {
			b.SymRY(ra.vec.At(idx).Expr(), q)
			idx++
		}
		// Entanglement layer (skip after last rotation).
		if rep < ra.reps {
			applyEntanglement(b, ra.numQubits, ra.ent)
		}
	}

	return b.Build()
}

func applyEntanglement(b *builder.Builder, n int, ent Entanglement) {
	switch ent {
	case Linear:
		for i := range n - 1 {
			b.CNOT(i, i+1)
		}
	case Full:
		for i := range n {
			for j := i + 1; j < n; j++ {
				b.CNOT(i, j)
			}
		}
	case Circular:
		for i := range n - 1 {
			b.CNOT(i, i+1)
		}
		if n > 1 {
			b.CNOT(n-1, 0)
		}
	}
}
