package viz

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

// maxCityDim is the maximum density matrix dimension (4 qubits = 16).
const maxCityDim = 16

// StateCity returns an SVG isometric 3D bar chart of a density matrix.
// The rho slice must be row-major with length dim*dim, where dim is a power of 2.
func StateCity(rho []complex128, dim int, opts ...Option) string {
	var sb strings.Builder
	_ = FprintStateCity(&sb, rho, dim, opts...)
	return sb.String()
}

// FprintStateCity writes an SVG state city plot to w.
func FprintStateCity(w io.Writer, rho []complex128, dim int, opts ...Option) error {
	cfg := applyOpts(opts)
	sty := cfg.style

	if len(rho) != dim*dim || dim < 1 || dim > maxCityDim || dim&(dim-1) != 0 {
		_, err := io.WriteString(w, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
		return err
	}

	// Determine number of qubits for labels.
	nQubits := 0
	for d := dim; d > 1; d >>= 1 {
		nQubits++
	}

	// Find max absolute value for height scaling.
	maxAbs := 0.0
	for _, v := range rho {
		if a := math.Abs(real(v)); a > maxAbs {
			maxAbs = a
		}
		if a := math.Abs(imag(v)); a > maxAbs {
			maxAbs = a
		}
	}
	if maxAbs < 1e-15 {
		maxAbs = 1
	}

	// Each panel (Real, Imag) gets half the width.
	panelW := (cfg.width - 3*sty.Padding) / 2
	panelH := cfg.height - 2*sty.Padding - 40 // leave room for panel titles

	titleOffsetY := 0.0
	if cfg.title != "" {
		titleOffsetY = 24
	}

	var sb strings.Builder
	sb.WriteString(svgHeader(cfg.width, cfg.height+titleOffsetY, sty))

	if cfg.title != "" {
		fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="middle" font-size="%.0f" font-weight="bold">%s</text>`+"\n",
			cfg.width/2, sty.Padding+14, sty.TextColor, sty.FontSize+2, xmlEscape(cfg.title))
	}

	// Draw two panels.
	panels := []struct {
		label   string
		fill    string
		extract func(complex128) float64
	}{
		{"Re(\u03C1)", sty.RealFill, func(c complex128) float64 { return real(c) }},
		{"Im(\u03C1)", sty.ImagFill, func(c complex128) float64 { return imag(c) }},
	}

	for pi, panel := range panels {
		offsetX := sty.Padding + float64(pi)*(panelW+sty.Padding)
		offsetY := sty.Padding + 30 + titleOffsetY

		// Panel title.
		fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="middle" font-size="%.0f">%s</text>`+"\n",
			offsetX+panelW/2, offsetY-8, sty.TextColor, sty.FontSize, xmlEscape(panel.label))

		renderCityPanel(&sb, rho, dim, nQubits, panel.extract, panel.fill,
			offsetX, offsetY, panelW, panelH, maxAbs, sty)
	}

	sb.WriteString(svgFooter())
	_, err := io.WriteString(w, sb.String())
	return err
}

// cityBar holds the data for one bar in the isometric chart.
type cityBar struct {
	row, col int
	height   float64
}

func renderCityPanel(sb *strings.Builder, rho []complex128, dim, nQubits int,
	extract func(complex128) float64, fill string,
	ox, oy, pw, ph, maxAbs float64, sty *Style) {

	cellSize := 1.0 // unit grid cells
	gridSize := float64(dim) * cellSize

	// Isometric scaling: map grid to panel size.
	// The isometric projection of a grid has width ~gridSize*sqrt(3) and height ~gridSize + maxBarH.
	maxBarH := gridSize * 0.6 // max bar height in grid units
	isoW := gridSize * math.Sqrt(3)
	isoH := gridSize + maxBarH*2 // space for bars above and below

	scale := math.Min(pw/isoW, ph/isoH) * 0.85
	centerX := ox + pw/2
	centerY := oy + ph/2

	// Collect bars.
	bars := make([]cityBar, 0, dim*dim)
	for r := range dim {
		for c := range dim {
			h := extract(rho[r*dim+c])
			bars = append(bars, cityBar{row: r, col: c, height: h})
		}
	}

	// Draw base grid.
	for i := range dim + 1 {
		fi := float64(i) * cellSize
		// Lines along col direction.
		x1, y1 := isoProject(fi, 0, 0, scale, centerX, centerY, gridSize)
		x2, y2 := isoProject(fi, gridSize, 0, scale, centerX, centerY, gridSize)
		fmt.Fprintf(sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.5" opacity="0.4"/>`+"\n",
			x1, y1, x2, y2, sty.GridColor)
		// Lines along row direction.
		x1, y1 = isoProject(0, fi, 0, scale, centerX, centerY, gridSize)
		x2, y2 = isoProject(gridSize, fi, 0, scale, centerX, centerY, gridSize)
		fmt.Fprintf(sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.5" opacity="0.4"/>`+"\n",
			x1, y1, x2, y2, sty.GridColor)
	}

	// Sort bars back-to-front for depth ordering.
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].row+bars[i].col < bars[j].row+bars[j].col
	})

	// Draw bars.
	topFill := fill
	rightFill := darken(fill, 0.85)
	frontFill := darken(fill, 0.70)

	for _, b := range bars {
		h := b.height
		if math.Abs(h) < 1e-15 {
			continue
		}

		barZ := (h / maxAbs) * maxBarH
		x0 := float64(b.col) * cellSize
		y0 := float64(b.row) * cellSize
		x1 := x0 + cellSize
		y1 := y0 + cellSize

		z0 := 0.0
		z1 := barZ
		if barZ < 0 {
			z0, z1 = barZ, 0
		}

		// Project all relevant corners.
		// Top face corners.
		tx0y0, ty0y0 := isoProject(x0, y0, z1, scale, centerX, centerY, gridSize)
		tx1y0, ty1y0 := isoProject(x1, y0, z1, scale, centerX, centerY, gridSize)
		tx1y1, ty1y1 := isoProject(x1, y1, z1, scale, centerX, centerY, gridSize)
		tx0y1, ty0y1 := isoProject(x0, y1, z1, scale, centerX, centerY, gridSize)

		// Bottom face corners (only need the front/right ones).
		bx1y0, by1y0 := isoProject(x1, y0, z0, scale, centerX, centerY, gridSize)
		bx1y1, by1y1 := isoProject(x1, y1, z0, scale, centerX, centerY, gridSize)
		bx0y1, by0y1 := isoProject(x0, y1, z0, scale, centerX, centerY, gridSize)

		// Right face: x1 edge (col side).
		fmt.Fprintf(sb, `<polygon points="%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="%s" stroke="%s" stroke-width="0.5"/>`+"\n",
			tx1y0, ty1y0, bx1y0, by1y0, bx1y1, by1y1, tx1y1, ty1y1,
			rightFill, sty.BarOutline)

		// Front face: y1 edge (row side).
		fmt.Fprintf(sb, `<polygon points="%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="%s" stroke="%s" stroke-width="0.5"/>`+"\n",
			tx0y1, ty0y1, bx0y1, by0y1, bx1y1, by1y1, tx1y1, ty1y1,
			frontFill, sty.BarOutline)

		// Top face.
		fmt.Fprintf(sb, `<polygon points="%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="%s" stroke="%s" stroke-width="0.5"/>`+"\n",
			tx0y0, ty0y0, tx1y0, ty1y0, tx1y1, ty1y1, tx0y1, ty0y1,
			topFill, sty.BarOutline)
	}

	// Row/column labels along the front edges.
	for i := range dim {
		label := formatKet(i, nQubits)
		// Column labels (along bottom-right edge).
		lx, ly := isoProject(float64(i)*cellSize+cellSize/2, gridSize+0.3, 0, scale, centerX, centerY, gridSize)
		fmt.Fprintf(sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="middle" dominant-baseline="hanging" font-size="%.0f">%s</text>`+"\n",
			lx, ly, sty.TextColor, sty.FontSize-2, xmlEscape(label))
		// Row labels (along bottom-left edge).
		lx, ly = isoProject(-0.3, float64(i)*cellSize+cellSize/2, 0, scale, centerX, centerY, gridSize)
		fmt.Fprintf(sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="end" dominant-baseline="middle" font-size="%.0f">%s</text>`+"\n",
			lx, ly, sty.TextColor, sty.FontSize-2, xmlEscape(label))
	}
}

// isoProject maps 3D grid coordinates to 2D SVG coordinates using isometric projection.
// gridSize is used to center the grid.
func isoProject(x, y, z, scale, cx, cy, gridSize float64) (sx, sy float64) {
	// Center the grid at origin.
	x -= gridSize / 2
	y -= gridSize / 2

	cos30 := math.Cos(math.Pi / 6)
	sin30 := math.Sin(math.Pi / 6)

	sx = cx + (x-y)*cos30*scale
	sy = cy + (x+y)*sin30*scale - z*scale
	return
}

// formatKet returns a ket label like "|01>" for a basis index.
func formatKet(idx, nQubits int) string {
	bs := make([]byte, nQubits)
	for i := range nQubits {
		if idx&(1<<(nQubits-1-i)) != 0 {
			bs[i] = '1'
		} else {
			bs[i] = '0'
		}
	}
	return "|" + string(bs) + "\u27E9"
}
