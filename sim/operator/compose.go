package operator

// Compose returns the sequential composition of two channels: first a, then b.
// The resulting Kraus operators are {B_j * A_i} for all pairs (i, j).
// Both channels must act on the same number of qubits.
func Compose(a, b *Kraus) *Kraus {
	if a.nq != b.nq {
		panic("operator.Compose: qubit count mismatch")
	}
	dim := 1 << a.nq
	operators := make([][]complex128, 0, len(a.operators)*len(b.operators))
	for _, ai := range a.operators {
		for _, bj := range b.operators {
			operators = append(operators, matMul(bj, ai, dim))
		}
	}
	return &Kraus{nq: a.nq, operators: operators}
}

// Tensor returns the tensor product of two channels acting on disjoint qubits.
// If a acts on nA qubits and b acts on nB qubits, the result acts on nA+nB qubits.
// The resulting Kraus operators are {A_i (x) B_j} for all pairs (i, j).
func Tensor(a, b *Kraus) *Kraus {
	dimA := 1 << a.nq
	dimB := 1 << b.nq
	dimR := dimA * dimB
	nq := a.nq + b.nq
	operators := make([][]complex128, 0, len(a.operators)*len(b.operators))
	for _, ai := range a.operators {
		for _, bj := range b.operators {
			op := make([]complex128, dimR*dimR)
			for ra := range dimA {
				for ca := range dimA {
					aij := ai[ra*dimA+ca]
					if aij == 0 {
						continue
					}
					for rb := range dimB {
						for cb := range dimB {
							row := ra*dimB + rb
							col := ca*dimB + cb
							op[row*dimR+col] = aij * bj[rb*dimB+cb]
						}
					}
				}
			}
			operators = append(operators, op)
		}
	}
	return &Kraus{nq: nq, operators: operators}
}
