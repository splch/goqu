// Package pauli provides Pauli algebra types and expectation value computation.
package pauli

import (
	"fmt"
	"math/bits"
	"strings"
)

// Pauli is a single-qubit Pauli operator using symplectic encoding.
//
// The symplectic encoding represents each single-qubit Pauli as a 2-bit
// value (x,z): I=(0,0), X=(1,0), Z=(0,1), Y=(1,1). This works because
// Y = iXZ, so the Pauli product (up to phase) is given by bitwise XOR:
// P1*P2 has x-bit = x1 XOR x2, z-bit = z1 XOR z2. The phase is tracked
// separately via the phaseTable lookup in algebra.go.
type Pauli uint8

const (
	I Pauli = 0b00 // Identity
	X Pauli = 0b10 // x=1, z=0
	Z Pauli = 0b01 // x=0, z=1
	Y Pauli = 0b11 // x=1, z=1 (Y = iXZ)
)

// String returns the single-character name of the Pauli operator.
func (p Pauli) String() string {
	switch p {
	case I:
		return "I"
	case X:
		return "X"
	case Y:
		return "Y"
	case Z:
		return "Z"
	default:
		return "?"
	}
}

// PauliString is an N-qubit tensor product of Paulis with a complex coefficient.
// Example: 0.5 * X⊗Z⊗I
type PauliString struct {
	coeff     complex128
	ops       []Pauli // ops[i] = Pauli on qubit i; len = numQubits
	numQubits int
}

// NewPauliString creates a PauliString from a sparse map of qubit->Pauli.
// Qubits not in the map default to I. numQubits must be >= 1.
func NewPauliString(coeff complex128, ops map[int]Pauli, numQubits int) PauliString {
	if numQubits < 1 {
		panic("pauli: numQubits must be >= 1")
	}
	paulis := make([]Pauli, numQubits)
	for q, p := range ops {
		if q < 0 || q >= numQubits {
			panic(fmt.Sprintf("pauli: qubit %d out of range [0, %d)", q, numQubits))
		}
		paulis[q] = p
	}
	return PauliString{coeff: coeff, ops: paulis, numQubits: numQubits}
}

// Parse parses a Pauli string from a string like "XZI" or "XYZI".
// Left-to-right corresponds to qubit 0, 1, 2, ... with coefficient 1.
func Parse(s string) (PauliString, error) {
	if len(s) == 0 {
		return PauliString{}, fmt.Errorf("pauli: empty string")
	}
	ops := make([]Pauli, len(s))
	for i, ch := range s {
		switch ch {
		case 'I':
			ops[i] = I
		case 'X':
			ops[i] = X
		case 'Y':
			ops[i] = Y
		case 'Z':
			ops[i] = Z
		default:
			return PauliString{}, fmt.Errorf("pauli: invalid character %q at position %d", ch, i)
		}
	}
	return PauliString{coeff: 1, ops: ops, numQubits: len(s)}, nil
}

// ZOn creates a PauliString with Z on the specified qubits and I elsewhere.
func ZOn(qubits []int, numQubits int) PauliString {
	m := make(map[int]Pauli, len(qubits))
	for _, q := range qubits {
		m[q] = Z
	}
	return NewPauliString(1, m, numQubits)
}

// Coeff returns the complex coefficient.
func (ps PauliString) Coeff() complex128 { return ps.coeff }

// Op returns the Pauli operator on the given qubit.
func (ps PauliString) Op(qubit int) Pauli {
	if qubit < 0 || qubit >= ps.numQubits {
		return I
	}
	return ps.ops[qubit]
}

// NumQubits returns the number of qubits in the Pauli string.
func (ps PauliString) NumQubits() int { return ps.numQubits }

// Scale returns a new PauliString with the coefficient multiplied by c.
func (ps PauliString) Scale(c complex128) PauliString {
	ops := make([]Pauli, len(ps.ops))
	copy(ops, ps.ops)
	return PauliString{coeff: ps.coeff * c, ops: ops, numQubits: ps.numQubits}
}

// IsIdentity returns true if all operators are I.
func (ps PauliString) IsIdentity() bool {
	for _, p := range ps.ops {
		if p != I {
			return false
		}
	}
	return true
}

// String returns a human-readable representation like "(0.5+0i)*XZI".
func (ps PauliString) String() string {
	var b strings.Builder
	if ps.coeff != 1 {
		fmt.Fprintf(&b, "%v*", ps.coeff)
	}
	for _, p := range ps.ops {
		b.WriteString(p.String())
	}
	return b.String()
}

// xMask returns the symplectic x-bitmask across all qubits in the PauliString.
// Bit i is set if qubit i has X or Y (i.e. the x-component of the symplectic
// pair is 1). When applied to a basis state |k>, the Pauli string maps it to
// |k XOR xMask>, so xMask encodes the bit-flip action of the entire string.
func (ps PauliString) xMask() int {
	var m int
	for i, p := range ps.ops {
		if p&0b10 != 0 { // X or Y
			m |= 1 << i
		}
	}
	return m
}

// zMask returns the symplectic z-bitmask across all qubits in the PauliString.
// Bit i is set if qubit i has Z or Y (i.e. the z-component of the symplectic
// pair is 1). The Z-component determines the sign contribution: each qubit
// where both the basis state bit and zMask bit are set contributes a factor
// of -1, giving an overall sign of (-1)^popcount(state AND zMask).
func (ps PauliString) zMask() int {
	var m int
	for i, p := range ps.ops {
		if p&0b01 != 0 { // Z or Y
			m |= 1 << i
		}
	}
	return m
}

// numY returns the count of Y operators.
func (ps PauliString) numY() int {
	n := 0
	for _, p := range ps.ops {
		if p == Y {
			n++
		}
	}
	return n
}

// PauliSum is a linear combination of PauliStrings (Hamiltonian).
type PauliSum struct {
	terms     []PauliString
	numQubits int
}

// NewPauliSum creates a PauliSum from a slice of PauliStrings.
// All terms must have the same number of qubits.
func NewPauliSum(terms []PauliString) (PauliSum, error) {
	if len(terms) == 0 {
		return PauliSum{}, fmt.Errorf("pauli: empty term list")
	}
	nq := terms[0].numQubits
	for i, t := range terms[1:] {
		if t.numQubits != nq {
			return PauliSum{}, fmt.Errorf("pauli: term %d has %d qubits, expected %d", i+1, t.numQubits, nq)
		}
	}
	cp := make([]PauliString, len(terms))
	copy(cp, terms)
	return PauliSum{terms: cp, numQubits: nq}, nil
}

// Terms returns the PauliStrings in the sum.
func (ps PauliSum) Terms() []PauliString { return ps.terms }

// NumQubits returns the number of qubits.
func (ps PauliSum) NumQubits() int { return ps.numQubits }

// iPow returns i^n where i is the imaginary unit. n is reduced mod 4.
func iPow(n int) complex128 {
	switch n % 4 {
	case 0:
		return 1
	case 1:
		return 1i
	case 2:
		return -1
	case 3:
		return -1i
	}
	return 1
}

// sign returns (-1)^(popcount(v)).
func sign(v int) float64 {
	if bits.OnesCount(uint(v))%2 == 0 {
		return 1
	}
	return -1
}
