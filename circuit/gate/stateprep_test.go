package gate

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestStatePrep_1Q(t *testing.T) {
	// |+> state: {1/sqrt2, 1/sqrt2}
	s2 := 1.0 / math.Sqrt2
	g, err := StatePrep([]complex128{complex(s2, 0), complex(s2, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if g.Name() != "StatePrep" {
		t.Errorf("Name() = %q, want %q", g.Name(), "StatePrep")
	}
	if g.Qubits() != 1 {
		t.Errorf("Qubits() = %d, want 1", g.Qubits())
	}
	if g.Matrix() != nil {
		t.Error("Matrix() should be nil for pseudo-gate")
	}
	if g.Params() != nil {
		t.Error("Params() should be nil")
	}
	sp := g.(StatePrepable)
	amps := sp.Amplitudes()
	if len(amps) != 2 {
		t.Fatalf("Amplitudes() length = %d, want 2", len(amps))
	}
	if cmplx.Abs(amps[0]-complex(s2, 0)) > eps {
		t.Errorf("Amplitudes()[0] = %v, want %v", amps[0], complex(s2, 0))
	}
	if cmplx.Abs(amps[1]-complex(s2, 0)) > eps {
		t.Errorf("Amplitudes()[1] = %v, want %v", amps[1], complex(s2, 0))
	}
}

func TestStatePrep_2Q(t *testing.T) {
	// Bell state: (|00> + |11>) / sqrt(2)
	s2 := 1.0 / math.Sqrt2
	amps := []complex128{complex(s2, 0), 0, 0, complex(s2, 0)}
	g, err := StatePrep(amps)
	if err != nil {
		t.Fatal(err)
	}
	if g.Qubits() != 2 {
		t.Errorf("Qubits() = %d, want 2", g.Qubits())
	}
}

func TestStatePrep_Invalid(t *testing.T) {
	// Empty.
	_, err := StatePrep(nil)
	if err == nil {
		t.Error("expected error for nil amplitudes")
	}

	// Not power of 2.
	_, err = StatePrep([]complex128{1, 0, 0})
	if err == nil {
		t.Error("expected error for length 3")
	}

	// Not normalized.
	_, err = StatePrep([]complex128{1, 1})
	if err == nil {
		t.Error("expected error for unnormalized amplitudes")
	}
}

func TestStatePrep_Inverse(t *testing.T) {
	s2 := 1.0 / math.Sqrt2
	g, err := StatePrep([]complex128{complex(s2, 0), complex(s2, 0)})
	if err != nil {
		t.Fatal(err)
	}
	inv := g.Inverse()
	if inv.Name() != "StatePrep†" {
		t.Errorf("Inverse().Name() = %q, want %q", inv.Name(), "StatePrep†")
	}
	if inv.Qubits() != 1 {
		t.Errorf("Inverse().Qubits() = %d, want 1", inv.Qubits())
	}
	// Inverse of inverse should give back original name.
	orig := inv.Inverse()
	if orig.Name() != "StatePrep" {
		t.Errorf("Inverse().Inverse().Name() = %q, want %q", orig.Name(), "StatePrep")
	}
}

func TestStatePrep_Decompose1Q(t *testing.T) {
	s2 := 1.0 / math.Sqrt2

	tests := []struct {
		name string
		amps []complex128
	}{
		{"|+>", []complex128{complex(s2, 0), complex(s2, 0)}},
		{"|1>", []complex128{0, 1}},
		{"|0>", []complex128{1, 0}},
		{"|->", []complex128{complex(s2, 0), complex(-s2, 0)}},
		{"|i>", []complex128{complex(s2, 0), complex(0, s2)}},
		// Test with nonzero arg(amps[0]) to verify global phase doesn't corrupt relative phase.
		{"complex_phase", []complex128{complex(0, s2), complex(s2, 0)}},
		{"both_complex", []complex128{complex(s2*0.5, s2*math.Sqrt(3)/2), complex(-s2*math.Sqrt(3)/2, s2*0.5)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, err := StatePrep(tc.amps)
			if err != nil {
				t.Fatal(err)
			}
			ops := g.Decompose([]int{0})
			if ops == nil && (cmplx.Abs(tc.amps[0]-1) > 1e-10 || cmplx.Abs(tc.amps[1]) > 1e-10) {
				t.Fatal("Decompose returned nil for non-trivial state")
			}

			// Simulate the decomposed gates manually.
			state := []complex128{1, 0}
			for _, op := range ops {
				m := op.Gate.Matrix()
				if m == nil {
					continue
				}
				a, b := state[0], state[1]
				state[0] = m[0]*a + m[1]*b
				state[1] = m[2]*a + m[3]*b
			}

			// Compare magnitudes (global phase is irrelevant).
			for i := range tc.amps {
				if math.Abs(cmplx.Abs(state[i])-cmplx.Abs(tc.amps[i])) > 1e-8 {
					t.Errorf("|state[%d]| = %f, want %f", i, cmplx.Abs(state[i]), cmplx.Abs(tc.amps[i]))
				}
			}

			// Check that relative phase between non-zero amplitudes matches.
			if cmplx.Abs(tc.amps[0]) > 1e-10 && cmplx.Abs(tc.amps[1]) > 1e-10 {
				wantPhase := cmplx.Phase(tc.amps[1]) - cmplx.Phase(tc.amps[0])
				gotPhase := cmplx.Phase(state[1]) - cmplx.Phase(state[0])
				phaseDiff := math.Abs(math.Remainder(gotPhase-wantPhase, 2*math.Pi))
				if phaseDiff > 1e-8 {
					t.Errorf("relative phase = %f, want %f (diff=%f)", gotPhase, wantPhase, phaseDiff)
				}
			}
		})
	}
}

func TestMustStatePrep_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid amplitudes")
		}
	}()
	MustStatePrep([]complex128{1, 1}) // not normalized
}

func TestMustStatePrep_OK(t *testing.T) {
	g := MustStatePrep([]complex128{1, 0})
	if g.Name() != "StatePrep" {
		t.Errorf("Name() = %q, want %q", g.Name(), "StatePrep")
	}
}

func TestStatePrep_AmplitudesDefensiveCopy(t *testing.T) {
	amps := []complex128{1, 0}
	g, err := StatePrep(amps)
	if err != nil {
		t.Fatal(err)
	}
	// Mutate original.
	amps[0] = 0
	// Gate should be unaffected.
	sp := g.(StatePrepable)
	got := sp.Amplitudes()
	if cmplx.Abs(got[0]-1) > eps {
		t.Error("amplitudes were not defensively copied on construction")
	}
	// Mutate returned value.
	got[0] = 0
	got2 := sp.Amplitudes()
	if cmplx.Abs(got2[0]-1) > eps {
		t.Error("amplitudes were not defensively copied on retrieval")
	}
}

func TestStatePrepInv_Decompose(t *testing.T) {
	s2 := 1.0 / math.Sqrt2
	g, err := StatePrep([]complex128{complex(s2, 0), complex(s2, 0)})
	if err != nil {
		t.Fatal(err)
	}
	inv := g.Inverse()
	ops := inv.Decompose([]int{0})
	// The inverse decomposition should exist.
	if ops == nil {
		t.Fatal("inverse decompose returned nil")
	}
}

func TestStatePrepInv_DecomposeBadQubits(t *testing.T) {
	g, err := StatePrep([]complex128{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	inv := g.Inverse()
	// Wrong number of qubits.
	ops := inv.Decompose([]int{0, 1})
	if ops != nil {
		t.Error("expected nil for wrong qubit count")
	}
}

func TestStatePrep_DecomposeBadQubits(t *testing.T) {
	g, err := StatePrep([]complex128{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	// Wrong number of qubits.
	ops := g.Decompose([]int{0, 1})
	if ops != nil {
		t.Error("expected nil for wrong qubit count")
	}
}
