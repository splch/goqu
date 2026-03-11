package pauli

import (
	"runtime"
	"sync"
)

// Expect computes <psi|P|psi> for a PauliString P and statevector psi.
// The statevector length must be 2^numQubits matching the PauliString.
//
// The algorithm exploits the fact that a Pauli string maps each basis state
// |i> to exactly one other state |j> = |i XOR xMask> with a known phase,
// giving an O(2^n) single-pass computation with O(1) extra space.
func Expect(state []complex128, ps PauliString) complex128 {
	n := len(state)
	if n == 0 {
		return 0
	}

	xm := ps.xMask()
	zm := ps.zMask()
	yPhase := iPow(ps.numY())

	// For 17+ qubits, parallelize.
	nQubits := 0
	for v := n; v > 1; v >>= 1 {
		nQubits++
	}
	if nQubits >= 17 {
		return ps.coeff * expectParallel(state, xm, zm, yPhase, nQubits)
	}

	var sum complex128
	for i, amp := range state {
		j := i ^ xm
		s := sign(i & zm)
		sum += complex(real(state[j]), -imag(state[j])) * complex(s, 0) * yPhase * amp
	}
	return ps.coeff * sum
}

func expectParallel(state []complex128, xm, zm int, yPhase complex128, nQubits int) complex128 {
	n := len(state)
	nWorkers := optimalWorkers(nQubits)
	if nWorkers < 1 {
		nWorkers = 1
	}
	if nWorkers > n {
		nWorkers = n
	}

	partials := make([]complex128, nWorkers)
	chunkSize := n / nWorkers

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	for w := range nWorkers {
		start := w * chunkSize
		end := start + chunkSize
		if w == nWorkers-1 {
			end = n
		}
		go func(w, start, end int) {
			defer wg.Done()
			var s complex128
			for i := start; i < end; i++ {
				j := i ^ xm
				sg := sign(i & zm)
				s += complex(real(state[j]), -imag(state[j])) * complex(sg, 0) * yPhase * state[i]
			}
			partials[w] = s
		}(w, start, end)
	}
	wg.Wait()

	var total complex128
	for _, p := range partials {
		total += p
	}
	return total
}

// ExpectSum computes <psi|H|psi> for a PauliSum H (Hamiltonian).
func ExpectSum(state []complex128, ps PauliSum) complex128 {
	var total complex128
	for _, term := range ps.terms {
		total += Expect(state, term)
	}
	return total
}

func optimalWorkers(nQubits int) int {
	if nQubits <= 16 {
		return 1
	}
	maxProcs := runtime.GOMAXPROCS(0)
	nAmps := 1 << nQubits
	maxByWork := nAmps / 8192
	if maxByWork < 1 {
		maxByWork = 1
	}
	if maxProcs < maxByWork {
		return maxProcs
	}
	return maxByWork
}
