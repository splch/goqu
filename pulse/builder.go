package pulse

import "fmt"

// Builder constructs a [Program] using a fluent API with eager validation
// and error short-circuiting.
type Builder struct {
	name         string
	ports        []Port
	frames       []Frame
	instructions []Instruction
	metadata     map[string]string
	portNames    map[string]bool
	frameNames   map[string]bool
	err          error
}

// NewBuilder creates a Builder for a pulse program with the given name.
func NewBuilder(name string) *Builder {
	return &Builder{
		name:       name,
		portNames:  make(map[string]bool),
		frameNames: make(map[string]bool),
	}
}

// AddPort registers a hardware port.
func (b *Builder) AddPort(p Port) *Builder {
	if b.err != nil {
		return b
	}
	if p.name == "" {
		b.err = fmt.Errorf("pulse: cannot add uninitialized port")
		return b
	}
	if b.portNames[p.name] {
		b.err = fmt.Errorf("pulse: duplicate port name %q", p.name)
		return b
	}
	b.portNames[p.name] = true
	b.ports = append(b.ports, p)
	return b
}

// AddFrame registers a software frame. The frame's port must have been
// added via [AddPort] first.
func (b *Builder) AddFrame(f Frame) *Builder {
	if b.err != nil {
		return b
	}
	if f.name == "" {
		b.err = fmt.Errorf("pulse: cannot add uninitialized frame")
		return b
	}
	if b.frameNames[f.name] {
		b.err = fmt.Errorf("pulse: duplicate frame name %q", f.name)
		return b
	}
	if !b.portNames[f.port.name] {
		b.err = fmt.Errorf("pulse: frame %q references unregistered port %q", f.name, f.port.name)
		return b
	}
	b.frameNames[f.name] = true
	b.frames = append(b.frames, f)
	return b
}

// Play appends a Play instruction.
func (b *Builder) Play(f Frame, wf Waveform) *Builder {
	if b.err != nil {
		return b
	}
	if err := b.checkFrame(f); err != nil {
		b.err = err
		return b
	}
	if wf == nil {
		b.err = fmt.Errorf("pulse: play on frame %q has nil waveform", f.name)
		return b
	}
	b.instructions = append(b.instructions, Play{Frame: f, Waveform: wf})
	return b
}

// Delay appends a Delay instruction.
func (b *Builder) Delay(f Frame, duration float64) *Builder {
	if b.err != nil {
		return b
	}
	if err := b.checkFrame(f); err != nil {
		b.err = err
		return b
	}
	if duration <= 0 {
		b.err = fmt.Errorf("pulse: delay on frame %q must have positive duration, got %g", f.name, duration)
		return b
	}
	b.instructions = append(b.instructions, Delay{Frame: f, Duration: duration})
	return b
}

// SetPhase appends a SetPhase instruction.
func (b *Builder) SetPhase(f Frame, phase float64) *Builder {
	if b.err != nil {
		return b
	}
	if err := b.checkFrame(f); err != nil {
		b.err = err
		return b
	}
	b.instructions = append(b.instructions, SetPhase{Frame: f, Phase: phase})
	return b
}

// ShiftPhase appends a ShiftPhase instruction.
func (b *Builder) ShiftPhase(f Frame, delta float64) *Builder {
	if b.err != nil {
		return b
	}
	if err := b.checkFrame(f); err != nil {
		b.err = err
		return b
	}
	b.instructions = append(b.instructions, ShiftPhase{Frame: f, Delta: delta})
	return b
}

// SetFrequency appends a SetFrequency instruction.
func (b *Builder) SetFrequency(f Frame, frequency float64) *Builder {
	if b.err != nil {
		return b
	}
	if err := b.checkFrame(f); err != nil {
		b.err = err
		return b
	}
	b.instructions = append(b.instructions, SetFrequency{Frame: f, Frequency: frequency})
	return b
}

// ShiftFrequency appends a ShiftFrequency instruction.
func (b *Builder) ShiftFrequency(f Frame, delta float64) *Builder {
	if b.err != nil {
		return b
	}
	if err := b.checkFrame(f); err != nil {
		b.err = err
		return b
	}
	b.instructions = append(b.instructions, ShiftFrequency{Frame: f, Delta: delta})
	return b
}

// Barrier appends a Barrier instruction that synchronizes the given frames.
func (b *Builder) Barrier(frames ...Frame) *Builder {
	if b.err != nil {
		return b
	}
	if len(frames) == 0 {
		b.err = fmt.Errorf("pulse: barrier requires at least one frame")
		return b
	}
	for _, f := range frames {
		if err := b.checkFrame(f); err != nil {
			b.err = err
			return b
		}
	}
	copied := make([]Frame, len(frames))
	copy(copied, frames)
	b.instructions = append(b.instructions, Barrier{Frames: copied})
	return b
}

// Capture appends a Capture instruction.
func (b *Builder) Capture(f Frame, duration float64) *Builder {
	if b.err != nil {
		return b
	}
	if err := b.checkFrame(f); err != nil {
		b.err = err
		return b
	}
	if duration <= 0 {
		b.err = fmt.Errorf("pulse: capture on frame %q must have positive duration, got %g", f.name, duration)
		return b
	}
	b.instructions = append(b.instructions, Capture{Frame: f, Duration: duration})
	return b
}

// WithMetadata adds a key-value pair to the program metadata.
func (b *Builder) WithMetadata(key, value string) *Builder {
	if b.err != nil {
		return b
	}
	if b.metadata == nil {
		b.metadata = make(map[string]string)
	}
	b.metadata[key] = value
	return b
}

// Build constructs the immutable [Program]. Returns an error if any
// builder method failed or if the program has no instructions.
func (b *Builder) Build() (*Program, error) {
	if b.err != nil {
		return nil, b.err
	}
	if len(b.instructions) == 0 {
		return nil, fmt.Errorf("pulse: program %q has no instructions", b.name)
	}
	return NewProgram(b.name, b.ports, b.frames, b.instructions, b.metadata), nil
}

// checkFrame validates that a frame has been registered.
func (b *Builder) checkFrame(f Frame) error {
	if f.name == "" {
		return fmt.Errorf("pulse: instruction references uninitialized frame")
	}
	if !b.frameNames[f.name] {
		return fmt.Errorf("pulse: instruction references unregistered frame %q", f.name)
	}
	return nil
}
