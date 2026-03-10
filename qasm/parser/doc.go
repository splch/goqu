// Package parser implements a hand-written recursive descent parser for
// OpenQASM source. Both QASM 2 (qreg/creg, measure q -> c) and QASM 3
// (qubit/bit, c = measure q) syntax are supported.
//
// Entry points are [Parse] (from an io.Reader) and [ParseString].
// The returned [ir.Circuit] contains all declared qubits, classical bits,
// gate operations, measurements, barriers, resets, and conditionals.
//
// Use [WithStrictMode] to reject unknown gate names; the default mode
// treats them as opaque gates.
package parser
