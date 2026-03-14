package ansatz

import (
	"math"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/circuit/param"
)

// UCCSD is a Unitary Coupled-Cluster Singles and Doubles ansatz for quantum
// chemistry. It implements the Jordan-Wigner-mapped UCCSD circuit, consisting
// of a Hartree-Fock reference state followed by fermionic single and double
// excitation operators.
//
// The parameter vector is ordered as [singles..., doubles...], but the circuit
// applies doubles before singles (matching PennyLane's convention).
type UCCSD struct {
	numQubits    int
	numElectrons int
	vec          *param.Vector
	singles      [][2]int // [occupied, virtual] pairs
	doubles      [][4]int // [occ1, occ2, virt1, virt2] quartets
}

// NewUCCSD creates a UCCSD ansatz for the given number of spin-orbitals
// (qubits) and electrons.
func NewUCCSD(numQubits, numElectrons int) *UCCSD {
	occupied := make([]int, numElectrons)
	for i := range numElectrons {
		occupied[i] = i
	}
	virtual := make([]int, numQubits-numElectrons)
	for i := range numQubits - numElectrons {
		virtual[i] = numElectrons + i
	}

	// Enumerate single excitations.
	var singles [][2]int
	for _, occ := range occupied {
		for _, virt := range virtual {
			singles = append(singles, [2]int{occ, virt})
		}
	}

	// Enumerate double excitations.
	var doubles [][4]int
	for a := range len(occupied) {
		for b := a + 1; b < len(occupied); b++ {
			for c := range len(virtual) {
				for d := c + 1; d < len(virtual); d++ {
					doubles = append(doubles, [4]int{
						occupied[a], occupied[b],
						virtual[c], virtual[d],
					})
				}
			}
		}
	}

	nParams := len(singles) + len(doubles)
	return &UCCSD{
		numQubits:    numQubits,
		numElectrons: numElectrons,
		vec:          param.NewVector("θ", nParams),
		singles:      singles,
		doubles:      doubles,
	}
}

func (u *UCCSD) NumParams() int             { return u.vec.Size() }
func (u *UCCSD) ParamVector() *param.Vector { return u.vec }

func (u *UCCSD) Circuit() (*ir.Circuit, error) {
	b := builder.New("UCCSD", u.numQubits)

	// Hartree-Fock reference state.
	for i := range u.numElectrons {
		b.X(i)
	}

	// Double excitations first (PennyLane convention).
	for i, d := range u.doubles {
		theta := u.vec.At(len(u.singles) + i).Expr()
		wires1 := makeRange(d[0], d[1])
		wires2 := makeRange(d[2], d[3])
		fermionicDoubleExcitation(b, wires1, wires2, theta)
	}

	// Then single excitations.
	for i, s := range u.singles {
		theta := u.vec.At(i).Expr()
		wires := makeRange(s[0], s[1])
		fermionicSingleExcitation(b, wires, theta)
	}

	return b.Build()
}

// makeRange returns [start, start+1, ..., end].
func makeRange(start, end int) []int {
	r := make([]int, end-start+1)
	for i := range r {
		r[i] = start + i
	}
	return r
}

// fermionicSingleExcitation applies the Jordan-Wigner-decomposed single
// excitation exp(theta * (a†_p a_r - h.c.)) for wires [r, r+1, ..., p].
// Implements two Pauli exponential layers following PennyLane's
// FermionicSingleExcitation decomposition.
func fermionicSingleExcitation(b *builder.Builder, wires []int, theta param.Expr) {
	r := wires[0]
	p := wires[len(wires)-1]
	halfTheta := param.Div(theta, param.Literal(2))
	negHalfTheta := param.Neg(halfTheta)

	// Layer 1: exp(+theta/2 * Z_mid * Y_r X_p)
	b.RX(-math.Pi/2, r) // Y basis on r
	b.H(p)              // X basis on p
	cnotLadderForward(b, wires)
	b.SymRZ(halfTheta, p)
	cnotLadderReverse(b, wires)
	b.RX(math.Pi/2, r)
	b.H(p)

	// Layer 2: exp(-theta/2 * Z_mid * X_r Y_p)
	b.H(r)              // X basis on r
	b.RX(-math.Pi/2, p) // Y basis on p
	cnotLadderForward(b, wires)
	b.SymRZ(negHalfTheta, p)
	cnotLadderReverse(b, wires)
	b.H(r)
	b.RX(math.Pi/2, p)
}

// fermionicDoubleExcitation applies the Jordan-Wigner-decomposed double
// excitation exp(theta * (a†_p a†_q a_r a_s - h.c.)) for wires1 [s,...,r]
// and wires2 [q,...,p]. Implements 8 Pauli exponential layers following
// PennyLane's FermionicDoubleExcitation decomposition.
func fermionicDoubleExcitation(b *builder.Builder, wires1, wires2 []int, theta param.Expr) {
	s := wires1[0]
	r := wires1[len(wires1)-1]
	q := wires2[0]
	p := wires2[len(wires2)-1]

	eighthTheta := param.Div(theta, param.Literal(8))
	negEighthTheta := param.Neg(eighthTheta)

	// Full CNOT ladder: wires1 + bridge(r,q) + wires2.
	fullWires := buildDoubleExcitationWires(wires1, wires2)

	// 8 layers, each with basis changes on {s, r, q, p} and ±theta/8.
	// H = Hadamard (X basis), Y = RX(-pi/2) (Y basis).
	type basisSpec struct {
		s, r, q, p byte // 'H' or 'Y'
		positive   bool
	}
	layers := [8]basisSpec{
		{'H', 'H', 'Y', 'H', true},
		{'Y', 'H', 'Y', 'Y', true},
		{'H', 'Y', 'Y', 'Y', true},
		{'H', 'H', 'H', 'Y', true},
		{'Y', 'H', 'H', 'H', false},
		{'H', 'Y', 'H', 'H', false},
		{'Y', 'Y', 'Y', 'H', false},
		{'Y', 'Y', 'H', 'Y', false},
	}

	for _, layer := range layers {
		// Apply basis changes.
		applyBasis(b, s, layer.s)
		applyBasis(b, r, layer.r)
		applyBasis(b, q, layer.q)
		applyBasis(b, p, layer.p)

		// CNOT ladder forward.
		cnotLadderForward(b, fullWires)

		// Parameterized rotation.
		if layer.positive {
			b.SymRZ(eighthTheta, p)
		} else {
			b.SymRZ(negEighthTheta, p)
		}

		// CNOT ladder reverse.
		cnotLadderReverse(b, fullWires)

		// Undo basis changes.
		undoBasis(b, s, layer.s)
		undoBasis(b, r, layer.r)
		undoBasis(b, q, layer.q)
		undoBasis(b, p, layer.p)
	}
}

// buildDoubleExcitationWires constructs the full wire list for the CNOT ladder:
// wires1 + bridge from r to q + wires2.
func buildDoubleExcitationWires(wires1, wires2 []int) []int {
	r := wires1[len(wires1)-1]
	q := wires2[0]
	// wires1 + [r+1, ..., q-1, q] (bridge) + wires2[1:]
	var full []int
	full = append(full, wires1...)
	for w := r + 1; w <= q; w++ {
		full = append(full, w)
	}
	if len(wires2) > 1 {
		full = append(full, wires2[1:]...)
	}
	return full
}

func applyBasis(b *builder.Builder, qubit int, basis byte) {
	switch basis {
	case 'H':
		b.H(qubit)
	case 'Y':
		b.RX(-math.Pi/2, qubit)
	}
}

func undoBasis(b *builder.Builder, qubit int, basis byte) {
	switch basis {
	case 'H':
		b.H(qubit) // H is self-inverse
	case 'Y':
		b.RX(math.Pi/2, qubit)
	}
}

func cnotLadderForward(b *builder.Builder, wires []int) {
	for i := 0; i < len(wires)-1; i++ {
		b.CNOT(wires[i], wires[i+1])
	}
}

func cnotLadderReverse(b *builder.Builder, wires []int) {
	for i := len(wires) - 2; i >= 0; i-- {
		b.CNOT(wires[i], wires[i+1])
	}
}
