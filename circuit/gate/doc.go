// Package gate defines the [Gate] interface and provides a standard library
// of quantum gates.
//
// Fixed gates ([I], [H], [X], [Y], [Z], [S], [Sdg], [T], [Tdg], [SX],
// [CNOT], [CZ], [CY], [SWAP], [CCX], [CSWAP]) are package-level singletons
// requiring zero allocation. Parameterized gates ([RX], [RY], [RZ], [Phase],
// [U3], [CP], [CRX], [CRY], [CRZ], [RXX], [RYY], [RZZ]) are created via
// constructor functions.
//
// [Unitary] and [MustUnitary] create custom gates from user-provided matrices
// with unitarity validation. Multi-controlled gates are built with [MCX],
// [MCZ], [MCP], and [Controlled]. IonQ native gates: [GPI], [GPI2], [MS].
//
// Gate matrices are stored as flat []complex128 slices in row-major order.
// For two-qubit gates the convention is: row bit 1 (MSB) = q0, bit 0 (LSB) = q1.
package gate
