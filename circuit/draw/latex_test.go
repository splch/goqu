package draw

import (
	"math"
	"strings"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

func TestLaTeX_NilCircuit(t *testing.T) {
	got := LaTeX(nil)
	if !strings.Contains(got, "Empty circuit") {
		t.Errorf("nil circuit should produce empty marker, got: %q", got)
	}
}

func TestLaTeX_ZeroQubits(t *testing.T) {
	c := ir.New("zero", 0, 0, nil, nil)
	got := LaTeX(c)
	if !strings.Contains(got, "Empty circuit") {
		t.Errorf("zero-qubit circuit should produce empty marker, got: %q", got)
	}
}

func TestLaTeX_EmptyCircuit(t *testing.T) {
	c, err := builder.New("empty", 2).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, `\begin{quantikz}`) {
		t.Error("missing \\begin{quantikz}")
	}
	if !strings.Contains(got, `\end{quantikz}`) {
		t.Error("missing \\end{quantikz}")
	}
	if !strings.Contains(got, `\lstick{$q_0$}`) {
		t.Error("missing q_0 label")
	}
	if !strings.Contains(got, `\lstick{$q_1$}`) {
		t.Error("missing q_1 label")
	}
}

func TestLaTeX_SingleH(t *testing.T) {
	c, err := builder.New("h", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, `\gate{H}`) {
		t.Errorf("missing H gate, got:\n%s", got)
	}
}

func TestLaTeX_BellCircuit(t *testing.T) {
	c, err := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, `\gate{H}`) {
		t.Error("missing H gate")
	}
	if !strings.Contains(got, `\ctrl{1}`) {
		t.Error("missing \\ctrl{1}")
	}
	if !strings.Contains(got, `\targ{}`) {
		t.Error("missing \\targ{}")
	}
	if !strings.Contains(got, `\begin{quantikz}`) {
		t.Error("missing \\begin{quantikz}")
	}
	if !strings.Contains(got, `\end{quantikz}`) {
		t.Error("missing \\end{quantikz}")
	}
}

func TestLaTeX_NonAdjacentCNOT(t *testing.T) {
	c, err := builder.New("na", 3).CNOT(0, 2).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, `\ctrl{2}`) {
		t.Errorf("non-adjacent CNOT should have \\ctrl{2}, got:\n%s", got)
	}
	if !strings.Contains(got, `\targ{}`) {
		t.Error("missing \\targ{}")
	}
}

func TestLaTeX_InvertedCNOT(t *testing.T) {
	c, err := builder.New("inv", 2).CNOT(1, 0).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, `\ctrl{-1}`) {
		t.Errorf("inverted CNOT should have \\ctrl{-1}, got:\n%s", got)
	}
}

func TestLaTeX_SWAP(t *testing.T) {
	c, err := builder.New("swap", 2).SWAP(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, `\swap{1}`) {
		t.Errorf("missing \\swap{1}, got:\n%s", got)
	}
	if !strings.Contains(got, `\targX{}`) {
		t.Errorf("missing \\targX{}, got:\n%s", got)
	}
}

func TestLaTeX_CZ(t *testing.T) {
	c := ir.New("cz", 2, 0, []ir.Operation{
		{Gate: gate.CZ, Qubits: []int{0, 1}},
	}, nil)
	got := LaTeX(c)
	if !strings.Contains(got, `\ctrl{1}`) {
		t.Errorf("missing \\ctrl{1}, got:\n%s", got)
	}
	if !strings.Contains(got, `\control{}`) {
		t.Errorf("missing \\control{}, got:\n%s", got)
	}
}

func TestLaTeX_CCX(t *testing.T) {
	c, err := builder.New("ccx", 3).CCX(0, 1, 2).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, `\ctrl{2}`) {
		t.Errorf("missing \\ctrl{2} for first control, got:\n%s", got)
	}
	if !strings.Contains(got, `\ctrl{1}`) {
		t.Errorf("missing \\ctrl{1} for second control, got:\n%s", got)
	}
	if !strings.Contains(got, `\targ{}`) {
		t.Error("missing \\targ{}")
	}
}

func TestLaTeX_CSWAP(t *testing.T) {
	c := ir.New("cswap", 3, 0, []ir.Operation{
		{Gate: gate.CSWAP, Qubits: []int{0, 1, 2}},
	}, nil)
	got := LaTeX(c)
	if !strings.Contains(got, `\ctrl{1}`) {
		t.Errorf("missing \\ctrl{1}, got:\n%s", got)
	}
	if !strings.Contains(got, `\swap{1}`) {
		t.Errorf("missing \\swap{1}, got:\n%s", got)
	}
	if !strings.Contains(got, `\targX{}`) {
		t.Errorf("missing \\targX{}, got:\n%s", got)
	}
}

func TestLaTeX_ParameterizedGate(t *testing.T) {
	c, err := builder.New("rz", 1).RZ(math.Pi/4, 0).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, `R_z`) {
		t.Errorf("missing R_z, got:\n%s", got)
	}
	if !strings.Contains(got, `\frac{\pi}{4}`) {
		t.Errorf("missing pi/4 fraction, got:\n%s", got)
	}
	if !strings.Contains(got, `\left(`) {
		t.Errorf("missing \\left(, got:\n%s", got)
	}
}

func TestLaTeX_ControlledParameterized(t *testing.T) {
	c := ir.New("crz", 2, 0, []ir.Operation{
		{Gate: gate.CRZ(math.Pi / 2), Qubits: []int{0, 1}},
	}, nil)
	got := LaTeX(c)
	if !strings.Contains(got, `\ctrl{1}`) {
		t.Errorf("missing \\ctrl{1}, got:\n%s", got)
	}
	if !strings.Contains(got, `R_z`) {
		t.Errorf("missing R_z, got:\n%s", got)
	}
	if !strings.Contains(got, `\frac{\pi}{2}`) {
		t.Errorf("missing pi/2, got:\n%s", got)
	}
}

func TestLaTeX_ControlledGateInterface(t *testing.T) {
	// Multi-controlled Z gate (3 controls).
	mcz := gate.MCZ(3)
	c := ir.New("mcz", 4, 0, []ir.Operation{
		{Gate: mcz, Qubits: []int{0, 1, 2, 3}},
	}, nil)
	got := LaTeX(c)
	// Should have ctrl commands from each control to the target.
	if !strings.Contains(got, `\ctrl{`) {
		t.Errorf("missing ctrl commands, got:\n%s", got)
	}
	if !strings.Contains(got, `\gate{Z}`) {
		t.Errorf("missing Z gate on target, got:\n%s", got)
	}
}

func TestLaTeX_Measurement(t *testing.T) {
	c, err := builder.New("meas", 2).
		WithClbits(2).
		H(0).
		Measure(0, 0).
		Measure(1, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if count := strings.Count(got, `\meter{}`); count != 2 {
		t.Errorf("expected 2 \\meter{}, got %d in:\n%s", count, got)
	}
}

func TestLaTeX_Barrier(t *testing.T) {
	c, err := builder.New("bar", 2).
		H(0).Barrier(0, 1).X(1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, `\slice{}`) {
		t.Errorf("missing \\slice{}, got:\n%s", got)
	}
}

func TestLaTeX_Reset(t *testing.T) {
	c := ir.New("reset", 1, 0, []ir.Operation{
		{Gate: gate.Reset, Qubits: []int{0}},
	}, nil)
	got := LaTeX(c)
	if !strings.Contains(got, `\ket{0}`) {
		t.Errorf("missing \\ket{0}, got:\n%s", got)
	}
}

func TestLaTeX_Conditioned(t *testing.T) {
	c := ir.New("cond", 1, 1, []ir.Operation{
		{Gate: gate.X, Qubits: []int{0}, Condition: &ir.Condition{Clbit: 0, Value: 1}},
	}, nil)
	got := LaTeX(c)
	if !strings.Contains(got, "c:") {
		t.Errorf("missing condition prefix, got:\n%s", got)
	}
}

func TestLaTeX_RequiresComment(t *testing.T) {
	c, err := builder.New("test", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := LaTeX(c)
	if !strings.Contains(got, "quantikz2") {
		t.Errorf("missing quantikz2 requires comment, got:\n%s", got)
	}
}

func TestFprintLaTeX_MatchesLaTeX(t *testing.T) {
	c, err := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	if err := FprintLaTeX(&sb, c); err != nil {
		t.Fatal(err)
	}
	got := sb.String()
	want := LaTeX(c)
	if got != want {
		t.Errorf("FprintLaTeX and LaTeX differ:\nFprintLaTeX:\n%s\nLaTeX:\n%s", got, want)
	}
}

func TestLaTeX_DaggerGate(t *testing.T) {
	c := ir.New("sdg", 1, 0, []ir.Operation{
		{Gate: gate.Sdg, Qubits: []int{0}},
	}, nil)
	got := LaTeX(c)
	if !strings.Contains(got, `\dagger`) {
		t.Errorf("missing dagger notation, got:\n%s", got)
	}
}

func TestLaTeX_WithMaxLabelWidth(t *testing.T) {
	c, err := builder.New("u3", 1).
		U3(math.Pi/4, math.Pi/2, math.Pi, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	// Should not panic.
	got := LaTeX(c, WithLaTeXMaxLabelWidth(5))
	if !strings.Contains(got, `\begin{quantikz}`) {
		t.Error("missing \\begin{quantikz}")
	}
	_ = got
}
