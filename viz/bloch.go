package viz

import (
	"fmt"
	"io"
	"math"
	"math/cmplx"
	"strings"
)

// Default viewing angles for the Bloch sphere projection.
const (
	blochAzimuth   = -math.Pi / 6  // 30 degrees
	blochElevation = math.Pi / 8   // 22.5 degrees
	blochSegments  = 72            // line segments per great circle
)

// Bloch returns an SVG rendering of a single-qubit state on the Bloch sphere.
// The state must have length 2.
func Bloch(state []complex128, opts ...Option) string {
	var sb strings.Builder
	_ = FprintBloch(&sb, state, opts...)
	return sb.String()
}

// FprintBloch writes an SVG Bloch sphere rendering to w.
func FprintBloch(w io.Writer, state []complex128, opts ...Option) error {
	cfg := applyOpts(opts)
	sty := cfg.style

	if len(state) != 2 {
		_, err := io.WriteString(w, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
		return err
	}

	size := math.Min(cfg.width, cfg.height)
	radius := (size - 2*sty.Padding - 40) / 2 // leave room for labels
	cx := cfg.width / 2
	cy := cfg.height / 2

	bx, by, bz := blochCoords(state)

	var sb strings.Builder
	sb.WriteString(svgHeader(cfg.width, cfg.height, sty))

	// Title.
	if cfg.title != "" {
		fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="middle" font-size="%.0f" font-weight="bold">%s</text>`+"\n",
			cx, sty.Padding+14, sty.TextColor, sty.FontSize+2, xmlEscape(cfg.title))
	}

	// Great circles (low opacity wireframe).
	circles := [3][2][3]float64{
		// XY-plane (equator)
		{{1, 0, 0}, {0, 1, 0}},
		// XZ-plane
		{{1, 0, 0}, {0, 0, 1}},
		// YZ-plane
		{{0, 1, 0}, {0, 0, 1}},
	}
	for _, c := range circles {
		path := greatCirclePath(c[0], c[1], radius, cx, cy)
		fmt.Fprintf(&sb, `<path d="%s" fill="none" stroke="%s" stroke-width="0.5" opacity="0.4"/>`+"\n",
			path, sty.SphereStroke)
	}

	// Outer sphere circle (projected boundary).
	fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="%s" stroke-width="1" opacity="0.6"/>`+"\n",
		cx, cy, radius, sty.SphereStroke)

	// Axis lines.
	type axisLabel struct {
		x, y, z float64
		label   string
	}
	axes := []axisLabel{
		{1, 0, 0, "|+\u27E9"},
		{-1, 0, 0, "|-\u27E9"},
		{0, 1, 0, "|+i\u27E9"},
		{0, -1, 0, "|-i\u27E9"},
		{0, 0, 1, "|0\u27E9"},
		{0, 0, -1, "|1\u27E9"},
	}
	for i := range 3 {
		pos := axes[i*2]
		neg := axes[i*2+1]
		px1, py1 := blochProject(pos.x, pos.y, pos.z, radius, cx, cy)
		px2, py2 := blochProject(neg.x, neg.y, neg.z, radius, cx, cy)
		fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.8" opacity="0.6"/>`+"\n",
			px1, py1, px2, py2, sty.AxisColor)
	}

	// Axis labels.
	for _, a := range axes {
		px, py := blochProject(a.x*1.15, a.y*1.15, a.z*1.15, radius, cx, cy)
		fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="middle" dominant-baseline="middle" font-size="%.0f">%s</text>`+"\n",
			px, py, sty.TextColor, sty.FontSize, xmlEscape(a.label))
	}

	// State vector: dashed projection to XY-plane.
	projXY_px, projXY_py := blochProject(bx, by, 0, radius, cx, cy)
	statePx, statePy := blochProject(bx, by, bz, radius, cx, cy)
	fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.8" stroke-dasharray="3,3" opacity="0.5"/>`+"\n",
		statePx, statePy, projXY_px, projXY_py, sty.StateColor)

	// State vector arrow: line from origin to state point.
	fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="2"/>`+"\n",
		cx, cy, statePx, statePy, sty.StateColor)

	// State vector dot.
	fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="5" fill="%s"/>`+"\n",
		statePx, statePy, sty.StateColor)

	sb.WriteString(svgFooter())
	_, err := io.WriteString(w, sb.String())
	return err
}

// blochCoords converts a single-qubit state to Bloch sphere coordinates.
func blochCoords(state []complex128) (x, y, z float64) {
	alpha, beta := state[0], state[1]
	ab := alpha * cmplx.Conj(beta)
	x = 2 * real(ab)
	y = -2 * imag(ab) // negative sign: y = Tr(rho * sigma_y) = -2*Im(alpha*conj(beta))
	z = cmplx.Abs(alpha)*cmplx.Abs(alpha) - cmplx.Abs(beta)*cmplx.Abs(beta)
	return
}

// blochProject projects 3D Bloch coordinates to 2D SVG coordinates.
func blochProject(bx, by, bz, radius, cx, cy float64) (sx, sy float64) {
	px, py := project3D(bx, by, bz)
	sx = cx + px*radius
	sy = cy - py*radius // SVG Y is inverted
	return
}

// project3D projects 3D coordinates to 2D using oblique projection.
func project3D(bx, by, bz float64) (sx, sy float64) {
	cosA, sinA := math.Cos(blochAzimuth), math.Sin(blochAzimuth)
	cosE, sinE := math.Cos(blochElevation), math.Sin(blochElevation)
	rx := bx*cosA - by*sinA
	ry := bx*sinA + by*cosA
	sx = rx
	sy = bz*cosE + ry*sinE
	return
}

// greatCirclePath generates an SVG path for a great circle defined by two
// orthogonal unit vectors u and v.
func greatCirclePath(u, v [3]float64, radius, cx, cy float64) string {
	var sb strings.Builder
	for i := range blochSegments + 1 {
		t := 2 * math.Pi * float64(i) / float64(blochSegments)
		bx := math.Cos(t)*u[0] + math.Sin(t)*v[0]
		by := math.Cos(t)*u[1] + math.Sin(t)*v[1]
		bz := math.Cos(t)*u[2] + math.Sin(t)*v[2]
		px, py := blochProject(bx, by, bz, radius, cx, cy)
		if i == 0 {
			fmt.Fprintf(&sb, "M%.1f,%.1f", px, py)
		} else {
			fmt.Fprintf(&sb, " L%.1f,%.1f", px, py)
		}
	}
	sb.WriteString(" Z")
	return sb.String()
}
