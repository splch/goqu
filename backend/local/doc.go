// Package local provides a [backend.Backend] backed by the in-process
// statevector simulator.
//
// [New] creates a backend with optional [WithMaxQubits] (default 28) and
// [WithLogger]. Execution is synchronous: [Backend.Submit] runs the circuit
// immediately and returns a completed job. Results are cached in memory.
package local
