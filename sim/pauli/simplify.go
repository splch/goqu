package pauli

import (
	"math/cmplx"
	"strings"
)

// Add returns a new PauliSum that is the sum of ps and other.
func (ps PauliSum) Add(other PauliSum) PauliSum {
	if ps.numQubits != other.numQubits {
		panic("pauli: PauliSum.Add qubit count mismatch")
	}
	terms := make([]PauliString, 0, len(ps.terms)+len(other.terms))
	terms = append(terms, ps.terms...)
	terms = append(terms, other.terms...)
	return PauliSum{terms: terms, numQubits: ps.numQubits}
}

// Scale returns a new PauliSum with all coefficients multiplied by c.
func (ps PauliSum) Scale(c complex128) PauliSum {
	terms := make([]PauliString, len(ps.terms))
	for i, t := range ps.terms {
		terms[i] = t.Scale(c)
	}
	return PauliSum{terms: terms, numQubits: ps.numQubits}
}

// Mul distributes and simplifies: (Sum a_i)(Sum b_j) = Sum a_i*b_j.
func (ps PauliSum) Mul(other PauliSum) PauliSum {
	if ps.numQubits != other.numQubits {
		panic("pauli: PauliSum.Mul qubit count mismatch")
	}
	terms := make([]PauliString, 0, len(ps.terms)*len(other.terms))
	for _, a := range ps.terms {
		for _, b := range other.terms {
			terms = append(terms, Mul(a, b))
		}
	}
	return PauliSum{terms: terms, numQubits: ps.numQubits}
}

// opsKey returns a string key for the operator part of a PauliString (without coefficient).
func opsKey(ps PauliString) string {
	var b strings.Builder
	b.Grow(ps.numQubits)
	for _, p := range ps.ops {
		b.WriteString(p.String())
	}
	return b.String()
}

// Simplify combines like terms (same Pauli operators) and drops terms with |coeff| < 1e-10.
func (ps PauliSum) Simplify() PauliSum {
	const eps = 1e-10

	// Combine like terms.
	type entry struct {
		coeff complex128
		ops   []Pauli
	}
	combined := make(map[string]*entry, len(ps.terms))
	order := make([]string, 0, len(ps.terms))
	for _, t := range ps.terms {
		key := opsKey(t)
		if e, ok := combined[key]; ok {
			e.coeff += t.coeff
		} else {
			opsCopy := make([]Pauli, len(t.ops))
			copy(opsCopy, t.ops)
			combined[key] = &entry{coeff: t.coeff, ops: opsCopy}
			order = append(order, key)
		}
	}

	// Collect non-zero terms in original insertion order.
	terms := make([]PauliString, 0, len(combined))
	for _, key := range order {
		e := combined[key]
		if cmplx.Abs(e.coeff) < eps {
			continue
		}
		terms = append(terms, PauliString{coeff: e.coeff, ops: e.ops, numQubits: ps.numQubits})
	}

	if len(terms) == 0 {
		// Return a single zero-coefficient identity term.
		return PauliSum{
			terms:     []PauliString{Identity(ps.numQubits).Scale(0)},
			numQubits: ps.numQubits,
		}
	}

	return PauliSum{terms: terms, numQubits: ps.numQubits}
}
