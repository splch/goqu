// Package builder provides a fluent API for constructing quantum circuits.
package builder

import (
	"fmt"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/circuit/param"
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
	// Check for duplicate qubit indices.
	if len(qubits) <= 4 {
		// O(n²) is fine for small gates.
		for i := 0; i < len(qubits); i++ {
			for j := i + 1; j < len(qubits); j++ {
				if qubits[i] == qubits[j] {
					b.err = fmt.Errorf("duplicate qubit %d in gate %s", qubits[i], g.Name())
					return b
				}
			}
		}
	} else {
		seen := make(map[int]bool, len(qubits))
		for _, q := range qubits {
			if seen[q] {
				b.err = fmt.Errorf("duplicate qubit %d in gate %s", q, g.Name())
				return b
			}
			seen[q] = true
		}
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

// MCX applies a multi-controlled X gate.
func (b *Builder) MCX(controls []int, target int) *Builder {
	qubits := make([]int, len(controls)+1)
	copy(qubits, controls)
	qubits[len(controls)] = target
	return b.Apply(gate.MCX(len(controls)), qubits...)
}

// MCZ applies a multi-controlled Z gate.
func (b *Builder) MCZ(controls []int, target int) *Builder {
	qubits := make([]int, len(controls)+1)
	copy(qubits, controls)
	qubits[len(controls)] = target
	return b.Apply(gate.MCZ(len(controls)), qubits...)
}

// MCP applies a multi-controlled Phase gate.
func (b *Builder) MCP(phi float64, controls []int, target int) *Builder {
	qubits := make([]int, len(controls)+1)
	copy(qubits, controls)
	qubits[len(controls)] = target
	return b.Apply(gate.MCP(phi, len(controls)), qubits...)
}

// Ctrl wraps any gate with additional control qubits.
func (b *Builder) Ctrl(g gate.Gate, controls []int, targets ...int) *Builder {
	cg := gate.Controlled(g, len(controls))
	qubits := make([]int, len(controls)+len(targets))
	copy(qubits, controls)
	copy(qubits[len(controls):], targets)
	return b.Apply(cg, qubits...)
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

// StatePrep adds a state preparation gate.
func (b *Builder) StatePrep(amplitudes []complex128, qubits ...int) *Builder {
	if b.err != nil {
		return b
	}
	g, err := gate.StatePrep(amplitudes)
	if err != nil {
		b.err = err
		return b
	}
	return b.Apply(g, qubits...)
}

// Unitary applies a custom unitary gate created from the given matrix.
// The name is used for display; the matrix must be a valid unitary (2x2, 4x4, or 8x8).
func (b *Builder) Unitary(name string, matrix []complex128, qubits ...int) *Builder {
	if b.err != nil {
		return b
	}
	g, err := gate.Unitary(name, matrix)
	if err != nil {
		b.err = err
		return b
	}
	return b.Apply(g, qubits...)
}

// RXX applies an Ising XX rotation gate.
func (b *Builder) RXX(theta float64, q0, q1 int) *Builder { return b.Apply(gate.RXX(theta), q0, q1) }

// RYY applies an Ising YY rotation gate.
func (b *Builder) RYY(theta float64, q0, q1 int) *Builder { return b.Apply(gate.RYY(theta), q0, q1) }

// RZZ applies an Ising ZZ rotation gate.
func (b *Builder) RZZ(theta float64, q0, q1 int) *Builder { return b.Apply(gate.RZZ(theta), q0, q1) }

// SymRX applies a symbolic RX gate.
func (b *Builder) SymRX(theta param.Expr, q int) *Builder {
	return b.Apply(param.SymRX(theta), q)
}

// SymRY applies a symbolic RY gate.
func (b *Builder) SymRY(theta param.Expr, q int) *Builder {
	return b.Apply(param.SymRY(theta), q)
}

// SymRZ applies a symbolic RZ gate.
func (b *Builder) SymRZ(theta param.Expr, q int) *Builder {
	return b.Apply(param.SymRZ(theta), q)
}

// SymPhase applies a symbolic Phase gate.
func (b *Builder) SymPhase(phi param.Expr, q int) *Builder {
	return b.Apply(param.SymPhase(phi), q)
}

// SymU3 applies a symbolic U3 gate.
func (b *Builder) SymU3(theta, phi, lambda param.Expr, q int) *Builder {
	return b.Apply(param.SymU3(theta, phi, lambda), q)
}

// SymCP applies a symbolic controlled-phase gate.
func (b *Builder) SymCP(phi param.Expr, q0, q1 int) *Builder {
	return b.Apply(param.SymCP(phi), q0, q1)
}

// SymRXX applies a symbolic Ising XX gate.
func (b *Builder) SymRXX(theta param.Expr, q0, q1 int) *Builder {
	return b.Apply(param.SymRXX(theta), q0, q1)
}

// SymRYY applies a symbolic Ising YY gate.
func (b *Builder) SymRYY(theta param.Expr, q0, q1 int) *Builder {
	return b.Apply(param.SymRYY(theta), q0, q1)
}

// SymRZZ applies a symbolic Ising ZZ gate.
func (b *Builder) SymRZZ(theta param.Expr, q0, q1 int) *Builder {
	return b.Apply(param.SymRZZ(theta), q0, q1)
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

// Reset resets a qubit to |0⟩.
func (b *Builder) Reset(qubit int) *Builder {
	if b.err != nil {
		return b
	}
	b.validateQubit(qubit)
	if b.err != nil {
		return b
	}
	b.ops = append(b.ops, ir.Operation{Gate: gate.Reset, Qubits: []int{qubit}})
	return b
}

// If adds a classically-conditioned gate. The gate is applied only when clbit == value.
func (b *Builder) If(clbit, value int, g gate.Gate, qubits ...int) *Builder {
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
	b.validateClbit(clbit)
	for _, q := range qubits {
		b.validateQubit(q)
	}
	if b.err != nil {
		return b
	}
	qs := make([]int, len(qubits))
	copy(qs, qubits)
	b.ops = append(b.ops, ir.Operation{
		Gate:      g,
		Qubits:    qs,
		Condition: &ir.Condition{Clbit: clbit, Value: value},
	})
	return b
}

// IfBlock conditions multiple operations on clbit == value.
// The function fn is called with a sub-builder; all ops added inside are conditioned.
func (b *Builder) IfBlock(clbit, value int, fn func(*Builder)) *Builder {
	if b.err != nil {
		return b
	}
	b.validateClbit(clbit)
	if b.err != nil {
		return b
	}
	opsBefore := len(b.ops)
	fn(b)
	if b.err != nil {
		return b
	}
	cond := &ir.Condition{Clbit: clbit, Value: value}
	for i := opsBefore; i < len(b.ops); i++ {
		b.ops[i].Condition = cond
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

// Compose appends all operations from c into the builder, remapping qubit indices.
// nil qubitMap uses identity mapping (c's qubit N → builder's qubit N).
// Classical bits use identity mapping (c's clbit N → builder's clbit N).
func (b *Builder) Compose(c *ir.Circuit, qubitMap map[int]int) *Builder {
	if b.err != nil {
		return b
	}
	for _, op := range c.Ops() {
		qubits := make([]int, len(op.Qubits))
		for i, q := range op.Qubits {
			if qubitMap != nil {
				mapped, ok := qubitMap[q]
				if !ok {
					b.err = fmt.Errorf("Compose: qubit %d has no mapping", q)
					return b
				}
				qubits[i] = mapped
			} else {
				qubits[i] = q
			}
			b.validateQubit(qubits[i])
			if b.err != nil {
				return b
			}
		}
		clbits := make([]int, len(op.Clbits))
		for i, c := range op.Clbits {
			clbits[i] = c
			b.validateClbit(clbits[i])
			if b.err != nil {
				return b
			}
		}
		newOp := ir.Operation{Gate: op.Gate, Qubits: qubits, Clbits: clbits}
		if op.Condition != nil {
			newOp.Condition = &ir.Condition{
				Clbit: op.Condition.Clbit,
				Value: op.Condition.Value,
			}
		}
		b.ops = append(b.ops, newOp)
	}
	return b
}

// ComposeInverse appends the inverse of c's operations (reversed order, each gate adjointed).
// Measurements, resets, and barriers are skipped.
// nil qubitMap uses identity mapping.
func (b *Builder) ComposeInverse(c *ir.Circuit, qubitMap map[int]int) *Builder {
	if b.err != nil {
		return b
	}
	ops := c.Ops()
	for i := len(ops) - 1; i >= 0; i-- {
		op := ops[i]
		// Skip measurements.
		if op.Gate == nil {
			continue
		}
		// Skip resets and barriers.
		name := op.Gate.Name()
		if name == "reset" || name == "barrier" {
			continue
		}

		qubits := make([]int, len(op.Qubits))
		for j, q := range op.Qubits {
			if qubitMap != nil {
				mapped, ok := qubitMap[q]
				if !ok {
					b.err = fmt.Errorf("ComposeInverse: qubit %d has no mapping", q)
					return b
				}
				qubits[j] = mapped
			} else {
				qubits[j] = q
			}
			b.validateQubit(qubits[j])
			if b.err != nil {
				return b
			}
		}
		newOp := ir.Operation{Gate: op.Gate.Inverse(), Qubits: qubits}
		if op.Condition != nil {
			newOp.Condition = &ir.Condition{
				Clbit: op.Condition.Clbit,
				Value: op.Condition.Value,
			}
		}
		b.ops = append(b.ops, newOp)
	}
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

func (g barrierGate) Name() string                     { return "barrier" }
func (g barrierGate) Qubits() int                      { return g.n }
func (g barrierGate) Matrix() []complex128             { return nil }
func (g barrierGate) Params() []float64                { return nil }
func (g barrierGate) Inverse() gate.Gate               { return g }
func (g barrierGate) Decompose(_ []int) []gate.Applied { return nil }
