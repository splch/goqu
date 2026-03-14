// Package metal provides a GPU-accelerated statevector simulator using Apple Metal compute shaders.
//
// The simulator satisfies [sim.Simulator] and can be used as a drop-in replacement
// for the CPU statevector simulator via [backend/local.WithSimulator]:
//
//	import gpumetal "github.com/splch/goqu/sim/gpu/metal"
//	local.New(local.WithSimulator(gpumetal.NewSimulator))
//
// Build requires the metal build tag and macOS/Apple Silicon.
// Without the tag, stub implementations return an error.
package metal

import (
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim"
)

var _ sim.Simulator = (*Sim)(nil)

// Sim is a GPU-accelerated statevector simulator backed by Apple Metal compute shaders.
// On Apple Silicon, unified memory eliminates explicit CPU-GPU transfers.
type Sim struct {
	numQubits int
	device    metalDevice
}

// NewSimulator is a [sim.SimFactory] that creates a Metal GPU simulator.
func NewSimulator(numQubits int) (sim.Simulator, error) {
	return New(numQubits)
}

// New creates a Metal GPU simulator initialized to |0...0>.
func New(numQubits int) (*Sim, error) {
	return newSim(numQubits)
}

// Run executes the circuit and returns measurement counts.
func (s *Sim) Run(c *ir.Circuit, shots int) (map[string]int, error) {
	return run(s, c, shots)
}

// Evolve applies all gate operations without measuring.
func (s *Sim) Evolve(c *ir.Circuit) error {
	return evolve(s, c)
}

// StateVector returns a copy of the current state vector.
func (s *Sim) StateVector() []complex128 {
	return stateVector(s)
}

// Close releases Metal device resources.
func (s *Sim) Close() error {
	return closeSim(s)
}
