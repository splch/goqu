package draw

import (
	"fmt"
	"io"
	"strings"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/internal/piformat"
)

// Option configures diagram rendering.
type Option func(*config)

type config struct {
	maxLabelWidth int
}

func defaultConfig() *config {
	return &config{maxLabelWidth: 10}
}

// WithMaxLabelWidth sets the maximum width for gate labels.
// Labels longer than this are truncated with an ellipsis.
func WithMaxLabelWidth(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxLabelWidth = n
		}
	}
}

// placement maps an operation to its assigned column.
type placement struct {
	op  ir.Operation
	col int
}

// String returns a text diagram of the circuit.
func String(c *ir.Circuit, opts ...Option) string {
	var sb strings.Builder
	_ = Fprint(&sb, c, opts...)
	return sb.String()
}

// Fprint writes a text diagram of the circuit to w.
func Fprint(w io.Writer, c *ir.Circuit, opts ...Option) error {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	nq := c.NumQubits()
	ops := c.Ops()

	if nq == 0 {
		return nil
	}
	if len(ops) == 0 {
		return writeEmpty(w, nq)
	}

	// Step 1: assign each op to a column.
	placements, numCols := assignColumns(ops, nq)

	// Step 2: build a grid of cell labels. grid[qubit][col].
	// Also track which cells are part of multi-qubit gates (for connectors).
	grid := make([][]string, nq)
	multiQubit := make([][]bool, nq) // true if cell is part of a multi-qubit gate
	for q := range nq {
		grid[q] = make([]string, numCols)
		multiQubit[q] = make([]bool, numCols)
	}

	for _, p := range placements {
		labels := gateLabels(p.op, cfg)
		qubits := p.op.Qubits

		isMulti := len(qubits) > 1

		// Find qubit range for vertical connectors.
		minQ, maxQ := qubits[0], qubits[0]
		for _, q := range qubits[1:] {
			if q < minQ {
				minQ = q
			}
			if q > maxQ {
				maxQ = q
			}
		}

		// Place labels on the actual qubits.
		for i, q := range qubits {
			grid[q][p.col] = labels[i]
			multiQubit[q][p.col] = isMulti
		}

		// Place vertical connectors on intermediate qubits.
		if isMulti {
			qubitSet := make(map[int]bool, len(qubits))
			for _, q := range qubits {
				qubitSet[q] = true
			}
			for q := minQ + 1; q < maxQ; q++ {
				if !qubitSet[q] {
					grid[q][p.col] = "+"
					multiQubit[q][p.col] = true
				}
			}
		}
	}

	// Step 3: compute column widths (minimum 1).
	colWidths := make([]int, numCols)
	for col := range numCols {
		w := 1
		for q := range nq {
			if l := len(grid[q][col]); l > w {
				w = l
			}
		}
		colWidths[col] = w
	}

	// Step 4: render.
	// Determine prefix width for qubit labels.
	maxLabel := fmt.Sprintf("q%d", nq-1)
	prefixWidth := len(maxLabel) + 2 // "qN: "

	var buf strings.Builder
	for q := range nq {
		// Qubit row.
		label := fmt.Sprintf("q%d: ", q)
		buf.WriteString(padRight(label, prefixWidth))

		for col := range numCols {
			cw := colWidths[col]
			cell := grid[q][col]

			switch cell {
			case "+":
				// Vertical connector crosses this wire.
				buf.WriteString(dashPad("+", cw))
			case "":
				// Empty wire.
				buf.WriteString(strings.Repeat("-", cw+2))
			default:
				// Gate label, centered with dashes.
				buf.WriteString(dashPad(cell, cw))
			}
			buf.WriteString("-")
		}
		buf.WriteString("\n")

		// Connector row between this qubit and the next.
		if q < nq-1 {
			hasConnector := false
			for col := range numCols {
				if multiQubit[q][col] && multiQubit[q+1][col] {
					hasConnector = true
					break
				}
			}
			if hasConnector {
				var connRow strings.Builder
				connRow.WriteString(strings.Repeat(" ", prefixWidth))
				for col := range numCols {
					cw := colWidths[col]
					if multiQubit[q][col] && multiQubit[q+1][col] {
						// Center "|" to align with the label center in dashPad.
						// dashPad puts label at index (cw+2-1)/2 = (cw+1)/2.
						center := (cw + 1) / 2
						connRow.WriteString(strings.Repeat(" ", center))
						connRow.WriteString("|")
						connRow.WriteString(strings.Repeat(" ", cw+2-center-1))
					} else {
						connRow.WriteString(strings.Repeat(" ", cw+2))
					}
					connRow.WriteString(" ")
				}
				buf.WriteString(strings.TrimRight(connRow.String(), " "))
				buf.WriteString("\n")
			}
		}
	}

	_, err := io.WriteString(w, buf.String())
	return err
}

// writeEmpty writes a diagram with no gates.
func writeEmpty(w io.Writer, nq int) error {
	maxLabel := fmt.Sprintf("q%d", nq-1)
	prefixWidth := len(maxLabel) + 2
	var buf strings.Builder
	for q := range nq {
		label := fmt.Sprintf("q%d: ", q)
		buf.WriteString(padRight(label, prefixWidth))
		buf.WriteString("---\n")
	}
	_, err := io.WriteString(w, buf.String())
	return err
}

// assignColumns assigns each operation to a column index using greedy scheduling.
// It tracks the qubit range (including intermediate qubits for multi-qubit gates).
func assignColumns(ops []ir.Operation, nq int) ([]placement, int) {
	nextFree := make([]int, nq)
	var placements []placement
	numCols := 0

	for _, op := range ops {
		qubits := op.Qubits
		if len(qubits) == 0 {
			continue
		}

		// Find the qubit range.
		minQ, maxQ := qubits[0], qubits[0]
		for _, q := range qubits[1:] {
			if q < minQ {
				minQ = q
			}
			if q > maxQ {
				maxQ = q
			}
		}

		// Find the first available column across the entire range.
		col := 0
		for q := minQ; q <= maxQ; q++ {
			if nextFree[q] > col {
				col = nextFree[q]
			}
		}

		placements = append(placements, placement{op: op, col: col})

		// Update nextFree for the full range.
		for q := minQ; q <= maxQ; q++ {
			nextFree[q] = col + 1
		}

		if col+1 > numCols {
			numCols = col + 1
		}
	}

	return placements, numCols
}

// gateLabels returns the display label for each qubit of the operation.
// The returned slice is parallel to op.Qubits.
func gateLabels(op ir.Operation, cfg *config) []string {
	// Measurement: nil gate, has clbits.
	if op.Gate == nil {
		labels := make([]string, len(op.Qubits))
		for i := range labels {
			labels[i] = "M"
		}
		return labels
	}

	name := op.Gate.Name()
	qubits := op.Qubits

	switch name {
	case "CNOT":
		// qubits[0] = control, qubits[1] = target.
		return []string{"@", "X"}
	case "CZ":
		return []string{"@", "@"}
	case "CY":
		return []string{"@", "Y"}
	case "SWAP":
		return []string{"x", "x"}
	case "CCX":
		return []string{"@", "@", "X"}
	case "CSWAP":
		return []string{"@", "x", "x"}
	case "barrier":
		labels := make([]string, len(qubits))
		for i := range labels {
			labels[i] = "|"
		}
		return labels
	}

	// Check for multi-controlled gates via ControlledGate interface.
	if cg, ok := op.Gate.(gate.ControlledGate); ok {
		nControls := cg.NumControls()
		innerName := gateBaseName(cg.Inner().Name())
		innerParams := cg.Inner().Params()
		targetLabel := formatGateWithParams(innerName, innerParams, cfg)

		labels := make([]string, len(qubits))
		for i := range nControls {
			labels[i] = "@"
		}
		for i := nControls; i < len(qubits); i++ {
			labels[i] = targetLabel
		}
		return labels
	}

	// Check for controlled parameterized gates (CRZ, CRX, CRY, CP).
	if strings.HasPrefix(name, "C") && len(qubits) == 2 {
		base := controlledBaseName(name)
		if base != "" {
			params := op.Gate.Params()
			targetLabel := formatGateWithParams(base, params, cfg)
			return []string{"@", targetLabel}
		}
	}

	// Single-qubit or other gates.
	baseName := gateBaseName(name)
	params := op.Gate.Params()
	label := formatGateWithParams(baseName, params, cfg)
	labels := make([]string, len(qubits))
	for i := range labels {
		labels[i] = label
	}
	return labels
}

// controlledBaseName extracts the target gate name from a controlled gate.
// Returns "" if the name doesn't match a known controlled pattern.
func controlledBaseName(name string) string {
	// Strip parameter suffix: "CRZ(0.7854)" -> "CRZ"
	base := name
	if idx := strings.Index(name, "("); idx != -1 {
		base = name[:idx]
	}
	switch base {
	case "CRZ":
		return "RZ"
	case "CRX":
		return "RX"
	case "CRY":
		return "RY"
	case "CP":
		return "P"
	default:
		return ""
	}
}

// gateBaseName returns the base gate name, stripping parameters.
func gateBaseName(name string) string {
	if idx := strings.Index(name, "("); idx != -1 {
		return name[:idx]
	}
	return name
}

// formatGateWithParams formats a gate label with pi-fraction parameters.
func formatGateWithParams(baseName string, params []float64, cfg *config) string {
	if len(params) == 0 {
		return truncate(baseName, cfg.maxLabelWidth)
	}
	pstrs := make([]string, len(params))
	for i, p := range params {
		pstrs[i] = piformat.FormatASCII(p)
	}
	label := baseName + "(" + strings.Join(pstrs, ",") + ")"
	return truncate(label, cfg.maxLabelWidth)
}

// truncate truncates a string to maxLen, adding ".." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}

// dashPad centers label in a field of width fieldWidth, padding with dashes.
// Total output width = fieldWidth + 2 (one dash on each side minimum).
func dashPad(label string, fieldWidth int) string {
	totalWidth := fieldWidth + 2
	labelLen := len(label)
	if labelLen >= totalWidth {
		return label
	}
	leftPad := (totalWidth - labelLen) / 2
	rightPad := totalWidth - labelLen - leftPad
	return strings.Repeat("-", leftPad) + label + strings.Repeat("-", rightPad)
}

// padRight pads s with spaces to width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
