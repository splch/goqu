// Package clifford implements an efficient stabilizer-state simulator
// using the Aaronson-Gottesman tableau representation. It can simulate
// Clifford circuits (H, S, CNOT, X, Y, Z, CZ, SWAP, CY, SX) on
// thousands of qubits in polynomial time.
//
// Clifford circuits (composed of H, S, CNOT, and Pauli gates) can be
// classically simulated in O(n^2) time per gate and O(n^2) space by the
// Gottesman-Knill theorem. The simulator uses the stabilizer tableau
// representation: each of the 2n stabilizer generators (n stabilizers and
// n destabilizers) is tracked as a row of n X-bits, n Z-bits, and a phase
// bit. Gate application updates tableau rows via simple bitwise operations,
// and measurement is performed by checking commutation with the stabilizers.
package clifford
