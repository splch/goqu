// Package emitter writes a Circuit as Quil source.
package emitter

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/internal/piformat"
)

// Option configures emitter behavior.
type Option func(*config)

type config struct {
	comments bool
}

// WithComments includes metadata as comments.
func WithComments(include bool) Option {
	return func(c *config) { c.comments = include }
}

// Emit writes a Circuit as Quil source.
func Emit(c *ir.Circuit, w io.Writer, opts ...Option) error {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	e := &emitter{c: c, w: w, cfg: cfg}
	return e.emit()
}

// EmitString returns the Circuit as a Quil string.
func EmitString(c *ir.Circuit, opts ...Option) (string, error) {
	var sb strings.Builder
	if err := Emit(c, &sb, opts...); err != nil {
		return "", err
	}
	return sb.String(), nil
}

type emitter struct {
	c   *ir.Circuit
	w   io.Writer
	cfg *config
	err error
}

func (e *emitter) emit() error {
	if e.cfg.comments && e.c.Name() != "" {
		e.writef("# Circuit: %s\n", e.c.Name())
	}

	// Declare classical readout register.
	if e.c.NumClbits() > 0 {
		e.writef("DECLARE ro BIT[%d]\n", e.c.NumClbits())
	}
	e.writef("\n")

	// Emit operations.
	for _, op := range e.c.Ops() {
		if err := e.emitOp(op); err != nil {
			return err
		}
	}
	return e.err
}

func (e *emitter) emitOp(op ir.Operation) error {
	// Measurement: no gate, has clbits.
	if op.Gate == nil {
		if len(op.Clbits) > 0 {
			e.writef("MEASURE %d ro[%d]\n", op.Qubits[0], op.Clbits[0])
		}
		return nil
	}

	name := op.Gate.Name()

	switch name {
	case "barrier":
		e.writef("PRAGMA PRESERVE_BLOCK\nPRAGMA END_PRESERVE_BLOCK\n")
		return nil
	case "reset":
		e.writef("RESET %d\n", op.Qubits[0])
		return nil
	case "delay":
		return nil // Quil has no delay instruction; skip silently.
	}

	// Check for multi-controlled gates: emit CONTROLLED modifier syntax.
	if cg, ok := op.Gate.(gate.ControlledGate); ok {
		return e.emitControlledOp(cg, op)
	}

	// Build gate call.
	quilName, params, err := quilGate(name, op.Gate.Params())
	if err != nil {
		return err
	}

	qargs := formatQubits(op.Qubits)

	if len(params) > 0 {
		pstrs := make([]string, len(params))
		for i, p := range params {
			pstrs[i] = piformat.FormatQASM(p)
		}
		e.writef("%s(%s) %s\n", quilName, strings.Join(pstrs, ", "), qargs)
	} else {
		e.writef("%s %s\n", quilName, qargs)
	}
	return nil
}

// emitControlledOp emits a multi-controlled gate using Quil CONTROLLED modifier.
func (e *emitter) emitControlledOp(cg gate.ControlledGate, op ir.Operation) error {
	nControls := cg.NumControls()
	innerGate := cg.Inner()

	innerName, params, err := quilGate(innerGate.Name(), innerGate.Params())
	if err != nil {
		return err
	}

	// Build CONTROLLED prefix (one per control qubit).
	prefix := strings.Repeat("CONTROLLED ", nControls)

	var gateCall string
	if len(params) > 0 {
		pstrs := make([]string, len(params))
		for i, p := range params {
			pstrs[i] = piformat.FormatQASM(p)
		}
		gateCall = fmt.Sprintf("%s(%s)", innerName, strings.Join(pstrs, ", "))
	} else {
		gateCall = innerName
	}

	qargs := formatQubits(op.Qubits)
	e.writef("%s%s %s\n", prefix, gateCall, qargs)
	return nil
}

func (e *emitter) writef(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

// quilGate maps a goqu gate name to Quil name and parameters.
// Returns an error for gates that cannot be directly represented in Quil.
func quilGate(name string, params []float64) (string, []float64, error) {
	// Fixed gates.
	switch name {
	case "H":
		return "H", nil, nil
	case "X":
		return "X", nil, nil
	case "Y":
		return "Y", nil, nil
	case "Z":
		return "Z", nil, nil
	case "I":
		return "I", nil, nil
	case "S":
		return "S", nil, nil
	case "S†":
		return "DAGGER S", nil, nil
	case "T":
		return "T", nil, nil
	case "T†":
		return "DAGGER T", nil, nil
	case "SX":
		// No native SX in Quil; decompose as RX(pi/2).
		return "RX", []float64{math.Pi / 2}, nil
	case "CNOT":
		return "CNOT", nil, nil
	case "CZ":
		return "CZ", nil, nil
	case "CY":
		return "CONTROLLED Y", nil, nil
	case "SWAP":
		return "SWAP", nil, nil
	case "CCX":
		return "CCNOT", nil, nil
	case "CSWAP":
		return "CSWAP", nil, nil
	case "iSWAP":
		return "ISWAP", nil, nil
	case "CCZ":
		return "CONTROLLED CONTROLLED Z", nil, nil
	}

	// Parameterized gates: strip the "(..." suffix from the name.
	if idx := strings.Index(name, "("); idx != -1 {
		base := name[:idx]
		switch base {
		case "RX":
			return "RX", params, nil
		case "RY":
			return "RY", params, nil
		case "RZ":
			return "RZ", params, nil
		case "P":
			return "PHASE", params, nil
		case "U3":
			return "", nil, fmt.Errorf("quil: U3 gate not supported in Quil; transpile to basis gates first")
		case "CP":
			return "CONTROLLED PHASE", params, nil
		case "CRX":
			return "CONTROLLED RX", params, nil
		case "CRY":
			return "CONTROLLED RY", params, nil
		case "CRZ":
			return "CONTROLLED RZ", params, nil
		case "RXX":
			return "", nil, fmt.Errorf("quil: RXX gate not natively supported in Quil; transpile to basis gates first")
		case "RYY":
			return "", nil, fmt.Errorf("quil: RYY gate not natively supported in Quil; transpile to basis gates first")
		case "RZZ":
			return "", nil, fmt.Errorf("quil: RZZ gate not natively supported in Quil; transpile to basis gates first")
		}
	}

	return strings.ToUpper(name), params, nil
}

// formatQubits formats qubit indices as space-separated numbers.
func formatQubits(qubits []int) string {
	parts := make([]string, len(qubits))
	for i, q := range qubits {
		parts[i] = fmt.Sprintf("%d", q)
	}
	return strings.Join(parts, " ")
}
