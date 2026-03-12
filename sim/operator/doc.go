// Package operator provides quantum channel representations and conversions.
//
// A quantum channel (completely positive trace-preserving map) can be
// represented in several equivalent forms:
//
//   - [Kraus]: a set of Kraus operators {E_k} satisfying sum_k E_k-dagger E_k = I.
//   - [SuperOp]: a d^2 x d^2 superoperator matrix S = sum_k (E_k (x) conj(E_k)).
//   - [Choi]: a d^2 x d^2 Choi matrix (Choi-Jamiolkowski representation).
//   - [PTM]: a d^2 x d^2 real Pauli transfer matrix.
//
// The package provides lossless conversions between all representations,
// validity checks ([IsCP], [IsTP], [IsCPTP]), composition ([Compose], [Tensor]),
// and channel fidelity measures ([AverageGateFidelity], [ProcessFidelity]).
//
// Existing [noise.Channel] values can be lifted into operator representations
// via [FromChannel].
package operator
