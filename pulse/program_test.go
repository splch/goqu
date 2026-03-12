package pulse

import "testing"

// stubWaveform is a minimal Waveform for testing.
type stubWaveform struct {
	dur float64
}

func (s stubWaveform) Name() string                { return "stub" }
func (s stubWaveform) Duration() float64           { return s.dur }
func (s stubWaveform) Sample(float64) []complex128 { return nil }

func TestNewProgram(t *testing.T) {
	port := MustPort("d0", 1e-9)
	frame := MustFrame("q0", port, 5e9, 0)
	wf := stubWaveform{dur: 1e-7}

	instrs := []Instruction{
		Play{Frame: frame, Waveform: wf},
		Delay{Frame: frame, Duration: 5e-8},
		Capture{Frame: frame, Duration: 2e-7},
	}
	md := map[string]string{"version": "1"}

	prog := NewProgram("test", []Port{port}, []Frame{frame}, instrs, md)

	if prog.Name() != "test" {
		t.Errorf("Name() = %q, want %q", prog.Name(), "test")
	}

	ports := prog.Ports()
	if len(ports) != 1 || ports[0].Name() != "d0" {
		t.Errorf("unexpected ports: %v", ports)
	}

	frames := prog.Frames()
	if len(frames) != 1 || frames[0].Name() != "q0" {
		t.Errorf("unexpected frames: %v", frames)
	}

	got := prog.Instructions()
	if len(got) != 3 {
		t.Errorf("len(Instructions()) = %d, want 3", len(got))
	}

	meta := prog.Metadata()
	if meta["version"] != "1" {
		t.Errorf("Metadata()[version] = %q, want %q", meta["version"], "1")
	}
}

func TestProgramDefensiveCopy(t *testing.T) {
	port := MustPort("d0", 1e-9)
	frame := MustFrame("q0", port, 5e9, 0)
	wf := stubWaveform{dur: 1e-7}

	instrs := []Instruction{Play{Frame: frame, Waveform: wf}}
	prog := NewProgram("test", []Port{port}, []Frame{frame}, instrs, nil)

	// Mutate original slice.
	instrs[0] = Delay{Frame: frame, Duration: 1}

	// Program should be unaffected.
	got := prog.Instructions()
	if _, ok := got[0].(Play); !ok {
		t.Error("Program should defensively copy instructions")
	}

	// Mutate returned slice.
	returned := prog.Instructions()
	returned[0] = Delay{Frame: frame, Duration: 1}
	got2 := prog.Instructions()
	if _, ok := got2[0].(Play); !ok {
		t.Error("Instructions() should return a copy")
	}
}

func TestProgramStats(t *testing.T) {
	port := MustPort("d0", 1e-9)
	frame := MustFrame("q0", port, 5e9, 0)
	wf := stubWaveform{dur: 1e-7}

	prog := NewProgram("test",
		[]Port{port},
		[]Frame{frame},
		[]Instruction{
			Play{Frame: frame, Waveform: wf},
			Delay{Frame: frame, Duration: 5e-8},
			SetPhase{Frame: frame, Phase: 1.0},
			Capture{Frame: frame, Duration: 2e-7},
		},
		nil,
	)

	stats := prog.Stats()
	if stats.NumPorts != 1 {
		t.Errorf("NumPorts = %d, want 1", stats.NumPorts)
	}
	if stats.NumFrames != 1 {
		t.Errorf("NumFrames = %d, want 1", stats.NumFrames)
	}
	if stats.NumInstructions != 4 {
		t.Errorf("NumInstructions = %d, want 4", stats.NumInstructions)
	}
	// 1e-7 + 5e-8 + 2e-7 = 3.5e-7
	expected := 3.5e-7
	if diff := stats.TotalDuration - expected; diff > 1e-15 || diff < -1e-15 {
		t.Errorf("TotalDuration = %g, want %g", stats.TotalDuration, expected)
	}
}

func TestProgramString(t *testing.T) {
	port := MustPort("d0", 1e-9)
	frame := MustFrame("q0", port, 5e9, 0)
	wf := stubWaveform{dur: 1e-7}

	prog := NewProgram("test", []Port{port}, []Frame{frame},
		[]Instruction{Play{Frame: frame, Waveform: wf}}, nil)

	s := prog.String()
	if s == "" {
		t.Error("String() should not be empty")
	}
}

func TestProgramNilMetadata(t *testing.T) {
	prog := NewProgram("test", nil, nil, nil, nil)
	if prog.Metadata() != nil {
		t.Error("Metadata() should be nil when constructed with nil")
	}
}
