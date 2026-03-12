// Package emitter serializes a quantum circuit as OpenQASM 3.0 source text.
//
// [Emit] writes to an [io.Writer]; [EmitString] returns a string. Rotation
// angles are formatted as pi fractions where possible. Use [WithComments]
// to include circuit metadata as comments.
//
// Combined with [parser.ParseString], emitter supports lossless round-trips.
package emitter
