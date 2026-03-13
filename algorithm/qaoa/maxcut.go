package qaoa

import (
	"fmt"

	"github.com/splch/goqu/sim/pauli"
)

// MaxCutHamiltonian encodes a graph as a cost Hamiltonian for the MaxCut problem.
// C = 0.5 * sum_{(i,j) in edges} (I - Z_i*Z_j)
// Minimizing -C maximizes the cut. The returned Hamiltonian is negated so that
// minimization yields the maximum cut.
func MaxCutHamiltonian(edges [][2]int, numQubits int) (pauli.PauliSum, error) {
	if len(edges) == 0 {
		return pauli.PauliSum{}, fmt.Errorf("qaoa: no edges provided")
	}
	if numQubits < 2 {
		return pauli.PauliSum{}, fmt.Errorf("qaoa: numQubits must be >= 2")
	}

	var terms []pauli.PauliString
	for _, e := range edges {
		if e[0] < 0 || e[0] >= numQubits || e[1] < 0 || e[1] >= numQubits {
			return pauli.PauliSum{}, fmt.Errorf("qaoa: edge (%d,%d) out of range", e[0], e[1])
		}
		// -0.5 * I (identity contributes constant offset, included for correctness)
		terms = append(terms, pauli.NewPauliString(-0.5, nil, numQubits))
		// +0.5 * Z_i*Z_j
		terms = append(terms, pauli.NewPauliString(0.5, map[int]pauli.Pauli{
			e[0]: pauli.Z,
			e[1]: pauli.Z,
		}, numQubits))
	}

	return pauli.NewPauliSum(terms)
}
