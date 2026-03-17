// Package statevector implements a full statevector quantum simulator
// supporting up to 28 qubits.
//
// The simulator stores the full quantum state as a []complex128 slice of
// length 2^n, where each element is the probability amplitude for the
// corresponding computational basis state. Gates are applied in-place
// via the block-stride pattern: for a gate on qubit k, the 2^n amplitudes
// are partitioned into blocks of size 2^(k+1). Within each block, the
// first 2^k indices have bit k = 0 and the second 2^k have bit k = 1.
// Each index i in the first half pairs with i + 2^k in the second half,
// and the gate's 2x2 unitary is applied to that (amplitude[i], amplitude[i+2^k])
// pair. This pattern generalizes to 2-qubit gates via nested block strides
// over two bit positions, and to N-qubit gates via bitmask-based index
// construction.
//
// For circuits with 17 or more qubits (2^17 = 131,072 amplitudes), gate
// application is automatically parallelized across available cores. Below
// that threshold, goroutine scheduling overhead exceeds the benefit of
// parallelism for the simple multiply-accumulate inner loop, so a single
// goroutine is used.
//
// Entry points:
//
//   - [Sim.Run] resets the state to |0...0>, evolves through the circuit,
//     then samples measurement outcomes via inverse-CDF sampling.
//   - [Sim.Evolve] resets and evolves without measuring, leaving the
//     statevector accessible via [Sim.StateVector] for inspection,
//     expectation values, or further manipulation.
//   - [Sim.StateVector] returns a defensive copy of the current amplitudes.
package statevector
