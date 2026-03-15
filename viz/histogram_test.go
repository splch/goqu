package viz

import (
	"encoding/xml"
	"strings"
	"testing"
)

func validXML(t *testing.T, svg string) {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(svg))
	for {
		if _, err := d.Token(); err != nil {
			if err.Error() == "EOF" {
				return
			}
			t.Errorf("invalid XML: %v\nSVG:\n%s", err, svg)
			return
		}
	}
}

func TestHistogram_ValidXML(t *testing.T) {
	counts := map[string]int{"00": 512, "11": 488}
	svg := Histogram(counts)
	validXML(t, svg)
}

func TestHistogram_ContainsBars(t *testing.T) {
	counts := map[string]int{"00": 512, "11": 488}
	svg := Histogram(counts)
	if !strings.Contains(svg, "<rect") {
		t.Error("missing bar rect elements")
	}
}

func TestHistogram_ContainsLabels(t *testing.T) {
	counts := map[string]int{"00": 512, "11": 488}
	svg := Histogram(counts)
	if !strings.Contains(svg, "00") {
		t.Error("missing label 00")
	}
	if !strings.Contains(svg, "11") {
		t.Error("missing label 11")
	}
}

func TestHistogram_EmptyMap(t *testing.T) {
	svg := Histogram(map[string]int{})
	validXML(t, svg)
	if !strings.Contains(svg, "<svg") {
		t.Error("empty map should produce valid SVG")
	}
}

func TestHistogram_NilMap(t *testing.T) {
	svg := Histogram(nil)
	validXML(t, svg)
}

func TestHistogram_SingleEntry(t *testing.T) {
	counts := map[string]int{"0": 1000}
	svg := Histogram(counts)
	validXML(t, svg)
	if strings.Count(svg, "fill=\""+DefaultStyle().BarFill+"\"") < 1 {
		t.Error("expected at least one bar with BarFill color")
	}
}

func TestHistogram_DarkStyle(t *testing.T) {
	counts := map[string]int{"00": 500, "11": 500}
	svg := Histogram(counts, WithStyle(DarkStyle()))
	if !strings.Contains(svg, DarkStyle().BackgroundColor) {
		t.Error("dark background color not found")
	}
}

func TestHistogram_WithTitle(t *testing.T) {
	counts := map[string]int{"00": 500, "11": 500}
	svg := Histogram(counts, WithTitle("Bell State"))
	validXML(t, svg)
	if !strings.Contains(svg, "Bell State") {
		t.Error("title not found in SVG")
	}
}

func TestHistogramProb_ValidXML(t *testing.T) {
	probs := map[string]float64{"00": 0.5, "11": 0.5}
	svg := HistogramProb(probs)
	validXML(t, svg)
}

func TestHistogramProb_ContainsLabels(t *testing.T) {
	probs := map[string]float64{"00": 0.5, "11": 0.5}
	svg := HistogramProb(probs)
	if !strings.Contains(svg, "00") {
		t.Error("missing label 00")
	}
}

func TestFprintHistogram_Basic(t *testing.T) {
	counts := map[string]int{"0": 100, "1": 100}
	var sb strings.Builder
	if err := FprintHistogram(&sb, counts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "<svg") {
		t.Error("missing svg element")
	}
}

func TestHistogram_ManyBars(t *testing.T) {
	// More than 8 bars triggers rotated labels.
	counts := map[string]int{
		"000": 100, "001": 90, "010": 80, "011": 70,
		"100": 60, "101": 50, "110": 40, "111": 30, "extra": 10,
	}
	svg := Histogram(counts)
	validXML(t, svg)
	if !strings.Contains(svg, "rotate") {
		t.Error("expected rotated labels for >8 bars")
	}
}
