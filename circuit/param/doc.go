// Package param provides symbolic parameters and expressions for
// parameterized quantum circuits.
//
// A [Parameter] represents a named symbolic value; related parameters can
// be grouped in a [Vector]. Parameters are combined into an [Expr] tree
// using [Literal], [Add], [Sub], [Mul], [Div], and [Neg].
//
// Symbolic gate constructors ([SymRX], [SymRY], [SymRZ], [SymPhase],
// [SymU3], [SymCP]) accept [Expr] arguments instead of float64 values.
// Before simulation, bind all free parameters to concrete values with
// [ir.Bind].
package param
