// Package noise defines quantum noise channels and noise models for
// use with the density matrix simulator.
//
// A quantum noise channel models the non-unitary evolution of an open
// quantum system interacting with its environment. It is represented by a
// set of Kraus operators {E_k} satisfying the trace-preservation condition
// sum_k(E_k^dag * E_k) = I (completely positive, trace-preserving / CPTP
// map). The density matrix evolves as rho -> sum_k(E_k * rho * E_k^dag).
// Common channels model energy dissipation (amplitude damping / T1 decay),
// phase decoherence (phase damping / T2 dephasing), and random Pauli errors
// (depolarizing noise).
//
// A [Channel] represents a quantum noise channel as a set of Kraus operators.
// A [NoiseModel] maps gate operations to noise channels, with a resolution
// order of qubit-specific > gate-name > qubit-count default.
package noise
