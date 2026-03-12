package densitymatrix

import (
	"fmt"
	"math/rand/v2"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
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
		// Reset to |0><0|.
		for i := range s.rho {
			s.rho[i] = 0
		}
		s.rho[0] = 1

		clbits := make([]int, numClbits)

		for _, op := range ops {
			// Measurement: nil gate with clbits.
			if op.Gate == nil {
				if len(op.Clbits) > 0 && len(op.Qubits) > 0 {
					outcome := s.measureQubitDM(op.Qubits[0], rng)
					clbits[op.Clbits[0]] = outcome
				}
				continue
			}

			name := op.Gate.Name()
			if name == "barrier" {
				continue
			}

			// Reset: measure then flip if |1>.
			if name == "reset" {
				outcome := s.measureQubitDM(op.Qubits[0], rng)
				if outcome == 1 {
					s.applyGate1(op.Qubits[0], gate.X.Matrix())
				}
				if s.noise != nil {
					ch := s.noise.Lookup("reset", op.Qubits)
					if ch != nil {
						s.applyChannel(ch, op.Qubits)
					}
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

			// Apply noise after gate.
			if s.noise != nil {
				ch := s.noise.Lookup(op.Gate.Name(), op.Qubits)
				if ch != nil {
					s.applyChannel(ch, op.Qubits)
				}
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

// applyOp applies a single gate operation to the density matrix.
func (s *Sim) applyOp(op ir.Operation) error {
	m := op.Gate.Matrix()
	if m == nil {
		return nil
	}
	switch op.Gate.Qubits() {
	case 1:
		s.applyGate1(op.Qubits[0], m)
	case 2:
		s.applyGate2(op.Qubits[0], op.Qubits[1], m)
	default:
		subOps := decomposeForDensity(op)
		if subOps == nil {
			return fmt.Errorf("densitymatrix: unsupported gate size: %d qubits", op.Gate.Qubits())
		}
		for _, sub := range subOps {
			sm := sub.Gate.Matrix()
			if sm == nil {
				continue
			}
			switch sub.Gate.Qubits() {
			case 1:
				s.applyGate1(sub.Qubits[0], sm)
			case 2:
				s.applyGate2(sub.Qubits[0], sub.Qubits[1], sm)
			default:
				return fmt.Errorf("densitymatrix: decomposition produced %d-qubit gate", sub.Gate.Qubits())
			}
		}
	}
	return nil
}

// measureQubitDM performs a projective measurement on a single qubit in the density matrix,
// collapsing the state and returning the outcome (0 or 1).
func (s *Sim) measureQubitDM(qubit int, rng *rand.Rand) int {
	dim := s.dim
	mask := 1 << qubit

	// Compute P(0) = sum of diagonal elements rho[i,i] where qubit bit = 0.
	prob0 := 0.0
	for i := range dim {
		if i&mask == 0 {
			prob0 += real(s.rho[i*dim+i])
		}
	}

	// Sample outcome.
	outcome := 0
	if rng.Float64() >= prob0 {
		outcome = 1
	}

	// Collapse: project onto the measured subspace, renormalize.
	var probOutcome float64
	if outcome == 0 {
		probOutcome = prob0
	} else {
		probOutcome = 1 - prob0
	}

	if probOutcome < 1e-15 {
		return outcome
	}
	invProb := 1.0 / probOutcome

	for r := range dim {
		rBit := (r >> qubit) & 1
		for c := range dim {
			cBit := (c >> qubit) & 1
			idx := r*dim + c
			if rBit == outcome && cBit == outcome {
				s.rho[idx] *= complex(invProb, 0)
			} else {
				s.rho[idx] = 0
			}
		}
	}
	return outcome
}
