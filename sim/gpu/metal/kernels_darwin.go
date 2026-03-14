//go:build darwin

package metal

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

func evolve(s *Sim, c *ir.Circuit) error {
	if c.NumQubits() != s.numQubits {
		return fmt.Errorf("metal: circuit has %d qubits, simulator has %d",
			c.NumQubits(), s.numQubits)
	}

	metalResetState(s)

	if err := metalBeginPass(s); err != nil {
		return err
	}
	passOpen := true

	endPass := func() error {
		if passOpen {
			passOpen = false
			return metalEndPass(s)
		}
		return nil
	}
	beginPass := func() error {
		if !passOpen {
			if err := metalBeginPass(s); err != nil {
				return err
			}
			passOpen = true
		}
		return nil
	}

	for _, op := range c.Ops() {
		if op.Gate == nil || op.Gate.Name() == "barrier" {
			continue
		}

		if op.Gate.Name() == "reset" {
			if err := endPass(); err != nil {
				return err
			}
			resetQubitCPU(s, op.Qubits[0])
			if err := beginPass(); err != nil {
				return err
			}
			continue
		}

		if sp, ok := op.Gate.(gate.StatePrepable); ok {
			amps := sp.Amplitudes()
			if len(op.Qubits) == s.numQubits {
				allInOrder := true
				for i, q := range op.Qubits {
					if q != i {
						allInOrder = false
						break
					}
				}
				if allInOrder {
					if err := endPass(); err != nil {
						return err
					}
					writeStateF32(s, amps)
					if err := beginPass(); err != nil {
						return err
					}
					continue
				}
			}
			applied := op.Gate.Decompose(op.Qubits)
			for _, a := range applied {
				m := a.Gate.Matrix()
				if m == nil {
					continue
				}
				if err := dispatchOp(s, a.Gate, a.Qubits); err != nil {
					_ = endPass()
					return err
				}
			}
			continue
		}

		if err := dispatchOp(s, op.Gate, op.Qubits); err != nil {
			_ = endPass()
			return err
		}
	}

	return endPass()
}

func dispatchOp(s *Sim, g gate.Gate, qubits []int) error {
	switch g.Qubits() {
	case 1:
		return metalGate1Q(s, qubits[0], g.Matrix())
	case 2:
		return dispatchGate2(s, qubits[0], qubits[1], g.Matrix())
	default:
		applied := g.Decompose(qubits)
		if applied != nil {
			for _, a := range applied {
				m := a.Gate.Matrix()
				if m == nil {
					continue
				}
				if err := dispatchOp(s, a.Gate, a.Qubits); err != nil {
					return err
				}
			}
			return nil
		}
		return fmt.Errorf("metal: unsupported gate %s (%d qubits)",
			g.Name(), g.Qubits())
	}
}

func dispatchGate2(s *Sim, q0, q1 int, m []complex128) error {
	if q0 < q1 {
		// CPU convention: row 1 = q1 set, row 2 = q0 set.
		// Metal convention: a[1] = lower qubit (q0) set, a[2] = higher qubit (q1) set.
		// When q0 < q1, rows 1 and 2 are swapped — permute the matrix.
		m = permuteMatrix2Q(m)
		return metalGate2Q(s, q0, q1, m)
	}
	// q0 > q1: swap qubit args for Metal (requires sorted). The CPU and Metal
	// conventions happen to align when the original q0 is the higher qubit.
	return metalGate2Q(s, q1, q0, m)
}

// permuteMatrix2Q reorders a 4x4 gate matrix when swapping qubit order.
func permuteMatrix2Q(m []complex128) []complex128 {
	perm := [4]int{0, 2, 1, 3}
	out := make([]complex128, 16)
	for r := range 4 {
		for c := range 4 {
			out[r*4+c] = m[perm[r]*4+perm[c]]
		}
	}
	return out
}

func run(s *Sim, c *ir.Circuit, shots int) (map[string]int, error) {
	if c.IsDynamic() {
		return nil, fmt.Errorf("metal: dynamic circuits not supported")
	}
	if err := evolve(s, c); err != nil {
		return nil, err
	}

	// Read float32 probabilities from shared buffer.
	buf := stateVectorF32(s)
	nAmps := 1 << s.numQubits
	probs := make([]float64, nAmps)
	for i := range nAmps {
		re := float64(buf[2*i])
		im := float64(buf[2*i+1])
		probs[i] = re*re + im*im
	}

	counts := make(map[string]int)
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	for range shots {
		idx := sampleIndex(probs, rng)
		bs := formatBitstring(idx, s.numQubits)
		counts[bs]++
	}
	return counts, nil
}

func resetQubitCPU(s *Sim, qubit int) {
	buf := stateVectorF32(s)
	nAmps := 1 << s.numQubits
	halfBlock := 1 << qubit
	block := halfBlock << 1
	for b0 := 0; b0 < nAmps; b0 += block {
		for offset := range halfBlock {
			i0 := b0 + offset
			i1 := i0 + halfBlock
			r0, im0 := float64(buf[2*i0]), float64(buf[2*i0+1])
			r1, im1 := float64(buf[2*i1]), float64(buf[2*i1+1])
			norm := math.Sqrt(r0*r0 + im0*im0 + r1*r1 + im1*im1)
			if norm > 1e-15 {
				buf[2*i0] = float32(norm)
			} else {
				buf[2*i0] = 0
			}
			buf[2*i0+1] = 0
			buf[2*i1] = 0
			buf[2*i1+1] = 0
		}
	}
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
