// Package statevector implements a full statevector quantum simulator.
package statevector

import (
	"fmt"
	"math"
	"math/bits"
	"math/rand/v2"
	"runtime"
	"sync"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/sim/pauli"
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
	if err := s.Evolve(c); err != nil {
		return nil, err
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
			s.dispatchGate2(op.Gate, op.Qubits[0], op.Qubits[1])
		case 3:
			s.dispatchGate3(op.Gate, op.Qubits[0], op.Qubits[1], op.Qubits[2])
		default:
			if cg, ok := op.Gate.(gate.ControlledGate); ok {
				s.dispatchControlled(cg, op.Qubits)
			} else {
				return fmt.Errorf("unsupported gate size: %d qubits", op.Gate.Qubits())
			}
		}
	}
	return nil
}

// applyGate1 applies a single-qubit gate using the stride pattern.
func (s *Sim) applyGate1(qubit int, m []complex128) {
	// At 17+ qubits the state vector has 131K+ entries; goroutine fan-out
	// overhead is amortized by the per-block work.
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

// ExpectPauliString computes <psi|P|psi> for an arbitrary Pauli string P.
// The result is real for Hermitian observables.
func (s *Sim) ExpectPauliString(ps pauli.PauliString) float64 {
	return real(pauli.Expect(s.state, ps))
}

// ExpectPauliSum computes <psi|H|psi> for a Hamiltonian H (sum of Pauli strings).
func (s *Sim) ExpectPauliSum(ps pauli.PauliSum) float64 {
	return real(pauli.ExpectSum(s.state, ps))
}

// ExpectationValue computes <psi|O|psi> for a diagonal Pauli-Z observable
// specified as a list of qubit indices. For example, [0, 1] computes <Z0 Z1>.
// The result is rounded to 14 decimal places to clean up floating-point noise.
func (s *Sim) ExpectationValue(qubits []int) float64 {
	var mask int
	for _, q := range qubits {
		mask |= 1 << q
	}
	var ev float64
	for i, amp := range s.state {
		prob := real(amp)*real(amp) + imag(amp)*imag(amp)
		if bits.OnesCount(uint(i&mask))%2 == 0 {
			ev += prob
		} else {
			ev -= prob
		}
	}
	return math.Round(ev*1e14) / 1e14
}
