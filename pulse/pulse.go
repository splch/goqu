package pulse

import "fmt"

// Port represents a hardware I/O endpoint on the quantum device.
// dt is the minimum time resolution in seconds.
type Port struct {
	name string
	dt   float64
}

// NewPort creates a Port with the given name and time resolution.
func NewPort(name string, dt float64) (Port, error) {
	if name == "" {
		return Port{}, fmt.Errorf("pulse: port name must not be empty")
	}
	if dt <= 0 {
		return Port{}, fmt.Errorf("pulse: port dt must be positive, got %g", dt)
	}
	return Port{name: name, dt: dt}, nil
}

// MustPort is like NewPort but panics on error.
func MustPort(name string, dt float64) Port {
	p, err := NewPort(name, dt)
	if err != nil {
		panic(err)
	}
	return p
}

// Name returns the port identifier.
func (p Port) Name() string { return p.name }

// Dt returns the minimum time resolution in seconds.
func (p Port) Dt() float64 { return p.dt }

// Frame represents a software reference clock attached to a port.
// Frames are stateless value types — phase and frequency changes
// are expressed as [SetPhase], [ShiftPhase], [SetFrequency], and
// [ShiftFrequency] instructions.
type Frame struct {
	name      string
	port      Port
	frequency float64
	phase     float64
}

// NewFrame creates a Frame with the given name, port, initial frequency (Hz),
// and initial phase (radians).
func NewFrame(name string, port Port, frequency, phase float64) (Frame, error) {
	if name == "" {
		return Frame{}, fmt.Errorf("pulse: frame name must not be empty")
	}
	if port.name == "" {
		return Frame{}, fmt.Errorf("pulse: frame %q has uninitialized port", name)
	}
	if frequency < 0 {
		return Frame{}, fmt.Errorf("pulse: frame %q frequency must be non-negative, got %g", name, frequency)
	}
	return Frame{name: name, port: port, frequency: frequency, phase: phase}, nil
}

// MustFrame is like NewFrame but panics on error.
func MustFrame(name string, port Port, frequency, phase float64) Frame {
	f, err := NewFrame(name, port, frequency, phase)
	if err != nil {
		panic(err)
	}
	return f
}

// Name returns the frame identifier.
func (f Frame) Name() string { return f.name }

// Port returns the hardware port this frame is attached to.
func (f Frame) Port() Port { return f.port }

// Frequency returns the initial frequency in Hz.
func (f Frame) Frequency() float64 { return f.frequency }

// Phase returns the initial phase in radians.
func (f Frame) Phase() float64 { return f.phase }

// Waveform defines a signal envelope that can be played on a frame.
// Standard waveforms are provided in the waveform sub-package.
type Waveform interface {
	// Name returns the waveform identifier (e.g., "gaussian(0.5, 1e-8, 2.5e-9)").
	Name() string

	// Duration returns the total waveform duration in seconds.
	Duration() float64

	// Sample returns the waveform envelope sampled at the given time
	// resolution dt (seconds). The returned slice length is ceil(Duration/dt).
	Sample(dt float64) []complex128
}

// Instruction is a sealed interface for pulse program instructions.
// Only the eight types defined in this package implement it.
type Instruction interface {
	instructionTag()
}

// Play outputs a waveform on a frame.
type Play struct {
	Frame    Frame
	Waveform Waveform
}

func (Play) instructionTag() {}

// Delay inserts a wait on a frame.
type Delay struct {
	Frame    Frame
	Duration float64 // seconds
}

func (Delay) instructionTag() {}

// SetPhase sets the absolute phase of a frame (radians).
type SetPhase struct {
	Frame Frame
	Phase float64
}

func (SetPhase) instructionTag() {}

// ShiftPhase adds a relative phase offset to a frame (radians).
type ShiftPhase struct {
	Frame Frame
	Delta float64
}

func (ShiftPhase) instructionTag() {}

// SetFrequency sets the absolute frequency of a frame (Hz).
type SetFrequency struct {
	Frame     Frame
	Frequency float64
}

func (SetFrequency) instructionTag() {}

// ShiftFrequency adds a relative frequency offset to a frame (Hz).
type ShiftFrequency struct {
	Frame Frame
	Delta float64
}

func (ShiftFrequency) instructionTag() {}

// Barrier synchronizes one or more frames.
type Barrier struct {
	Frames []Frame
}

func (Barrier) instructionTag() {}

// Capture records the signal on a frame for the given duration.
type Capture struct {
	Frame    Frame
	Duration float64 // seconds
}

func (Capture) instructionTag() {}
