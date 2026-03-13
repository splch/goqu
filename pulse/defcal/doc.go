// Package defcal provides gate-to-pulse calibration mapping.
//
// A [Table] stores [ProgramFunc] entries keyed by (gate name, qubit list).
// [Compile] walks a circuit and replaces each gate with its calibrated
// pulse schedule, producing a single merged [pulse.Program].
//
// Resolution order: qubit-specific calibration > gate-level default.
// Measurements become Capture instructions; barriers synchronize all frames.
package defcal
