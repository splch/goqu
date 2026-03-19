package draw

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

func TestEmptyCircuit(t *testing.T) {
	c := ir.New("empty", 2, 0, nil, nil)
	got := String(c)
	want := "q0: ---\nq1: ---\n"
	if got != want {
		t.Errorf("empty circuit:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestZeroQubits(t *testing.T) {
	c := ir.New("zero", 0, 0, nil, nil)
	got := String(c)
	if got != "" {
		t.Errorf("zero qubit circuit: got %q, want empty", got)
	}
}

func TestSingleH(t *testing.T) {
	c, err := builder.New("h", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -H--\n"
	if got != want {
		t.Errorf("single H:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestBellCircuit(t *testing.T) {
	c, err := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -H---@--\n" +
		"         |\n" +
		"q1: -----X--\n"
	if got != want {
		t.Errorf("Bell circuit:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestGHZ3(t *testing.T) {
	c, err := builder.New("ghz", 3).
		H(0).CNOT(0, 1).CNOT(1, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -H---@------\n" +
		"         |\n" +
		"q1: -----X---@--\n" +
		"             |\n" +
		"q2: ---------X--\n"
	if got != want {
		t.Errorf("GHZ-3:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestNonAdjacentCNOT(t *testing.T) {
	c, err := builder.New("na", 3).CNOT(0, 2).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -@--\n" +
		"     |\n" +
		"q1: -+--\n" +
		"     |\n" +
		"q2: -X--\n"
	if got != want {
		t.Errorf("Non-adjacent CNOT:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestParameterizedGate(t *testing.T) {
	c, err := builder.New("rz", 1).RZ(math.Pi/4, 0).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -RZ(pi/4)--\n"
	if got != want {
		t.Errorf("RZ(pi/4):\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestCCX(t *testing.T) {
	c, err := builder.New("ccx", 3).CCX(0, 1, 2).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -@--\n" +
		"     |\n" +
		"q1: -@--\n" +
		"     |\n" +
		"q2: -X--\n"
	if got != want {
		t.Errorf("CCX:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestMeasurement(t *testing.T) {
	c, err := builder.New("meas", 2).
		WithClbits(2).
		H(0).
		Measure(0, 0).
		Measure(1, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -H---M--\n" +
		"q1: -M------\n"
	if got != want {
		t.Errorf("Measurement:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestBarrier(t *testing.T) {
	c, err := builder.New("bar", 2).
		H(0).
		Barrier(0, 1).
		X(1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -H---|------\n" +
		"         |\n" +
		"q1: -----|---X--\n"
	if got != want {
		t.Errorf("Barrier:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSWAP(t *testing.T) {
	c, err := builder.New("swap", 2).SWAP(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -x--\n" +
		"     |\n" +
		"q1: -x--\n"
	if got != want {
		t.Errorf("SWAP:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestMultipleGatesSequence(t *testing.T) {
	c, err := builder.New("seq", 2).
		H(0).X(1).CNOT(0, 1).H(1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	want := "q0: -H---@------\n" +
		"         |\n" +
		"q1: -X---X---H--\n"
	if got != want {
		t.Errorf("Sequence:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestFprint(t *testing.T) {
	c, err := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Fprint(&buf, c); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := String(c)
	if got != want {
		t.Errorf("Fprint and String differ:\nFprint:\n%s\nString:\n%s", got, want)
	}
}

func TestWithMaxLabelWidth(t *testing.T) {
	c, err := builder.New("u3", 1).
		U3(math.Pi/4, math.Pi/2, math.Pi, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	// Default should work.
	got := String(c)
	t.Logf("U3 default:\n%s", got)
	if !strings.Contains(got, "U3") {
		t.Errorf("expected U3 in output:\n%s", got)
	}

	// Narrow width.
	got = String(c, WithMaxLabelWidth(5))
	t.Logf("U3 narrow:\n%s", got)
	// Label should be truncated.
}

func TestControlledParameterized(t *testing.T) {
	c := ir.New("crz", 2, 0, []ir.Operation{
		{Gate: gate.CRZ(math.Pi / 2), Qubits: []int{0, 1}},
	}, nil)
	got := String(c)
	want := "q0: ----@------\n" +
		"        |\n" +
		"q1: -RZ(pi/2)--\n"
	if got != want {
		t.Errorf("CRZ(pi/2):\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestTenQubits(t *testing.T) {
	b := builder.New("wide", 10)
	b.H(0)
	b.CNOT(0, 9)
	c, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c)
	t.Logf("10 qubits:\n%s", got)
	// All qubit labels should be right-aligned.
	if !strings.Contains(got, "q0: ") {
		t.Errorf("expected q0 label:\n%s", got)
	}
	if !strings.Contains(got, "q9: ") {
		t.Errorf("expected q9 label:\n%s", got)
	}
}

func TestWithMaxLabelWidth_One(t *testing.T) {
	c, err := builder.New("rz", 1).RZ(math.Pi/7, 0).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := String(c, WithMaxLabelWidth(1))
	if got == "" {
		t.Error("expected non-empty output")
	}
}

func TestEmptyQubitsOp(t *testing.T) {
	// Op with nil qubits - should not panic.
	c := ir.New("noop", 1, 0, []ir.Operation{{Gate: gate.H, Qubits: nil}}, nil)
	got := String(c)
	// Just verify no panic, output can be anything.
	_ = got
}
