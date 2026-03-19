// Package statevector implements a full statevector quantum simulator.
package statevector

import (
	"fmt"
	"math"
	"math/bits"
	"math/rand/v2"
	"runtime"
	"sync"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim"
	"github.com/splch/goqu/sim/pauli"
)

var _ sim.Simulator = (*Sim)(nil)

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
// For dynamic circuits (mid-circuit measurement, feed-forward, reset), it
// automatically uses per-shot simulation with state collapse.
func (s *Sim) Run(c *ir.Circuit, shots int) (map[string]int, error) {
	if c.IsDynamic() {
		return s.RunDynamic(c, shots)
	}
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

// Close is a no-op for the CPU statevector simulator. It satisfies sim.Simulator.
func (s *Sim) Close() error { return nil }

// Reset returns the simulator state to |0...0>.
func (s *Sim) Reset() {
	for i := range s.state {
		s.state[i] = 0
	}
	s.state[0] = 1
}

// Evolve resets the state to |0...0> and then applies the circuit's gate
// operations without measuring. The resulting statevector is accessible via
// [Sim.StateVector]. To apply gates without resetting (e.g. to compose
// circuits incrementally), use [Sim.Apply] instead.
func (s *Sim) Evolve(c *ir.Circuit) error {
	s.Reset()
	return s.Apply(c)
}

// Apply applies the circuit's gate operations to the current state without
// resetting. Use this to compose circuits incrementally:
//
//	sim.Evolve(circuitA) // resets to |0>, then applies A
//	sim.Apply(circuitB)  // applies B to the state left by A
func (s *Sim) Apply(c *ir.Circuit) error {
	if c.NumQubits() != s.numQubits {
		return fmt.Errorf("circuit has %d qubits, simulator has %d", c.NumQubits(), s.numQubits)
	}
	for i := range c.NumOps() {
		op := c.Op(i)
		if op.Gate == nil || op.Gate.Name() == "barrier" || op.Gate.Name() == "delay" {
			continue
		}
		if op.Gate.Name() == "reset" {
			s.resetQubit(op.Qubits[0])
			continue
		}
		if sp, ok := op.Gate.(gate.StatePrepable); ok {
			amps := sp.Amplitudes()
			// Fast path: full-state preparation on qubits [0..n-1].
			if len(op.Qubits) == s.numQubits {
				allInOrder := true
				for j, q := range op.Qubits {
					if q != j {
						allInOrder = false
						break
					}
				}
				if allInOrder {
					copy(s.state, amps)
					continue
				}
			}
			// Slow path: decompose into 1Q/2Q gates and apply.
			applied := op.Gate.Decompose(op.Qubits)
			for _, a := range applied {
				m := a.Gate.Matrix()
				if m == nil {
					continue
				}
				switch a.Gate.Qubits() {
				case 1:
					s.applyGate1(a.Qubits[0], m)
				case 2:
					s.dispatchGate2(a.Gate, a.Qubits[0], a.Qubits[1])
				}
			}
			continue
		}
		switch op.Gate.Qubits() {
		case 1:
			m := op.Gate.Matrix()
			if m == nil {
				return fmt.Errorf("gate %q has no matrix representation", op.Gate.Name())
			}
			s.applyGate1(op.Qubits[0], m)
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

// resetQubit deterministically resets a qubit to |0⟩ via partial-trace
// collapse. For each paired index (i0, i1) where i0 has the target qubit's
// bit = 0 and i1 has it = 1, the local probability is p = |a0|^2 + |a1|^2.
// The reset concentrates all of that probability into the |0⟩ component:
// a0 becomes sqrt(p) (real, positive) and a1 becomes 0. This is the
// deterministic (non-measurement) version of reset, equivalent to tracing
// out the target qubit and replacing it with |0⟩. Phase information in the
// |1⟩ subspace is discarded, which is physically correct because reset is
// a non-unitary operation.
func (s *Sim) resetQubit(qubit int) {
	halfBlock := 1 << qubit
	block := halfBlock << 1
	for b0 := 0; b0 < len(s.state); b0 += block {
		for offset := range halfBlock {
			i0 := b0 + offset    // qubit = 0
			i1 := i0 + halfBlock // qubit = 1
			a0, a1 := s.state[i0], s.state[i1]
			norm := math.Sqrt(real(a0)*real(a0) + imag(a0)*imag(a0) +
				real(a1)*real(a1) + imag(a1)*imag(a1))
			if norm > 1e-15 {
				s.state[i0] = complex(norm, 0)
			} else {
				s.state[i0] = 0
			}
			s.state[i1] = 0
		}
	}
}

// applyGate1 applies a single-qubit 2x2 unitary to qubit k using the
// block-stride pattern.
//
// The statevector has 2^n amplitudes, each indexed by an n-bit string.
// Qubit k corresponds to bit position k in that index. To apply a 2x2
// matrix U, we pair every index i where bit k = 0 with j = i | (1<<k)
// where bit k = 1, then compute:
//
//	state[i], state[j] = U[0,0]*state[i] + U[0,1]*state[j],
//	                      U[1,0]*state[i] + U[1,1]*state[j]
//
// The block-stride loop makes this cache-friendly. halfBlock = 2^k is the
// distance between paired indices. We iterate in blocks of 2^(k+1):
//
//	block = 2^(k+1)
//	|<--- halfBlock --->|<--- halfBlock --->|
//	| bit k = 0 (i)     | bit k = 1 (j)     |
//	|  offset 0..2^k-1  |  offset 0..2^k-1  |
//	     i = b0+offset     j = i + halfBlock
//
// Within each block, the first half has bit k = 0 and the second half has
// bit k = 1. Consecutive indices within each half differ only in bits
// below k, so the paired accesses are stride-2^k apart.
func (s *Sim) applyGate1(qubit int, m []complex128) {
	// At 17+ qubits the statevector has 131K+ entries; goroutine fan-out
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

// sampleIndex draws one sample from the discrete probability distribution
// using inverse-CDF (cumulative distribution function) sampling. A uniform
// random number r in [0,1) is drawn, then we scan the probability array
// accumulating a running sum. The first index where the cumulative sum
// exceeds r is returned. The final fallback to len(probs)-1 handles
// floating-point rounding where cumulative probabilities don't quite reach 1.
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

// formatBitstring converts a basis-state index to its n-bit string
// representation. The bit ordering convention follows quantum computing
// standard: qubit 0 is the least-significant bit (LSB, rightmost position),
// and qubit n-1 is the most-significant bit (MSB, leftmost position).
// For example, with n=3 and idx=5 (binary 101), qubit 0 = 1, qubit 1 = 0,
// qubit 2 = 1, producing the string "101".
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

// optimalWorkers returns the number of goroutines to use for parallel gate
// application. Below 17 qubits (2^17 = 131,072 amplitudes), the function
// returns 1 because goroutine creation, scheduling, and WaitGroup
// synchronization overhead exceeds the benefit of parallelism for the
// simple multiply-accumulate inner loop (each iteration is ~4 complex
// multiplies and adds). Above 17 qubits, workers are capped at both
// GOMAXPROCS and nAmps/8192 to ensure each goroutine processes at least
// 8192 amplitudes, keeping the work-per-goroutine large enough to
// amortize scheduling cost.
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

// ExpectPauliString computes Re(⟨psi|P|psi⟩) for a Pauli string P.
// For Hermitian observables (real coefficients), the imaginary part is zero.
// For non-Hermitian observables, use pauli.Expect directly for complex128.
func (s *Sim) ExpectPauliString(ps pauli.PauliString) float64 {
	if ps.NumQubits() != s.numQubits {
		panic(fmt.Sprintf("statevector: PauliString has %d qubits, simulator has %d",
			ps.NumQubits(), s.numQubits))
	}
	return real(pauli.Expect(s.state, ps))
}

// ExpectPauliSum computes Re(⟨psi|H|psi⟩) for a Hamiltonian H (sum of Pauli strings).
// For Hermitian observables (real coefficients), the imaginary part is zero.
// For non-Hermitian observables, use pauli.ExpectSum directly for complex128.
func (s *Sim) ExpectPauliSum(ps pauli.PauliSum) float64 {
	if ps.NumQubits() != s.numQubits {
		panic(fmt.Sprintf("statevector: PauliSum has %d qubits, simulator has %d",
			ps.NumQubits(), s.numQubits))
	}
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
