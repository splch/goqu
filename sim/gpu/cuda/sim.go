// Package cuda provides a GPU-accelerated statevector simulator using NVIDIA cuStateVec.
//
// The simulator satisfies [sim.Simulator] and can be used as a drop-in replacement
// for the CPU statevector simulator via [backend/local.WithSimulator]:
//
//	import gpucuda "github.com/splch/goqu/sim/gpu/cuda"
//	local.New(local.WithSimulator(gpucuda.NewSimulator))
//
// Build requires the cuda build tag and a working cuStateVec installation.
// Without the tag, stub implementations return an error.
package cuda

import (
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim"
)

var _ sim.Simulator = (*Sim)(nil)

// Sim is a GPU-accelerated statevector simulator backed by cuStateVec.
// The state vector lives on the GPU for its entire lifetime; only gate matrices
// (16-128 bytes each) are transferred per operation.
type Sim struct {
	numQubits int
	handle    cusvHandle  // cuStateVec handle + CUDA stream
	devicePtr deviceAlloc // GPU memory for the state vector
}

// NewSimulator is a [sim.SimFactory] that creates a GPU simulator.
// It can be passed directly to [backend/local.WithSimulator].
func NewSimulator(numQubits int) (sim.Simulator, error) {
	return New(numQubits)
}

// New creates a GPU simulator initialized to |0...0>.
// Returns an error if CUDA or cuStateVec initialization fails.
func New(numQubits int) (*Sim, error) {
	return newSim(numQubits)
}

// Run executes the circuit for the given number of shots and returns measurement counts.
func (s *Sim) Run(c *ir.Circuit, shots int) (map[string]int, error) {
	return run(s, c, shots)
}

// Evolve applies all gate operations without measuring.
func (s *Sim) Evolve(c *ir.Circuit) error {
	return evolve(s, c)
}

// StateVector copies the state vector from GPU to host and returns it.
func (s *Sim) StateVector() []complex128 {
	return stateVector(s)
}

// Close releases the cuStateVec handle and frees GPU memory.
func (s *Sim) Close() error {
	return closeSim(s)
}
