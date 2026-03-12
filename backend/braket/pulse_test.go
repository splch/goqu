package braket

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

func TestSerializePulseProgram(t *testing.T) {
	port := pulse.MustPort("d0", 1e-9)
	frame := pulse.MustFrame("q0_drive", port, 5e9, 0)
	wf := waveform.MustGaussian(1.0, 1e-7, 2e-8)

	prog := pulse.NewProgram("test",
		[]pulse.Port{port},
		[]pulse.Frame{frame},
		[]pulse.Instruction{
			pulse.Play{Frame: frame, Waveform: wf},
			pulse.Delay{Frame: frame, Duration: 5e-8},
			pulse.SetPhase{Frame: frame, Phase: 1.57},
			pulse.ShiftPhase{Frame: frame, Delta: 0.5},
			pulse.SetFrequency{Frame: frame, Frequency: 5.1e9},
			pulse.ShiftFrequency{Frame: frame, Delta: 1e6},
			pulse.Barrier{Frames: []pulse.Frame{frame}},
			pulse.Capture{Frame: frame, Duration: 2e-7},
		},
		nil,
	)

	action, err := serializePulseProgram(prog)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's valid JSON.
	var parsed braketProgram
	if err := json.Unmarshal([]byte(action), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed.Header.Name != "braket.ir.openqasm.program" {
		t.Errorf("schema = %q, want braket.ir.openqasm.program", parsed.Header.Name)
	}

	src := parsed.Source
	// Verify QASM structure.
	if !strings.HasPrefix(src, "OPENQASM 3.0;") {
		t.Error("source should start with OPENQASM 3.0;")
	}
	if !strings.Contains(src, "extern port d0;") {
		t.Error("missing port declaration")
	}
	if !strings.Contains(src, "cal {") {
		t.Error("missing cal block")
	}
	if !strings.Contains(src, "frame q0_drive = newframe(d0,") {
		t.Error("missing frame declaration")
	}
	if !strings.Contains(src, "play(q0_drive,") {
		t.Error("missing play instruction")
	}
	if !strings.Contains(src, "delay[") {
		t.Error("missing delay instruction")
	}
	if !strings.Contains(src, "set_phase(q0_drive,") {
		t.Error("missing set_phase instruction")
	}
	if !strings.Contains(src, "shift_phase(q0_drive,") {
		t.Error("missing shift_phase instruction")
	}
	if !strings.Contains(src, "set_frequency(q0_drive,") {
		t.Error("missing set_frequency instruction")
	}
	if !strings.Contains(src, "shift_frequency(q0_drive,") {
		t.Error("missing shift_frequency instruction")
	}
	if !strings.Contains(src, "barrier q0_drive;") {
		t.Error("missing barrier instruction")
	}
	if !strings.Contains(src, "capture_v0(q0_drive,") {
		t.Error("missing capture instruction")
	}
}

func TestSerializePulseProgramNil(t *testing.T) {
	_, err := serializePulseProgram(nil)
	if err == nil {
		t.Error("expected error for nil program")
	}
}

func TestSerializePulseProgramMultipleFrames(t *testing.T) {
	port0 := pulse.MustPort("d0", 1e-9)
	port1 := pulse.MustPort("d1", 1e-9)
	frame0 := pulse.MustFrame("q0_drive", port0, 5e9, 0)
	frame1 := pulse.MustFrame("q1_drive", port1, 5.1e9, 0)

	prog := pulse.NewProgram("multi",
		[]pulse.Port{port0, port1},
		[]pulse.Frame{frame0, frame1},
		[]pulse.Instruction{
			pulse.Play{Frame: frame0, Waveform: waveform.MustConstant(0.5, 1e-7)},
			pulse.Play{Frame: frame1, Waveform: waveform.MustConstant(0.5, 1e-7)},
			pulse.Barrier{Frames: []pulse.Frame{frame0, frame1}},
			pulse.Capture{Frame: frame0, Duration: 1e-6},
			pulse.Capture{Frame: frame1, Duration: 1e-6},
		},
		nil,
	)

	action, err := serializePulseProgram(prog)
	if err != nil {
		t.Fatal(err)
	}

	var parsed braketProgram
	if err := json.Unmarshal([]byte(action), &parsed); err != nil {
		t.Fatal(err)
	}

	src := parsed.Source
	if !strings.Contains(src, "extern port d0;") || !strings.Contains(src, "extern port d1;") {
		t.Error("missing port declarations")
	}
	if !strings.Contains(src, "barrier q0_drive, q1_drive;") {
		t.Error("missing multi-frame barrier")
	}
}
