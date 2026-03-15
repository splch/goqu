package viz

import (
	"math"
	"strings"
	"testing"
)

func TestBloch_ValidXML(t *testing.T) {
	state := []complex128{1, 0} // |0>
	svg := Bloch(state)
	validXML(t, svg)
}

func TestBloch_ContainsElements(t *testing.T) {
	state := []complex128{1, 0}
	svg := Bloch(state)
	if !strings.Contains(svg, "<circle") {
		t.Error("missing circle elements")
	}
	if !strings.Contains(svg, "<line") {
		t.Error("missing line elements")
	}
	if !strings.Contains(svg, "<path") {
		t.Error("missing path elements (great circles)")
	}
}

func TestBloch_ZeroState(t *testing.T) {
	state := []complex128{1, 0} // |0>
	x, y, z := blochCoords(state)
	if math.Abs(x) > 1e-10 || math.Abs(y) > 1e-10 || math.Abs(z-1) > 1e-10 {
		t.Errorf("|0> Bloch coords: got (%.4f, %.4f, %.4f), want (0, 0, 1)", x, y, z)
	}
}

func TestBloch_OneState(t *testing.T) {
	state := []complex128{0, 1} // |1>
	x, y, z := blochCoords(state)
	if math.Abs(x) > 1e-10 || math.Abs(y) > 1e-10 || math.Abs(z+1) > 1e-10 {
		t.Errorf("|1> Bloch coords: got (%.4f, %.4f, %.4f), want (0, 0, -1)", x, y, z)
	}
}

func TestBloch_PlusState(t *testing.T) {
	s := 1 / math.Sqrt2
	state := []complex128{complex(s, 0), complex(s, 0)} // |+>
	x, y, z := blochCoords(state)
	if math.Abs(x-1) > 1e-10 || math.Abs(y) > 1e-10 || math.Abs(z) > 1e-10 {
		t.Errorf("|+> Bloch coords: got (%.4f, %.4f, %.4f), want (1, 0, 0)", x, y, z)
	}
}

func TestBloch_MinusState(t *testing.T) {
	s := 1 / math.Sqrt2
	state := []complex128{complex(s, 0), complex(-s, 0)} // |->
	x, y, z := blochCoords(state)
	if math.Abs(x+1) > 1e-10 || math.Abs(y) > 1e-10 || math.Abs(z) > 1e-10 {
		t.Errorf("|-> Bloch coords: got (%.4f, %.4f, %.4f), want (-1, 0, 0)", x, y, z)
	}
}

func TestBloch_PlusIState(t *testing.T) {
	s := 1 / math.Sqrt2
	state := []complex128{complex(s, 0), complex(0, s)} // |+i>
	x, y, z := blochCoords(state)
	if math.Abs(x) > 1e-10 || math.Abs(y-1) > 1e-10 || math.Abs(z) > 1e-10 {
		t.Errorf("|+i> Bloch coords: got (%.4f, %.4f, %.4f), want (0, 1, 0)", x, y, z)
	}
}

func TestBloch_InvalidLength(t *testing.T) {
	svg := Bloch([]complex128{1, 0, 0, 0})
	validXML(t, svg)
	if strings.Contains(svg, "<circle") {
		t.Error("invalid state should produce empty SVG")
	}
}

func TestBloch_NilState(t *testing.T) {
	svg := Bloch(nil)
	validXML(t, svg)
}

func TestBloch_DarkStyle(t *testing.T) {
	state := []complex128{1, 0}
	svg := Bloch(state, WithStyle(DarkStyle()))
	if !strings.Contains(svg, DarkStyle().BackgroundColor) {
		t.Error("dark background color not found")
	}
}

func TestBloch_WithTitle(t *testing.T) {
	state := []complex128{1, 0}
	svg := Bloch(state, WithTitle("Qubit State"))
	validXML(t, svg)
	if !strings.Contains(svg, "Qubit State") {
		t.Error("title not found")
	}
}

func TestFprintBloch_Basic(t *testing.T) {
	state := []complex128{1, 0}
	var sb strings.Builder
	if err := FprintBloch(&sb, state); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "<svg") {
		t.Error("missing svg element")
	}
}

func TestBloch_AxisLabels(t *testing.T) {
	state := []complex128{1, 0}
	svg := Bloch(state)
	for _, label := range []string{"|0\u27E9", "|1\u27E9", "|+\u27E9", "|-\u27E9"} {
		if !strings.Contains(svg, xmlEscape(label)) {
			t.Errorf("missing axis label %s", label)
		}
	}
}
