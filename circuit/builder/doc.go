// Package builder provides a fluent API for constructing quantum circuits.
//
// A [Builder] accumulates gate operations and measurements, then produces
// an immutable [ir.Circuit] via [Builder.Build]. Qubit indices are validated
// eagerly; the first error short-circuits all subsequent calls and is
// returned by Build.
//
//	c, err := builder.New("bell", 2).
//	    H(0).
//	    CNOT(0, 1).
//	    MeasureAll().
//	    Build()
package builder
