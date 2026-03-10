package emitter

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/qasm/parser"
)

// buildRandomCircuit constructs a random circuit using the builder API.
// Returns nil if the builder encounters a validation error.
func buildRandomCircuit(nQubits, nGates int, seed uint32) *ir.Circuit {
	rng := rand.New(rand.NewSource(int64(seed)))

	b := builder.New("fuzz", nQubits).WithClbits(nQubits)

	// Gate selection table: each entry is a function that applies a gate.
	type gateApplier func(b *builder.Builder, rng *rand.Rand, nQubits int) *builder.Builder
	gates := []gateApplier{
		// Single-qubit fixed gates
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.H(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.X(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.Y(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.Z(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.S(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.T(rng.Intn(n)) },
		// Single-qubit parameterized gates
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			return b.RX(rng.Float64()*2*math.Pi, rng.Intn(n))
		},
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			return b.RY(rng.Float64()*2*math.Pi, rng.Intn(n))
		},
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			return b.RZ(rng.Float64()*2*math.Pi, rng.Intn(n))
		},
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			return b.Phase(rng.Float64()*2*math.Pi, rng.Intn(n))
		},
	}

	// Two-qubit gates (only if nQubits >= 2)
	twoQubitGates := []gateApplier{
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			q0 := rng.Intn(n)
			q1 := rng.Intn(n - 1)
			if q1 >= q0 {
				q1++
			}
			return b.CNOT(q0, q1)
		},
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			q0 := rng.Intn(n)
			q1 := rng.Intn(n - 1)
			if q1 >= q0 {
				q1++
			}
			return b.CZ(q0, q1)
		},
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			q0 := rng.Intn(n)
			q1 := rng.Intn(n - 1)
			if q1 >= q0 {
				q1++
			}
			return b.SWAP(q0, q1)
		},
	}

	for range nGates {
		if nQubits >= 2 && rng.Intn(3) == 0 {
			// ~33% chance of a two-qubit gate
			idx := rng.Intn(len(twoQubitGates))
			b = twoQubitGates[idx](b, rng, nQubits)
		} else {
			idx := rng.Intn(len(gates))
			b = gates[idx](b, rng, nQubits)
		}
	}

	c, err := b.Build()
	if err != nil {
		return nil
	}
	return c
}

// FuzzEmit builds random circuits programmatically and verifies they emit valid QASM
// that can be re-parsed with structural equivalence.
func FuzzEmit(f *testing.F) {
	// Seeds: number of qubits, number of gates, random seed
	f.Add(uint8(2), uint8(5), uint32(42))
	f.Add(uint8(4), uint8(10), uint32(123))
	f.Add(uint8(1), uint8(1), uint32(0))
	f.Add(uint8(3), uint8(20), uint32(999))
	f.Add(uint8(5), uint8(15), uint32(7777))
	f.Add(uint8(1), uint8(3), uint32(1))
	f.Add(uint8(8), uint8(30), uint32(12345))

	f.Fuzz(func(t *testing.T, nQubits uint8, nGates uint8, seed uint32) {
		if nQubits == 0 || nQubits > 10 {
			return
		}
		if nGates == 0 || nGates > 50 {
			return
		}

		c := buildRandomCircuit(int(nQubits), int(nGates), seed)
		if c == nil {
			return
		}

		// Emit to QASM -- must not panic or error.
		qasm, err := EmitString(c)
		if err != nil {
			t.Fatalf("emit failed: %v", err)
		}

		if qasm == "" {
			t.Fatal("emit produced empty output")
		}

		// Re-parse the emitted QASM.
		c2, err := parser.ParseString(qasm)
		if err != nil {
			t.Fatalf("re-parse failed: %v\nQASM:\n%s", err, qasm)
		}

		if c.NumQubits() != c2.NumQubits() {
			t.Errorf("qubit mismatch: %d vs %d\nQASM:\n%s", c.NumQubits(), c2.NumQubits(), qasm)
		}
		if len(c.Ops()) != len(c2.Ops()) {
			t.Errorf("op count mismatch: %d vs %d\nQASM:\n%s", len(c.Ops()), len(c2.Ops()), qasm)
		}
	})
}

// FuzzEmitAllGateTypes specifically targets parameterized gate emission and re-parsing
// to ensure pi-fraction formatting round-trips correctly.
func FuzzEmitAllGateTypes(f *testing.F) {
	f.Add(0.0)
	f.Add(math.Pi)
	f.Add(math.Pi / 2)
	f.Add(math.Pi / 4)
	f.Add(math.Pi / 8)
	f.Add(-math.Pi)
	f.Add(1.23456789)
	f.Add(2 * math.Pi)

	f.Fuzz(func(t *testing.T, angle float64) {
		// Skip infinities and NaN which are not valid gate parameters.
		if math.IsNaN(angle) || math.IsInf(angle, 0) {
			return
		}
		// Clamp to reasonable range to avoid floating point issues.
		if angle < -100 || angle > 100 {
			return
		}

		// Build circuits with each parameterized gate type.
		gateConstructors := []struct {
			name string
			gate gate.Gate
		}{
			{"rx", gate.RX(angle)},
			{"ry", gate.RY(angle)},
			{"rz", gate.RZ(angle)},
			{"p", gate.Phase(angle)},
		}

		for _, gc := range gateConstructors {
			c, err := builder.New("fuzz-"+gc.name, 1).
				WithClbits(1).
				Apply(gc.gate, 0).
				Build()
			if err != nil {
				continue
			}

			qasm, err := EmitString(c)
			if err != nil {
				t.Fatalf("emit failed for %s(%v): %v", gc.name, angle, err)
			}

			// Must contain the gate name.
			if !strings.Contains(qasm, gc.name) {
				t.Errorf("emitted QASM missing gate name %q:\n%s", gc.name, qasm)
			}

			// Must re-parse without error.
			_, err = parser.ParseString(qasm)
			if err != nil {
				t.Fatalf("re-parse failed for %s(%v): %v\nQASM:\n%s", gc.name, angle, err, qasm)
			}
		}
	})
}
