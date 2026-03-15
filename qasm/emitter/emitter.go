// Package emitter writes a Circuit as OpenQASM 3.0 source.
package emitter

import (
	"fmt"
	"io"
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

// Emit writes a Circuit as OpenQASM 3.0 source.
func Emit(c *ir.Circuit, w io.Writer, opts ...Option) error {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	e := &emitter{c: c, w: w, cfg: cfg}
	return e.emit()
}

// EmitString returns the Circuit as an OpenQASM 3.0 string.
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
	e.writef("OPENQASM 3.0;\n")
	e.writef("include \"stdgates.inc\";\n")

	if e.cfg.comments && e.c.Name() != "" {
		e.writef("// Circuit: %s\n", e.c.Name())
	}

	// Emit qubit and classical bit declarations.
	if e.c.NumQubits() > 0 {
		e.writef("qubit[%d] q;\n", e.c.NumQubits())
	}
	if e.c.NumClbits() > 0 {
		e.writef("bit[%d] c;\n", e.c.NumClbits())
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
	// Control flow operations.
	if op.ControlFlow != nil {
		return e.emitControlFlow(op.ControlFlow)
	}

	// Measurement: no gate, has clbits.
	if op.Gate == nil {
		if len(op.Clbits) > 0 {
			e.writef("c[%d] = measure q[%d];\n", op.Clbits[0], op.Qubits[0])
		}
		return nil
	}

	name := op.Gate.Name()

	switch name {
	case "barrier":
		qargs := make([]string, len(op.Qubits))
		for i, q := range op.Qubits {
			qargs[i] = fmt.Sprintf("q[%d]", q)
		}
		e.writef("barrier %s;\n", strings.Join(qargs, ", "))
		return nil
	case "reset":
		e.writef("reset q[%d];\n", op.Qubits[0])
		return nil
	case "delay":
		if d, ok := op.Gate.(gate.Delayable); ok {
			e.writef("delay[%g%s] q[%d];\n", d.Duration(), d.Unit(), op.Qubits[0])
		}
		return nil
	}

	// Check for multi-controlled gates: emit ctrl(N) @ syntax.
	if cg, ok := op.Gate.(gate.ControlledGate); ok {
		return e.emitControlledOp(cg, op)
	}

	// Build gate call.
	gateName := qasmGateName(name)
	params := op.Gate.Params()
	qargs := make([]string, len(op.Qubits))
	for i, q := range op.Qubits {
		qargs[i] = fmt.Sprintf("q[%d]", q)
	}

	if op.Condition != nil {
		if op.Condition.Register != "" {
			e.writef("if (%s == %d) ", op.Condition.Register, op.Condition.Value)
		} else {
			e.writef("if (c[%d] == %d) ", op.Condition.Clbit, op.Condition.Value)
		}
	}

	// For symbolic (unbound) gates, emit parameter names from the gate name.
	if params == nil {
		// Check if this is a symbolic gate by looking for {param} in name.
		if idx := strings.Index(name, "({"); idx >= 0 {
			// Extract the symbolic parameter string between ({ and })
			end := strings.LastIndex(name, "})")
			if end > idx {
				symParams := name[idx+2 : end]
				gateName = qasmGateName(name[:idx])
				e.writef("%s(%s) %s;\n", gateName, symParams, strings.Join(qargs, ", "))
				return nil
			}
		}
	}

	if len(params) > 0 {
		pstrs := make([]string, len(params))
		for i, p := range params {
			pstrs[i] = piformat.FormatQASM(p)
		}
		e.writef("%s(%s) %s;\n", gateName, strings.Join(pstrs, ", "), strings.Join(qargs, ", "))
	} else {
		e.writef("%s %s;\n", gateName, strings.Join(qargs, ", "))
	}
	return nil
}

// emitControlledOp emits a multi-controlled gate using OpenQASM 3.0 ctrl @ syntax.
func (e *emitter) emitControlledOp(cg gate.ControlledGate, op ir.Operation) error {
	nControls := cg.NumControls()
	innerGate := cg.Inner()

	// Build ctrl modifier.
	var ctrl string
	if nControls == 1 {
		ctrl = "ctrl"
	} else {
		ctrl = fmt.Sprintf("ctrl(%d)", nControls)
	}

	// Build inner gate name + params.
	innerName := qasmGateName(innerGate.Name())
	params := innerGate.Params()
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

	// Build qubit arguments.
	qargs := make([]string, len(op.Qubits))
	for i, q := range op.Qubits {
		qargs[i] = fmt.Sprintf("q[%d]", q)
	}

	e.writef("%s @ %s %s;\n", ctrl, gateCall, strings.Join(qargs, ", "))
	return nil
}

// emitControlFlow emits structured control flow as OpenQASM 3.0.
func (e *emitter) emitControlFlow(cf *ir.ControlFlow) error {
	switch cf.Type {
	case ir.ControlFlowWhile:
		e.emitConditionPrefix("while", cf.Condition)
		e.writef(" {\n")
		e.emitOps(cf.Bodies[0])
		e.writef("}\n")

	case ir.ControlFlowFor:
		r := cf.ForRange
		e.writef("for int i in [%d:%d", r.Start, r.End)
		if r.Step != 1 {
			e.writef(":%d", r.Step)
		}
		e.writef("] {\n")
		e.emitOps(cf.Bodies[0])
		e.writef("}\n")

	case ir.ControlFlowIfElse:
		e.emitConditionPrefix("if", cf.Condition)
		e.writef(" {\n")
		e.emitOps(cf.Bodies[0])
		e.writef("}")
		if len(cf.Bodies) > 1 && len(cf.Bodies[1]) > 0 {
			e.writef(" else {\n")
			e.emitOps(cf.Bodies[1])
			e.writef("}")
		}
		e.writef("\n")

	case ir.ControlFlowSwitch:
		sa := cf.SwitchArg
		if sa.Register != "" {
			e.writef("switch (%s) {\n", sa.Register)
		} else if len(sa.Clbits) == 1 {
			e.writef("switch (c[%d]) {\n", sa.Clbits[0])
		} else {
			e.writef("switch (c) {\n")
		}
		for i, caseVal := range sa.Cases {
			if i < len(cf.Bodies) {
				e.writef("case %d: {\n", caseVal)
				e.emitOps(cf.Bodies[i])
				e.writef("}\n")
			}
		}
		// Default case.
		if len(cf.Bodies) > len(sa.Cases) {
			e.writef("default: {\n")
			e.emitOps(cf.Bodies[len(cf.Bodies)-1])
			e.writef("}\n")
		}
		e.writef("}\n")
	}
	return nil
}

// emitConditionPrefix emits "keyword (c[i] == val)" or "keyword (reg == val)".
func (e *emitter) emitConditionPrefix(keyword string, cond ir.Condition) {
	if cond.Register != "" {
		e.writef("%s (%s == %d)", keyword, cond.Register, cond.Value)
	} else {
		e.writef("%s (c[%d] == %d)", keyword, cond.Clbit, cond.Value)
	}
}

// emitOps emits a slice of operations (used for control flow bodies).
func (e *emitter) emitOps(ops []ir.Operation) {
	for _, op := range ops {
		if e.err != nil {
			return
		}
		e.emitOp(op)
	}
}

func (e *emitter) writef(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

func qasmGateName(name string) string {
	switch name {
	case "H":
		return "h"
	case "X":
		return "x"
	case "Y":
		return "y"
	case "Z":
		return "z"
	case "S":
		return "s"
	case "S†":
		return "sdg"
	case "T":
		return "t"
	case "T†":
		return "tdg"
	case "SX":
		return "sx"
	case "CNOT":
		return "cx"
	case "CZ":
		return "cz"
	case "CY":
		return "cy"
	case "SWAP":
		return "swap"
	case "CCX":
		return "ccx"
	case "CSWAP":
		return "cswap"
	case "I":
		return "id"
	case "iSWAP":
		return "iswap"
	case "ECR":
		return "ecr"
	case "DCX":
		return "dcx"
	case "CH":
		return "ch"
	case "CSX":
		return "csx"
	case "CCZ":
		return "ccz"
	case "Sycamore":
		return "sycamore"
	}
	// For parameterized gates, strip the parameter suffix.
	if idx := strings.Index(name, "("); idx != -1 {
		base := name[:idx]
		switch base {
		case "RX":
			return "rx"
		case "RY":
			return "ry"
		case "RZ":
			return "rz"
		case "P":
			return "p"
		case "U3":
			return "U"
		case "CP":
			return "cp"
		case "CRX":
			return "crx"
		case "CRY":
			return "cry"
		case "CRZ":
			return "crz"
		case "RXX":
			return "rxx"
		case "RYY":
			return "ryy"
		case "RZZ":
			return "rzz"
		case "U1":
			return "u1"
		case "U2":
			return "u2"
		case "FSim":
			return "fsim"
		case "PSwap":
			return "pswap"
		case "Rot":
			return "rot"
		case "PhasedXZ":
			return "phasedxz"
		case "GlobalPhase":
			return "gphase"
		}
	}
	return strings.ToLower(name)
}
