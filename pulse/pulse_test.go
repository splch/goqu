package pulse

import "testing"

func TestNewPort(t *testing.T) {
	p, err := NewPort("d0", 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "d0" {
		t.Errorf("Name() = %q, want %q", p.Name(), "d0")
	}
	if p.Dt() != 1e-9 {
		t.Errorf("Dt() = %g, want %g", p.Dt(), 1e-9)
	}
}

func TestNewPortErrors(t *testing.T) {
	tests := []struct {
		name string
		dt   float64
	}{
		{"", 1e-9},
		{"d0", 0},
		{"d0", -1},
	}
	for _, tt := range tests {
		_, err := NewPort(tt.name, tt.dt)
		if err == nil {
			t.Errorf("NewPort(%q, %g) expected error", tt.name, tt.dt)
		}
	}
}

func TestMustPortPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustPort with empty name should panic")
		}
	}()
	MustPort("", 1e-9)
}

func TestNewFrame(t *testing.T) {
	port := MustPort("d0", 1e-9)
	f, err := NewFrame("q0_drive", port, 5e9, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name() != "q0_drive" {
		t.Errorf("Name() = %q, want %q", f.Name(), "q0_drive")
	}
	if f.Port().Name() != "d0" {
		t.Errorf("Port().Name() = %q, want %q", f.Port().Name(), "d0")
	}
	if f.Frequency() != 5e9 {
		t.Errorf("Frequency() = %g, want %g", f.Frequency(), 5e9)
	}
	if f.Phase() != 0.0 {
		t.Errorf("Phase() = %g, want %g", f.Phase(), 0.0)
	}
}

func TestNewFrameErrors(t *testing.T) {
	port := MustPort("d0", 1e-9)
	tests := []struct {
		name string
		port Port
		freq float64
	}{
		{"", port, 5e9},     // empty name
		{"q0", Port{}, 5e9}, // uninitialized port
		{"q0", port, -1},    // negative frequency
	}
	for _, tt := range tests {
		_, err := NewFrame(tt.name, tt.port, tt.freq, 0)
		if err == nil {
			t.Errorf("NewFrame(%q, ...) expected error", tt.name)
		}
	}
}

func TestMustFramePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustFrame with empty name should panic")
		}
	}()
	MustFrame("", Port{}, 0, 0)
}

func TestInstructionTypes(t *testing.T) {
	// Verify all instruction types satisfy the interface.
	var instructions []Instruction
	port := MustPort("d0", 1e-9)
	frame := MustFrame("q0", port, 5e9, 0)

	instructions = append(instructions,
		Play{Frame: frame},
		Delay{Frame: frame, Duration: 1e-6},
		SetPhase{Frame: frame, Phase: 1.57},
		ShiftPhase{Frame: frame, Delta: 0.5},
		SetFrequency{Frame: frame, Frequency: 5.1e9},
		ShiftFrequency{Frame: frame, Delta: 1e6},
		Barrier{Frames: []Frame{frame}},
		Capture{Frame: frame, Duration: 1e-6},
	)
	if len(instructions) != 8 {
		t.Errorf("expected 8 instruction types, got %d", len(instructions))
	}
}
