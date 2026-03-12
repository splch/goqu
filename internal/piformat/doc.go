// Package piformat formats rotation angles as human-readable pi fractions.
//
// [FormatUnicode] uses the π symbol, [FormatASCII] uses "pi", and
// [FormatQASM] uses QASM 3.0 conventions with higher precision.
// Values within 1e-10 of a recognized fraction are snapped; others
// fall back to %.4g.
package piformat
