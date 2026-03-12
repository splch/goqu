// Package decompose provides gate decomposition algorithms for single-qubit
// and two-qubit unitaries.
//
// Single-qubit gates are decomposed via Euler angles: [EulerZYZ], [EulerZXZ],
// or the target-adaptive [EulerDecomposeForBasis] with [BasisForTarget].
// Two-qubit gates use [KAK] (Cartan/KAK decomposition) or [KAKForBasis].
//
// [DecomposeByRule] handles known gate identities and [DecomposeMultiControlled]
// implements Barenco et al. no-ancilla recursion for multi-controlled gates.
package decompose
