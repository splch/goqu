// Package emitter writes a Circuit as OpenQASM 3.0 source.
package emitter

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/splch/qgo/circuit/ir"
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
	return nil
}

func (e *emitter) emitOp(op ir.Operation) error {
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
	}

	// Build gate call.
	gateName := qasmGateName(name)
	params := op.Gate.Params()
	qargs := make([]string, len(op.Qubits))
	for i, q := range op.Qubits {
		qargs[i] = fmt.Sprintf("q[%d]", q)
	}

	if op.Condition != nil {
		e.writef("if (%s == %d) ", op.Condition.Register, op.Condition.Value)
	}

	if len(params) > 0 {
		pstrs := make([]string, len(params))
		for i, p := range params {
			pstrs[i] = formatParam(p)
		}
		e.writef("%s(%s) %s;\n", gateName, strings.Join(pstrs, ", "), strings.Join(qargs, ", "))
	} else {
		e.writef("%s %s;\n", gateName, strings.Join(qargs, ", "))
	}
	return nil
}

func (e *emitter) writef(format string, args ...any) {
	fmt.Fprintf(e.w, format, args...)
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
		}
	}
	return strings.ToLower(name)
}

func formatParam(v float64) string {
	// Try to express as a multiple of pi for readability.
	ratio := v / math.Pi
	if ratio == 1 {
		return "pi"
	}
	if ratio == -1 {
		return "-pi"
	}
	if ratio == 0.5 {
		return "pi/2"
	}
	if ratio == 0.25 {
		return "pi/4"
	}
	if ratio == 0.125 {
		return "pi/8"
	}
	// Check if ratio is a simple fraction of pi.
	for denom := 2; denom <= 16; denom++ {
		num := ratio * float64(denom)
		if math.Abs(num-math.Round(num)) < 1e-10 {
			n := int(math.Round(num))
			if n == 1 {
				return fmt.Sprintf("pi/%d", denom)
			}
			if n == -1 {
				return fmt.Sprintf("-pi/%d", denom)
			}
			return fmt.Sprintf("%d*pi/%d", n, denom)
		}
	}
	return fmt.Sprintf("%.10g", v)
}
