// Package verify checks circuit equivalence using statevector simulation.
//
// [EquivalentOnZero] compares output states from |0…0⟩ (up to 14 qubits).
// [Equivalent] compares full unitaries on all basis states (up to 10 qubits).
// Both tolerate global phase differences.
//
// Intended for testing transpilation correctness, not production use.
package verify
