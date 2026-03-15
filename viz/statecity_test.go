package viz

import (
	"strings"
	"testing"
)

func TestStateCity_ValidXML(t *testing.T) {
	// |0><0| identity-like for 1 qubit.
	rho := []complex128{1, 0, 0, 0}
	svg := StateCity(rho, 2)
	validXML(t, svg)
}

func TestStateCity_ContainsPolygons(t *testing.T) {
	rho := []complex128{1, 0, 0, 0}
	svg := StateCity(rho, 2)
	if !strings.Contains(svg, "<polygon") {
		t.Error("missing polygon elements for 3D bars")
	}
}

func TestStateCity_ContainsLabels(t *testing.T) {
	rho := []complex128{1, 0, 0, 0}
	svg := StateCity(rho, 2)
	if !strings.Contains(svg, xmlEscape("|0\u27E9")) {
		t.Error("missing |0> label")
	}
	if !strings.Contains(svg, xmlEscape("|1\u27E9")) {
		t.Error("missing |1> label")
	}
}

func TestStateCity_ContainsPanelTitles(t *testing.T) {
	rho := []complex128{1, 0, 0, 0}
	svg := StateCity(rho, 2)
	if !strings.Contains(svg, xmlEscape("Re(\u03C1)")) {
		t.Error("missing Re(rho) panel title")
	}
	if !strings.Contains(svg, xmlEscape("Im(\u03C1)")) {
		t.Error("missing Im(rho) panel title")
	}
}

func TestStateCity_TwoQubits(t *testing.T) {
	// Bell state |Φ+> = (|00>+|11>)/√2, ρ has nonzero entries at corners.
	rho := make([]complex128, 16)
	rho[0] = 0.5  // |00><00|
	rho[3] = 0.5  // |00><11|
	rho[12] = 0.5 // |11><00|
	rho[15] = 0.5 // |11><11|
	svg := StateCity(rho, 4)
	validXML(t, svg)
	if !strings.Contains(svg, "<polygon") {
		t.Error("missing bars for Bell state")
	}
	// Should have 2-qubit ket labels.
	if !strings.Contains(svg, xmlEscape("|00\u27E9")) {
		t.Error("missing |00> label")
	}
}

func TestStateCity_EmptyMatrix(t *testing.T) {
	svg := StateCity(nil, 0)
	validXML(t, svg)
}

func TestStateCity_DimMismatch(t *testing.T) {
	rho := []complex128{1, 0, 0} // length 3 != 2*2
	svg := StateCity(rho, 2)
	validXML(t, svg)
	if strings.Contains(svg, "<polygon") {
		t.Error("dim mismatch should produce empty SVG")
	}
}

func TestStateCity_NonPowerOfTwo(t *testing.T) {
	rho := make([]complex128, 9) // 3x3, not power of 2
	svg := StateCity(rho, 3)
	validXML(t, svg)
	if strings.Contains(svg, "<polygon") {
		t.Error("non-power-of-2 dim should produce empty SVG")
	}
}

func TestStateCity_DarkStyle(t *testing.T) {
	rho := []complex128{1, 0, 0, 0}
	svg := StateCity(rho, 2, WithStyle(DarkStyle()))
	if !strings.Contains(svg, DarkStyle().BackgroundColor) {
		t.Error("dark background color not found")
	}
}

func TestStateCity_WithTitle(t *testing.T) {
	rho := []complex128{1, 0, 0, 0}
	svg := StateCity(rho, 2, WithTitle("Density Matrix"))
	validXML(t, svg)
	if !strings.Contains(svg, "Density Matrix") {
		t.Error("title not found")
	}
}

func TestStateCity_TooLarge(t *testing.T) {
	// dim=32 exceeds maxCityDim=16.
	rho := make([]complex128, 32*32)
	svg := StateCity(rho, 32)
	validXML(t, svg)
	if strings.Contains(svg, "<polygon") {
		t.Error("dim>16 should produce empty SVG")
	}
}

func TestFprintStateCity_Basic(t *testing.T) {
	rho := []complex128{1, 0, 0, 0}
	var sb strings.Builder
	if err := FprintStateCity(&sb, rho, 2); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "<svg") {
		t.Error("missing svg element")
	}
}

func TestStateCity_ImaginaryPart(t *testing.T) {
	// Matrix with imaginary off-diagonal: ρ = |+i><+i|.
	rho := []complex128{
		0.5, complex(0, -0.5),
		complex(0, 0.5), 0.5,
	}
	svg := StateCity(rho, 2)
	validXML(t, svg)
	// Should have bars in the imaginary panel.
	if !strings.Contains(svg, "<polygon") {
		t.Error("missing imaginary bars")
	}
}

func TestFormatKet(t *testing.T) {
	tests := []struct {
		idx     int
		nQubits int
		want    string
	}{
		{0, 1, "|0\u27E9"},
		{1, 1, "|1\u27E9"},
		{0, 2, "|00\u27E9"},
		{1, 2, "|01\u27E9"},
		{2, 2, "|10\u27E9"},
		{3, 2, "|11\u27E9"},
	}
	for _, tt := range tests {
		got := formatKet(tt.idx, tt.nQubits)
		if got != tt.want {
			t.Errorf("formatKet(%d, %d) = %q, want %q", tt.idx, tt.nQubits, got, tt.want)
		}
	}
}
