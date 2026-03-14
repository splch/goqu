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

// Bindable is optionally implemented by gates with symbolic parameters.
// It enables parameterized/variational circuits.
type Bindable interface {
	Bind(bindings map[string]float64) (Gate, error)
	FreeParameters() []string
	IsBound() bool
}

// Delayable is optionally implemented by delay instructions to expose
// duration metadata beyond what Params() provides.
type Delayable interface {
	Duration() float64 // raw numeric duration value
	Unit() string      // time unit: "ns", "us", "ms", "s", "dt"
	Seconds() float64  // duration converted to seconds (panics for "dt")
}

// Applied pairs a Gate with specific qubit indices.
type Applied struct {
	Gate   Gate
	Qubits []int
}
