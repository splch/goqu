// Package statevector implements a full statevector quantum simulator.
package statevector

import (
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"sync"

	"github.com/splch/qgo/circuit/ir"
)

// Sim simulates a circuit via full statevector evolution.
type Sim struct {
	numQubits int
	state     []complex128
}

// New creates a simulator initialized to |0...0>.
func New(numQubits int) *Sim {
	if numQubits < 1 || numQubits > 28 {
		panic(fmt.Sprintf("statevector: numQubits %d out of range [1, 28]", numQubits))
	}
	n := 1 << numQubits
	state := make([]complex128, n)
	state[0] = 1
	return &Sim{numQubits: numQubits, state: state}
}

// Run executes the circuit and returns measurement counts.
func (s *Sim) Run(c *ir.Circuit, shots int) (map[string]int, error) {
	if c.NumQubits() != s.numQubits {
		return nil, fmt.Errorf("circuit has %d qubits, simulator has %d", c.NumQubits(), s.numQubits)
	}

	// Reset to |0...0>.
	for i := range s.state {
		s.state[i] = 0
	}
	s.state[0] = 1

	// Apply all gate operations (skip measurements in the evolution phase).
	for _, op := range c.Ops() {
		if op.Gate == nil || op.Gate.Name() == "barrier" {
			continue
		}
		switch op.Gate.Qubits() {
		case 1:
			s.applyGate1(op.Qubits[0], op.Gate.Matrix())
		case 2:
			s.applyGate2(op.Qubits[0], op.Qubits[1], op.Gate.Matrix())
		case 3:
			s.applyGate3(op.Qubits[0], op.Qubits[1], op.Qubits[2], op.Gate.Matrix())
		default:
			return nil, fmt.Errorf("unsupported gate size: %d qubits", op.Gate.Qubits())
		}
	}

	// Sample measurement results.
	probs := s.probabilities()
	counts := make(map[string]int)
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	for range shots {
		idx := sampleIndex(probs, rng)
		bs := formatBitstring(idx, s.numQubits)
		counts[bs]++
	}
	return counts, nil
}

// StateVector returns a copy of the current statevector.
func (s *Sim) StateVector() []complex128 {
	out := make([]complex128, len(s.state))
	copy(out, s.state)
	return out
}

// Evolve applies all gate operations without measuring, leaving the statevector accessible.
func (s *Sim) Evolve(c *ir.Circuit) error {
	if c.NumQubits() != s.numQubits {
		return fmt.Errorf("circuit has %d qubits, simulator has %d", c.NumQubits(), s.numQubits)
	}
	for i := range s.state {
		s.state[i] = 0
	}
	s.state[0] = 1
	for _, op := range c.Ops() {
		if op.Gate == nil || op.Gate.Name() == "barrier" {
			continue
		}
		switch op.Gate.Qubits() {
		case 1:
			s.applyGate1(op.Qubits[0], op.Gate.Matrix())
		case 2:
			s.applyGate2(op.Qubits[0], op.Qubits[1], op.Gate.Matrix())
		case 3:
			s.applyGate3(op.Qubits[0], op.Qubits[1], op.Qubits[2], op.Gate.Matrix())
		default:
			return fmt.Errorf("unsupported gate size: %d qubits", op.Gate.Qubits())
		}
	}
	return nil
}

// applyGate1 applies a single-qubit gate using the stride pattern.
func (s *Sim) applyGate1(qubit int, m []complex128) {
	if s.numQubits >= 17 {
		s.applyGate1Parallel(qubit, m)
		return
	}
	halfBlock := 1 << qubit
	block := halfBlock << 1
	n := len(s.state)
	for b0 := 0; b0 < n; b0 += block {
		for offset := range halfBlock {
			i0 := b0 + offset
			i1 := i0 + halfBlock
			a0, a1 := s.state[i0], s.state[i1]
			s.state[i0] = m[0]*a0 + m[1]*a1
			s.state[i1] = m[2]*a0 + m[3]*a1
		}
	}
}

// applyGate1Parallel is the parallel version for large statevectors.
func (s *Sim) applyGate1Parallel(qubit int, m []complex128) {
	halfBlock := 1 << qubit
	block := halfBlock << 1
	n := len(s.state)
	nBlocks := n / block

	nWorkers := optimalWorkers(s.numQubits)
	if nBlocks < nWorkers {
		nWorkers = nBlocks
	}
	if nWorkers < 1 {
		nWorkers = 1
	}

	var wg sync.WaitGroup
	wg.Add(nWorkers)
	blocksPerWorker := nBlocks / nWorkers

	for w := range nWorkers {
		startBlock := w * blocksPerWorker
		endBlock := startBlock + blocksPerWorker
		if w == nWorkers-1 {
			endBlock = nBlocks
		}
		go func(start, end int) {
			defer wg.Done()
			for b := start; b < end; b++ {
				b0 := b * block
				for offset := range halfBlock {
					i0 := b0 + offset
					i1 := i0 + halfBlock
					a0, a1 := s.state[i0], s.state[i1]
					s.state[i0] = m[0]*a0 + m[1]*a1
					s.state[i1] = m[2]*a0 + m[3]*a1
				}
			}
		}(startBlock, endBlock)
	}
	wg.Wait()
}

// applyGate2 applies a two-qubit gate.
// Matrix convention: row/col index bit 1 (MSB) = q0, bit 0 (LSB) = q1.
func (s *Sim) applyGate2(q0, q1 int, m []complex128) {
	mask0, mask1 := 1<<q0, 1<<q1
	n := len(s.state)
	for i := range n {
		if i&mask0 != 0 || i&mask1 != 0 {
			continue
		}
		// Map matrix rows to state indices: row bit1=q0, bit0=q1.
		idx := [4]int{
			i,              // |00>: q0=0, q1=0
			i | mask1,      // |01>: q0=0, q1=1
			i | mask0,      // |10>: q0=1, q1=0
			i | mask0 | mask1, // |11>: q0=1, q1=1
		}
		a := [4]complex128{s.state[idx[0]], s.state[idx[1]], s.state[idx[2]], s.state[idx[3]]}
		s.state[idx[0]] = m[0]*a[0] + m[1]*a[1] + m[2]*a[2] + m[3]*a[3]
		s.state[idx[1]] = m[4]*a[0] + m[5]*a[1] + m[6]*a[2] + m[7]*a[3]
		s.state[idx[2]] = m[8]*a[0] + m[9]*a[1] + m[10]*a[2] + m[11]*a[3]
		s.state[idx[3]] = m[12]*a[0] + m[13]*a[1] + m[14]*a[2] + m[15]*a[3]
	}
}

// applyGate3 applies a three-qubit gate.
// Matrix convention: row/col bit 2 (MSB) = q0, bit 1 = q1, bit 0 (LSB) = q2.
func (s *Sim) applyGate3(q0, q1, q2 int, m []complex128) {
	mask0, mask1, mask2 := 1<<q0, 1<<q1, 1<<q2
	n := len(s.state)
	for i := range n {
		if i&mask0 != 0 || i&mask1 != 0 || i&mask2 != 0 {
			continue
		}
		// Map matrix rows: bit2=q0, bit1=q1, bit0=q2.
		var indices [8]int
		for r := range 8 {
			idx := i
			if r&4 != 0 {
				idx |= mask0
			}
			if r&2 != 0 {
				idx |= mask1
			}
			if r&1 != 0 {
				idx |= mask2
			}
			indices[r] = idx
		}
		var a [8]complex128
		for j := range 8 {
			a[j] = s.state[indices[j]]
		}
		for r := range 8 {
			var sum complex128
			for c := range 8 {
				sum += m[r*8+c] * a[c]
			}
			s.state[indices[r]] = sum
		}
	}
}

func (s *Sim) probabilities() []float64 {
	probs := make([]float64, len(s.state))
	for i, amp := range s.state {
		probs[i] = real(amp)*real(amp) + imag(amp)*imag(amp)
	}
	return probs
}

func sampleIndex(probs []float64, rng *rand.Rand) int {
	r := rng.Float64()
	cum := 0.0
	for i, p := range probs {
		cum += p
		if r < cum {
			return i
		}
	}
	return len(probs) - 1
}

func formatBitstring(idx, n int) string {
	bs := make([]byte, n)
	for i := range n {
		if idx&(1<<i) != 0 {
			bs[n-1-i] = '1'
		} else {
			bs[n-1-i] = '0'
		}
	}
	return string(bs)
}

func optimalWorkers(nQubits int) int {
	if nQubits <= 16 {
		return 1
	}
	maxProcs := runtime.GOMAXPROCS(0)
	nAmps := 1 << nQubits
	maxByWork := nAmps / 8192
	if maxByWork < 1 {
		maxByWork = 1
	}
	if maxProcs < maxByWork {
		return maxProcs
	}
	return maxByWork
}

// ExpectationValue computes <psi|O|psi> for a diagonal Pauli-Z observable
// specified as a list of qubit indices. For example, [0, 1] computes <Z0 Z1>.
func (s *Sim) ExpectationValue(qubits []int) float64 {
	var ev float64
	for i, amp := range s.state {
		prob := real(amp)*real(amp) + imag(amp)*imag(amp)
		// Count parity of measured qubits
		parity := 0
		for _, q := range qubits {
			if i&(1<<q) != 0 {
				parity++
			}
		}
		if parity%2 == 0 {
			ev += prob
		} else {
			ev -= prob
		}
	}
	return math.Round(ev*1e14) / 1e14
}
