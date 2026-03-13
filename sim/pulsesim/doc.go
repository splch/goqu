// Package pulsesim simulates pulse programs via statevector evolution.
//
// The simulator models single-qubit drives using the rotating-frame
// Hamiltonian H = Omega(t)/2 * (cos(phi)*X + sin(phi)*Y), where Omega
// is the complex waveform envelope and phi is the accumulated frame phase.
// Each waveform sample produces an analytical 2x2 unitary applied via
// the same stride-based kernel used in [github.com/splch/qgo/sim/statevector].
//
// [Sim.Evolve] processes all instructions without measuring.
// [Sim.Run] evolves the state and samples measurement counts.
// [Sim.StateVector] returns the current amplitudes for inspection.
package pulsesim
