package statevector_test

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/sim/statevector"
)

func TestMCXFlipsOnlyAllControlsSet(t *testing.T) {
	// 4 qubits: 3 controls + 1 target.
	// Prepare |1110> (controls q0,q1,q2 = 1, target q3 = 0).
	// MCX should flip target to |1111>.
	c, err := builder.New("mcx-test", 4).
		X(0).X(1).X(2). // set controls
		Apply(gate.MCX(3), 0, 1, 2, 3).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := statevector.New(4)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	// Should be |1111> = index 15.
	for i, amp := range sv {
		prob := real(amp)*real(amp) + imag(amp)*imag(amp)
		if i == 15 {
			if math.Abs(prob-1.0) > 1e-10 {
				t.Errorf("state[%d] prob = %f, want 1.0", i, prob)
			}
		} else if prob > 1e-10 {
			t.Errorf("state[%d] prob = %f, want 0.0", i, prob)
		}
	}
}

func TestMCXNoFlipPartialControls(t *testing.T) {
	// Only 2 of 3 controls set: MCX should NOT flip.
	c, err := builder.New("mcx-noop", 4).
		X(0).X(1). // only 2 controls set
		Apply(gate.MCX(3), 0, 1, 2, 3).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := statevector.New(4)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	// Should be |0011> = index 3 (q0=1, q1=1, q2=0, q3=0).
	for i, amp := range sv {
		prob := real(amp)*real(amp) + imag(amp)*imag(amp)
		if i == 3 {
			if math.Abs(prob-1.0) > 1e-10 {
				t.Errorf("state[%d] prob = %f, want 1.0", i, prob)
			}
		} else if prob > 1e-10 {
			t.Errorf("state[%d] prob = %f, want 0.0", i, prob)
		}
	}
}

func TestMCXMatchesFullMatrix(t *testing.T) {
	// Compare controlled kernel result with explicit full-matrix simulation
	// by building MCX(2) and comparing with CCX (which uses the 3q kernel).
	c1, _ := builder.New("ccx", 3).X(0).X(1).Apply(gate.CCX, 0, 1, 2).Build()
	c2, _ := builder.New("mcx", 3).X(0).X(1).Apply(gate.MCX(2), 0, 1, 2).Build()

	sim1 := statevector.New(3)
	sim2 := statevector.New(3)
	sim1.Evolve(c1)
	sim2.Evolve(c2)

	sv1 := sim1.StateVector()
	sv2 := sim2.StateVector()
	for i := range sv1 {
		if cmplx.Abs(sv1[i]-sv2[i]) > 1e-10 {
			t.Errorf("state[%d]: CCX=%v MCX(2)=%v", i, sv1[i], sv2[i])
		}
	}
}

func TestMCZPhase(t *testing.T) {
	// MCZ(2) on |111>: should apply -1 phase.
	c, _ := builder.New("mcz", 3).X(0).X(1).X(2).Apply(gate.MCZ(2), 0, 1, 2).Build()
	sim := statevector.New(3)
	sim.Evolve(c)
	sv := sim.StateVector()
	// |111> = index 7, should have amplitude -1.
	if cmplx.Abs(sv[7]-(-1)) > 1e-10 {
		t.Errorf("MCZ(2) on |111>: state[7] = %v, want -1", sv[7])
	}
}

func TestMCPPhase(t *testing.T) {
	// MCP(pi, 2) on |111>: should apply exp(i*pi) = -1.
	c, _ := builder.New("mcp", 3).X(0).X(1).X(2).Apply(gate.MCP(math.Pi, 2), 0, 1, 2).Build()
	sim := statevector.New(3)
	sim.Evolve(c)
	sv := sim.StateVector()
	if cmplx.Abs(sv[7]-(-1)) > 1e-10 {
		t.Errorf("MCP(pi,2) on |111>: state[7] = %v, want -1", sv[7])
	}
}

func TestControlledNonAdjacentQubits(t *testing.T) {
	// Controls on q0, q3 with target on q1 (non-adjacent).
	c, err := builder.New("nonadj", 4).
		X(0).X(3). // set controls
		Apply(gate.Controlled(gate.X, 2), 0, 3, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := statevector.New(4)
	sim.Evolve(c)
	sv := sim.StateVector()
	// |1001> = 9, target flipped -> |1011> = 11.
	for i, amp := range sv {
		prob := real(amp)*real(amp) + imag(amp)*imag(amp)
		if i == 11 {
			if math.Abs(prob-1.0) > 1e-10 {
				t.Errorf("state[%d] prob = %f, want 1.0", i, prob)
			}
		} else if prob > 1e-10 {
			t.Errorf("state[%d] prob = %f, want 0.0", i, prob)
		}
	}
}

func TestControlledSWAP(t *testing.T) {
	// C1-SWAP on qubits 0 (control), 1, 2 (targets).
	// Prepare |101>: control=1, targets=01 -> should swap to |110>.
	c, _ := builder.New("cswap", 3).
		X(0).X(2). // |101>
		Apply(gate.Controlled(gate.SWAP, 1), 0, 1, 2).
		Build()
	sim := statevector.New(3)
	sim.Evolve(c)
	sv := sim.StateVector()
	// |110> = index 3 (q0=1, q1=1, q2=0 -> bit pattern: 011 = 3)
	// Wait: q0=bit0, q1=bit1, q2=bit2.
	// |101> means q0=1, q1=0, q2=1 -> index = 1 + 4 = 5.
	// After CSWAP with control=q0: swap q1 and q2.
	// q1 gets q2's value (1), q2 gets q1's value (0).
	// Result: q0=1, q1=1, q2=0 -> index = 1 + 2 = 3.

	// But Controlled(SWAP,1) should match gate.CSWAP.
	c2, _ := builder.New("cswap-ref", 3).
		X(0).X(2).
		Apply(gate.CSWAP, 0, 1, 2).
		Build()
	sim2 := statevector.New(3)
	sim2.Evolve(c2)
	sv2 := sim2.StateVector()

	for i := range sv {
		if cmplx.Abs(sv[i]-sv2[i]) > 1e-10 {
			t.Errorf("state[%d]: C1-SWAP=%v CSWAP=%v", i, sv[i], sv2[i])
		}
	}
}

func TestMCX4Controls(t *testing.T) {
	// 5 qubits: 4 controls + 1 target.
	c, _ := builder.New("mcx4", 5).
		X(0).X(1).X(2).X(3).
		Apply(gate.MCX(4), 0, 1, 2, 3, 4).
		Build()
	sim := statevector.New(5)
	sim.Evolve(c)
	sv := sim.StateVector()
	// All controls set -> target flipped. |11111> = 31.
	for i, amp := range sv {
		prob := real(amp)*real(amp) + imag(amp)*imag(amp)
		if i == 31 {
			if math.Abs(prob-1.0) > 1e-10 {
				t.Errorf("state[%d] prob = %f, want 1.0", i, prob)
			}
		} else if prob > 1e-10 {
			t.Errorf("state[%d] prob = %f, want 0.0", i, prob)
		}
	}
}
