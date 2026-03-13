package ionq

import (
	"encoding/json"
	"math"
	"math/rand"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

// buildQISCircuit constructs a random circuit using only IonQ QIS-compatible gates.
// Returns nil if the builder encounters a validation error.
func buildQISCircuit(nQubits, nGates int, seed uint32) *ir.Circuit {
	rng := rand.New(rand.NewSource(int64(seed)))

	b := builder.New("fuzz-qis", nQubits)

	type gateApplier func(b *builder.Builder, rng *rand.Rand, nQubits int) *builder.Builder
	singleGates := []gateApplier{
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.H(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.X(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.Y(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.Z(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.S(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder { return b.T(rng.Intn(n)) },
		func(b *builder.Builder, rng *rand.Rand, n int) *builder.Builder {
			return b.Apply(gate.SX, rng.Intn(n))
		},
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
			return b.SWAP(q0, q1)
		},
	}

	for range nGates {
		if nQubits >= 2 && rng.Intn(4) == 0 {
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

// buildNativeCircuit constructs a random circuit using only IonQ native gates.
func buildNativeCircuit(nQubits, nGates int, seed uint32) *ir.Circuit {
	rng := rand.New(rand.NewSource(int64(seed)))

	var ops []ir.Operation
	for range nGates {
		angle := rng.Float64() * 2 * math.Pi
		if nQubits >= 2 && rng.Intn(3) == 0 {
			// MS gate on two qubits
			q0, q1 := distinctPair(rng, nQubits)
			ops = append(ops, ir.Operation{
				Gate:   gate.MS(angle, rng.Float64()*2*math.Pi),
				Qubits: []int{q0, q1},
			})
		} else {
			q := rng.Intn(nQubits)
			if rng.Intn(2) == 0 {
				ops = append(ops, ir.Operation{
					Gate:   gate.GPI(angle),
					Qubits: []int{q},
				})
			} else {
				ops = append(ops, ir.Operation{
					Gate:   gate.GPI2(angle),
					Qubits: []int{q},
				})
			}
		}
	}

	return ir.New("fuzz-native", nQubits, 0, ops, nil)
}

func distinctPair(rng *rand.Rand, n int) (int, int) {
	q0 := rng.Intn(n)
	q1 := rng.Intn(n - 1)
	if q1 >= q0 {
		q1++
	}
	return q0, q1
}

// FuzzMarshalCircuit builds random QIS circuits and tries to marshal them.
// Must not panic.
func FuzzMarshalCircuit(f *testing.F) {
	f.Add(uint8(2), uint8(5), uint32(42))
	f.Add(uint8(1), uint8(3), uint32(0))
	f.Add(uint8(4), uint8(10), uint32(123))
	f.Add(uint8(8), uint8(20), uint32(99999))
	f.Add(uint8(3), uint8(1), uint32(1))

	f.Fuzz(func(t *testing.T, nQubits uint8, nGates uint8, seed uint32) {
		if nQubits == 0 || nQubits > 8 {
			return
		}
		if nGates == 0 || nGates > 20 {
			return
		}

		c := buildQISCircuit(int(nQubits), int(nGates), seed)
		if c == nil {
			return
		}

		// Must not panic (errors are OK for unsupported gate combinations).
		input, err := marshalCircuit(c)
		if err != nil {
			return // unsupported gates are fine
		}

		// If marshaling succeeded, verify basic invariants.
		if input.Qubits != c.NumQubits() {
			t.Errorf("qubit count mismatch: circuit=%d, input=%d", c.NumQubits(), input.Qubits)
		}
		if input.Gateset != "qis" {
			t.Errorf("expected qis gateset, got %q", input.Gateset)
		}
	})
}

// FuzzMarshalNativeCircuit builds random native circuits and marshals them.
func FuzzMarshalNativeCircuit(f *testing.F) {
	f.Add(uint8(2), uint8(5), uint32(42))
	f.Add(uint8(3), uint8(10), uint32(0))
	f.Add(uint8(1), uint8(3), uint32(1))

	f.Fuzz(func(t *testing.T, nQubits uint8, nGates uint8, seed uint32) {
		if nQubits == 0 || nQubits > 8 {
			return
		}
		if nGates == 0 || nGates > 20 {
			return
		}

		c := buildNativeCircuit(int(nQubits), int(nGates), seed)

		input, err := marshalCircuit(c)
		if err != nil {
			t.Fatalf("marshalCircuit failed for native circuit: %v", err)
		}

		if input.Qubits != c.NumQubits() {
			t.Errorf("qubit count mismatch: circuit=%d, input=%d", c.NumQubits(), input.Qubits)
		}
		if input.Gateset != "native" {
			t.Errorf("expected native gateset, got %q", input.Gateset)
		}

		// Verify gate count: all native ops should marshal (no measurements/barriers).
		if len(input.Circuit) != len(c.Ops()) {
			t.Errorf("gate count mismatch: circuit ops=%d, marshaled=%d", len(c.Ops()), len(input.Circuit))
		}
	})
}

// FuzzDetectGateset verifies that detectGateset correctly identifies QIS vs native circuits.
func FuzzDetectGateset(f *testing.F) {
	f.Add(uint8(2), uint8(5), uint32(42), true)
	f.Add(uint8(3), uint8(8), uint32(0), false)

	f.Fuzz(func(t *testing.T, nQubits uint8, nGates uint8, seed uint32, useNative bool) {
		if nQubits == 0 || nQubits > 8 {
			return
		}
		if nGates == 0 || nGates > 20 {
			return
		}

		var c *ir.Circuit
		if useNative {
			c = buildNativeCircuit(int(nQubits), int(nGates), seed)
		} else {
			c = buildQISCircuit(int(nQubits), int(nGates), seed)
			if c == nil {
				return
			}
		}

		gs, err := detectGateset(c)
		if err != nil {
			return // mixed gatesets or unsupported gates
		}

		if useNative && gs != "native" {
			t.Errorf("expected native gateset for native circuit, got %q", gs)
		}
		// QIS circuits with S†, T† will still be detected as QIS.
		if !useNative && gs != "qis" {
			t.Errorf("expected qis gateset for QIS circuit, got %q", gs)
		}
	})
}

// FuzzBitstring verifies that bitstring never panics and produces correct-length output.
func FuzzBitstring(f *testing.F) {
	f.Add(0, 2)
	f.Add(3, 2)
	f.Add(7, 3)
	f.Add(255, 8)
	f.Add(0, 1)

	f.Fuzz(func(t *testing.T, key int, numQubits int) {
		if numQubits <= 0 || numQubits > 20 {
			return
		}
		if key < 0 {
			return
		}

		bs := bitstring(key, numQubits)
		if len(bs) != numQubits {
			t.Errorf("bitstring(%d, %d) length = %d, want %d", key, numQubits, len(bs), numQubits)
		}
		// Every character should be '0' or '1'.
		for i, ch := range bs {
			if ch != '0' && ch != '1' {
				t.Errorf("bitstring(%d, %d)[%d] = %c, want '0' or '1'", key, numQubits, i, ch)
			}
		}
	})
}

// FuzzRadiansToTurns verifies the conversion is consistent.
func FuzzRadiansToTurns(f *testing.F) {
	f.Add(0.0)
	f.Add(math.Pi)
	f.Add(2 * math.Pi)
	f.Add(-math.Pi)

	f.Fuzz(func(t *testing.T, rad float64) {
		if math.IsNaN(rad) || math.IsInf(rad, 0) {
			return
		}

		turns := radiansToTurns(rad)
		// Verify the inverse relationship: turns * 2*pi should equal rad.
		reconstructed := turns * 2 * math.Pi
		if math.Abs(reconstructed-rad) > 1e-10 {
			t.Errorf("radiansToTurns(%v) = %v, but %v * 2pi = %v (want %v)",
				rad, turns, turns, reconstructed, rad)
		}
	})
}

// FuzzMarshalPulseShapes builds random PulseShapes and marshals them.
// Must not panic and must produce valid JSON.
func FuzzMarshalPulseShapes(f *testing.F) {
	f.Add(uint8(1), uint32(42))
	f.Add(uint8(2), uint32(0))
	f.Add(uint8(3), uint32(123))

	f.Fuzz(func(t *testing.T, nPairs uint8, seed uint32) {
		if nPairs == 0 || nPairs > 6 {
			return
		}

		rng := rand.New(rand.NewSource(int64(seed)))
		pairs := make([]PulsePair, nPairs)
		for i := range pairs {
			nAmps := rng.Intn(10) + 1
			amps := make([]float64, nAmps)
			for j := range amps {
				amps[j] = rng.Float64() * 2
			}
			pairs[i] = PulsePair{
				Q0:              rng.Intn(8),
				Q1:              rng.Intn(8),
				Amplitudes:      amps,
				DurationUsec:    0.1 + rng.Float64()*100,
				Scale:           rng.Float64(),
				NearestModesIdx: [2]int{rng.Intn(5), rng.Intn(5)},
				RelDet:          [2]float64{rng.Float64(), rng.Float64()},
			}
		}

		ps, err := NewPulseShapes(rng.Intn(100), "fuzz", pairs...)
		if err != nil {
			return
		}

		result, err := marshalPulseShapes(ps)
		if err != nil {
			t.Fatalf("marshalPulseShapes failed: %v", err)
		}

		// Must marshal to valid JSON.
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		if len(data) == 0 {
			t.Error("empty JSON output")
		}
	})
}
