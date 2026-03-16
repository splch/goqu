// Package decompose provides gate decomposition algorithms for single-qubit
// and two-qubit unitaries.
//
// Single-qubit gates are decomposed via Euler angles: [EulerZYZ], [EulerZXZ],
// or the target-adaptive [EulerDecomposeForBasis] with [BasisForTarget].
// Euler decomposition expresses any single-qubit unitary as a product of three
// rotation gates (e.g., Rz-Ry-Rz), supporting six standard conventions.
//
// Two-qubit gates use [KAK] (Cartan/KAK decomposition) or [KAKForBasis].
// The KAK (Cartan) decomposition factors any two-qubit unitary into at most
// 3 CNOT gates plus local single-qubit rotations by mapping the non-local
// content of the gate into the Weyl chamber.
// Reference: Vatan-Williams, arXiv:quant-ph/0308006.
//
// [DecomposeByRule] handles known gate identities and [DecomposeMultiControlled]
// implements Barenco et al. no-ancilla recursion for multi-controlled gates.
// Multi-control decomposition recursively breaks C^n(U) gates into O(n^2) CNOT
// plus single-qubit gates without requiring ancilla qubits.
// Reference: Barenco et al., Phys. Rev. A 52, 3457 (1995).
package decompose
