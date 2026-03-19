package statevector

import (
	"math"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

// TestDynamicSimpleConditional: X(0), measure(0→c0), if(c0==1, X, 1).
// Qubit 1 should end in |1> with certainty.
func TestDynamicSimpleConditional(t *testing.T) {
	c, err := builder.New("cond", 2).WithClbits(1).
		X(0).
		Measure(0, 0).
		If(0, 1, gate.X, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsDynamic() {
		t.Fatal("expected dynamic circuit")
	}

	sim := New(2)
	counts, err := sim.Run(c, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// c0 should always be 1 (we applied X first).
	// And if c0==1, X is applied to q1, so q1 should always be 1.
	// The only bitstring should be "1" (1 clbit).
	if counts["1"] != 1000 {
		t.Errorf("expected all shots to yield '1', got %v", counts)
	}
}

// TestDynamicConditionalNotFired: measure(0→c0) (q0 in |0>), if(c0==1, X, 1).
// The condition should not fire since c0=0.
func TestDynamicConditionalNotFired(t *testing.T) {
	c, err := builder.New("cond_no", 2).WithClbits(1).
		Measure(0, 0). // q0 is |0>, so c0=0
		If(0, 1, gate.X, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	counts, err := sim.Run(c, 1000)
	if err != nil {
		t.Fatal(err)
	}

	// c0=0, condition not met, X not applied. Only outcome is "0".
	if counts["0"] != 1000 {
		t.Errorf("expected all shots to yield '0', got %v", counts)
	}
}

// TestDynamicResetAndReuse: H(0), measure(0→c0), reset(0), H(0), measure(0→c1).
// c1 should be 50/50 regardless of c0.
func TestDynamicResetAndReuse(t *testing.T) {
	c, err := builder.New("reset_reuse", 1).WithClbits(2).
		H(0).
		Measure(0, 0).
		Reset(0).
		H(0).
		Measure(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsDynamic() {
		t.Fatal("expected dynamic circuit")
	}

	sim := New(1)
	counts, err := sim.Run(c, 10000)
	if err != nil {
		t.Fatal(err)
	}

	// c1 (bit index 1, MSB in 2-bit string) should be ~50/50 regardless of c0.
	// Sum up counts where c1=0 vs c1=1.
	c1zero := 0 // "0x" strings
	c1one := 0  // "1x" strings
	for bs, n := range counts {
		if len(bs) == 2 && bs[0] == '1' {
			c1one += n
		} else {
			c1zero += n
		}
	}

	ratio := float64(c1one) / float64(c1zero+c1one)
	if math.Abs(ratio-0.5) > 0.05 {
		t.Errorf("c1 should be ~50/50, got ratio %.3f, counts=%v", ratio, counts)
	}
}

// TestDynamicTeleportation: canonical MCM + feed-forward test.
func TestDynamicTeleportation(t *testing.T) {
	// Teleport |1> from q0 to q2.
	// Prepare |1> on q0.
	// Bell pair on q1,q2.
	// Bell measurement on q0,q1.
	// Feed-forward corrections on q2.
	c, err := builder.New("teleport", 3).WithClbits(2).
		X(0).                // State to teleport: |1>
		H(1).                // Bell pair
		CNOT(1, 2).          // Bell pair
		CNOT(0, 1).          // Bell measurement
		H(0).                // Bell measurement
		Measure(0, 0).       // c0
		Measure(1, 1).       // c1
		If(1, 1, gate.X, 2). // X correction
		If(0, 1, gate.Z, 2). // Z correction
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Build a circuit that also measures q2 to verify teleportation.
	c2, err := builder.New("teleport_verify", 3).WithClbits(3).
		X(0).
		H(1).
		CNOT(1, 2).
		CNOT(0, 1).
		H(0).
		Measure(0, 0).
		Measure(1, 1).
		If(1, 1, gate.X, 2).
		If(0, 1, gate.Z, 2).
		Measure(2, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	_ = c
	sim := New(3)
	counts, err := sim.Run(c2, 10000)
	if err != nil {
		t.Fatal(err)
	}

	// After teleportation, q2 (c2, MSB) should always be 1.
	for bs, n := range counts {
		if len(bs) != 3 {
			t.Errorf("unexpected bitstring length: %q", bs)
			continue
		}
		if bs[0] != '1' { // MSB = c2
			t.Errorf("expected c2=1 in all shots, got %q (%d times)", bs, n)
		}
	}
}

// TestDynamicNonDynamicFastPath ensures non-dynamic circuits use the fast evolve-then-sample path.
func TestDynamicNonDynamicFastPath(t *testing.T) {
	// Simple Bell state - not dynamic.
	c, err := builder.New("bell", 2).WithClbits(2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if c.IsDynamic() {
		t.Fatal("Bell state circuit should not be dynamic")
	}

	sim := New(2)
	counts, err := sim.Run(c, 1000)
	if err != nil {
		t.Fatal(err)
	}
	// Should only get "00" and "11".
	for bs := range counts {
		if bs != "00" && bs != "11" {
			t.Errorf("unexpected bitstring %q in Bell state", bs)
		}
	}
}

// TestDynamicIsDynamic tests IsDynamic for various circuits.
func TestDynamicIsDynamic(t *testing.T) {
	tests := []struct {
		name    string
		build   func() (*ir.Circuit, error)
		dynamic bool
	}{
		{
			"end-of-circuit measurement",
			func() (*ir.Circuit, error) {
				return builder.New("static", 2).WithClbits(2).
					H(0).CNOT(0, 1).MeasureAll().Build()
			},
			false,
		},
		{
			"mid-circuit measurement",
			func() (*ir.Circuit, error) {
				return builder.New("mcm", 2).WithClbits(1).
					H(0).Measure(0, 0).X(1).Build()
			},
			true,
		},
		{
			"conditioned gate",
			func() (*ir.Circuit, error) {
				return builder.New("cond", 2).WithClbits(1).
					If(0, 1, gate.X, 0).Build()
			},
			true,
		},
		{
			"reset",
			func() (*ir.Circuit, error) {
				return builder.New("rst", 1).WithClbits(0).
					Reset(0).Build()
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := tt.build()
			if err != nil {
				t.Fatal(err)
			}
			if c.IsDynamic() != tt.dynamic {
				t.Errorf("IsDynamic() = %v, want %v", c.IsDynamic(), tt.dynamic)
			}
		})
	}
}

// TestDynamicIfBlock tests the IfBlock builder method.
func TestDynamicIfBlock(t *testing.T) {
	c, err := builder.New("ifblock", 2).WithClbits(1).
		X(0).
		Measure(0, 0).
		IfBlock(0, 1, func(b *builder.Builder) {
			b.X(1).Z(1)
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	counts, err := sim.Run(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	// c0=1 always, then X(1) and Z(1) are applied.
	// q1 after X then Z: Z·X|0> = Z|1> = -|1>, same measurement outcome as |1>.
	// Single classical bit = 1.
	if counts["1"] != 100 {
		t.Errorf("expected all '1', got %v", counts)
	}
}

// TestDynamicRunDirectly tests calling RunDynamic directly.
func TestDynamicRunDirectly(t *testing.T) {
	c, err := builder.New("direct", 1).WithClbits(1).
		X(0).
		Measure(0, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(1)
	counts, err := sim.RunDynamic(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	if counts["1"] != 100 {
		t.Errorf("expected all '1', got %v", counts)
	}
}

// TestDynamicStats tests the Stats struct new fields.
func TestDynamicStats(t *testing.T) {
	c, err := builder.New("stats", 2).WithClbits(2).
		X(0).
		Measure(0, 0).
		Reset(0).
		If(0, 1, gate.X, 1).
		Measure(1, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	s := c.Stats()
	if s.Measurements != 2 {
		t.Errorf("Measurements = %d, want 2", s.Measurements)
	}
	if s.Resets != 1 {
		t.Errorf("Resets = %d, want 1", s.Resets)
	}
	if s.ConditionalGates != 1 {
		t.Errorf("ConditionalGates = %d, want 1", s.ConditionalGates)
	}
	if !s.Dynamic {
		t.Error("Dynamic = false, want true")
	}
}
