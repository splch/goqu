// Package gate defines the quantum gate interface and standard gate library.
package gate

// Gate represents a quantum gate operation.
type Gate interface {
	// Name returns the canonical gate name (e.g., "H", "CNOT", "RZ").
	Name() string

	// Qubits returns the number of qubits this gate acts on.
	Qubits() int

	// Matrix returns the unitary matrix as a flat row-major slice.
	// Length is (2^n)^2 where n = Qubits().
	Matrix() []complex128

	// Params returns gate parameters (rotation angles, etc.).
	// Returns nil for non-parameterized gates.
	Params() []float64

	// Inverse returns the adjoint (inverse) of this gate.
	Inverse() Gate

	// Decompose breaks this gate into a sequence of simpler gates
	// targeting the given qubit indices. Returns nil if already primitive.
	Decompose(qubits []int) []Applied
}

// Applied pairs a Gate with specific qubit indices.
type Applied struct {
	Gate   Gate
	Qubits []int
}
