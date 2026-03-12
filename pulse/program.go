package pulse

import "fmt"

// Program is an immutable pulse schedule consisting of ports, frames,
// and a sequence of instructions. Construct programs using [Builder].
type Program struct {
	name         string
	ports        []Port
	frames       []Frame
	instructions []Instruction
	metadata     map[string]string
}

// ProgramStats holds summary statistics for a pulse program.
type ProgramStats struct {
	NumPorts        int
	NumFrames       int
	NumInstructions int
	TotalDuration   float64 // seconds (sum of play/delay/capture durations)
}

// NewProgram creates an immutable Program. All slices and maps are
// defensively copied. Prefer the [Builder] for validated construction.
func NewProgram(name string, ports []Port, frames []Frame, instrs []Instruction, metadata map[string]string) *Program {
	cp := make([]Port, len(ports))
	copy(cp, ports)

	cf := make([]Frame, len(frames))
	copy(cf, frames)

	ci := make([]Instruction, len(instrs))
	copy(ci, instrs)

	var md map[string]string
	if metadata != nil {
		md = make(map[string]string, len(metadata))
		for k, v := range metadata {
			md[k] = v
		}
	}

	return &Program{
		name:         name,
		ports:        cp,
		frames:       cf,
		instructions: ci,
		metadata:     md,
	}
}

// Name returns the program identifier.
func (p *Program) Name() string { return p.name }

// Ports returns a copy of the registered ports.
func (p *Program) Ports() []Port {
	out := make([]Port, len(p.ports))
	copy(out, p.ports)
	return out
}

// Frames returns a copy of the registered frames.
func (p *Program) Frames() []Frame {
	out := make([]Frame, len(p.frames))
	copy(out, p.frames)
	return out
}

// Instructions returns a copy of the instruction sequence.
func (p *Program) Instructions() []Instruction {
	out := make([]Instruction, len(p.instructions))
	copy(out, p.instructions)
	return out
}

// Metadata returns a copy of the key-value metadata.
func (p *Program) Metadata() map[string]string {
	if p.metadata == nil {
		return nil
	}
	out := make(map[string]string, len(p.metadata))
	for k, v := range p.metadata {
		out[k] = v
	}
	return out
}

// Stats returns summary statistics for the program.
func (p *Program) Stats() ProgramStats {
	var totalDur float64
	for _, inst := range p.instructions {
		switch v := inst.(type) {
		case Play:
			totalDur += v.Waveform.Duration()
		case Delay:
			totalDur += v.Duration
		case Capture:
			totalDur += v.Duration
		}
	}
	return ProgramStats{
		NumPorts:        len(p.ports),
		NumFrames:       len(p.frames),
		NumInstructions: len(p.instructions),
		TotalDuration:   totalDur,
	}
}

// String returns a summary of the program.
func (p *Program) String() string {
	s := p.Stats()
	return fmt.Sprintf("Program(%s: %d ports, %d frames, %d instructions, %.3g s)",
		p.name, s.NumPorts, s.NumFrames, s.NumInstructions, s.TotalDuration)
}
