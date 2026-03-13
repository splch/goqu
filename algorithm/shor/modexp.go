package shor

import (
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
)

// modExpCircuit builds a circuit for |x> -> |(a^power mod N) * x mod N>.
func modExpCircuit(a, power, modulus, nTarget int) *ir.Circuit {
	mult := modPow(a, power, modulus)
	return multiplyModCircuit(mult, modulus, nTarget)
}

// multiplyModCircuit builds a circuit for |x> -> |mult * x mod N>.
// For small N (suitable for simulation), it decomposes the permutation
// into transpositions and implements each using multi-controlled X gates
// via a Gray code path.
func multiplyModCircuit(mult, modulus, nTarget int) *ir.Circuit {
	b := builder.New("ModMul", nTarget)

	if mult%modulus <= 1 {
		return mustBuild(b)
	}

	// Build permutation table.
	dim := 1 << nTarget
	perm := make([]int, dim)
	for x := range dim {
		if x > 0 && x < modulus {
			perm[x] = (mult * x) % modulus
		} else {
			perm[x] = x
		}
	}

	// Decompose permutation into cycles, then transpositions.
	done := make([]bool, dim)
	for start := range dim {
		if done[start] || perm[start] == start {
			done[start] = true
			continue
		}

		// Extract cycle starting at start.
		var cycle []int
		j := start
		for {
			cycle = append(cycle, j)
			done[j] = true
			j = perm[j]
			if j == start {
				break
			}
		}

		// Implement cycle (c0 c1 c2 ... ck) as transpositions:
		// (c0 c1)(c0 c2)...(c0 c_{k-1})
		for i := 1; i < len(cycle); i++ {
			applyTransposition(b, cycle[0], cycle[i], nTarget)
		}
	}

	return mustBuild(b)
}

func mustBuild(b *builder.Builder) *ir.Circuit {
	c, err := b.Build()
	if err != nil {
		panic("modexp: build: " + err.Error())
	}
	return c
}

// applyTransposition swaps computational basis states |s1> and |s2>,
// leaving all other basis states unchanged. For states differing in
// multiple bits, uses a Gray code decomposition into single-bit swaps.
func applyTransposition(b *builder.Builder, s1, s2, n int) {
	if s1 == s2 {
		return
	}
	diff := s1 ^ s2

	// Find all bit positions that differ.
	var bits []int
	for i := range n {
		if diff&(1<<i) != 0 {
			bits = append(bits, i)
		}
	}

	if len(bits) == 1 {
		// Single bit difference: one multi-controlled X suffices.
		applySingleBitSwap(b, s1, bits[0], n)
		return
	}

	// Multi-bit difference: use Gray code path from s1 to s2.
	// Path: s1 -> s1^bit[0] -> s1^bit[0]^bit[1] -> ... -> s2
	//
	// Forward pass: swap along the Gray code path.
	cur := s1
	for _, bit := range bits[:len(bits)-1] {
		next := cur ^ (1 << bit)
		applySingleBitSwap(b, cur, bit, n)
		cur = next
	}
	// Now cur differs from s2 in exactly one bit.
	applySingleBitSwap(b, cur, bits[len(bits)-1], n)

	// Backward pass: undo intermediate swaps.
	// The forward pass moved s1 through intermediate states;
	// we undo those to leave only the (s1, s2) transposition.
	cur = s2
	for i := len(bits) - 2; i >= 0; i-- {
		cur ^= (1 << bits[i])
		applySingleBitSwap(b, cur, bits[i], n)
	}
}

// applySingleBitSwap swaps |state> <-> |state ^ (1<<bit)>.
// Uses multi-controlled X with all bits except the target bit as controls,
// with X-gate setup to match the control pattern of |state>.
func applySingleBitSwap(b *builder.Builder, state, bit, n int) {
	// Setup: X gates to make all control bits 1 for the given state.
	for i := range n {
		if i == bit {
			continue
		}
		if state&(1<<i) == 0 {
			b.X(i)
		}
	}

	// Multi-controlled X: flip the target bit when all controls are 1.
	if n == 1 {
		b.X(bit)
	} else {
		controls := make([]int, 0, n-1)
		for i := range n {
			if i != bit {
				controls = append(controls, i)
			}
		}
		b.MCX(controls, bit)
	}

	// Undo setup.
	for i := range n {
		if i == bit {
			continue
		}
		if state&(1<<i) == 0 {
			b.X(i)
		}
	}
}
