package gate

import (
	"fmt"
	"sync"
)

// ControlledGate is a gate wrapping an inner gate with additional control qubits.
type ControlledGate interface {
	Gate
	Inner() Gate
	NumControls() int
}

// controlled wraps any gate with N control qubits.
type controlled struct {
	inner     Gate
	nControls int
	name      string
	once      sync.Once
	matrix    []complex128
}

// Controlled wraps inner with nControls control qubits.
// For well-known cases it returns existing singletons:
// Controlled(X,1)=CNOT, Controlled(X,2)=CCX, Controlled(Z,1)=CZ.
func Controlled(inner Gate, nControls int) Gate {
	if nControls < 1 {
		panic("gate: Controlled requires at least 1 control qubit")
	}
	// Return existing singletons for well-known controlled gates.
	if nControls == 1 {
		switch inner {
		case X:
			return CNOT
		case Z:
			return CZ
		case Y:
			return CY
		}
	}
	if nControls == 2 && inner == X {
		return CCX
	}
	n := fmt.Sprintf("C%d-%s", nControls, inner.Name())
	return &controlled{inner: inner, nControls: nControls, name: n}
}

// MCX returns a multi-controlled X gate with nControls control qubits.
func MCX(nControls int) Gate {
	return Controlled(X, nControls)
}

// MCZ returns a multi-controlled Z gate with nControls control qubits.
func MCZ(nControls int) Gate {
	return Controlled(Z, nControls)
}

// MCP returns a multi-controlled Phase gate with nControls control qubits.
func MCP(phi float64, nControls int) Gate {
	return Controlled(Phase(phi), nControls)
}

func (g *controlled) Name() string      { return g.name }
func (g *controlled) Qubits() int       { return g.nControls + g.inner.Qubits() }
func (g *controlled) Inner() Gate       { return g.inner }
func (g *controlled) NumControls() int  { return g.nControls }
func (g *controlled) Params() []float64 { return g.inner.Params() }

func (g *controlled) Inverse() Gate {
	return &controlled{
		inner:     g.inner.Inverse(),
		nControls: g.nControls,
		name:      fmt.Sprintf("C%d-%s", g.nControls, g.inner.Inverse().Name()),
	}
}

func (g *controlled) Decompose(_ []int) []Applied { return nil }

// Matrix returns the full unitary matrix, lazily computed.
// The matrix is identity except the bottom-right block where all controls are |1>,
// which contains the inner gate's matrix.
// Panics if total qubits > 10 to prevent multi-GB allocations.
func (g *controlled) Matrix() []complex128 {
	g.once.Do(func() {
		totalQubits := g.Qubits()
		if totalQubits > 10 {
			panic(fmt.Sprintf("gate: Matrix() not supported for %d-qubit controlled gate (max 10); use simulation kernel instead", totalQubits))
		}
		dim := 1 << totalQubits
		g.matrix = make([]complex128, dim*dim)

		// Identity for all rows/cols except the bottom block.
		innerDim := 1 << g.inner.Qubits()
		blockStart := dim - innerDim
		for i := range blockStart {
			g.matrix[i*dim+i] = 1
		}

		// Bottom-right block: inner gate's matrix.
		innerM := g.inner.Matrix()
		for r := range innerDim {
			for c := range innerDim {
				g.matrix[(blockStart+r)*dim+(blockStart+c)] = innerM[r*innerDim+c]
			}
		}
	})
	return g.matrix
}
