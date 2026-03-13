// Package qasmparse parses OpenQASM 3.0 with OpenPulse cal {} blocks
// into [pulse.Program] objects. It is the inverse of the QASM emission
// performed by the Braket backend serializer.
//
// The parser handles the subset of OpenQASM used for pulse-level control:
// port declarations, frame definitions, and all eight pulse instruction types
// (play, delay, set_phase, shift_phase, set_frequency, shift_frequency,
// barrier, capture_v0).
//
// Usage:
//
//	prog, err := qasmparse.ParseString(qasmSource)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// prog is a *pulse.Program
//
// Waveform constructor calls (e.g., gaussian(0.5, 2e-08, 1e-08)) are
// resolved against built-in waveforms from the waveform package. Custom
// waveforms can be registered via [WithWaveform].
package qasmparse
