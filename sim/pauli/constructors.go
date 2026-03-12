package pauli

// XOn creates a PauliString with X on the specified qubits and I elsewhere.
func XOn(qubits []int, numQubits int) PauliString {
	m := make(map[int]Pauli, len(qubits))
	for _, q := range qubits {
		m[q] = X
	}
	return NewPauliString(1, m, numQubits)
}

// YOn creates a PauliString with Y on the specified qubits and I elsewhere.
func YOn(qubits []int, numQubits int) PauliString {
	m := make(map[int]Pauli, len(qubits))
	for _, q := range qubits {
		m[q] = Y
	}
	return NewPauliString(1, m, numQubits)
}

// Identity returns the N-qubit identity PauliString with coefficient 1.
func Identity(numQubits int) PauliString {
	return NewPauliString(1, nil, numQubits)
}

// FromLabel creates a PauliString from a string like "XZI" with coefficient 1.
// This is a convenience wrapper around Parse that panics on error.
func FromLabel(s string) PauliString {
	ps, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return ps
}
