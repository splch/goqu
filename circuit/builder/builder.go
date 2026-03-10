// Package builder provides a fluent API for constructing quantum circuits.
package builder

import (
	"fmt"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

// Builder accumulates operations and produces an immutable Circuit.
type Builder struct {
	name      string
	numQubits int
	numClbits int
	ops       []ir.Operation
	metadata  map[string]string
	err       error
}

// New creates a circuit builder for n qubits.
func New(name string, nQubits int) *Builder {
	return &Builder{
		name:      name,
		numQubits: nQubits,
		metadata:  make(map[string]string),
	}
}

// WithClbits sets the number of classical bits.
func (b *Builder) WithClbits(n int) *Builder {
	b.numClbits = n
	return b
}

// SetMetadata sets a metadata key-value pair.
func (b *Builder) SetMetadata(key, value string) *Builder {
	b.metadata[key] = value
	return b
}

func (b *Builder) validateQubit(q int) {
	if b.err != nil {
		return
	}
	if q < 0 || q >= b.numQubits {
		b.err = fmt.Errorf("qubit %d out of range [0, %d)", q, b.numQubits)
	}
}

func (b *Builder) validateClbit(c int) {
	if b.err != nil {
		return
	}
	if c < 0 || c >= b.numClbits {
		b.err = fmt.Errorf("classical bit %d out of range [0, %d)", c, b.numClbits)
	}
}

// Apply adds an arbitrary gate on the specified qubits.
func (b *Builder) Apply(g gate.Gate, qubits ...int) *Builder {
	if b.err != nil {
		return b
	}
	if g == nil {
		b.err = fmt.Errorf("gate is nil")
		return b
	}
	if len(qubits) != g.Qubits() {
		b.err = fmt.Errorf("gate %s requires %d qubits, got %d", g.Name(), g.Qubits(), len(qubits))
		return b
	}
	for _, q := range qubits {
		b.validateQubit(q)
	}
	if b.err != nil {
		return b
	}
	qs := make([]int, len(qubits))
	copy(qs, qubits)
	b.ops = append(b.ops, ir.Operation{Gate: g, Qubits: qs})
	return b
}

// H applies a Hadamard gate.
func (b *Builder) H(q int) *Builder { return b.Apply(gate.H, q) }

// X applies a Pauli-X gate.
func (b *Builder) X(q int) *Builder { return b.Apply(gate.X, q) }

// Y applies a Pauli-Y gate.
func (b *Builder) Y(q int) *Builder { return b.Apply(gate.Y, q) }

// Z applies a Pauli-Z gate.
func (b *Builder) Z(q int) *Builder { return b.Apply(gate.Z, q) }

// S applies an S gate.
func (b *Builder) S(q int) *Builder { return b.Apply(gate.S, q) }

// T applies a T gate.
func (b *Builder) T(q int) *Builder { return b.Apply(gate.T, q) }

// CNOT applies a CNOT (controlled-X) gate.
func (b *Builder) CNOT(control, target int) *Builder {
	return b.Apply(gate.CNOT, control, target)
}

// CZ applies a CZ (controlled-Z) gate.
func (b *Builder) CZ(control, target int) *Builder {
	return b.Apply(gate.CZ, control, target)
}

// SWAP applies a SWAP gate.
func (b *Builder) SWAP(q0, q1 int) *Builder {
	return b.Apply(gate.SWAP, q0, q1)
}

// CCX applies a Toffoli (CCX) gate.
func (b *Builder) CCX(c0, c1, target int) *Builder {
	return b.Apply(gate.CCX, c0, c1, target)
}

// RX applies an RX rotation gate.
func (b *Builder) RX(theta float64, q int) *Builder { return b.Apply(gate.RX(theta), q) }

// RY applies an RY rotation gate.
func (b *Builder) RY(theta float64, q int) *Builder { return b.Apply(gate.RY(theta), q) }

// RZ applies an RZ rotation gate.
func (b *Builder) RZ(theta float64, q int) *Builder { return b.Apply(gate.RZ(theta), q) }

// Phase applies a phase gate.
func (b *Builder) Phase(phi float64, q int) *Builder { return b.Apply(gate.Phase(phi), q) }

// U3 applies the universal single-qubit gate.
func (b *Builder) U3(theta, phi, lambda float64, q int) *Builder {
	return b.Apply(gate.U3(theta, phi, lambda), q)
}

// Measure adds a measurement of qubit to classical bit.
func (b *Builder) Measure(qubit, clbit int) *Builder {
	if b.err != nil {
		return b
	}
	b.validateQubit(qubit)
	b.validateClbit(clbit)
	if b.err != nil {
		return b
	}
	b.ops = append(b.ops, ir.Operation{
		Qubits: []int{qubit},
		Clbits: []int{clbit},
	})
	return b
}

// MeasureAll adds measurements for all qubits to corresponding classical bits.
// Automatically sets numClbits to numQubits if not already set.
func (b *Builder) MeasureAll() *Builder {
	if b.err != nil {
		return b
	}
	if b.numClbits < b.numQubits {
		b.numClbits = b.numQubits
	}
	for i := range b.numQubits {
		b.ops = append(b.ops, ir.Operation{
			Qubits: []int{i},
			Clbits: []int{i},
		})
	}
	return b
}

// Barrier adds a barrier instruction (no-op marker for transpilation).
func (b *Builder) Barrier(qubits ...int) *Builder {
	if b.err != nil {
		return b
	}
	if len(qubits) == 0 {
		// Barrier on all qubits.
		qubits = make([]int, b.numQubits)
		for i := range qubits {
			qubits[i] = i
		}
	}
	for _, q := range qubits {
		b.validateQubit(q)
	}
	if b.err != nil {
		return b
	}
	qs := make([]int, len(qubits))
	copy(qs, qubits)
	b.ops = append(b.ops, ir.Operation{Gate: barrierGate{n: len(qs)}, Qubits: qs})
	return b
}

// Build finalizes and returns an immutable Circuit.
func (b *Builder) Build() (*ir.Circuit, error) {
	if b.err != nil {
		return nil, b.err
	}
	return ir.New(b.name, b.numQubits, b.numClbits, b.ops, b.metadata), nil
}

// barrierGate is a pseudo-gate representing a barrier.
type barrierGate struct{ n int }

func (g barrierGate) Name() string            { return "barrier" }
func (g barrierGate) Qubits() int             { return g.n }
func (g barrierGate) Matrix() []complex128     { return nil }
func (g barrierGate) Params() []float64       { return nil }
func (g barrierGate) Inverse() gate.Gate      { return g }
func (g barrierGate) Decompose(_ []int) []gate.Applied { return nil }
