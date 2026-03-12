// Package pauli provides Pauli algebra types and efficient expectation value
// computation for statevector and density matrix simulators.
//
// A [PauliString] represents a tensor product of single-qubit Pauli operators
// (I, X, Y, Z) with a complex coefficient. A [PauliSum] is a linear
// combination of PauliStrings, suitable for representing Hamiltonians.
//
// Expectation values are computed via symplectic-encoding tricks that avoid
// constructing the full observable matrix:
//
//   - [Expect] and [ExpectSum] operate on statevectors in O(2^n) per term
//   - [ExpectDM] and [ExpectSumDM] operate on density matrices
//   - [ExpectFromCounts] estimates Z-basis values from measurement counts
//
// Use [Parse] for string notation ("XZI") and [ZOn] to construct Z
// operators on specific qubits.
package pauli
