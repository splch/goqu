// Package ir defines the immutable circuit intermediate representation.
//
// A [Circuit] is an ordered sequence of [Operation] values, each pairing a
// gate with target qubit indices and optional classical bit indices (for
// measurement) or classical conditions.
//
// Use [Circuit.Stats] to obtain depth, gate count, two-qubit gate count,
// and total parameter count without mutating the circuit.
//
// Circuits are typically constructed via the builder package rather than
// calling [New] directly.
package ir
