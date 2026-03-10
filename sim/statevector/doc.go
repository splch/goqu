// Package statevector implements a full statevector quantum simulator
// supporting up to 28 qubits.
//
// The simulator stores the state as a []complex128 slice and applies gates
// via stride-based index arithmetic. For 17 or more qubits, gate
// application is automatically parallelized across available cores.
//
// [Sim.Run] evolves the state and samples measurement counts.
// [Sim.Evolve] applies gates without measuring, leaving the statevector
// accessible via [Sim.StateVector] for inspection or expectation values.
package statevector
