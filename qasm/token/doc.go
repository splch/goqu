// Package token defines the lexical token types for OpenQASM 3.0 parsing.
//
// [Type] is the token type enum with constants for keywords (GATE, QUBIT,
// MEASURE, …), operators, literals, and punctuation. [Token] pairs a Type
// with its literal text and source position. [LookupIdent] classifies an
// identifier as a keyword or plain IDENT.
//
// Most users should use [parser.Parse] rather than importing this package
// directly.
package token
