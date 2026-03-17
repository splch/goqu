package clifford

import (
	"fmt"
	"math/rand/v2"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

// Sim simulates Clifford circuits using the stabilizer tableau.
type Sim struct {
	tab *Tableau
}

// New creates a Clifford simulator initialized to |0...0>.
func New(numQubits int) *Sim {
	if numQubits < 1 || numQubits > 100000 {
		panic(fmt.Sprintf("clifford: numQubits %d out of range [1, 100000]", numQubits))
	}
	return &Sim{tab: newTableau(numQubits)}
}

// NumQubits returns the number of qubits.
func (s *Sim) NumQubits() int { return s.tab.n }

// Evolve applies all gate operations in the circuit without measuring.
func (s *Sim) Evolve(c *ir.Circuit) error {
	if c.NumQubits() != s.tab.n {
		return fmt.Errorf("circuit has %d qubits, simulator has %d", c.NumQubits(), s.tab.n)
	}
	for _, op := range c.Ops() {
		if op.Gate == nil || op.Gate.Name() == "barrier" {
			continue
		}
		if err := s.applyGate(op); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sim) applyGate(op ir.Operation) error {
	switch op.Gate {
	case gate.H:
		s.tab.H(op.Qubits[0])
	case gate.S:
		s.tab.S(op.Qubits[0])
	case gate.Sdg:
		s.tab.S(op.Qubits[0])
		s.tab.S(op.Qubits[0])
		s.tab.S(op.Qubits[0])
	case gate.X:
		s.tab.X(op.Qubits[0])
	case gate.Y:
		s.tab.Y(op.Qubits[0])
	case gate.Z:
		s.tab.Z(op.Qubits[0])
	case gate.SX:
		s.tab.SX(op.Qubits[0])
	case gate.CNOT:
		s.tab.CNOT(op.Qubits[0], op.Qubits[1])
	case gate.CZ:
		s.tab.CZ(op.Qubits[0], op.Qubits[1])
	case gate.SWAP:
		s.tab.SWAP(op.Qubits[0], op.Qubits[1])
	case gate.CY:
		s.tab.CY(op.Qubits[0], op.Qubits[1])
	case gate.I:
		// no-op
	default:
		return s.applyGateByName(op)
	}
	return nil
}

func (s *Sim) applyGateByName(op ir.Operation) error {
	switch op.Gate.Name() {
	case "H":
		s.tab.H(op.Qubits[0])
	case "S":
		s.tab.S(op.Qubits[0])
	case "S†":
		s.tab.S(op.Qubits[0])
		s.tab.S(op.Qubits[0])
		s.tab.S(op.Qubits[0])
	case "X":
		s.tab.X(op.Qubits[0])
	case "Y":
		s.tab.Y(op.Qubits[0])
	case "Z":
		s.tab.Z(op.Qubits[0])
	case "SX":
		s.tab.SX(op.Qubits[0])
	case "I":
		// no-op
	case "CNOT":
		s.tab.CNOT(op.Qubits[0], op.Qubits[1])
	case "CZ":
		s.tab.CZ(op.Qubits[0], op.Qubits[1])
	case "SWAP":
		s.tab.SWAP(op.Qubits[0], op.Qubits[1])
	case "CY":
		s.tab.CY(op.Qubits[0], op.Qubits[1])
	default:
		return fmt.Errorf("clifford: non-Clifford gate %q not supported", op.Gate.Name())
	}
	return nil
}

// Run executes the circuit for the given number of shots and returns measurement counts.
func (s *Sim) Run(c *ir.Circuit, shots int) (map[string]int, error) {
	if c.NumQubits() != s.tab.n {
		return nil, fmt.Errorf("circuit has %d qubits, simulator has %d", c.NumQubits(), s.tab.n)
	}

	counts := make(map[string]int)
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	ops := c.Ops()

	for range shots {
		// Reset tableau to |0...0>.
		s.tab = newTableau(s.tab.n)

		// Find measurement ops and apply gates.
		var measQubits []int
		for _, op := range ops {
			if op.Gate == nil {
				if len(op.Clbits) > 0 && len(op.Qubits) > 0 {
					measQubits = append(measQubits, op.Qubits[0])
				}
				continue
			}
			if op.Gate.Name() == "barrier" {
				continue
			}
			if err := s.applyGate(op); err != nil {
				return nil, err
			}
		}

		// If no explicit measurements, measure all qubits.
		if len(measQubits) == 0 {
			measQubits = make([]int, s.tab.n)
			for i := range measQubits {
				measQubits[i] = i
			}
		}

		bs := make([]byte, s.tab.n)
		for i := range bs {
			bs[i] = '0'
		}
		for _, q := range measQubits {
			outcome := s.tab.Measure(q, rng)
			bs[s.tab.n-1-q] = byte('0' + outcome)
		}
		counts[string(bs)]++
	}

	return counts, nil
}
