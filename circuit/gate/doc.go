// Package gate defines the Gate interface and provides a standard library
// of quantum gates.
//
// Fixed gates (I, H, X, Y, Z, S, Sdg, T, Tdg, SX, CNOT, CZ, CY, SWAP,
// CCX, CSWAP) are package-level singletons requiring zero allocation.
// Parameterized gates (RX, RY, RZ, Phase, U3, CP, CRX, CRY, CRZ) are
// created via constructor functions that accept rotation angles.
//
// IonQ native gates (GPI, GPI2, MS) are also provided for hardware-native
// circuit construction.
//
// Gate matrices are stored as flat []complex128 slices in row-major order.
// For two-qubit gates the convention is: row bit 1 (MSB) = q0, bit 0 (LSB) = q1.
package gate
