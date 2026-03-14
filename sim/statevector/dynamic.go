package statevector

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

// RunDynamic executes a dynamic circuit (mid-circuit measurement, feed-forward, reset)
// by simulating each shot independently with projective measurement and state collapse.
func (s *Sim) RunDynamic(c *ir.Circuit, shots int) (map[string]int, error) {
	if c.NumQubits() != s.numQubits {
		return nil, fmt.Errorf("circuit has %d qubits, simulator has %d", c.NumQubits(), s.numQubits)
	}
	if shots <= 0 {
		return nil, fmt.Errorf("shots must be positive, got %d", shots)
	}

	ops := c.Ops()
	numClbits := c.NumClbits()
	counts := make(map[string]int)

	for range shots {
		rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
		// Reset state to |0...0>.
		for i := range s.state {
			s.state[i] = 0
		}
		s.state[0] = 1

		clbits := make([]int, numClbits)

		for _, op := range ops {
			// Measurement: nil gate with clbits.
			if op.Gate == nil {
				if len(op.Clbits) > 0 && len(op.Qubits) > 0 {
					outcome := s.measureQubit(op.Qubits[0], rng)
					clbits[op.Clbits[0]] = outcome
				}
				continue
			}

			name := op.Gate.Name()
			if name == "barrier" || name == "delay" {
				continue
			}

			// Reset: measure then flip if |1>.
			if name == "reset" {
				outcome := s.measureQubit(op.Qubits[0], rng)
				if outcome == 1 {
					s.applyGate1(op.Qubits[0], gate.X.Matrix())
				}
				continue
			}

			// Conditioned gate: check classical bit.
			if op.Condition != nil {
				if clbits[op.Condition.Clbit] != op.Condition.Value {
					continue
				}
			}

			// Apply gate.
			if err := s.applyOp(op); err != nil {
				return nil, err
			}
		}

		// Build bitstring from classical bits.
		bs := make([]byte, numClbits)
		for i, v := range clbits {
			if v != 0 {
				bs[numClbits-1-i] = '1'
			} else {
				bs[numClbits-1-i] = '0'
			}
		}
		counts[string(bs)]++
	}
	return counts, nil
}

// applyOp applies a single gate operation using the existing kernel dispatch.
func (s *Sim) applyOp(op ir.Operation) error {
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
	return nil
}

// measureQubit performs a projective measurement on a single qubit,
// collapsing the state and returning the outcome (0 or 1).
func (s *Sim) measureQubit(qubit int, rng *rand.Rand) int {
	halfBlock := 1 << qubit
	block := halfBlock << 1
	n := len(s.state)

	// Compute P(0): sum |amp|^2 where qubit bit = 0.
	prob0 := 0.0
	for b0 := 0; b0 < n; b0 += block {
		for offset := range halfBlock {
			i0 := b0 + offset
			a := s.state[i0]
			prob0 += real(a)*real(a) + imag(a)*imag(a)
		}
	}

	// Sample outcome.
	outcome := 0
	if rng.Float64() >= prob0 {
		outcome = 1
	}

	// Collapse: zero amplitudes inconsistent with outcome, renormalize.
	var norm float64
	if outcome == 0 {
		norm = math.Sqrt(prob0)
	} else {
		norm = math.Sqrt(1 - prob0)
	}

	if norm < 1e-15 {
		// Degenerate case: outcome was certain, no renormalization needed.
		return outcome
	}

	invNorm := 1.0 / norm
	scale := complex(invNorm, 0)

	for b0 := 0; b0 < n; b0 += block {
		for offset := range halfBlock {
			i0 := b0 + offset    // qubit bit = 0
			i1 := i0 + halfBlock // qubit bit = 1
			if outcome == 0 {
				s.state[i0] *= scale
				s.state[i1] = 0
			} else {
				s.state[i0] = 0
				s.state[i1] *= scale
			}
		}
	}
	return outcome
}
