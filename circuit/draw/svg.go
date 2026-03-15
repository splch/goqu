package draw

import (
	"fmt"
	"io"
	"strings"

	"github.com/splch/goqu/circuit/ir"
)

// SVGOption configures SVG rendering.
type SVGOption func(*svgConfig)

type svgConfig struct {
	style         *Style
	maxLabelWidth int
}

// WithStyle sets the SVG rendering style.
func WithStyle(s *Style) SVGOption {
	return func(c *svgConfig) {
		if s != nil {
			c.style = s
		}
	}
}

// WithSVGMaxLabelWidth sets the maximum width for gate labels in SVG.
func WithSVGMaxLabelWidth(n int) SVGOption {
	return func(c *svgConfig) {
		if n > 0 {
			c.maxLabelWidth = n
		}
	}
}

// SVG returns an SVG string of the circuit diagram.
func SVG(c *ir.Circuit, opts ...SVGOption) string {
	var sb strings.Builder
	_ = FprintSVG(&sb, c, opts...)
	return sb.String()
}

// FprintSVG writes an SVG diagram of the circuit to w.
func FprintSVG(w io.Writer, c *ir.Circuit, opts ...SVGOption) error {
	cfg := &svgConfig{
		style:         DefaultStyle(),
		maxLabelWidth: 10,
	}
	for _, o := range opts {
		o(cfg)
	}

	if c == nil {
		_, err := io.WriteString(w, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
		return err
	}

	sty := cfg.style
	nq := c.NumQubits()
	ops := c.Ops()

	if nq == 0 {
		_, err := io.WriteString(w, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
		return err
	}

	// Reuse the existing assignColumns function from draw.go.
	placements, numCols := assignColumns(ops, nq)

	// Compute dimensions.
	labelWidth := sty.Padding // space for qubit labels on left
	totalWidth := labelWidth + float64(numCols)*sty.ColWidth + sty.Padding
	totalHeight := float64(nq)*sty.RowHeight + sty.Padding

	var sb strings.Builder

	// SVG header.
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" font-family="%s" font-size="%.0f">`,
		totalWidth, totalHeight, sty.FontFamily, sty.FontSize)
	sb.WriteString("\n")

	// Background.
	fmt.Fprintf(&sb, `<rect width="100%%" height="100%%" fill="%s"/>`, sty.BackgroundColor)
	sb.WriteString("\n")

	// Draw qubit labels and wires.
	for q := range nq {
		y := wireY(q, sty)

		// Qubit label.
		fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="end" dominant-baseline="middle">q%d</text>`,
			labelWidth-8, y, sty.TextColor, q)
		sb.WriteString("\n")

		// Horizontal wire.
		fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`,
			labelWidth, y, totalWidth-sty.Padding/2, y, sty.WireColor)
		sb.WriteString("\n")
	}

	// Build an ASCII config for label generation.
	asciiCfg := &config{maxLabelWidth: cfg.maxLabelWidth}

	// Draw gates.
	for _, p := range placements {
		// Control flow: draw a dashed rectangle spanning all qubits.
		if p.op.ControlFlow != nil {
			label := controlFlowLabel(p.op.ControlFlow)
			cx := labelWidth + (float64(p.col)+0.5)*sty.ColWidth
			y0 := wireY(0, sty) - sty.GateHeight/2
			y1 := wireY(nq-1, sty) + sty.GateHeight/2
			hw := sty.GateHeight * 1.2
			fmt.Fprintf(&sb, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="4" fill="none" stroke="%s" stroke-width="1.5" stroke-dasharray="4,3"/>`,
				cx-hw/2, y0, hw, y1-y0, sty.WireColor)
			sb.WriteString("\n")
			fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="middle" dominant-baseline="middle" font-size="%.0f">%s</text>`,
				cx, (y0+y1)/2, sty.TextColor, sty.FontSize*0.8, label)
			sb.WriteString("\n")
			continue
		}

		labels := gateLabels(p.op, asciiCfg)
		qubits := p.op.Qubits

		if len(qubits) == 0 {
			continue
		}

		// Find qubit range for connectors.
		minQ, maxQ := qubits[0], qubits[0]
		for _, q := range qubits[1:] {
			if q < minQ {
				minQ = q
			}
			if q > maxQ {
				maxQ = q
			}
		}

		// Draw vertical connector line for multi-qubit gates.
		if len(qubits) > 1 {
			x := gateX(p.col, sty)
			y1 := wireY(minQ, sty)
			y2 := wireY(maxQ, sty)
			fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`,
				x, y1, x, y2, sty.WireColor)
			sb.WriteString("\n")
		}

		// Draw each qubit's gate element.
		for i, q := range qubits {
			label := labels[i]
			x := gateX(p.col, sty)
			y := wireY(q, sty)

			switch label {
			case "@":
				// Control dot.
				fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="5" fill="%s"/>`,
					x, y, sty.ControlFill)
				sb.WriteString("\n")

			case "X":
				// Check if this is a CNOT target (has a control "@" sibling).
				isCNOTTarget := false
				for j, l := range labels {
					if j != i && l == "@" {
						isCNOTTarget = true
						break
					}
				}
				if isCNOTTarget {
					// CNOT target: circle with crosshairs.
					r := sty.GateHeight / 2.5
					fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="%s" stroke-width="1.5"/>`,
						x, y, r, sty.WireColor)
					fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1.5"/>`,
						x, y-r, x, y+r, sty.WireColor)
					fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1.5"/>`,
						x-r, y, x+r, y, sty.WireColor)
					sb.WriteString("\n")
				} else {
					// Regular X gate box.
					drawGateBox(&sb, x, y, label, sty.Gate1QFill, sty)
				}

			case "x":
				// SWAP cross.
				r := 6.0
				fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="2"/>`,
					x-r, y-r, x+r, y+r, sty.WireColor)
				fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="2"/>`,
					x-r, y+r, x+r, y-r, sty.WireColor)
				sb.WriteString("\n")

			case "+":
				// Vertical connector pass-through (already handled by the connector line).

			case "M":
				// Measurement box.
				drawGateBox(&sb, x, y, "M", sty.MeasureFill, sty)

			case "|":
				// Barrier: dashed vertical line.
				fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1" stroke-dasharray="4,4"/>`,
					x, y-sty.GateHeight/2, x, y+sty.GateHeight/2, sty.WireColor)
				sb.WriteString("\n")

			case "|0>":
				// Reset.
				drawGateBox(&sb, x, y, "|0\u27E9", sty.Gate1QFill, sty)

			default:
				// Standard gate box.
				fill := sty.Gate1QFill
				if len(qubits) > 1 {
					fill = sty.Gate2QFill
				}
				drawGateBox(&sb, x, y, label, fill, sty)
			}
		}
	}

	sb.WriteString("</svg>")

	_, err := io.WriteString(w, sb.String())
	return err
}

// wireY returns the Y coordinate for a qubit wire.
func wireY(qubit int, sty *Style) float64 {
	return sty.Padding/2 + float64(qubit)*sty.RowHeight + sty.RowHeight/2
}

// gateX returns the X coordinate for a gate in a given column.
func gateX(col int, sty *Style) float64 {
	return sty.Padding + float64(col)*sty.ColWidth + sty.ColWidth/2
}

// drawGateBox draws a labeled rectangle gate.
func drawGateBox(sb *strings.Builder, cx, cy float64, label, fill string, sty *Style) {
	// Estimate width from label length.
	w := sty.GateWidth
	labelW := float64(len(label)) * sty.FontSize * 0.6
	if labelW+10 > w {
		w = labelW + 10
	}
	h := sty.GateHeight

	fmt.Fprintf(sb, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="4" fill="%s" stroke="%s" stroke-width="1"/>`,
		cx-w/2, cy-h/2, w, h, fill, sty.WireColor)
	fmt.Fprintf(sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="middle" dominant-baseline="middle">%s</text>`,
		cx, cy, sty.TextColor, xmlEscape(label))
	sb.WriteString("\n")
}

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
