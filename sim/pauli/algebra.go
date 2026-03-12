package pauli

import "fmt"

// phaseTable[a][b] gives the power of i for single-qubit Pauli multiplication a*b.
// Indices use the uint8 encoding: I=0, Z=1, X=2, Y=3.
//
//	I*I=I(0)  I*Z=Z(0)  I*X=X(0)  I*Y=Y(0)
//	Z*I=Z(0)  Z*Z=I(0)  Z*X=iY(1) Z*Y=-iX(3)
//	X*I=X(0)  X*Z=-iY(3) X*X=I(0) X*Y=iZ(1)
//	Y*I=Y(0)  Y*Z=iX(1) Y*X=-iZ(3) Y*Y=I(0)
var phaseTable = [4][4]int{
	{0, 0, 0, 0}, // I * {I,Z,X,Y}
	{0, 0, 1, 3}, // Z * {I,Z,X,Y}
	{0, 3, 0, 1}, // X * {I,Z,X,Y}
	{0, 1, 3, 0}, // Y * {I,Z,X,Y}
}

// Mul multiplies two PauliStrings (tensor-product-wise).
// Both must have the same number of qubits.
// The result coefficient includes the phase from single-qubit Pauli multiplication.
func Mul(a, b PauliString) PauliString {
	if a.numQubits != b.numQubits {
		panic(fmt.Sprintf("pauli.Mul: qubit count mismatch: %d vs %d", a.numQubits, b.numQubits))
	}
	ops := make([]Pauli, a.numQubits)
	totalPhase := 0
	for i := 0; i < a.numQubits; i++ {
		ai := a.ops[i]
		bi := b.ops[i]
		ops[i] = ai ^ bi
		totalPhase += phaseTable[ai][bi]
	}
	coeff := a.coeff * b.coeff * iPow(totalPhase%4)
	return PauliString{coeff: coeff, ops: ops, numQubits: a.numQubits}
}

// Commutes returns true if a and b commute.
// Two Pauli strings commute iff they anticommute at an even number of positions.
func Commutes(a, b PauliString) bool {
	if a.numQubits != b.numQubits {
		panic(fmt.Sprintf("pauli.Commutes: qubit count mismatch: %d vs %d", a.numQubits, b.numQubits))
	}
	odd := 0
	for i := 0; i < a.numQubits; i++ {
		p := phaseTable[a.ops[i]][b.ops[i]]
		if p%2 != 0 { // phase 1 or 3 means anticommuting at this position
			odd++
		}
	}
	return odd%2 == 0
}

// AntiCommutes returns true if a and b anticommute.
func AntiCommutes(a, b PauliString) bool {
	return !Commutes(a, b)
}

// Tensor returns the tensor product of two PauliStrings (concatenate operators, multiply coefficients).
func Tensor(a, b PauliString) PauliString {
	nq := a.numQubits + b.numQubits
	ops := make([]Pauli, nq)
	copy(ops, a.ops)
	copy(ops[a.numQubits:], b.ops)
	return PauliString{coeff: a.coeff * b.coeff, ops: ops, numQubits: nq}
}
