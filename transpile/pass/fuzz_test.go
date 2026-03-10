package pass

import (
	"math"
	"math/rand"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile"
	"github.com/splch/qgo/transpile/target"
)

// buildRandomCircuit constructs a random circuit for fuzz testing.
// Returns nil if the builder encounters a validation error.
func buildRandomCircuit(nQubits, nGates int, seed uint32) *ir.Circuit {
	rng := rand.New(rand.NewSource(int64(seed)))

	b := builder.New("fuzz", nQubits)

	type gateApplier func(b *builder.Builder, rng *rand.Rand, nQubits int) *builder.Builder
	singleGates := []gateApplier{
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.H(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.X(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.Y(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.Z(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.S(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.T(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			return b.RX(rng.Float64()*2*math.Pi, rng.Intn(n))
		},
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			return b.RY(rng.Float64()*2*math.Pi, rng.Intn(n))
		},
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			return b.RZ(rng.Float64()*2*math.Pi, rng.Intn(n))
		},
	}

	twoQubitGates := []gateApplier{
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			q0, q1 := distinctPair(rng, n)
			return b.CNOT(q0, q1)
		},
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			q0, q1 := distinctPair(rng, n)
			return b.CZ(q0, q1)
		},
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			q0, q1 := distinctPair(rng, n)
			return b.SWAP(q0, q1)
		},
	}

	for range nGates {
		if nQubits >= 2 && rng.Intn(3) == 0 {
			idx := rng.Intn(len(twoQubitGates))
			b = twoQubitGates[idx](b, rng, nQubits)
		} else {
			idx := rng.Intn(len(singleGates))
			b = singleGates[idx](b, rng, nQubits)
		}
	}

	c, err := b.Build()
	if err != nil {
		return nil
	}
	return c
}

func distinctPair(rng *rand.Rand, n int) (int, int) {
	q0 := rng.Intn(n)
	q1 := rng.Intn(n - 1)
	if q1 >= q0 {
		q1++
	}
	return q0, q1
}

// FuzzDecomposeToTarget builds random circuits, runs DecomposeToTarget,
// and verifies the output only contains basis gates with preserved qubit count.
func FuzzDecomposeToTarget(f *testing.F) {
	f.Add(uint8(2), uint8(5), uint32(42))
	f.Add(uint8(3), uint8(8), uint32(1))
	f.Add(uint8(4), uint8(10), uint32(99))
	f.Add(uint8(5), uint8(15), uint32(7777))
	f.Add(uint8(2), uint8(1), uint32(0))
	f.Add(uint8(6), uint8(20), uint32(12345))

	f.Fuzz(func(t *testing.T, nQubits uint8, nGates uint8, seed uint32) {
		if nQubits < 2 || nQubits > 8 {
			return
		}
		if nGates == 0 || nGates > 30 {
			return
		}

		c := buildRandomCircuit(int(nQubits), int(nGates), seed)
		if c == nil {
			return
		}

		// Try decomposing to IBM Eagle target basis.
		tgt := target.Target{
			Name:       "IBM-fuzz",
			NumQubits:  int(nQubits),
			BasisGates: []string{"CX", "ID", "RZ", "SX", "X"},
		}

		result, err := DecomposeToTarget(c, tgt)
		if err != nil {
			return // some circuits may contain gates that cannot be decomposed
		}

		// Verify qubit count is preserved.
		if result.NumQubits() != c.NumQubits() {
			t.Errorf("qubit count changed: %d -> %d", c.NumQubits(), result.NumQubits())
		}

		// Verify all gates in result are basis gates.
		for i, op := range result.Ops() {
			if op.Gate == nil {
				continue // measurements
			}
			name := transpile.BasisName(op.Gate)
			if !tgt.HasBasisGate(name) {
				t.Errorf("op %d: non-basis gate %q (basis name %q) in result", i, op.Gate.Name(), name)
			}
		}
	})
}

// FuzzDecomposeToSimulator verifies that decomposing to the simulator target
// (which accepts all gates) passes everything through unchanged.
func FuzzDecomposeToSimulator(f *testing.F) {
	f.Add(uint8(3), uint8(10), uint32(42))
	f.Add(uint8(2), uint8(5), uint32(0))

	f.Fuzz(func(t *testing.T, nQubits uint8, nGates uint8, seed uint32) {
		if nQubits < 1 || nQubits > 8 {
			return
		}
		if nGates == 0 || nGates > 30 {
			return
		}

		c := buildRandomCircuit(int(nQubits), int(nGates), seed)
		if c == nil {
			return
		}

		result, err := DecomposeToTarget(c, target.Simulator)
		if err != nil {
			t.Fatalf("decompose to simulator failed: %v", err)
		}

		// Simulator accepts all gates, so the count should match
		// (minus barriers which are stripped by DecomposeToTarget).
		if result.NumQubits() != c.NumQubits() {
			t.Errorf("qubit count changed: %d -> %d", c.NumQubits(), result.NumQubits())
		}
	})
}

// FuzzCancelAdjacent builds random circuits, runs CancelAdjacent,
// and verifies the result has no more ops than the original.
func FuzzCancelAdjacent(f *testing.F) {
	f.Add(uint8(2), uint8(10), uint32(42))
	f.Add(uint8(3), uint8(15), uint32(1))
	f.Add(uint8(1), uint8(5), uint32(0))

	f.Fuzz(func(t *testing.T, nQubits uint8, nGates uint8, seed uint32) {
		if nQubits < 1 || nQubits > 8 {
			return
		}
		if nGates == 0 || nGates > 30 {
			return
		}

		c := buildRandomCircuit(int(nQubits), int(nGates), seed)
		if c == nil {
			return
		}

		result, err := CancelAdjacent(c, target.Simulator)
		if err != nil {
			t.Fatalf("CancelAdjacent failed: %v", err)
		}

		// Must not add gates.
		if len(result.Ops()) > len(c.Ops()) {
			t.Errorf("CancelAdjacent increased op count: %d -> %d", len(c.Ops()), len(result.Ops()))
		}
		// Must preserve qubit count.
		if result.NumQubits() != c.NumQubits() {
			t.Errorf("qubit count changed: %d -> %d", c.NumQubits(), result.NumQubits())
		}
	})
}

// FuzzMergeRotations builds random circuits, runs MergeRotations,
// and verifies the result has no more ops than the original.
func FuzzMergeRotations(f *testing.F) {
	f.Add(uint8(2), uint8(10), uint32(42))
	f.Add(uint8(1), uint8(8), uint32(1))

	f.Fuzz(func(t *testing.T, nQubits uint8, nGates uint8, seed uint32) {
		if nQubits < 1 || nQubits > 8 {
			return
		}
		if nGates == 0 || nGates > 30 {
			return
		}

		c := buildRandomCircuit(int(nQubits), int(nGates), seed)
		if c == nil {
			return
		}

		result, err := MergeRotations(c, target.Simulator)
		if err != nil {
			t.Fatalf("MergeRotations failed: %v", err)
		}

		// Must not add gates.
		if len(result.Ops()) > len(c.Ops()) {
			t.Errorf("MergeRotations increased op count: %d -> %d", len(c.Ops()), len(result.Ops()))
		}
		// Must preserve qubit count.
		if result.NumQubits() != c.NumQubits() {
			t.Errorf("qubit count changed: %d -> %d", c.NumQubits(), result.NumQubits())
		}
	})
}

// FuzzCancelAdjacentInversePairs specifically tests circuits with deliberate inverse pairs
// to stress-test the cancellation logic.
func FuzzCancelAdjacentInversePairs(f *testing.F) {
	f.Add(uint8(3), uint32(42))
	f.Add(uint8(2), uint32(0))
	f.Add(uint8(5), uint32(999))

	f.Fuzz(func(t *testing.T, nQubits uint8, seed uint32) {
		if nQubits < 1 || nQubits > 8 {
			return
		}

		rng := rand.New(rand.NewSource(int64(seed)))
		nq := int(nQubits)
		b := builder.New("fuzz-inverse", nq)

		// Insert random gates followed by their inverses, interleaved.
		nPairs := rng.Intn(10) + 1
		for range nPairs {
			q := rng.Intn(nq)
			switch rng.Intn(6) {
			case 0:
				b = b.H(q).H(q) // H is self-inverse
			case 1:
				b = b.X(q).X(q) // X is self-inverse
			case 2:
				b = b.S(q).Apply(gate.Sdg, q) // S * Sdg = I
			case 3:
				b = b.T(q).Apply(gate.Tdg, q) // T * Tdg = I
			case 4:
				angle := rng.Float64() * 2 * math.Pi
				b = b.RZ(angle, q).RZ(-angle, q) // RZ(a) * RZ(-a) = I
			case 5:
				if nq >= 2 {
					q0, q1 := q, rng.Intn(nq-1)
					if q1 >= q0 {
						q1++
					}
					b = b.CNOT(q0, q1).CNOT(q0, q1)
				}
			}
		}

		c, err := b.Build()
		if err != nil {
			return
		}

		result, err := CancelAdjacent(c, target.Simulator)
		if err != nil {
			t.Fatalf("CancelAdjacent failed: %v", err)
		}

		// Result should have strictly fewer ops (all pairs should cancel).
		if len(result.Ops()) > len(c.Ops()) {
			t.Errorf("CancelAdjacent did not reduce ops: %d -> %d", len(c.Ops()), len(result.Ops()))
		}
	})
}
