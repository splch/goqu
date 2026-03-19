package gate

import "fmt"

// Reset is a pseudo-gate that resets a qubit to |0⟩.
// It has no matrix representation - simulators handle it directly.
var Reset Gate = resetGate{}

type resetGate struct{}

func (g resetGate) Name() string                { return "reset" }
func (g resetGate) Qubits() int                 { return 1 }
func (g resetGate) Matrix() []complex128        { return nil }
func (g resetGate) Params() []float64           { return nil }
func (g resetGate) Inverse() Gate               { return g }
func (g resetGate) Decompose(_ []int) []Applied { return nil }

// Duration unit constants for use with [Delay].
const (
	UnitNs = "ns" // nanoseconds
	UnitUs = "us" // microseconds
	UnitMs = "ms" // milliseconds
	UnitS  = "s"  // seconds
	UnitDt = "dt" // backend-dependent time step
)

// Delay returns a delay pseudo-gate that idles a qubit for the given duration.
// It has no matrix representation - simulators skip it (or apply decoherence
// noise in noisy simulation). The unit should be one of the Unit* constants.
func Delay(duration float64, unit string) Gate {
	return delayGate{duration: duration, unit: unit}
}

type delayGate struct {
	duration float64
	unit     string
}

func (g delayGate) Name() string                { return "delay" }
func (g delayGate) Qubits() int                 { return 1 }
func (g delayGate) Matrix() []complex128        { return nil }
func (g delayGate) Params() []float64           { return []float64{g.duration} }
func (g delayGate) Inverse() Gate               { return g }
func (g delayGate) Decompose(_ []int) []Applied { return nil }

// Delayable interface implementation.

func (g delayGate) Duration() float64 { return g.duration }
func (g delayGate) Unit() string      { return g.unit }

// Seconds converts the duration to seconds. Panics if the unit is "dt"
// (backend-dependent and cannot be converted without hardware information).
func (g delayGate) Seconds() float64 {
	switch g.unit {
	case UnitS:
		return g.duration
	case UnitMs:
		return g.duration * 1e-3
	case UnitUs:
		return g.duration * 1e-6
	case UnitNs:
		return g.duration * 1e-9
	default:
		panic(fmt.Sprintf("gate.Delay.Seconds: cannot convert unit %q to seconds", g.unit))
	}
}

// Barrier returns a barrier pseudo-gate spanning n qubits. Barriers have no
// matrix representation - they prevent gate reordering across them during
// transpilation. Simulators skip them.
func Barrier(n int) Gate {
	return barrierGate{n: n}
}

type barrierGate struct{ n int }

func (g barrierGate) Name() string                { return "barrier" }
func (g barrierGate) Qubits() int                 { return g.n }
func (g barrierGate) Matrix() []complex128        { return nil }
func (g barrierGate) Params() []float64           { return nil }
func (g barrierGate) Inverse() Gate               { return g }
func (g barrierGate) Decompose(_ []int) []Applied { return nil }
