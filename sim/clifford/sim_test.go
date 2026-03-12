package clifford

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
)

func TestBellState(t *testing.T) {
	b := builder.New("bell", 2)
	b.H(0).CNOT(0, 1).MeasureAll()
	c := mustBuild(t, b)

	sim := New(2)
	counts, err := sim.Run(c, 10000)
	if err != nil {
		t.Fatal(err)
	}

	for k := range counts {
		if k != "00" && k != "11" {
			t.Fatalf("unexpected outcome %q", k)
		}
	}
	if counts["00"] == 0 || counts["11"] == 0 {
		t.Fatalf("expected both 00 and 11, got %v", counts)
	}
	// Rough check: each should be ~50%.
	ratio := float64(counts["00"]) / float64(counts["00"]+counts["11"])
	if ratio < 0.4 || ratio > 0.6 {
		t.Fatalf("Bell state ratio out of range: %v", ratio)
	}
}

func TestGHZ(t *testing.T) {
	b := builder.New("ghz3", 3)
	b.H(0).CNOT(0, 1).CNOT(1, 2).MeasureAll()
	c := mustBuild(t, b)

	sim := New(3)
	counts, err := sim.Run(c, 10000)
	if err != nil {
		t.Fatal(err)
	}

	for k := range counts {
		if k != "000" && k != "111" {
			t.Fatalf("unexpected outcome %q", k)
		}
	}
	if counts["000"] == 0 || counts["111"] == 0 {
		t.Fatalf("expected both 000 and 111, got %v", counts)
	}
}

func TestAllCliffordGates(t *testing.T) {
	b := builder.New("all-clifford", 4)
	b.H(0).S(1).X(2).Y(3).Z(0)
	b.Apply(gate.Sdg, 1)
	b.Apply(gate.SX, 2)
	b.CNOT(0, 1).CZ(2, 3).SWAP(0, 2)
	b.Apply(gate.CY, 1, 3)
	b.MeasureAll()
	c := mustBuild(t, b)

	sim := New(4)
	counts, err := sim.Run(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) == 0 {
		t.Fatal("no measurement results")
	}
	for k := range counts {
		if len(k) != 4 {
			t.Fatalf("bitstring has wrong length: %q", k)
		}
	}
}

func TestNonClifford(t *testing.T) {
	// T gate is not Clifford; should return an error.
	ops := []ir.Operation{
		{Gate: gate.T, Qubits: []int{0}},
	}
	c := ir.New("t-gate", 1, 0, ops, nil)

	sim := New(1)
	_, err := sim.Run(c, 1)
	if err == nil {
		t.Fatal("expected error for non-Clifford gate T")
	}
	if !strings.Contains(err.Error(), "non-Clifford") {
		t.Fatalf("expected non-Clifford error, got: %v", err)
	}
}

func TestDeterministicMeasurement(t *testing.T) {
	// |0> should always measure 0.
	sim0 := New(1)
	rng := rand.New(rand.NewPCG(42, 99))
	for range 100 {
		sim0.tab = newTableau(1)
		outcome := sim0.tab.Measure(0, rng)
		if outcome != 0 {
			t.Fatalf("expected 0 for |0>, got %d", outcome)
		}
	}

	// X|0> = |1> should always measure 1.
	for range 100 {
		sim0.tab = newTableau(1)
		sim0.tab.X(0)
		outcome := sim0.tab.Measure(0, rng)
		if outcome != 1 {
			t.Fatalf("expected 1 for X|0>, got %d", outcome)
		}
	}
}

func TestHMeasurement(t *testing.T) {
	// H|0> should be 50/50.
	rng := rand.New(rand.NewPCG(42, 99))
	zeros := 0
	ones := 0
	for range 10000 {
		tab := newTableau(1)
		tab.H(0)
		outcome := tab.Measure(0, rng)
		if outcome == 0 {
			zeros++
		} else {
			ones++
		}
	}
	ratio := float64(zeros) / float64(zeros+ones)
	if ratio < 0.45 || ratio > 0.55 {
		t.Fatalf("H|0> measurement ratio out of range: %.3f (zeros=%d, ones=%d)", ratio, zeros, ones)
	}
}

func Test1000QubitGHZ(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 1000-qubit test in short mode")
	}
	n := 200 // Enough to validate scaling; 1000 is too slow under race+coverage in CI.
	b := builder.New("ghz", n)
	b.H(0)
	for i := 1; i < n; i++ {
		b.CNOT(i-1, i)
	}
	b.MeasureAll()
	c := mustBuild(t, b)

	sim := New(n)
	counts, err := sim.Run(c, 20)
	if err != nil {
		t.Fatal(err)
	}

	allZeros := strings.Repeat("0", n)
	allOnes := strings.Repeat("1", n)
	for k := range counts {
		if k != allZeros && k != allOnes {
			t.Fatalf("unexpected outcome: not all-0 or all-1")
		}
	}
	if counts[allZeros] == 0 || counts[allOnes] == 0 {
		t.Fatalf("expected both outcomes, got %d zeros, %d ones", counts[allZeros], counts[allOnes])
	}
}

func TestIdentityGate(t *testing.T) {
	ops := []ir.Operation{
		{Gate: gate.I, Qubits: []int{0}},
	}
	c := ir.New("id", 1, 0, ops, nil)

	sim := New(1)
	err := sim.Evolve(c)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEvolveQubitMismatch(t *testing.T) {
	b := builder.New("two", 2)
	b.H(0)
	c := mustBuild(t, b)

	sim := New(3)
	err := sim.Evolve(c)
	if err == nil {
		t.Fatal("expected qubit mismatch error")
	}
}

func TestSdgGate(t *testing.T) {
	// S · S† = I. Apply S then S†, measure: should always be 0.
	rng := rand.New(rand.NewPCG(42, 99))
	for range 100 {
		tab := newTableau(1)
		tab.S(0)
		// S† = S^3
		tab.S(0)
		tab.S(0)
		tab.S(0)
		outcome := tab.Measure(0, rng)
		if outcome != 0 {
			t.Fatalf("expected 0 after S·S†, got %d", outcome)
		}
	}
}

func TestSXGate(t *testing.T) {
	// SX^2 = X, so SX·SX|0> = X|0> = |1>.
	rng := rand.New(rand.NewPCG(42, 99))
	for range 100 {
		tab := newTableau(1)
		tab.SX(0)
		tab.SX(0)
		outcome := tab.Measure(0, rng)
		if outcome != 1 {
			t.Fatalf("expected 1 after SX·SX|0>, got %d", outcome)
		}
	}
}

func TestCZGate(t *testing.T) {
	// CZ|1,1> should give phase -1 on |11>.
	// Prepare |11> via X(0)X(1), apply CZ, then check: CZ|11> = -|11>.
	// Since global phase is unobservable, instead test that CZ creates
	// correct entanglement: H(1)·CZ(0,1)·H(1) = CNOT(0,1).
	// So: X(0), H(1), CZ(0,1), H(1) should give |11> deterministically.
	rng := rand.New(rand.NewPCG(42, 99))
	for range 100 {
		tab := newTableau(2)
		tab.X(0)
		tab.H(1)
		tab.CZ(0, 1)
		tab.H(1)
		o0 := tab.Measure(0, rng)
		o1 := tab.Measure(1, rng)
		if o0 != 1 || o1 != 1 {
			t.Fatalf("expected |11>, got |%d%d>", o1, o0)
		}
	}

	// Also test CZ is symmetric: CZ(0,1) = CZ(1,0).
	for range 100 {
		tab := newTableau(2)
		tab.X(1)
		tab.H(0)
		tab.CZ(1, 0)
		tab.H(0)
		o0 := tab.Measure(0, rng)
		o1 := tab.Measure(1, rng)
		if o0 != 1 || o1 != 1 {
			t.Fatalf("expected |11> for reversed CZ, got |%d%d>", o1, o0)
		}
	}
}

func TestSWAPGate(t *testing.T) {
	// Prepare |10>, SWAP, should get |01>.
	rng := rand.New(rand.NewPCG(42, 99))
	for range 100 {
		tab := newTableau(2)
		tab.X(0) // |10>
		tab.SWAP(0, 1)
		o0 := tab.Measure(0, rng)
		o1 := tab.Measure(1, rng)
		if o0 != 0 || o1 != 1 {
			t.Fatalf("expected |01> after SWAP|10>, got |%d%d>", o1, o0)
		}
	}
}

func TestBarrierIgnored(t *testing.T) {
	// Build a circuit with a barrier; it should be ignored.
	b := builder.New("barrier", 2)
	b.H(0).Barrier(0, 1).CNOT(0, 1).MeasureAll()
	c := mustBuild(t, b)

	sim := New(2)
	counts, err := sim.Run(c, 1000)
	if err != nil {
		t.Fatal(err)
	}
	for k := range counts {
		if k != "00" && k != "11" {
			t.Fatalf("unexpected outcome %q", k)
		}
	}
}

func TestNumQubits(t *testing.T) {
	sim := New(5)
	if sim.NumQubits() != 5 {
		t.Fatalf("expected 5, got %d", sim.NumQubits())
	}
}

func TestNewPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for 0 qubits")
		}
	}()
	New(0)
}

func mustBuild(t *testing.T, b *builder.Builder) *ir.Circuit {
	t.Helper()
	c, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	return c
}
