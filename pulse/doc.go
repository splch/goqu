// Package pulse provides types for pulse-level quantum control.
//
// Pulse programming operates at a lower level than gate-based circuits,
// directly controlling the analog signals sent to quantum hardware.
// This package implements the OpenPulse model used by AWS Braket and
// compatible with the OpenQASM 3.0 pulse grammar.
//
// The three core types are [Port] (hardware I/O endpoint), [Frame]
// (software reference clock), and [Waveform] (signal envelope interface).
// Eight instruction types ([Play], [Delay], [SetPhase], [ShiftPhase],
// [SetFrequency], [ShiftFrequency], [Barrier], [Capture]) compose into
// an immutable [Program] via a fluent [Builder].
//
// Timing uses float64 seconds (not time.Duration) for sub-nanosecond
// precision required by quantum hardware. Frames are stateless value
// types - phase and frequency changes are expressed as instructions.
//
// Standard waveform shapes are provided in the [waveform] sub-package.
// IonQ custom pulse envelopes use a separate model; see the
// [ionq.PulseShapes] type in the backend/ionq package.
package pulse
