package clifford

import (
	"math/bits"
	"math/rand/v2"
)

// Tableau is the Aaronson-Gottesman stabilizer tableau.
// It has 2n rows: rows 0..n-1 are destabilizers, rows n..2n-1 are stabilizers.
// Each row encodes a Pauli product via n X-bits and n Z-bits plus one phase bit.
type Tableau struct {
	n     int      // number of qubits
	xs    []uint64 // X bits, row-major: xs[row*words + word]
	zs    []uint64 // Z bits, same layout
	rs    []bool   // phase bits: rs[row], true means -1 phase
	words int      // ceil(n/64)
}

func newTableau(n int) *Tableau {
	words := (n + 63) / 64
	rows := 2 * n
	t := &Tableau{
		n:     n,
		xs:    make([]uint64, rows*words),
		zs:    make([]uint64, rows*words),
		rs:    make([]bool, rows),
		words: words,
	}
	// Initial state |0...0>:
	//   destabilizer i has X_i set
	//   stabilizer  i has Z_i set
	for i := range n {
		t.setXBit(i, i, true)   // destabilizer row i: X on qubit i
		t.setZBit(n+i, i, true) // stabilizer row n+i: Z on qubit i
	}
	return t
}

func (t *Tableau) xBit(row, col int) bool {
	w := col / 64
	b := uint(col % 64)
	return t.xs[row*t.words+w]&(1<<b) != 0
}

func (t *Tableau) zBit(row, col int) bool {
	w := col / 64
	b := uint(col % 64)
	return t.zs[row*t.words+w]&(1<<b) != 0
}

func (t *Tableau) setXBit(row, col int, v bool) {
	w := col / 64
	b := uint(col % 64)
	idx := row*t.words + w
	if v {
		t.xs[idx] |= 1 << b
	} else {
		t.xs[idx] &^= 1 << b
	}
}

func (t *Tableau) setZBit(row, col int, v bool) {
	w := col / 64
	b := uint(col % 64)
	idx := row*t.words + w
	if v {
		t.zs[idx] |= 1 << b
	} else {
		t.zs[idx] &^= 1 << b
	}
}

// rowMul multiplies row target by row source in the tableau, updating
// the target's X/Z bits and phase according to the Pauli group product.
// Uses word-level bit operations for O(n/64) performance.
func (t *Tableau) rowMul(target, source int) {
	tOff := target * t.words
	sOff := source * t.words
	phase := rowMulPhase(t.xs[tOff:tOff+t.words], t.zs[tOff:tOff+t.words],
		t.xs[sOff:sOff+t.words], t.zs[sOff:sOff+t.words],
		t.rs[target], t.rs[source])
	t.rs[target] = phase

	// XOR the X and Z bits
	for w := range t.words {
		t.xs[tOff+w] ^= t.xs[sOff+w]
		t.zs[tOff+w] ^= t.zs[sOff+w]
	}
}

// rowMulPhase computes the phase bit for the product of two Pauli rows
// using word-level bit operations. This is the performance-critical inner loop.
//
// For each qubit j, the phase contribution g(x1,z1,x2,z2) depends on the
// Pauli types. We compute the total phase mod 4 using popcount tricks:
//
//	total = 2*r1 + 2*r2 + Σ_j g(x1_j,z1_j,x2_j,z2_j)
//
// The function g can be decomposed: g counts +1 for each position where
// the left Pauli "advances" (X→Y→Z→X cycle) and -1 (i.e., +3 mod 4)
// where it "retreats". Using bit logic:
//
//	advance  = (x1 & z2 & ~x2 & ~z1) | (z1 & x2 & ~z2 & ~x1) | (x1 & z1 & x2 & z2 & ... )
//
// We use the simplified formula: count positions where the product gives
// +i and -i contributions. Sum of g mod 4 can be computed as:
//
//	sum = popcount(+1 positions) - popcount(-1 positions)  (mod 4)
func rowMulPhase(xt, zt, xs, zs []uint64, rt, rs bool) bool {
	// We accumulate sum of g values mod 4.
	// g(x1,z1,x2,z2) for non-identity left Pauli:
	//   X*Z=+1, X*Y=-1, Y*X=+1, Y*Z=-1, Z*Y=+1, Z*X=-1
	// Expressed in bits: +1 when (x1,z1,x2,z2) gives an "advance" in the Pauli cycle
	//
	// Using the Aaronson-Gottesman technique:
	// For each word, compute the number of +1 and +3 contributions.
	//
	// +1 positions: X*Z, Y*X, Z*Y → where left "advances" to right in XYZ cycle
	// -1 positions: X*Y, Y*Z, Z*X → where left "retreats"
	//
	// X*Z (+1): x1 & !z1 & !x2 & z2
	// Y*X (+1): x1 & z1 & x2 & !z2
	// Z*Y (+1): !x1 & z1 & x2 & z2
	// X*Y (-1): x1 & !z1 & x2 & z2
	// Y*Z (-1): x1 & z1 & !x2 & z2
	// Z*X (-1): !x1 & z1 & x2 & !z2
	var accum int
	for w := range xt {
		x1, z1, x2, z2 := xt[w], zt[w], xs[w], zs[w]
		nx1, nz1, nx2, nz2 := ^x1, ^z1, ^x2, ^z2

		pos := (x1 & nz1 & nx2 & z2) | // X*Z
			(x1 & z1 & x2 & nz2) | // Y*X
			(nx1 & z1 & x2 & z2) // Z*Y

		neg := (x1 & nz1 & x2 & z2) | // X*Y
			(x1 & z1 & nx2 & z2) | // Y*Z
			(nx1 & z1 & x2 & nz2) // Z*X

		accum += bits.OnesCount64(pos) - bits.OnesCount64(neg)
	}
	phase := accum % 4
	if phase < 0 {
		phase += 4
	}
	if rt {
		phase += 2
	}
	if rs {
		phase += 2
	}
	return (phase % 4) >= 2
}

// Gate operations (all O(n) per gate)

// H applies a Hadamard gate on qubit q.
// For each row: swap x,z at col q; then r ^= (x & z) where x is the new x bit.
func (t *Tableau) H(q int) {
	w := q / 64
	b := uint(q % 64)
	mask := uint64(1) << b
	rows := 2 * t.n
	for i := range rows {
		idx := i*t.words + w
		xv := t.xs[idx] & mask
		zv := t.zs[idx] & mask
		// phase update: r ^= (old_x & old_z)
		if xv != 0 && zv != 0 {
			t.rs[i] = !t.rs[i]
		}
		// swap x and z
		t.xs[idx] = (t.xs[idx] &^ mask) | zv
		t.zs[idx] = (t.zs[idx] &^ mask) | xv
	}
}

// S applies an S gate (phase gate) on qubit q.
// For each row: r ^= (x & z); z ^= x.
func (t *Tableau) S(q int) {
	w := q / 64
	b := uint(q % 64)
	mask := uint64(1) << b
	rows := 2 * t.n
	for i := range rows {
		idx := i*t.words + w
		xv := t.xs[idx] & mask
		zv := t.zs[idx] & mask
		if xv != 0 && zv != 0 {
			t.rs[i] = !t.rs[i]
		}
		// z ^= x
		t.zs[idx] ^= xv
	}
}

// CNOT applies a CNOT gate with control c and target tgt.
// For each row i:
//
//	r_i ^= x_{i,c} & z_{i,tgt} & (x_{i,tgt} XOR z_{i,c} XOR 1)
//	x_{i,tgt} ^= x_{i,c}
//	z_{i,c}   ^= z_{i,tgt}
func (t *Tableau) CNOT(c, tgt int) {
	cw := c / 64
	cb := uint(c % 64)
	cmask := uint64(1) << cb
	tw := tgt / 64
	tb := uint(tgt % 64)
	tmask := uint64(1) << tb
	rows := 2 * t.n
	for i := range rows {
		cidx := i*t.words + cw
		tidx := i*t.words + tw
		xc := t.xs[cidx] & cmask
		zt := t.zs[tidx] & tmask
		xt := t.xs[tidx] & tmask
		zc := t.zs[cidx] & cmask

		// Phase update
		xcBool := xc != 0
		ztBool := zt != 0
		xtBool := xt != 0
		zcBool := zc != 0
		if xcBool && ztBool && (xtBool == zcBool) {
			// x_c & z_t & NOT(x_t XOR z_c) = x_c & z_t & (x_t XNOR z_c)
			t.rs[i] = !t.rs[i]
		}

		// x_{tgt} ^= x_{c}
		if xc != 0 {
			t.xs[tidx] ^= tmask
		}
		// z_{c} ^= z_{tgt}
		if zt != 0 {
			t.zs[cidx] ^= cmask
		}
	}
}

// X applies a Pauli-X gate on qubit q.
// For each row: r ^= z at col q.
func (t *Tableau) X(q int) {
	w := q / 64
	b := uint(q % 64)
	mask := uint64(1) << b
	rows := 2 * t.n
	for i := range rows {
		if t.zs[i*t.words+w]&mask != 0 {
			t.rs[i] = !t.rs[i]
		}
	}
}

// Y applies a Pauli-Y gate on qubit q.
// For each row: r ^= (x XOR z) at col q.
func (t *Tableau) Y(q int) {
	w := q / 64
	b := uint(q % 64)
	mask := uint64(1) << b
	rows := 2 * t.n
	for i := range rows {
		idx := i*t.words + w
		xv := t.xs[idx] & mask
		zv := t.zs[idx] & mask
		if (xv != 0) != (zv != 0) {
			t.rs[i] = !t.rs[i]
		}
	}
}

// Z applies a Pauli-Z gate on qubit q.
// For each row: r ^= x at col q.
func (t *Tableau) Z(q int) {
	w := q / 64
	b := uint(q % 64)
	mask := uint64(1) << b
	rows := 2 * t.n
	for i := range rows {
		if t.xs[i*t.words+w]&mask != 0 {
			t.rs[i] = !t.rs[i]
		}
	}
}

// CZ applies a controlled-Z gate on qubits q0 and q1.
func (t *Tableau) CZ(q0, q1 int) {
	t.H(q1)
	t.CNOT(q0, q1)
	t.H(q1)
}

// SWAP swaps qubits q0 and q1.
func (t *Tableau) SWAP(q0, q1 int) {
	t.CNOT(q0, q1)
	t.CNOT(q1, q0)
	t.CNOT(q0, q1)
}

// CY applies a controlled-Y gate on qubits q0 (control) and q1 (target).
// CY = S†(q1) · CNOT(q0,q1) · S(q1).
func (t *Tableau) CY(q0, q1 int) {
	// S†(q1) = S^3(q1)
	t.S(q1)
	t.S(q1)
	t.S(q1)
	t.CNOT(q0, q1)
	t.S(q1)
}

// SX applies a sqrt(X) gate on qubit q.
// SX = H·S·H up to global phase (which doesn't matter for stabilizer formalism).
func (t *Tableau) SX(q int) {
	t.H(q)
	t.S(q)
	t.H(q)
}

// Measure performs a projective measurement on qubit q,
// returning 0 or 1. Uses rng for random outcomes.
func (t *Tableau) Measure(q int, rng *rand.Rand) int {
	n := t.n

	// Step 1: Look for any stabilizer row p (n <= p < 2n) with x bit set at column q.
	p := -1
	for row := n; row < 2*n; row++ {
		if t.xBit(row, q) {
			p = row
			break
		}
	}

	if p >= 0 {
		// Random outcome.
		// For all rows i != p with x bit set at q: rowMul(i, p).
		for i := range 2 * n {
			if i != p && t.xBit(i, q) {
				t.rowMul(i, p)
			}
		}
		// Copy stabilizer row p to destabilizer row p-n.
		dest := p - n
		dOff := dest * t.words
		pOff := p * t.words
		copy(t.xs[dOff:dOff+t.words], t.xs[pOff:pOff+t.words])
		copy(t.zs[dOff:dOff+t.words], t.zs[pOff:pOff+t.words])
		t.rs[dest] = t.rs[p]

		// Set row p to all zeros except z bit at q.
		for w := range t.words {
			t.xs[pOff+w] = 0
			t.zs[pOff+w] = 0
		}
		outcome := rng.IntN(2)
		t.rs[p] = outcome == 1
		t.setZBit(p, q, true)
		return outcome
	}

	// Step 2: Deterministic outcome.
	// For each destabilizer row i (0..n-1): if x bit at col q is set,
	// multiply scratch by the corresponding stabilizer row i+n.
	scratchXs := make([]uint64, t.words)
	scratchZs := make([]uint64, t.words)
	scratchR := false

	for i := range n {
		if !t.xBit(i, q) { // check destabilizer row i
			continue
		}
		// Multiply scratch by stabilizer row i+n using word-level phase computation.
		src := i + n
		sOff := src * t.words
		scratchR = rowMulPhase(scratchXs, scratchZs,
			t.xs[sOff:sOff+t.words], t.zs[sOff:sOff+t.words],
			scratchR, t.rs[src])

		// XOR bits
		for w := range t.words {
			scratchXs[w] ^= t.xs[sOff+w]
			scratchZs[w] ^= t.zs[sOff+w]
		}
	}

	if scratchR {
		return 1
	}
	return 0
}
