package viz

import (
	"fmt"
	"strings"
)

// Style configures SVG rendering appearance for all visualizations.
type Style struct {
	BackgroundColor string
	TextColor       string
	AxisColor       string
	GridColor       string
	FontFamily      string
	FontSize        float64

	// Histogram bar colors.
	BarFill   string
	BarStroke string

	// Bloch sphere colors.
	SphereStroke string
	StateColor   string

	// State city plot colors.
	RealFill   string
	ImagFill   string
	BarOutline string

	Padding float64
}

// DefaultStyle returns a light-theme style.
func DefaultStyle() *Style {
	return &Style{
		BackgroundColor: "#FFFFFF",
		TextColor:       "#333333",
		AxisColor:       "#333333",
		GridColor:       "#DDDDDD",
		FontFamily:      "monospace",
		FontSize:        12,
		BarFill:         "#4C72B0",
		BarStroke:       "#2E4A7A",
		SphereStroke:    "#AAAAAA",
		StateColor:      "#C44E52",
		RealFill:        "#4C72B0",
		ImagFill:        "#DD8452",
		BarOutline:      "#333333",
		Padding:         20,
	}
}

// DarkStyle returns a dark-theme style.
func DarkStyle() *Style {
	return &Style{
		BackgroundColor: "#1E1E1E",
		TextColor:       "#CCCCCC",
		AxisColor:       "#999999",
		GridColor:       "#333333",
		FontFamily:      "monospace",
		FontSize:        12,
		BarFill:         "#6A9BD2",
		BarStroke:       "#4A7BB2",
		SphereStroke:    "#555555",
		StateColor:      "#E07070",
		RealFill:        "#6A9BD2",
		ImagFill:        "#E8A070",
		BarOutline:      "#999999",
		Padding:         20,
	}
}

// svgHeader returns the opening SVG tag with dimensions and background.
func svgHeader(w, h float64, sty *Style) string {
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" font-family="%s" font-size="%.0f">`+"\n"+
			`<rect width="100%%" height="100%%" fill="%s"/>`+"\n",
		w, h, sty.FontFamily, sty.FontSize, sty.BackgroundColor)
}

// svgFooter returns the closing SVG tag.
func svgFooter() string { return "</svg>" }

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// darken returns a hex color darkened by factor (0.0 = black, 1.0 = unchanged).
func darken(hex string, factor float64) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "#000000"
	}
	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	r = int(float64(r) * factor)
	g = int(float64(g) * factor)
	b = int(float64(b) * factor)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}
