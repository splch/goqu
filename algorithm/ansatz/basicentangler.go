package ansatz

import (
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
)

// BasicEntanglerLayers is an ansatz with single-parameter rotation layers
// and a CNOT ring entanglement pattern. Each layer applies RX(theta) on
// every qubit followed by a circular CNOT entangling layer.
// The circuit has numQubits*layers parameters.
type BasicEntanglerLayers struct {
	numQubits int
	layers    int
	vec       *param.Vector
}

// NewBasicEntanglerLayers creates a BasicEntanglerLayers ansatz.
func NewBasicEntanglerLayers(numQubits, layers int) *BasicEntanglerLayers {
	nParams := numQubits * layers
	return &BasicEntanglerLayers{
		numQubits: numQubits,
		layers:    layers,
		vec:       param.NewVector("θ", nParams),
	}
}

func (be *BasicEntanglerLayers) NumParams() int             { return be.vec.Size() }
func (be *BasicEntanglerLayers) ParamVector() *param.Vector { return be.vec }

func (be *BasicEntanglerLayers) Circuit() (*ir.Circuit, error) {
	b := builder.New("BasicEntanglerLayers", be.numQubits)
	idx := 0

	for range be.layers {
		// RX rotation on each qubit.
		for q := range be.numQubits {
			b.SymRX(be.vec.At(idx).Expr(), q)
			idx++
		}
		// CNOT entanglement.
		if be.numQubits == 2 {
			// Two-qubit case: single CNOT only (no wraparound).
			b.CNOT(0, 1)
		} else if be.numQubits > 2 {
			// Full CNOT ring: (0,1), (1,2), ..., (n-1, 0).
			for i := range be.numQubits - 1 {
				b.CNOT(i, i+1)
			}
			b.CNOT(be.numQubits-1, 0)
		}
	}

	return b.Build()
}
