package quantinuum

import (
	"fmt"
	"strings"

	"github.com/splch/goqu/backend"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/internal/piformat"
)

// serializeCircuit converts a circuit IR to an OpenQASM 2.0 string
// suitable for Quantinuum API submission.
func serializeCircuit(c *ir.Circuit) (string, error) {
	var sb strings.Builder

	sb.WriteString("OPENQASM 2.0;\n")
	sb.WriteString("include \"qelib1.inc\";\n")

	if c.NumQubits() > 0 {
		fmt.Fprintf(&sb, "qreg q[%d];\n", c.NumQubits())
	}
	if c.NumClbits() > 0 {
		fmt.Fprintf(&sb, "creg c[%d];\n", c.NumClbits())
	}
	sb.WriteString("\n")

	for _, op := range c.Ops() {
		if err := emitOp(&sb, op); err != nil {
			return "", err
		}
	}

	return sb.String(), nil
}

func emitOp(sb *strings.Builder, op ir.Operation) error {
	// Measurement: no gate, has clbits.
	if op.Gate == nil {
		if len(op.Clbits) > 0 {
			fmt.Fprintf(sb, "measure q[%d] -> c[%d];\n", op.Qubits[0], op.Clbits[0])
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
		fmt.Fprintf(sb, "barrier %s;\n", strings.Join(qargs, ", "))
		return nil
	case "reset":
		fmt.Fprintf(sb, "reset q[%d];\n", op.Qubits[0])
		return nil
	case "delay":
		if d, ok := op.Gate.(gate.Delayable); ok {
			fmt.Fprintf(sb, "delay[%g%s] q[%d];\n", d.Duration(), d.Unit(), op.Qubits[0])
		}
		return nil
	}

	// Error on multi-controlled gates (require prior decomposition).
	if _, ok := op.Gate.(gate.ControlledGate); ok {
		return fmt.Errorf("quantinuum: multi-controlled gate %q not supported in QASM 2.0; decompose before submission", name)
	}

	gateName := qasmGateName(name)
	params := op.Gate.Params()
	qargs := make([]string, len(op.Qubits))
	for i, q := range op.Qubits {
		qargs[i] = fmt.Sprintf("q[%d]", q)
	}

	if op.Condition != nil {
		if op.Condition.Register != "" {
			fmt.Fprintf(sb, "if(%s==%d) ", op.Condition.Register, op.Condition.Value)
		} else {
			fmt.Fprintf(sb, "if(c==%d) ", op.Condition.Value)
		}
	}

	if len(params) > 0 {
		pstrs := make([]string, len(params))
		for i, p := range params {
			pstrs[i] = piformat.FormatQASM(p)
		}
		fmt.Fprintf(sb, "%s(%s) %s;\n", gateName, strings.Join(pstrs, ", "), strings.Join(qargs, ", "))
	} else {
		fmt.Fprintf(sb, "%s %s;\n", gateName, strings.Join(qargs, ", "))
	}
	return nil
}

// qasmGateName maps internal gate names to OpenQASM 2.0 gate identifiers.
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
		}
	}
	return strings.ToLower(name)
}

// parseResults converts a Quantinuum job result to a backend.Result.
// Quantinuum returns results as map[string]int (bitstring counts).
func parseResults(resp jobStatusResponse, shots int) (*backend.Result, error) {
	counts := make(map[string]int, len(resp.Results))
	for k, v := range resp.Results {
		counts[k] = v
	}
	return &backend.Result{
		Counts: counts,
		Shots:  shots,
	}, nil
}
