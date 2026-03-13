// Package algoutil provides shared helper functions for the algorithm subpackages.
package algoutil

import "strconv"

// TopKey returns the measurement bitstring with the highest count.
// If n > 0, bitstrings are truncated to the last n characters
// (corresponding to the lowest-indexed qubits in the phase register).
func TopKey(counts map[string]int, n int) string {
	best := ""
	bestN := -1
	for k, v := range counts {
		bs := k
		if n > 0 && len(k) > n {
			bs = k[len(k)-n:]
		}
		if v > bestN {
			bestN = v
			best = bs
		}
	}
	return best
}

// BitstringToPhase converts a binary fraction bitstring to a phase in [0, 1).
func BitstringToPhase(bs string, n int) float64 {
	val, _ := strconv.ParseInt(ReverseString(bs), 2, 64)
	return float64(val) / float64(int(1)<<n)
}

// ReverseString reverses a string.
func ReverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// IdentityMap returns a qubit mapping {0:0, 1:1, ..., n-1:n-1}.
func IdentityMap(n int) map[int]int {
	m := make(map[int]int, n)
	for i := range n {
		m[i] = i
	}
	return m
}
