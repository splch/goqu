package noise

import "strconv"

// Channel represents a quantum noise channel via Kraus operators.
// Each Kraus operator is a flat row-major (2^n)x(2^n) matrix.
// The channel satisfies sum_k(E_k^dag * E_k) = I, which guarantees that
// the map is completely positive and trace-preserving (CPTP).
type Channel interface {
	Name() string
	Qubits() int
	Kraus() [][]complex128
}

// NoiseModel maps quantum operations to noise channels.
// Resolution order: qubit-specific > gate-name > qubit-count default.
type NoiseModel struct {
	// byGateQubits maps "gatename:q0,q1" to a channel
	byGateQubits map[string]Channel
	// byGate maps gate name to a channel
	byGate map[string]Channel
	// byQubits maps qubit count to a default channel
	byQubits map[int]Channel
	// readout maps qubit index to ReadoutError
	readout map[int]*ReadoutError
}

// New creates an empty NoiseModel.
func New() *NoiseModel {
	return &NoiseModel{
		byGateQubits: make(map[string]Channel),
		byGate:       make(map[string]Channel),
		byQubits:     make(map[int]Channel),
		readout:      make(map[int]*ReadoutError),
	}
}

// AddGateError adds a noise channel for a specific gate name.
func (m *NoiseModel) AddGateError(gateName string, ch Channel) {
	m.byGate[gateName] = ch
}

// AddGateQubitError adds a noise channel for a specific gate on specific qubits.
// The key is formatted as "gatename:q0,q1,...".
func (m *NoiseModel) AddGateQubitError(gateName string, qubits []int, ch Channel) {
	key := formatKey(gateName, qubits)
	m.byGateQubits[key] = ch
}

// AddDefaultError adds a default noise channel for all gates of given qubit count.
func (m *NoiseModel) AddDefaultError(nQubits int, ch Channel) {
	m.byQubits[nQubits] = ch
}

// AddReadoutError adds a readout error for a specific qubit.
func (m *NoiseModel) AddReadoutError(qubit int, re *ReadoutError) {
	m.readout[qubit] = re
}

// Lookup returns the noise channel for a given gate and qubits.
// Returns nil if no matching channel is found.
func (m *NoiseModel) Lookup(gateName string, qubits []int) Channel {
	// Most specific first
	if ch, ok := m.byGateQubits[formatKey(gateName, qubits)]; ok {
		return ch
	}
	if ch, ok := m.byGate[gateName]; ok {
		return ch
	}
	if ch, ok := m.byQubits[len(qubits)]; ok {
		return ch
	}
	return nil
}

// ReadoutFor returns the readout error for a qubit, or nil.
func (m *NoiseModel) ReadoutFor(qubit int) *ReadoutError {
	return m.readout[qubit]
}

func formatKey(gateName string, qubits []int) string {
	key := gateName + ":"
	for i, q := range qubits {
		if i > 0 {
			key += ","
		}
		key += strconv.Itoa(q)
	}
	return key
}
