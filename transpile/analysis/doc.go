// Package analysis provides per-qubit timeline helpers used by transpilation
// passes.
//
// [BuildTimelines] returns a [QubitTimeline] per qubit containing ordered
// operation indices. [NextOnQubit] and [PrevOnQubit] enable adjacency
// queries for gate cancellation and commutation passes.
package analysis
