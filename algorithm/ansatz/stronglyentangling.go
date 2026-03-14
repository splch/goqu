package ansatz

import (
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
)

// StronglyEntanglingLayers is an ansatz with Rot(phi, theta, omega) rotation
// layers and a shifted CNOT entanglement pattern. Each layer uses 3 parameters
// per qubit and a CNOT offset that cycles through 1, 2, ..., numQubits-1.
// The circuit has 3*numQubits*layers parameters.
type StronglyEntanglingLayers struct {
	numQubits int
	layers    int
	vec       *param.Vector
}

// NewStronglyEntanglingLayers creates a StronglyEntanglingLayers ansatz.
func NewStronglyEntanglingLayers(numQubits, layers int) *StronglyEntanglingLayers {
	nParams := 3 * numQubits * layers
	return &StronglyEntanglingLayers{
		numQubits: numQubits,
		layers:    layers,
		vec:       param.NewVector("θ", nParams),
	}
}

func (s *StronglyEntanglingLayers) NumParams() int             { return s.vec.Size() }
func (s *StronglyEntanglingLayers) ParamVector() *param.Vector { return s.vec }

func (s *StronglyEntanglingLayers) Circuit() (*ir.Circuit, error) {
	b := builder.New("StronglyEntanglingLayers", s.numQubits)
	idx := 0

	for l := range s.layers {
		// Rot(phi, theta, omega) on each qubit.
		for q := range s.numQubits {
			phi := s.vec.At(idx).Expr()
			idx++
			theta := s.vec.At(idx).Expr()
			idx++
			omega := s.vec.At(idx).Expr()
			idx++
			b.SymRot(phi, theta, omega, q)
		}
		// Shifted CNOT entanglement (skip for single qubit).
		if s.numQubits > 1 {
			r := (l % (s.numQubits - 1)) + 1
			for i := range s.numQubits {
				target := (i + r) % s.numQubits
				b.CNOT(i, target)
			}
		}
	}

	return b.Build()
}
