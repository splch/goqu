// Package defcal maps gate-level circuits to pulse programs via
// calibration tables. Each gate is replaced by a user-supplied
// [ProgramFunc] that constructs the corresponding pulse schedule.
package defcal

import (
	"fmt"
	"sort"

	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/internal/mathutil"
	"github.com/splch/qgo/pulse"
)

// ProgramFunc constructs a pulse program for a gate.
// params contains the gate's rotation angles (nil for fixed gates).
type ProgramFunc func(params []float64) (*pulse.Program, error)

// Table maps (gate name, qubit list) pairs to calibration functions.
// Resolution order: qubit-specific entry > gate-level default.
type Table struct {
	byGateQubits map[string]ProgramFunc // "H:[0]" → fn
	byGate       map[string]ProgramFunc // "H" → fn (default)
}

// NewTable creates an empty calibration table.
func NewTable() *Table {
	return &Table{
		byGateQubits: make(map[string]ProgramFunc),
		byGate:       make(map[string]ProgramFunc),
	}
}

// Add registers a calibration. If qubits is nil or empty, it registers
// a gate-level default; otherwise a qubit-specific override.
func (t *Table) Add(gateName string, qubits []int, fn ProgramFunc) {
	if len(qubits) == 0 {
		t.byGate[gateName] = fn
	} else {
		t.byGateQubits[qubitKey(gateName, qubits)] = fn
	}
}

// Lookup finds the best calibration for a gate on the given qubits.
// It checks qubit-specific entries first, then falls back to the gate default.
func (t *Table) Lookup(gateName string, qubits []int) (ProgramFunc, bool) {
	if fn, ok := t.byGateQubits[qubitKey(gateName, qubits)]; ok {
		return fn, true
	}
	if fn, ok := t.byGate[gateName]; ok {
		return fn, true
	}
	return nil, false
}

func qubitKey(name string, qubits []int) string {
	sorted := make([]int, len(qubits))
	copy(sorted, qubits)
	sort.Ints(sorted)
	return fmt.Sprintf("%s:%v", name, sorted)
}

// defaultCaptureDuration is the measurement capture window (1 us).
const defaultCaptureDuration = 1e-6

// CompileConfig holds optional configuration for [CompileWithConfig].
type CompileConfig struct {
	// CRFrames maps cross-resonance frame names to [control, target] qubit pairs.
	// These are stored in the program metadata as "cr_frame:<name>" → "ctrl,tgt".
	CRFrames map[string][2]int
}

// Compile replaces every gate in the circuit with its pulse calibration,
// producing a single merged pulse program.
//
// ports lists all hardware ports. frameMap maps qubit indices to the
// drive frame used for that qubit. Measurements become Capture instructions;
// barriers synchronize all frames.
func Compile(c *ir.Circuit, table *Table, ports []pulse.Port, frameMap map[int]pulse.Frame) (*pulse.Program, error) {
	return CompileWithConfig(c, table, ports, frameMap, nil)
}

// CompileWithConfig is like [Compile] but accepts an optional [CompileConfig]
// for cross-resonance frame metadata and other extended settings.
func CompileWithConfig(c *ir.Circuit, table *Table, ports []pulse.Port, frameMap map[int]pulse.Frame, config *CompileConfig) (*pulse.Program, error) {
	b := pulse.NewBuilder(c.Name() + "_pulse")

	for _, p := range ports {
		b.AddPort(p)
	}

	// Collect all frames (deterministic order by qubit index).
	qubits := make([]int, 0, len(frameMap))
	for q := range frameMap {
		qubits = append(qubits, q)
	}
	sort.Ints(qubits)

	allFrames := make([]pulse.Frame, 0, len(frameMap))
	for _, q := range qubits {
		f := frameMap[q]
		b.AddFrame(f)
		allFrames = append(allFrames, f)
	}

	for _, op := range c.Ops() {
		// Measurement → Capture.
		if op.Gate == nil && len(op.Clbits) > 0 {
			for _, q := range op.Qubits {
				f, ok := frameMap[q]
				if !ok {
					return nil, fmt.Errorf("defcal: no frame for qubit %d", q)
				}
				b.Capture(f, defaultCaptureDuration)
			}
			continue
		}
		if op.Gate == nil {
			continue
		}

		// Barrier → synchronize all frames.
		if op.Gate.Name() == "barrier" {
			b.Barrier(allFrames...)
			continue
		}

		// Gate → lookup calibration (strip params: "RX(1.23)" → "RX").
		gateName := mathutil.StripParams(op.Gate.Name())
		fn, ok := table.Lookup(gateName, op.Qubits)
		if !ok {
			return nil, fmt.Errorf("defcal: no calibration for gate %q on qubits %v",
				gateName, op.Qubits)
		}

		subProg, err := fn(op.Gate.Params())
		if err != nil {
			return nil, fmt.Errorf("defcal: gate %q on qubits %v: %w",
				gateName, op.Qubits, err)
		}

		// Append all instructions from the sub-program.
		for _, inst := range subProg.Instructions() {
			switch v := inst.(type) {
			case pulse.Play:
				b.Play(v.Frame, v.Waveform)
			case pulse.Delay:
				b.Delay(v.Frame, v.Duration)
			case pulse.SetPhase:
				b.SetPhase(v.Frame, v.Phase)
			case pulse.ShiftPhase:
				b.ShiftPhase(v.Frame, v.Delta)
			case pulse.SetFrequency:
				b.SetFrequency(v.Frame, v.Frequency)
			case pulse.ShiftFrequency:
				b.ShiftFrequency(v.Frame, v.Delta)
			case pulse.Barrier:
				b.Barrier(v.Frames...)
			case pulse.Capture:
				b.Capture(v.Frame, v.Duration)
			}
		}
	}

	// Store CR frame metadata if configured.
	if config != nil {
		for name, qubits := range config.CRFrames {
			b.WithMetadata(
				fmt.Sprintf("cr_frame:%s", name),
				fmt.Sprintf("%d,%d", qubits[0], qubits[1]),
			)
		}
	}

	return b.Build()
}
