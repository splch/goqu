// Package ansatz provides parameterized circuit templates for variational algorithms.
package ansatz

import (
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
)

// Ansatz is a parameterized quantum circuit template.
type Ansatz interface {
	// Circuit returns the parameterized circuit.
	Circuit() (*ir.Circuit, error)
	// NumParams returns the number of free parameters.
	NumParams() int
	// ParamVector returns the parameter vector used by the circuit.
	ParamVector() *param.Vector
}

// Entanglement describes the entangling layer pattern.
type Entanglement int

const (
	// Linear applies CNOT between adjacent qubits: (0,1), (1,2), ...
	Linear Entanglement = iota
	// Full applies CNOT between all pairs.
	Full
	// Circular extends Linear with an additional (n-1, 0) CNOT.
	Circular
)
