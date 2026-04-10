// Package builder provides a fluent API for constructing quantum circuits.
package builder

import (
	"fmt"
	"slices"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
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

// ISWAP applies an iSWAP gate.
func (b *Builder) ISWAP(q0, q1 int) *Builder { return b.Apply(gate.ISWAP, q0, q1) }

// ECR applies an ECR (echoed cross-resonance) gate.
func (b *Builder) ECR(q0, q1 int) *Builder { return b.Apply(gate.ECR, q0, q1) }

// DCX applies a DCX (double CNOT) gate.
func (b *Builder) DCX(q0, q1 int) *Builder { return b.Apply(gate.DCX, q0, q1) }

// CH applies a controlled-Hadamard gate.
func (b *Builder) CH(control, target int) *Builder { return b.Apply(gate.CH, control, target) }

// CSX applies a controlled-√X gate.
func (b *Builder) CSX(control, target int) *Builder { return b.Apply(gate.CSX, control, target) }

// Sycamore applies Google's Sycamore gate.
func (b *Builder) Sycamore(q0, q1 int) *Builder { return b.Apply(gate.Sycamore, q0, q1) }

// CCX applies a Toffoli (CCX) gate.
func (b *Builder) CCX(c0, c1, target int) *Builder {
	return b.Apply(gate.CCX, c0, c1, target)
}

// CCZ applies a doubly-controlled Z gate.
func (b *Builder) CCZ(c0, c1, target int) *Builder { return b.Apply(gate.CCZ, c0, c1, target) }

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

// U1 applies a U1 (phase) gate.
func (b *Builder) U1(lambda float64, q int) *Builder { return b.Apply(gate.U1(lambda), q) }

// U2 applies a U2 gate.
func (b *Builder) U2(phi, lambda float64, q int) *Builder { return b.Apply(gate.U2(phi, lambda), q) }

// Rot applies a PennyLane-style Rot gate.
func (b *Builder) Rot(phi, theta, omega float64, q int) *Builder {
	return b.Apply(gate.Rot(phi, theta, omega), q)
}

// PhasedXZ applies a Cirq-style PhasedXZ gate.
func (b *Builder) PhasedXZ(x, z, a float64, q int) *Builder {
	return b.Apply(gate.PhasedXZ(x, z, a), q)
}

// GlobalPhase applies a global phase gate.
func (b *Builder) GlobalPhase(phi float64, q int) *Builder {
	return b.Apply(gate.GlobalPhase(phi), q)
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

// ZZ applies an IonQ native ZZ entangling gate.
func (b *Builder) ZZ(theta float64, q0, q1 int) *Builder { return b.Apply(gate.ZZ(theta), q0, q1) }

// FSim applies a fermionic simulation gate.
func (b *Builder) FSim(theta, phi float64, q0, q1 int) *Builder {
	return b.Apply(gate.FSim(theta, phi), q0, q1)
}

// PSwap applies a parameterized SWAP gate.
func (b *Builder) PSwap(phi float64, q0, q1 int) *Builder {
	return b.Apply(gate.PSwap(phi), q0, q1)
}

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

// SymU1 applies a symbolic U1 gate.
func (b *Builder) SymU1(lambda param.Expr, q int) *Builder {
	return b.Apply(param.SymU1(lambda), q)
}

// SymU2 applies a symbolic U2 gate.
func (b *Builder) SymU2(phi, lambda param.Expr, q int) *Builder {
	return b.Apply(param.SymU2(phi, lambda), q)
}

// SymRot applies a symbolic Rot gate.
func (b *Builder) SymRot(phi, theta, omega param.Expr, q int) *Builder {
	return b.Apply(param.SymRot(phi, theta, omega), q)
}

// SymFSim applies a symbolic FSim gate.
func (b *Builder) SymFSim(theta, phi param.Expr, q0, q1 int) *Builder {
	return b.Apply(param.SymFSim(theta, phi), q0, q1)
}

// SymPSwap applies a symbolic PSwap gate.
func (b *Builder) SymPSwap(phi param.Expr, q0, q1 int) *Builder {
	return b.Apply(param.SymPSwap(phi), q0, q1)
}

// SymGlobalPhase applies a symbolic GlobalPhase gate.
func (b *Builder) SymGlobalPhase(phi param.Expr, q int) *Builder {
	return b.Apply(param.SymGlobalPhase(phi), q)
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

// Delay idles a qubit for the given duration.
// The unit should be one of the gate.Unit* constants (e.g., gate.UnitNs).
func (b *Builder) Delay(qubit int, duration float64, unit string) *Builder {
	if b.err != nil {
		return b
	}
	b.validateQubit(qubit)
	if b.err != nil {
		return b
	}
	b.ops = append(b.ops, ir.Operation{Gate: gate.Delay(duration, unit), Qubits: []int{qubit}})
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

// IfElseBlock conditions operations on clbit == value, with an else branch.
func (b *Builder) IfElseBlock(clbit, value int, ifBody, elseBody func(*Builder)) *Builder {
	if b.err != nil {
		return b
	}
	b.validateClbit(clbit)
	if b.err != nil {
		return b
	}

	ifSub := b.subBuilder()
	ifBody(ifSub)
	if ifSub.err != nil {
		b.err = ifSub.err
		return b
	}

	elseSub := b.subBuilder()
	elseBody(elseSub)
	if elseSub.err != nil {
		b.err = elseSub.err
		return b
	}

	b.ops = append(b.ops, ir.Operation{
		ControlFlow: &ir.ControlFlow{
			Type:      ir.ControlFlowIfElse,
			Condition: ir.Condition{Clbit: clbit, Value: value},
			Bodies:    [][]ir.Operation{ifSub.ops, elseSub.ops},
		},
	})
	return b
}

// While executes body repeatedly while clbit == value.
// At simulation time, iterations are capped at ir.MaxControlFlowIterations.
func (b *Builder) While(clbit, value int, body func(*Builder)) *Builder {
	if b.err != nil {
		return b
	}
	b.validateClbit(clbit)
	if b.err != nil {
		return b
	}

	sub := b.subBuilder()
	body(sub)
	if sub.err != nil {
		b.err = sub.err
		return b
	}

	b.ops = append(b.ops, ir.Operation{
		ControlFlow: &ir.ControlFlow{
			Type:      ir.ControlFlowWhile,
			Condition: ir.Condition{Clbit: clbit, Value: value},
			Bodies:    [][]ir.Operation{sub.ops},
		},
	})
	return b
}

// For executes body for each iteration in [start, end) with the given step.
// The iteration count is stored in the specified clbit (least-significant bit).
func (b *Builder) For(clbit int, start, end, step int, body func(*Builder)) *Builder {
	if b.err != nil {
		return b
	}
	b.validateClbit(clbit)
	if b.err != nil {
		return b
	}
	if step == 0 {
		b.err = fmt.Errorf("for loop step must be non-zero")
		return b
	}

	sub := b.subBuilder()
	body(sub)
	if sub.err != nil {
		b.err = sub.err
		return b
	}

	b.ops = append(b.ops, ir.Operation{
		ControlFlow: &ir.ControlFlow{
			Type:     ir.ControlFlowFor,
			Bodies:   [][]ir.Operation{sub.ops},
			ForRange: &ir.ForRange{Start: start, End: end, Step: step},
		},
	})
	return b
}

// Switch branches on the integer value of the given classical bits.
// cases maps integer values to body functions. defaultBody (may be nil) executes if no case matches.
func (b *Builder) Switch(clbits []int, cases map[int]func(*Builder), defaultBody func(*Builder)) *Builder {
	if b.err != nil {
		return b
	}
	for _, c := range clbits {
		b.validateClbit(c)
	}
	if b.err != nil {
		return b
	}

	// Build ordered case values and bodies.
	caseVals := make([]int, 0, len(cases))
	bodies := make([][]ir.Operation, 0, len(cases)+1)
	for val := range cases {
		caseVals = append(caseVals, val)
	}
	// Sort for deterministic ordering.
	slices.Sort(caseVals)
	for _, val := range caseVals {
		sub := b.subBuilder()
		cases[val](sub)
		if sub.err != nil {
			b.err = sub.err
			return b
		}
		bodies = append(bodies, sub.ops)
	}
	if defaultBody != nil {
		sub := b.subBuilder()
		defaultBody(sub)
		if sub.err != nil {
			b.err = sub.err
			return b
		}
		bodies = append(bodies, sub.ops)
	}

	cb := make([]int, len(clbits))
	copy(cb, clbits)

	b.ops = append(b.ops, ir.Operation{
		ControlFlow: &ir.ControlFlow{
			Type:   ir.ControlFlowSwitch,
			Bodies: bodies,
			SwitchArg: &ir.SwitchArg{
				Clbits: cb,
				Cases:  caseVals,
			},
		},
	})
	return b
}

// subBuilder creates a child builder sharing the same qubit/clbit dimensions.
func (b *Builder) subBuilder() *Builder {
	return &Builder{
		name:      b.name,
		numQubits: b.numQubits,
		numClbits: b.numClbits,
		metadata:  b.metadata,
	}
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
	b.ops = append(b.ops, ir.Operation{Gate: gate.Barrier(len(qs)), Qubits: qs})
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
		// Skip resets, barriers, and delays.
		name := op.Gate.Name()
		if name == "reset" || name == "barrier" || name == "delay" {
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
