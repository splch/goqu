// Package sim defines the Simulator interface satisfied by all backends
// (CPU statevector, density matrix, GPU, etc.).
package sim

import "github.com/splch/goqu/circuit/ir"

// Simulator is the common interface for quantum circuit simulators.
// Both CPU and GPU implementations satisfy this interface.
type Simulator interface {
	// Run executes the circuit for the given number of shots and returns measurement counts.
	Run(c *ir.Circuit, shots int) (map[string]int, error)

	// Evolve applies all gate operations without measuring, leaving the state accessible.
	Evolve(c *ir.Circuit) error

	// StateVector returns a copy of the current state vector.
	StateVector() []complex128

	// Close releases any resources held by the simulator.
	// For CPU simulators this is a no-op; GPU simulators free device memory.
	Close() error
}
