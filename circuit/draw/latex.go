package draw

import (
	"fmt"
	"io"
	"strings"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/internal/piformat"
)

// LaTeXOption configures LaTeX rendering.
type LaTeXOption func(*latexConfig)

type latexConfig struct {
	maxLabelWidth int
}

// WithLaTeXMaxLabelWidth sets the maximum width for gate labels in LaTeX output.
func WithLaTeXMaxLabelWidth(n int) LaTeXOption {
	return func(c *latexConfig) {
		if n > 0 {
			c.maxLabelWidth = n
		}
	}
}

// LaTeX returns a LaTeX (quantikz) string of the circuit diagram.
func LaTeX(c *ir.Circuit, opts ...LaTeXOption) string {
	var sb strings.Builder
	_ = FprintLaTeX(&sb, c, opts...)
	return sb.String()
}

// FprintLaTeX writes a LaTeX (quantikz) diagram of the circuit to w.
func FprintLaTeX(w io.Writer, c *ir.Circuit, opts ...LaTeXOption) error {
	cfg := &latexConfig{maxLabelWidth: 10}
	for _, o := range opts {
		o(cfg)
	}

	if c == nil {
		_, err := io.WriteString(w, "% Empty circuit\n")
		return err
	}

	nq := c.NumQubits()
	ops := c.Ops()

	if nq == 0 {
		_, err := io.WriteString(w, "% Empty circuit\n")
		return err
	}

	// Step 1: assign each op to a column.
	placements, numCols := assignColumns(ops, nq)

	// Step 2: build grid of quantikz commands. grid[qubit][col].
	grid := make([][]string, nq)
	for q := range nq {
		grid[q] = make([]string, numCols)
		for col := range numCols {
			grid[q][col] = `\qw`
		}
	}

	for _, p := range placements {
		cmds := latexGateCommands(p)
		for q, cmd := range cmds {
			grid[q][p.col] = cmd
		}
	}

	// Step 3: render quantikz environment.
	var sb strings.Builder
	sb.WriteString(`% Requires: \usepackage{tikz} \usetikzlibrary{quantikz2}`)
	sb.WriteString("\n")
	sb.WriteString(`\begin{quantikz}`)
	sb.WriteString("\n")

	for q := range nq {
		fmt.Fprintf(&sb, `\lstick{$q_%d$}`, q)
		for col := range numCols {
			sb.WriteString(" & ")
			sb.WriteString(grid[q][col])
		}
		sb.WriteString(` & \qw`)
		if q < nq-1 {
			sb.WriteString(` \\`)
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`\end{quantikz}`)
	sb.WriteString("\n")

	_, err := io.WriteString(w, sb.String())
	return err
}

// latexGateCommands maps an operation to per-qubit quantikz commands.
func latexGateCommands(p placement) map[int]string {
	op := p.op
	cmds := make(map[int]string)
	qubits := op.Qubits

	if len(qubits) == 0 {
		return cmds
	}

	// Measurement: nil gate, has clbits.
	if op.Gate == nil {
		for _, q := range qubits {
			cmds[q] = `\meter{}`
		}
		return cmds
	}

	name := op.Gate.Name()
	condPrefix := ""
	if op.Condition != nil {
		condPrefix = "c:"
	}

	switch name {
	case "CNOT":
		d := qubits[1] - qubits[0]
		cmds[qubits[0]] = fmt.Sprintf(`\ctrl{%d}`, d)
		cmds[qubits[1]] = `\targ{}`
	case "CZ":
		d := qubits[1] - qubits[0]
		cmds[qubits[0]] = fmt.Sprintf(`\ctrl{%d}`, d)
		cmds[qubits[1]] = `\control{}`
	case "CY":
		d := qubits[1] - qubits[0]
		cmds[qubits[0]] = fmt.Sprintf(`\ctrl{%d}`, d)
		cmds[qubits[1]] = latexGateBox("Y", condPrefix)
	case "SWAP":
		d := qubits[1] - qubits[0]
		cmds[qubits[0]] = fmt.Sprintf(`\swap{%d}`, d)
		cmds[qubits[1]] = `\targX{}`
	case "CCX":
		// Two controls pointing to target.
		cmds[qubits[0]] = fmt.Sprintf(`\ctrl{%d}`, qubits[2]-qubits[0])
		cmds[qubits[1]] = fmt.Sprintf(`\ctrl{%d}`, qubits[2]-qubits[1])
		cmds[qubits[2]] = `\targ{}`
	case "CSWAP":
		cmds[qubits[0]] = fmt.Sprintf(`\ctrl{%d}`, qubits[1]-qubits[0])
		d := qubits[2] - qubits[1]
		cmds[qubits[1]] = fmt.Sprintf(`\swap{%d}`, d)
		cmds[qubits[2]] = `\targX{}`
	case "delay":
		label := `\text{D}`
		if d, ok := op.Gate.(gate.Delayable); ok {
			label = fmt.Sprintf(`\text{D(%g%s)}`, d.Duration(), d.Unit())
		}
		cmds[qubits[0]] = latexGateBox(label, condPrefix)
	case "barrier":
		// Vertical dashed line on first qubit.
		cmds[qubits[0]] = `\slice{}`
		for _, q := range qubits[1:] {
			cmds[q] = `\qw`
		}
	case "reset":
		cmds[qubits[0]] = latexGateBox(`\ket{0}`, condPrefix)
	case "StatePrep", "StatePrep†":
		if len(qubits) > 1 {
			cmds[qubits[0]] = fmt.Sprintf(`\gate[%d]{\text{Prep}}`, len(qubits))
			for _, q := range qubits[1:] {
				// ghosted wires consumed by the spanning gate
				cmds[q] = `\qw`
			}
		} else {
			cmds[qubits[0]] = latexGateBox(`\text{Prep}`, condPrefix)
		}
	default:
		// Check for ControlledGate interface.
		if cg, ok := op.Gate.(gate.ControlledGate); ok {
			nControls := cg.NumControls()
			innerName := latexGateName(gateBaseName(cg.Inner().Name()))
			innerParams := cg.Inner().Params()
			targetLabel := formatLaTeXGateWithParams(innerName, innerParams)

			targetQ := qubits[nControls:]
			for i := range nControls {
				// Point each control to the first target qubit.
				d := targetQ[0] - qubits[i]
				cmds[qubits[i]] = fmt.Sprintf(`\ctrl{%d}`, d)
			}
			if len(targetQ) == 1 {
				if isTargetGate(innerName) {
					cmds[targetQ[0]] = `\targ{}`
				} else {
					cmds[targetQ[0]] = latexGateBox(targetLabel, condPrefix)
				}
			} else {
				cmds[targetQ[0]] = fmt.Sprintf(`\gate[%d]{%s}`, len(targetQ), condPrefix+targetLabel)
				for _, q := range targetQ[1:] {
					cmds[q] = `\qw`
				}
			}
			return cmds
		}

		// Check for controlled parameterized gates (CRZ, CRX, CRY, CP).
		if strings.HasPrefix(name, "C") && len(qubits) == 2 {
			base := controlledBaseName(name)
			if base != "" {
				params := op.Gate.Params()
				targetLabel := formatLaTeXGateWithParams(latexGateName(base), params)
				d := qubits[1] - qubits[0]
				cmds[qubits[0]] = fmt.Sprintf(`\ctrl{%d}`, d)
				cmds[qubits[1]] = latexGateBox(targetLabel, condPrefix)
				return cmds
			}
		}

		// Generic gate.
		baseName := latexGateName(gateBaseName(name))
		params := op.Gate.Params()
		label := formatLaTeXGateWithParams(baseName, params)

		if len(qubits) > 1 {
			cmds[qubits[0]] = fmt.Sprintf(`\gate[%d]{%s}`, len(qubits), condPrefix+label)
			for _, q := range qubits[1:] {
				cmds[q] = `\qw`
			}
		} else {
			cmds[qubits[0]] = latexGateBox(label, condPrefix)
		}
	}

	// Apply condition prefix to control-target gates.
	if condPrefix != "" {
		switch name {
		case "CNOT", "CZ", "CY", "SWAP", "CCX", "CSWAP":
			// For these gates, prefix the target qubit's label.
			// Controls don't get the prefix; the target does.
		}
	}

	return cmds
}

// latexGateName converts gate names to LaTeX notation.
func latexGateName(name string) string {
	switch name {
	case "RX":
		return "R_x"
	case "RY":
		return "R_y"
	case "RZ":
		return "R_z"
	case "RXX":
		return "R_{xx}"
	case "RYY":
		return "R_{yy}"
	case "RZZ":
		return "R_{zz}"
	default:
		// Handle dagger notation.
		if base, ok := strings.CutSuffix(name, "†"); ok {
			return base + `^\dagger`
		}
		return name
	}
}

// formatLaTeXGateWithParams formats a gate label with LaTeX-formatted parameters.
func formatLaTeXGateWithParams(baseName string, params []float64) string {
	if len(params) == 0 {
		return baseName
	}
	pstrs := make([]string, len(params))
	for i, p := range params {
		pstrs[i] = piformat.FormatLaTeX(p)
	}
	return baseName + `\left(` + strings.Join(pstrs, ",") + `\right)`
}

// latexGateBox wraps a label in a \gate{} command with optional condition prefix.
func latexGateBox(label, condPrefix string) string {
	return fmt.Sprintf(`\gate{%s%s}`, condPrefix, label)
}

// isTargetGate returns true if the inner gate name should render as \targ{}.
func isTargetGate(name string) bool {
	return name == "X"
}
