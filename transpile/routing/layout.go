// Package routing implements qubit routing algorithms for constrained connectivity.
package routing

import "math/rand/v2"

// TrivialLayout returns the identity mapping: logical qubit i → physical qubit i.
func TrivialLayout(n int) []int {
	layout := make([]int, n)
	for i := range n {
		layout[i] = i
	}
	return layout
}

// RandomLayout selects n distinct physical qubits from [0, numPhys) at random
// and returns a mapping from logical qubit i to its randomly chosen physical qubit.
// If numPhys <= n it falls back to a permutation of [0, n).
func RandomLayout(n, numPhys int, rng *rand.Rand) []int {
	if numPhys <= n {
		layout := TrivialLayout(n)
		for i := n - 1; i > 0; i-- {
			j := rng.IntN(i + 1)
			layout[i], layout[j] = layout[j], layout[i]
		}
		return layout
	}
	// Fisher-Yates partial shuffle: pick n items from [0, numPhys).
	pool := make([]int, numPhys)
	for i := range pool {
		pool[i] = i
	}
	for i := range n {
		j := i + rng.IntN(numPhys-i)
		pool[i], pool[j] = pool[j], pool[i]
	}
	layout := make([]int, n)
	copy(layout, pool)
	return layout
}

// InverseLayout returns the inverse of a layout mapping.
// If layout[logical] = physical, then inverse[physical] = logical.
// numPhys sets the length of the returned slice (for devices with more physical
// qubits than logical); unoccupied slots are set to -1.
func InverseLayout(layout []int, numPhys int) []int {
	n := max(numPhys, len(layout))
	inv := make([]int, n)
	for i := range inv {
		inv[i] = -1
	}
	for log, phys := range layout {
		if phys >= 0 && phys < n {
			inv[phys] = log
		}
	}
	return inv
}

// copyLayout returns a copy of the layout slice.
func copyLayout(layout []int) []int {
	out := make([]int, len(layout))
	copy(out, layout)
	return out
}
