package densitymatrix

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/noise"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

func TestNew(t *testing.T) {
	s := New(2)
	rho := s.DensityMatrix()
	if len(rho) != 16 {
		t.Fatalf("expected 16 elements, got %d", len(rho))
	}
	if rho[0] != 1 {
		t.Errorf("rho[0][0] = %v, want 1", rho[0])
	}
	for i := 1; i < 16; i++ {
		if rho[i] != 0 {
			t.Errorf("rho[%d] = %v, want 0", i, rho[i])
		}
	}
}

func TestPurity_PureState(t *testing.T) {
	s := New(2)
	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	if err := s.Evolve(c); err != nil {
		t.Fatal(err)
	}
	p := s.Purity()
	if math.Abs(p-1.0) > 1e-10 {
		t.Errorf("purity = %v, want 1.0 for pure state", p)
	}
}

// TestNoiselessVsStatevector verifies that density matrix matches statevector
// for noiseless evolution.
func TestNoiselessVsStatevector(t *testing.T) {
	tests := []struct {
		name    string
		circuit func() *ir.Circuit
	}{
		{
			name: "Bell",
			circuit: func() *ir.Circuit {
				c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
				return c
			},
		},
		{
			name: "GHZ3",
			circuit: func() *ir.Circuit {
				c, _ := builder.New("ghz3", 3).H(0).CNOT(0, 1).CNOT(1, 2).Build()
				return c
			},
		},
		{
			name: "SingleH",
			circuit: func() *ir.Circuit {
				c, _ := builder.New("h", 1).H(0).Build()
				return c
			},
		},
		{
			name: "RZ",
			circuit: func() *ir.Circuit {
				c, _ := builder.New("rz", 1).RZ(math.Pi/4, 0).Build()
				return c
			},
		},
		{
			name: "TwoQubitSequence",
			circuit: func() *ir.Circuit {
				c, _ := builder.New("seq", 2).H(0).RZ(math.Pi/3, 1).CNOT(0, 1).H(1).Build()
				return c
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.circuit()
			nq := c.NumQubits()

			// Statevector simulation.
			sv := statevector.New(nq)
			if err := sv.Evolve(c); err != nil {
				t.Fatal(err)
			}
			psi := sv.StateVector()

			// Density matrix simulation.
			dm := New(nq)
			if err := dm.Evolve(c); err != nil {
				t.Fatal(err)
			}
			rho := dm.DensityMatrix()

			// Verify rho = |psi><psi|
			dim := 1 << nq
			for i := range dim {
				for j := range dim {
					expected := psi[i] * conj(psi[j])
					got := rho[i*dim+j]
					if cmplx.Abs(got-expected) > 1e-10 {
						t.Errorf("rho[%d][%d] = %v, want %v", i, j, got, expected)
					}
				}
			}
		})
	}
}

func TestFidelity_PureState(t *testing.T) {
	c, _ := builder.New("h", 1).H(0).Build()

	sv := statevector.New(1)
	if err := sv.Evolve(c); err != nil {
		t.Fatal(err)
	}
	psi := sv.StateVector()

	dm := New(1)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	f := dm.Fidelity(psi)
	if math.Abs(f-1.0) > 1e-10 {
		t.Errorf("fidelity = %v, want 1.0", f)
	}
}

func TestDepolarizing_ReducesPurity(t *testing.T) {
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.1))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.1))

	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()

	dm := New(2)
	dm.WithNoise(nm)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	p := dm.Purity()
	if p >= 1.0-1e-10 {
		t.Errorf("purity = %v, expected < 1.0 with depolarizing noise", p)
	}
	if p <= 0 {
		t.Errorf("purity = %v, expected > 0", p)
	}
}

func TestDepolarizing_MaximallyMixed(t *testing.T) {
	// With Kraus ops sqrt(1-p)I, sqrt(p/3){X,Y,Z}, maximally mixed occurs at p=3/4.
	// E(ρ) = (1-p)ρ + (p/3)(XρX + YρY + ZρZ) = I/2 when p=3/4.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.75))

	c, _ := builder.New("x", 1).X(0).Build()

	dm := New(1)
	dm.WithNoise(nm)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	rho := dm.DensityMatrix()
	// Maximally mixed: diag(0.5, 0.5), off-diag = 0.
	if math.Abs(real(rho[0])-0.5) > 1e-10 {
		t.Errorf("rho[0][0] = %v, want 0.5", rho[0])
	}
	if math.Abs(real(rho[3])-0.5) > 1e-10 {
		t.Errorf("rho[1][1] = %v, want 0.5", rho[3])
	}
}

func TestAmplitudeDamping(t *testing.T) {
	// Apply X (|0> -> |1>), then amplitude damping with gamma=0.5.
	// rho[0][0] should be gamma = 0.5 (probability of decaying to |0>).
	nm := noise.New()
	nm.AddGateError("X", noise.AmplitudeDamping(0.5))

	c, _ := builder.New("x", 1).X(0).Build()

	dm := New(1)
	dm.WithNoise(nm)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	rho := dm.DensityMatrix()
	// After X: |1><1|. After AD(0.5): rho[0][0] = gamma = 0.5.
	if math.Abs(real(rho[0])-0.5) > 1e-10 {
		t.Errorf("rho[0][0] = %v, want 0.5", rho[0])
	}
	// rho[1][1] = 1 - gamma = 0.5.
	if math.Abs(real(rho[3])-0.5) > 1e-10 {
		t.Errorf("rho[1][1] = %v, want 0.5", rho[3])
	}
}

func TestPhaseDamping(t *testing.T) {
	// H on |0> gives |+> = (|0>+|1>)/sqrt(2).
	// rho = [[0.5, 0.5], [0.5, 0.5]].
	// Phase damping with lambda: off-diag *= sqrt(1-lambda).
	lambda := 0.5
	nm := noise.New()
	nm.AddGateError("H", noise.PhaseDamping(lambda))

	c, _ := builder.New("h", 1).H(0).Build()

	dm := New(1)
	dm.WithNoise(nm)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	rho := dm.DensityMatrix()
	expected01 := complex(0.5*math.Sqrt(1-lambda), 0)
	if cmplx.Abs(rho[1]-expected01) > 1e-10 {
		t.Errorf("rho[0][1] = %v, want %v", rho[1], expected01)
	}
}

func TestReadoutError(t *testing.T) {
	dm := New(1)
	nm := noise.New()
	nm.AddReadoutError(0, noise.NewReadoutError(0.1, 0.2))
	dm.WithNoise(nm)

	// |0> state: probs = [1, 0].
	probs := dm.DiagonalProbs()
	noisy := dm.ApplyReadoutError(probs)

	// Expected: P(0) = (1-0.1)*1 + 0.2*0 = 0.9, P(1) = 0.1*1 + 0.8*0 = 0.1.
	if math.Abs(noisy[0]-0.9) > 1e-10 {
		t.Errorf("P(0) = %v, want 0.9", noisy[0])
	}
	if math.Abs(noisy[1]-0.1) > 1e-10 {
		t.Errorf("P(1) = %v, want 0.1", noisy[1])
	}
}

func TestTracePreservation(t *testing.T) {
	// After any evolution, Tr(rho) should be 1.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.1))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.1))

	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()

	dm := New(2)
	dm.WithNoise(nm)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	rho := dm.DensityMatrix()
	var tr float64
	dim := 1 << 2
	for i := range dim {
		tr += real(rho[i*dim+i])
	}
	if math.Abs(tr-1.0) > 1e-10 {
		t.Errorf("Tr(rho) = %v, want 1.0", tr)
	}
}

func TestPositivity(t *testing.T) {
	// Diagonal elements should be non-negative.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.3))

	c, _ := builder.New("seq", 2).H(0).RZ(math.Pi/3, 1).CNOT(0, 1).Build()

	dm := New(2)
	dm.WithNoise(nm)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	rho := dm.DensityMatrix()
	dim := 1 << 2
	for i := range dim {
		d := real(rho[i*dim+i])
		if d < -1e-10 {
			t.Errorf("rho[%d][%d] = %v, negative diagonal", i, i, d)
		}
	}
}

func TestRun(t *testing.T) {
	c, _ := builder.New("x", 1).WithClbits(1).X(0).Build()
	dm := New(1)
	counts, err := dm.Run(c, 100)
	if err != nil {
		t.Fatal(err)
	}
	// X gate should give |1> deterministically.
	if counts["1"] != 100 {
		t.Errorf("counts = %v, want {1: 100}", counts)
	}
}

func TestNoiseModelLookup(t *testing.T) {
	nm := noise.New()
	ch1 := noise.Depolarizing1Q(0.01)
	ch2 := noise.Depolarizing1Q(0.05)
	ch3 := noise.Depolarizing1Q(0.10)

	nm.AddGateQubitError("H", []int{0}, ch1)
	nm.AddGateError("H", ch2)
	nm.AddDefaultError(1, ch3)

	// Most specific: gate+qubits.
	if got := nm.Lookup("H", []int{0}); got != ch1 {
		t.Error("expected qubit-specific channel")
	}
	// Gate name match.
	if got := nm.Lookup("H", []int{1}); got != ch2 {
		t.Error("expected gate-name channel")
	}
	// Default.
	if got := nm.Lookup("X", []int{0}); got != ch3 {
		t.Error("expected default channel")
	}
	// No match.
	if got := nm.Lookup("CNOT", []int{0, 1}); got != nil {
		t.Error("expected nil for unmatched gate")
	}
}

func TestBitFlip(t *testing.T) {
	// |0> + bit flip p=1 should give |1>.
	nm := noise.New()
	nm.AddDefaultError(1, noise.BitFlip(1.0))

	c, _ := builder.New("id", 1).Apply(gate.I, 0).Build()
	dm := New(1)
	dm.WithNoise(nm)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	rho := dm.DensityMatrix()
	// Should be |1><1|.
	if math.Abs(real(rho[3])-1.0) > 1e-10 {
		t.Errorf("rho[1][1] = %v, want 1.0", rho[3])
	}
}

func TestPhaseFlip(t *testing.T) {
	// |+> + phase flip p=1 should give |->.
	// |+> = [[0.5, 0.5],[0.5, 0.5]]
	// Z|+> = |-> = [[0.5, -0.5],[-0.5, 0.5]]
	nm := noise.New()
	nm.AddGateError("H", noise.PhaseFlip(1.0))

	c, _ := builder.New("h", 1).H(0).Build()
	dm := New(1)
	dm.WithNoise(nm)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	rho := dm.DensityMatrix()
	// |-> state: rho[0][1] = -0.5.
	if math.Abs(real(rho[1])+0.5) > 1e-10 {
		t.Errorf("rho[0][1] = %v, want -0.5", rho[1])
	}
}

func BenchmarkEvolve8Q(b *testing.B) {
	c, _ := builder.New("bench", 8).
		H(0).H(1).H(2).H(3).H(4).H(5).H(6).H(7).
		CNOT(0, 1).CNOT(2, 3).CNOT(4, 5).CNOT(6, 7).
		Build()
	b.ResetTimer()
	for range b.N {
		dm := New(8)
		dm.Evolve(c) //nolint:errcheck
	}
}

func BenchmarkEvolveNoisy8Q(b *testing.B) {
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.01))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.01))

	c, _ := builder.New("bench", 8).
		H(0).H(1).H(2).H(3).H(4).H(5).H(6).H(7).
		CNOT(0, 1).CNOT(2, 3).CNOT(4, 5).CNOT(6, 7).
		Build()
	b.ResetTimer()
	for range b.N {
		dm := New(8)
		dm.WithNoise(nm)
		dm.Evolve(c) //nolint:errcheck
	}
}

func TestEvolveMCZ(t *testing.T) {
	// MCZ(2) on |111>: density matrix should match statevector.
	c, _ := builder.New("mcz", 3).
		X(0).X(1).X(2).
		Apply(gate.MCZ(2), 0, 1, 2).
		Build()

	dm := New(3)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := statevector.New(3)
	sv.Evolve(c)
	svState := sv.StateVector()

	fid := dm.Fidelity(svState)
	if math.Abs(fid-1.0) > 1e-8 {
		t.Errorf("MCZ(2) fidelity = %f, want 1.0", fid)
	}
}

func TestEvolveControlledH(t *testing.T) {
	// C2-H on |110>: density matrix should match statevector.
	c, _ := builder.New("c2h", 3).
		X(0).X(1).
		Apply(gate.Controlled(gate.H, 2), 0, 1, 2).
		Build()

	dm := New(3)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := statevector.New(3)
	sv.Evolve(c)
	svState := sv.StateVector()

	fid := dm.Fidelity(svState)
	if math.Abs(fid-1.0) > 1e-6 {
		t.Errorf("C2-H fidelity = %f, want 1.0", fid)
	}
}

func TestEvolveMCP(t *testing.T) {
	// MCP(π, 2) on |111>: should apply phase π to |111> (equivalent to MCZ).
	c, _ := builder.New("mcp", 3).
		X(0).X(1).X(2).
		Apply(gate.MCP(math.Pi, 2), 0, 1, 2).
		Build()

	dm := New(3)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := statevector.New(3)
	sv.Evolve(c)
	svState := sv.StateVector()

	// |111> = index 7, should get -1 phase.
	if cmplx.Abs(svState[7]-(-1)) > 1e-8 {
		t.Errorf("MCP(π,2) sv[7] = %v, want -1", svState[7])
	}

	fid := dm.Fidelity(svState)
	if math.Abs(fid-1.0) > 1e-8 {
		t.Errorf("MCP(π,2) fidelity = %f, want 1.0", fid)
	}
}

func TestRun_ZeroShots(t *testing.T) {
	c, _ := builder.New("x", 1).WithClbits(1).X(0).Build()
	dm := New(1)
	counts, err := dm.Run(c, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 0 {
		t.Errorf("expected empty counts for 0 shots, got %v", counts)
	}
}

func TestEvolve_EmptyCircuit(t *testing.T) {
	c, _ := builder.New("empty", 2).Build()
	dm := New(2)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}
	rho := dm.DensityMatrix()
	// Should be |00><00|: rho[0] = 1, rest = 0.
	if cmplx.Abs(rho[0]-1) > 1e-10 {
		t.Errorf("rho[0] = %v, want 1", rho[0])
	}
	for i := 1; i < len(rho); i++ {
		if cmplx.Abs(rho[i]) > 1e-10 {
			t.Errorf("rho[%d] = %v, want 0", i, rho[i])
		}
	}
}

func TestExpectPauliString_Mismatched(t *testing.T) {
	c, _ := builder.New("id", 2).Build()
	dm := New(2)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}
	ps := pauli.NewPauliString(1.0, map[int]pauli.Pauli{0: pauli.Z}, 3)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for mismatched qubit count")
		}
	}()
	dm.ExpectPauliString(ps)
}

func TestExpectPauliSum_Mismatched(t *testing.T) {
	c, _ := builder.New("id", 2).Build()
	dm := New(2)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}
	ps := pauli.NewPauliString(1.0, map[int]pauli.Pauli{0: pauli.Z}, 3)
	sum, err := pauli.NewPauliSum([]pauli.PauliString{ps})
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for mismatched qubit count")
		}
	}()
	dm.ExpectPauliSum(sum)
}

func TestEvolve_Reset(t *testing.T) {
	t.Run("X_then_Reset", func(t *testing.T) {
		// X(0) puts qubit in |1>, Reset(0) should bring it back to |0>.
		c, _ := builder.New("xreset", 1).X(0).Reset(0).Build()
		dm := New(1)
		if err := dm.Evolve(c); err != nil {
			t.Fatal(err)
		}
		rho := dm.DensityMatrix()
		// Should be |0><0|.
		if math.Abs(real(rho[0])-1.0) > 1e-10 {
			t.Errorf("rho[0][0] = %v, want 1.0", rho[0])
		}
		for i := 1; i < len(rho); i++ {
			if cmplx.Abs(rho[i]) > 1e-10 {
				t.Errorf("rho[%d] = %v, want 0", i, rho[i])
			}
		}
	})

	t.Run("H_then_Reset", func(t *testing.T) {
		// H(0) gives |+><+| = [[0.5, 0.5],[0.5, 0.5]].
		// Reset should give |0><0|.
		c, _ := builder.New("hreset", 1).H(0).Reset(0).Build()
		dm := New(1)
		if err := dm.Evolve(c); err != nil {
			t.Fatal(err)
		}
		rho := dm.DensityMatrix()
		if math.Abs(real(rho[0])-1.0) > 1e-10 {
			t.Errorf("rho[0][0] = %v, want 1.0", rho[0])
		}
		for i := 1; i < len(rho); i++ {
			if cmplx.Abs(rho[i]) > 1e-10 {
				t.Errorf("rho[%d] = %v, want 0", i, rho[i])
			}
		}
	})

	t.Run("Bell_Reset_Qubit0", func(t *testing.T) {
		// Bell state (|00>+|11>)/sqrt(2) then Reset(0).
		// ρ = 0.5*(|00><00| + |00><11| + |11><00| + |11><11|)
		// After reset qubit 0: ρ' = |0><0|_q0 ⊗ Tr_q0(ρ) = |0><0|_q0 ⊗ 0.5*(|0><0| + |1><1|)
		// = 0.5*(|00><00| + |10><10|)
		// 2-qubit density matrix is 4x4. Index (r,c) = r*4+c.
		// |00>=0, |01>=1 (q0=1), |10>=2 (q1=1), |11>=3.
		// After reset: rho[0*4+0]=0.5 (|00><00|), rho[2*4+2]=0.5 (|10><10|)
		c, _ := builder.New("bellreset", 2).H(0).CNOT(0, 1).Reset(0).Build()
		dm := New(2)
		if err := dm.Evolve(c); err != nil {
			t.Fatal(err)
		}
		rho := dm.DensityMatrix()
		dim := 4
		// Check diagonal: [0.5, 0, 0.5, 0]
		expectedDiag := []float64{0.5, 0, 0.5, 0}
		for i := range dim {
			got := real(rho[i*dim+i])
			if math.Abs(got-expectedDiag[i]) > 1e-10 {
				t.Errorf("rho[%d][%d] = %v, want %v", i, i, got, expectedDiag[i])
			}
		}
		// All off-diagonal should be zero (mixed state).
		for r := range dim {
			for c := range dim {
				if r != c {
					if cmplx.Abs(rho[r*dim+c]) > 1e-10 {
						t.Errorf("rho[%d][%d] = %v, want 0", r, c, rho[r*dim+c])
					}
				}
			}
		}
		// Trace should be 1.
		var tr float64
		for i := range dim {
			tr += real(rho[i*dim+i])
		}
		if math.Abs(tr-1.0) > 1e-10 {
			t.Errorf("Tr(rho) = %v, want 1.0", tr)
		}
	})

	t.Run("Reset_on_zero", func(t *testing.T) {
		// Reset on |0> should leave state unchanged.
		c, _ := builder.New("reset0", 1).Reset(0).Build()
		dm := New(1)
		if err := dm.Evolve(c); err != nil {
			t.Fatal(err)
		}
		rho := dm.DensityMatrix()
		if math.Abs(real(rho[0])-1.0) > 1e-10 {
			t.Errorf("rho[0][0] = %v, want 1.0", rho[0])
		}
		for i := 1; i < len(rho); i++ {
			if cmplx.Abs(rho[i]) > 1e-10 {
				t.Errorf("rho[%d] = %v, want 0", i, rho[i])
			}
		}
	})
}

// StatePrep tests

func TestStatePrep_FastPath_Plus(t *testing.T) {
	// Full-state prep on 1 qubit: |+> via fast path.
	s2 := 1.0 / math.Sqrt2
	c, err := builder.New("sp1", 1).
		StatePrep([]complex128{complex(s2, 0), complex(s2, 0)}, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	dm := New(1)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}
	rho := dm.DensityMatrix()
	// |+><+| = [[0.5, 0.5], [0.5, 0.5]]
	expected := []complex128{0.5, 0.5, 0.5, 0.5}
	for i, want := range expected {
		if cmplx.Abs(rho[i]-want) > 1e-10 {
			t.Errorf("rho[%d] = %v, want %v", i, rho[i], want)
		}
	}
}

func TestStatePrep_FastPath_Bell(t *testing.T) {
	// Full-state prep on 2 qubits: Bell state via fast path.
	s2 := 1.0 / math.Sqrt2
	c, err := builder.New("sp-bell", 2).
		StatePrep([]complex128{complex(s2, 0), 0, 0, complex(s2, 0)}, 0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	dm := New(2)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}

	// Compare with statevector approach: rho = |psi><psi|.
	psi := []complex128{complex(s2, 0), 0, 0, complex(s2, 0)}
	rho := dm.DensityMatrix()
	dim := 4
	for i := range dim {
		for j := range dim {
			want := psi[i] * conj(psi[j])
			got := rho[i*dim+j]
			if cmplx.Abs(got-want) > 1e-10 {
				t.Errorf("rho[%d][%d] = %v, want %v", i, j, got, want)
			}
		}
	}
}

func TestStatePrep_Purity(t *testing.T) {
	// State prep should produce a pure state (purity = 1).
	s2 := 1.0 / math.Sqrt2
	c, err := builder.New("sp-purity", 1).
		StatePrep([]complex128{complex(s2, 0), complex(s2, 0)}, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	dm := New(1)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}
	p := dm.Purity()
	if math.Abs(p-1.0) > 1e-10 {
		t.Errorf("purity = %v, want 1.0 for pure state", p)
	}
}

func TestNoiseAccumulation(t *testing.T) {
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.1))

	c, _ := builder.New("hhh", 1).H(0).H(0).H(0).Build()
	dm := New(1)
	dm.WithNoise(nm)
	if err := dm.Evolve(c); err != nil {
		t.Fatal(err)
	}
	p := dm.Purity()
	if p >= 1.0-1e-10 {
		t.Errorf("purity = %v, expected < 1.0 after noisy gates", p)
	}
}
