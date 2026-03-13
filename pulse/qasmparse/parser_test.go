package qasmparse

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

const floatTol = 1e-10

func TestParseMinimal(t *testing.T) {
	src := `OPENQASM 3.0;
extern port d0;
cal {
  frame q0_drive = newframe(d0, 5e+09, 0);
  play(q0_drive, constant(0.5+0i, 1e-07));
}
`
	prog, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Ports()) != 1 {
		t.Errorf("ports = %d, want 1", len(prog.Ports()))
	}
	if len(prog.Frames()) != 1 {
		t.Errorf("frames = %d, want 1", len(prog.Frames()))
	}
	if len(prog.Instructions()) != 1 {
		t.Errorf("instructions = %d, want 1", len(prog.Instructions()))
	}
	play, ok := prog.Instructions()[0].(pulse.Play)
	if !ok {
		t.Fatal("expected Play instruction")
	}
	if play.Frame.Name() != "q0_drive" {
		t.Errorf("frame = %q, want q0_drive", play.Frame.Name())
	}
}

func TestParseAllInstructions(t *testing.T) {
	src := `OPENQASM 3.0;
extern port d0;
extern port d1;
cal {
  frame f0 = newframe(d0, 5e+09, 0);
  frame f1 = newframe(d1, 5e+09, 0);
  play(f0, constant(0.5+0i, 1e-07));
  delay[1e-08s] f0;
  set_phase(f0, 1.5708);
  shift_phase(f0, 0.7854);
  set_frequency(f0, 5.1e+09);
  shift_frequency(f0, 1e+06);
  barrier f0, f1;
  capture_v0(f0, 1e-06s);
}
`
	prog, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	instrs := prog.Instructions()
	if len(instrs) != 8 {
		t.Fatalf("instructions = %d, want 8", len(instrs))
	}

	// Verify types.
	types := []string{"Play", "Delay", "SetPhase", "ShiftPhase", "SetFrequency", "ShiftFrequency", "Barrier", "Capture"}
	for i, inst := range instrs {
		var got string
		switch inst.(type) {
		case pulse.Play:
			got = "Play"
		case pulse.Delay:
			got = "Delay"
		case pulse.SetPhase:
			got = "SetPhase"
		case pulse.ShiftPhase:
			got = "ShiftPhase"
		case pulse.SetFrequency:
			got = "SetFrequency"
		case pulse.ShiftFrequency:
			got = "ShiftFrequency"
		case pulse.Barrier:
			got = "Barrier"
		case pulse.Capture:
			got = "Capture"
		}
		if got != types[i] {
			t.Errorf("instruction[%d] type = %s, want %s", i, got, types[i])
		}
	}

	// Check delay value.
	delay := instrs[1].(pulse.Delay)
	if math.Abs(delay.Duration-1e-8) > floatTol {
		t.Errorf("delay duration = %g, want 1e-8", delay.Duration)
	}

	// Check barrier has 2 frames.
	barrier := instrs[6].(pulse.Barrier)
	if len(barrier.Frames) != 2 {
		t.Errorf("barrier frames = %d, want 2", len(barrier.Frames))
	}
}

func TestParseWaveforms(t *testing.T) {
	tests := []struct {
		name   string
		wfCall string
	}{
		{"constant", "constant(0.5+0i, 1e-07)"},
		{"gaussian", "gaussian(0.5, 2e-08, 1e-08)"},
		{"drag", "drag(0.5, 2e-08, 1e-08, 0.1)"},
		{"gaussian_square", "gaussian_square(0.5, 1e-07, 1e-08, 5e-08)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, 0);
  play(f0, ` + tt.wfCall + `);
}
`
			prog, err := ParseString(src)
			if err != nil {
				t.Fatal(err)
			}
			play := prog.Instructions()[0].(pulse.Play)
			if play.Waveform == nil {
				t.Error("waveform is nil")
			}
		})
	}
}

func TestParseComplexConstant(t *testing.T) {
	src := `OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, 0);
  play(f0, constant(0.5+0i, 1e-07));
}
`
	prog, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	play := prog.Instructions()[0].(pulse.Play)
	if play.Waveform == nil {
		t.Error("waveform is nil")
	}
	// Verify the constant waveform produces samples.
	samples := play.Waveform.Sample(1e-9)
	if len(samples) == 0 {
		t.Error("no samples from constant waveform")
	}
	// Real part should be 0.5.
	if math.Abs(real(samples[0])-0.5) > floatTol {
		t.Errorf("real(samples[0]) = %g, want 0.5", real(samples[0]))
	}
}

func TestParseComplexConstantNegImag(t *testing.T) {
	src := `OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, 0);
  play(f0, constant(0.3-0.4i, 1e-07));
}
`
	prog, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	play := prog.Instructions()[0].(pulse.Play)
	samples := play.Waveform.Sample(1e-9)
	if math.Abs(real(samples[0])-0.3) > floatTol {
		t.Errorf("real = %g, want 0.3", real(samples[0]))
	}
	if math.Abs(imag(samples[0])-(-0.4)) > floatTol {
		t.Errorf("imag = %g, want -0.4", imag(samples[0]))
	}
}

func TestParseExpressions(t *testing.T) {
	// Test scientific notation and negative numbers in frame params.
	src := `OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, -1.5708);
  play(f0, constant(1e-3+0i, 1e-07));
}
`
	prog, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	frames := prog.Frames()
	if math.Abs(frames[0].Phase()-(-1.5708)) > floatTol {
		t.Errorf("phase = %g, want -1.5708", frames[0].Phase())
	}
}

func TestParseCustomWaveform(t *testing.T) {
	src := `OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, 0);
  play(f0, mywf(0.5, 1e-07));
}
`
	called := false
	prog, err := ParseString(src, WithWaveform("mywf", func(args []float64) (pulse.Waveform, error) {
		called = true
		if len(args) != 2 {
			t.Errorf("args = %v, want 2 args", args)
		}
		return waveform.Constant(complex(args[0], 0), args[1])
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("custom waveform constructor was not called")
	}
	if len(prog.Instructions()) != 1 {
		t.Errorf("instructions = %d, want 1", len(prog.Instructions()))
	}
}

func TestParseDefaultDt(t *testing.T) {
	src := `OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, 0);
  play(f0, constant(0.5+0i, 1e-07));
}
`
	customDt := 2e-9
	prog, err := ParseString(src, WithDefaultDt(customDt))
	if err != nil {
		t.Fatal(err)
	}
	ports := prog.Ports()
	if math.Abs(ports[0].Dt()-customDt) > floatTol {
		t.Errorf("port dt = %g, want %g", ports[0].Dt(), customDt)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		errSub string
	}{
		{
			"missing semicolon",
			`OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, 0)
}
`,
			"expected",
		},
		{
			"undefined frame",
			`OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, 0);
  play(f_undefined, constant(0.5+0i, 1e-07));
}
`,
			"undefined frame",
		},
		{
			"unknown waveform",
			`OPENQASM 3.0;
extern port d0;
cal {
  frame f0 = newframe(d0, 5e+09, 0);
  play(f0, unknownwf(0.5));
}
`,
			"unknown waveform",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseString(tt.src)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// Build a program with the builder.
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 5e9, 0)
	wf := waveform.MustConstant(complex(0.5, 0), 1e-7)

	prog := pulse.NewProgram("test",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		[]pulse.Instruction{
			pulse.Play{Frame: frame, Waveform: wf},
			pulse.Delay{Frame: frame, Duration: 1e-8},
			pulse.SetPhase{Frame: frame, Phase: 1.5708},
			pulse.ShiftPhase{Frame: frame, Delta: 0.7854},
			pulse.SetFrequency{Frame: frame, Frequency: 5.1e9},
			pulse.ShiftFrequency{Frame: frame, Delta: 1e6},
			pulse.Barrier{Frames: []pulse.Frame{frame}},
			pulse.Capture{Frame: frame, Duration: 1e-6},
		},
		nil,
	)

	// Serialize to QASM (matching braket serializer format).
	qasmSource := buildQASMSource(prog)

	// Parse back.
	parsed, err := ParseString(qasmSource)
	if err != nil {
		t.Fatalf("parse failed: %v\nSource:\n%s", err, qasmSource)
	}

	// Verify structural equivalence.
	if len(parsed.Ports()) != len(prog.Ports()) {
		t.Errorf("ports: got %d, want %d", len(parsed.Ports()), len(prog.Ports()))
	}
	if len(parsed.Frames()) != len(prog.Frames()) {
		t.Errorf("frames: got %d, want %d", len(parsed.Frames()), len(prog.Frames()))
	}
	if len(parsed.Instructions()) != len(prog.Instructions()) {
		t.Errorf("instructions: got %d, want %d", len(parsed.Instructions()), len(prog.Instructions()))
	}

	// Check frame values.
	origFrames := prog.Frames()
	parsedFrames := parsed.Frames()
	for i := range origFrames {
		if origFrames[i].Name() != parsedFrames[i].Name() {
			t.Errorf("frame[%d] name: got %q, want %q", i, parsedFrames[i].Name(), origFrames[i].Name())
		}
		if math.Abs(origFrames[i].Frequency()-parsedFrames[i].Frequency()) > 1 {
			t.Errorf("frame[%d] freq: got %g, want %g", i, parsedFrames[i].Frequency(), origFrames[i].Frequency())
		}
	}

	// Check instruction values where possible.
	origInstrs := prog.Instructions()
	parsedInstrs := parsed.Instructions()
	for i := range origInstrs {
		switch ov := origInstrs[i].(type) {
		case pulse.Delay:
			pv := parsedInstrs[i].(pulse.Delay)
			if relDiff(ov.Duration, pv.Duration) > 1e-6 {
				t.Errorf("delay[%d] duration: got %g, want %g", i, pv.Duration, ov.Duration)
			}
		case pulse.SetPhase:
			pv := parsedInstrs[i].(pulse.SetPhase)
			if math.Abs(ov.Phase-pv.Phase) > 1e-3 {
				t.Errorf("set_phase[%d]: got %g, want %g", i, pv.Phase, ov.Phase)
			}
		case pulse.ShiftPhase:
			pv := parsedInstrs[i].(pulse.ShiftPhase)
			if math.Abs(ov.Delta-pv.Delta) > 1e-3 {
				t.Errorf("shift_phase[%d]: got %g, want %g", i, pv.Delta, ov.Delta)
			}
		case pulse.SetFrequency:
			pv := parsedInstrs[i].(pulse.SetFrequency)
			if relDiff(ov.Frequency, pv.Frequency) > 1e-6 {
				t.Errorf("set_frequency[%d]: got %g, want %g", i, pv.Frequency, ov.Frequency)
			}
		case pulse.ShiftFrequency:
			pv := parsedInstrs[i].(pulse.ShiftFrequency)
			if relDiff(ov.Delta, pv.Delta) > 1e-6 {
				t.Errorf("shift_frequency[%d]: got %g, want %g", i, pv.Delta, ov.Delta)
			}
		}
	}
}

func TestParseComments(t *testing.T) {
	src := `OPENQASM 3.0;
// This is a line comment
extern port d0;
cal {
  /* block comment */
  frame f0 = newframe(d0, 5e+09, 0);
  // another comment
  play(f0, constant(0.5+0i, 1e-07)); /* inline comment */
}
`
	prog, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Instructions()) != 1 {
		t.Errorf("instructions = %d, want 1", len(prog.Instructions()))
	}
}

func relDiff(a, b float64) float64 {
	d := math.Abs(a - b)
	m := math.Max(math.Abs(a), math.Abs(b))
	if m == 0 {
		return d
	}
	return d / m
}

// buildQASMSource emits OpenPulse QASM (matching braket/serialize.go format).
func buildQASMSource(p *pulse.Program) string {
	var sb strings.Builder
	sb.WriteString("OPENQASM 3.0;\n")

	for _, port := range p.Ports() {
		fmt.Fprintf(&sb, "extern port %s;\n", port.Name())
	}

	sb.WriteString("cal {\n")

	for _, f := range p.Frames() {
		fmt.Fprintf(&sb, "  frame %s = newframe(%s, %g, %g);\n",
			f.Name(), f.Port().Name(), f.Frequency(), f.Phase())
	}

	for _, inst := range p.Instructions() {
		switch v := inst.(type) {
		case pulse.Play:
			fmt.Fprintf(&sb, "  play(%s, %s);\n", v.Frame.Name(), v.Waveform.Name())
		case pulse.Delay:
			fmt.Fprintf(&sb, "  delay[%gs] %s;\n", v.Duration, v.Frame.Name())
		case pulse.SetPhase:
			fmt.Fprintf(&sb, "  set_phase(%s, %g);\n", v.Frame.Name(), v.Phase)
		case pulse.ShiftPhase:
			fmt.Fprintf(&sb, "  shift_phase(%s, %g);\n", v.Frame.Name(), v.Delta)
		case pulse.SetFrequency:
			fmt.Fprintf(&sb, "  set_frequency(%s, %g);\n", v.Frame.Name(), v.Frequency)
		case pulse.ShiftFrequency:
			fmt.Fprintf(&sb, "  shift_frequency(%s, %g);\n", v.Frame.Name(), v.Delta)
		case pulse.Barrier:
			names := make([]string, len(v.Frames))
			for i, f := range v.Frames {
				names[i] = f.Name()
			}
			fmt.Fprintf(&sb, "  barrier %s;\n", strings.Join(names, ", "))
		case pulse.Capture:
			fmt.Fprintf(&sb, "  capture_v0(%s, %gs);\n", v.Frame.Name(), v.Duration)
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}
