package pulsesim

// Coupling describes the interaction between a pair of qubits.
type Coupling struct {
	ZZ float64 // static ZZ coupling strength in rad/s
}

// CouplingMap maps ordered qubit pairs to their coupling parameters.
// Use [orderedPair] to normalize pair ordering.
type CouplingMap map[[2]int]Coupling

// CRFrameMap associates cross-resonance frame names with [control, target]
// qubit pairs. When a Play instruction uses a CR frame, the simulator
// applies a 2Q cross-resonance Hamiltonian instead of a 1Q drive.
type CRFrameMap map[string][2]int

// Option configures a pulse simulator.
type Option func(*Sim)

// WithCoupling sets the static qubit-qubit coupling map.
func WithCoupling(cm CouplingMap) Option {
	return func(s *Sim) { s.coupling = cm }
}

// WithCRFrames declares which frames are cross-resonance drives.
func WithCRFrames(cr CRFrameMap) Option {
	return func(s *Sim) { s.crFrames = cr }
}

// orderedPair returns a normalized [2]int with the smaller index first.
func orderedPair(q0, q1 int) [2]int {
	if q0 <= q1 {
		return [2]int{q0, q1}
	}
	return [2]int{q1, q0}
}
