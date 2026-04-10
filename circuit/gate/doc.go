// Package gate defines the [Gate] interface and provides a standard library
// of quantum gates.
//
// A quantum gate is a unitary transformation on one or more qubits. Unitary
// means the transformation is reversible: U * U† = I. Every gate's Matrix
// method returns the unitary as a flat row-major []complex128 slice of length
// (2^n)^2, where n is the number of qubits. The simulator applies this matrix
// to the relevant amplitudes of the statevector to evolve the quantum state.
//
// # Fixed gates
//
// Single-qubit: [I] (identity), [H] (Hadamard/superposition), [X] (NOT/bit-flip),
// [Y] (bit-flip with phase), [Z] (phase-flip), [S] (sqrt-Z), [Sdg] (S†),
// [T] (pi/8 gate), [Tdg] (T†), [SX] (sqrt-X).
//
// Two-qubit: [CNOT] (controlled-NOT, entangling), [CZ] (controlled-Z),
// [CY] (controlled-Y), [SWAP], [ISWAP] (Google native), [ECR] (IBM native),
// [DCX] (double-CNOT), [CH] (controlled-H), [CSX] (controlled-sqrt-X),
// [Sycamore] (Google native FSim(pi/2, pi/6)).
//
// Three-qubit: [CCX] (Toffoli), [CSWAP] (Fredkin), [CCZ].
//
// These are package-level singletons requiring zero allocation.
//
// # Parameterized gates
//
// Rotation gates ([RX], [RY], [RZ]) rotate the qubit state around the
// corresponding axis of the Bloch sphere by a given angle. [Phase], [U3],
// [U1], [U2], [Rot], [PhasedXZ], [GlobalPhase] provide various single-qubit
// parameterizations. Two-qubit: [CP], [CRX], [CRY], [CRZ] (controlled
// rotations), [RXX], [RYY], [RZZ] (Ising interactions), [FSim], [PSwap].
// IonQ native: [GPI], [GPI2], [MS], [ZZ], [NOP].
//
// # Custom and multi-controlled gates
//
// [Unitary] and [MustUnitary] create custom gates from user-provided matrices
// with unitarity validation. Multi-controlled gates are built with [MCX],
// [MCZ], [MCP], and [Controlled].
//
// # Pseudo-gates
//
// [Reset] resets a qubit to |0>. [Delay] idles a qubit for a duration.
// [Barrier] prevents gate reordering across it during transpilation.
// These have no matrix representation - simulators handle them directly.
//
// # Matrix convention
//
// Gate matrices are stored as flat []complex128 slices in row-major order.
// For two-qubit gates the convention is: row bit 1 (MSB) = q0, bit 0 (LSB) = q1.
package gate
