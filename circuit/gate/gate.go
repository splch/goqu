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

// The following optional interfaces enable the simulator to select optimized
// kernels via type assertions rather than string-matching gate names.

// Diagonal2Q is optionally implemented by 2-qubit gates whose matrix is fully
// diagonal (only diagonal elements are non-zero, e.g. RZZ). The simulator
// applies only phase factors, avoiding full 4x4 matrix multiplication.
type Diagonal2Q interface {
	// Diagonal returns the 4 diagonal elements [d00, d01, d10, d11].
	Diagonal() (d00, d01, d10, d11 complex128)
}

// ControlDiagonal2Q is optionally implemented by 2-qubit controlled gates
// whose matrix is diag(1, 1, d10, d11) — identity on the |0x> subspace.
// Examples: CP, CRZ. The simulator only multiplies the |1x> amplitudes.
type ControlDiagonal2Q interface {
	// ControlDiagonal returns the two non-trivial diagonal elements [d10, d11].
	ControlDiagonal() (d10, d11 complex128)
}

// ControlU2Q is optionally implemented by 2-qubit controlled gates whose
// matrix is I on the |0x> subspace and a 2x2 unitary on the |1x> subspace.
// Examples: CRX, CRY. The simulator applies only the 2x2 submatrix.
type ControlU2Q interface {
	// ControlSubmatrix returns the 2x2 unitary [u00, u01, u10, u11] applied
	// when the control qubit is |1>.
	ControlSubmatrix() (u00, u01, u10, u11 complex128)
}
