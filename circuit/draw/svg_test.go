package draw

import (
	"encoding/xml"
	"math"
	"strings"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

func TestSVG_ValidXML(t *testing.T) {
	c, err := builder.New("bell", 2).H(0).CNOT(0, 1).MeasureAll().Build()
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(c)
	d := xml.NewDecoder(strings.NewReader(svg))
	for {
		if _, err := d.Token(); err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Errorf("invalid XML: %v\nSVG:\n%s", err, svg)
			break
		}
	}
}

func TestSVG_ContainsElements(t *testing.T) {
	c, err := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(c)
	// Should contain qubit labels.
	if !strings.Contains(svg, "q0") {
		t.Error("missing q0 label")
	}
	if !strings.Contains(svg, "q1") {
		t.Error("missing q1 label")
	}
	// Should contain H gate.
	if !strings.Contains(svg, ">H<") {
		t.Error("missing H gate label")
	}
	// Should contain control dot (circle with fill for control).
	if !strings.Contains(svg, "<circle") {
		t.Error("missing control dot circle")
	}
	// Should have SVG namespace.
	if !strings.Contains(svg, `xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("missing SVG namespace")
	}
}

func TestSVG_EmptyCircuit(t *testing.T) {
	c, err := builder.New("empty", 2).Build()
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(c)
	if !strings.Contains(svg, "<svg") {
		t.Error("missing svg element")
	}
	// Should have wires but no gate boxes.
	if !strings.Contains(svg, "<line") {
		t.Error("missing wire lines")
	}
}

func TestSVG_ParameterizedGates(t *testing.T) {
	c, err := builder.New("params", 1).RZ(math.Pi, 0).Build()
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(c)
	if !strings.Contains(svg, "RZ") {
		t.Error("missing RZ gate label")
	}
}

func TestSVG_DarkStyle(t *testing.T) {
	c, err := builder.New("dark", 2).H(0).CNOT(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(c, WithStyle(DarkStyle()))
	if !strings.Contains(svg, "#1E1E1E") {
		t.Error("dark background color not found")
	}
}

func TestSVG_Measurement(t *testing.T) {
	c, err := builder.New("meas", 1).H(0).MeasureAll().Build()
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(c)
	if !strings.Contains(svg, ">M<") {
		t.Error("missing measurement label")
	}
}

func TestSVG_SWAP(t *testing.T) {
	c, err := builder.New("swap", 2).SWAP(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(c)
	// SWAP should have crossing lines.
	if !strings.Contains(svg, `stroke-width="2"`) {
		t.Error("missing SWAP cross lines")
	}
}

func TestFprintSVG_Basic(t *testing.T) {
	c, err := builder.New("test", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	if err := FprintSVG(&sb, c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "<svg") {
		t.Error("missing svg element")
	}
}

func TestSVG_NilCircuit(t *testing.T) {
	svg := SVG(nil)
	if !strings.Contains(svg, "<svg") {
		t.Error("nil circuit should produce empty SVG element")
	}
	if !strings.Contains(svg, `xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("nil circuit SVG missing namespace")
	}
}

func TestSVG_ZeroQubits(t *testing.T) {
	c := ir.New("zero", 0, 0, nil, nil)
	svg := SVG(c)
	if !strings.Contains(svg, "<svg") {
		t.Error("zero-qubit circuit should produce empty SVG element")
	}
}

func TestSVG_WithSVGMaxLabelWidth(t *testing.T) {
	c, err := builder.New("u3", 1).
		U3(math.Pi/4, math.Pi/2, math.Pi, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(c, WithSVGMaxLabelWidth(5))
	if !strings.Contains(svg, "<svg") {
		t.Error("missing svg element")
	}
	// Label should be truncated.
	if !strings.Contains(svg, "U3") {
		t.Error("missing U3 gate")
	}
}

func TestSVG_WithStyleNil(t *testing.T) {
	c, err := builder.New("test", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	// WithStyle(nil) should keep default style.
	svg := SVG(c, WithStyle(nil))
	if !strings.Contains(svg, "#FFFFFF") {
		t.Error("nil style should fall back to default (white background)")
	}
}

func TestSVG_MultiQubitGateFill(t *testing.T) {
	c := ir.New("crz", 2, 0, []ir.Operation{
		{Gate: gate.CRZ(math.Pi / 2), Qubits: []int{0, 1}},
	}, nil)
	svg := SVG(c)
	// The target qubit box should use Gate2QFill color.
	sty := DefaultStyle()
	if !strings.Contains(svg, sty.Gate2QFill) {
		t.Errorf("multi-qubit gate should use Gate2QFill (%s)", sty.Gate2QFill)
	}
}

func TestSVG_Barrier(t *testing.T) {
	c, err := builder.New("bar", 2).
		H(0).Barrier(0, 1).X(1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	svg := SVG(c)
	// Barrier should render as dashed line.
	if !strings.Contains(svg, "stroke-dasharray") {
		t.Error("barrier should use dashed line")
	}
}
