package pulse

import "testing"

func makeTestFrame() (Port, Frame) {
	port := MustPort("d0", 1e-9)
	frame := MustFrame("q0", port, 5e9, 0)
	return port, frame
}

func TestBuilderHappyPath(t *testing.T) {
	port, frame := makeTestFrame()
	wf := stubWaveform{dur: 1e-7}

	prog, err := NewBuilder("test").
		AddPort(port).
		AddFrame(frame).
		Play(frame, wf).
		Delay(frame, 5e-8).
		SetPhase(frame, 1.57).
		ShiftPhase(frame, 0.5).
		SetFrequency(frame, 5.1e9).
		ShiftFrequency(frame, 1e6).
		Barrier(frame).
		Capture(frame, 1e-6).
		WithMetadata("key", "val").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if prog.Name() != "test" {
		t.Errorf("Name() = %q, want %q", prog.Name(), "test")
	}

	stats := prog.Stats()
	if stats.NumPorts != 1 {
		t.Errorf("NumPorts = %d, want 1", stats.NumPorts)
	}
	if stats.NumFrames != 1 {
		t.Errorf("NumFrames = %d, want 1", stats.NumFrames)
	}
	if stats.NumInstructions != 8 {
		t.Errorf("NumInstructions = %d, want 8", stats.NumInstructions)
	}
	if prog.Metadata()["key"] != "val" {
		t.Error("metadata not preserved")
	}
}

func TestBuilderErrorShortCircuit(t *testing.T) {
	port, _ := makeTestFrame()

	// First error: duplicate port. All subsequent calls should be no-ops.
	_, err := NewBuilder("test").
		AddPort(port).
		AddPort(port). // duplicate → error
		Play(Frame{}, nil).
		Build()
	if err == nil {
		t.Fatal("expected error for duplicate port")
	}
}

func TestBuilderDuplicateFrame(t *testing.T) {
	port, frame := makeTestFrame()
	_, err := NewBuilder("test").
		AddPort(port).
		AddFrame(frame).
		AddFrame(frame). // duplicate
		Build()
	if err == nil {
		t.Fatal("expected error for duplicate frame")
	}
}

func TestBuilderUnregisteredPort(t *testing.T) {
	port := MustPort("d0", 1e-9)
	frame := MustFrame("q0", port, 5e9, 0)

	// Frame references port, but port not added to builder.
	_, err := NewBuilder("test").
		AddFrame(frame).
		Build()
	if err == nil {
		t.Fatal("expected error for unregistered port")
	}
}

func TestBuilderUnregisteredFrame(t *testing.T) {
	port := MustPort("d0", 1e-9)
	frame := MustFrame("q0", port, 5e9, 0)

	_, err := NewBuilder("test").
		AddPort(port).
		Play(frame, stubWaveform{dur: 1e-7}). // frame not registered
		Build()
	if err == nil {
		t.Fatal("expected error for unregistered frame")
	}
}

func TestBuilderNilWaveform(t *testing.T) {
	port, frame := makeTestFrame()
	_, err := NewBuilder("test").
		AddPort(port).
		AddFrame(frame).
		Play(frame, nil).
		Build()
	if err == nil {
		t.Fatal("expected error for nil waveform")
	}
}

func TestBuilderNonPositiveDelay(t *testing.T) {
	port, frame := makeTestFrame()
	_, err := NewBuilder("test").
		AddPort(port).
		AddFrame(frame).
		Delay(frame, 0).
		Build()
	if err == nil {
		t.Fatal("expected error for zero delay")
	}
}

func TestBuilderNonPositiveCapture(t *testing.T) {
	port, frame := makeTestFrame()
	_, err := NewBuilder("test").
		AddPort(port).
		AddFrame(frame).
		Capture(frame, -1).
		Build()
	if err == nil {
		t.Fatal("expected error for negative capture duration")
	}
}

func TestBuilderEmptyBarrier(t *testing.T) {
	port, _ := makeTestFrame()
	_, err := NewBuilder("test").
		AddPort(port).
		Barrier().
		Build()
	if err == nil {
		t.Fatal("expected error for empty barrier")
	}
}

func TestBuilderNoInstructions(t *testing.T) {
	port, frame := makeTestFrame()
	_, err := NewBuilder("test").
		AddPort(port).
		AddFrame(frame).
		Build()
	if err == nil {
		t.Fatal("expected error for no instructions")
	}
}

func TestBuilderUninitializedPort(t *testing.T) {
	_, err := NewBuilder("test").
		AddPort(Port{}).
		Build()
	if err == nil {
		t.Fatal("expected error for uninitialized port")
	}
}

func TestBuilderUninitializedFrame(t *testing.T) {
	_, err := NewBuilder("test").
		AddFrame(Frame{}).
		Build()
	if err == nil {
		t.Fatal("expected error for uninitialized frame")
	}
}

func TestBuilderUninitializedFrameInInstruction(t *testing.T) {
	port, _ := makeTestFrame()
	_, err := NewBuilder("test").
		AddPort(port).
		Play(Frame{}, stubWaveform{dur: 1e-7}).
		Build()
	if err == nil {
		t.Fatal("expected error for uninitialized frame in instruction")
	}
}
