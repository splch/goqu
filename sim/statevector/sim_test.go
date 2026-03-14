package statevector

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/sim/pauli"
)

const eps = 1e-10

func TestBellState(t *testing.T) {
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2

	// |Φ+> = (|00> + |11>) / √2
	want := []complex128{complex(s2, 0), 0, 0, complex(s2, 0)}
	assertStateClose(t, sv, want)
}

func TestGHZ3(t *testing.T) {
	c, err := builder.New("ghz3", 3).
		H(0).
		CNOT(0, 1).
		CNOT(1, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(3)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2
	want := []complex128{complex(s2, 0), 0, 0, 0, 0, 0, 0, complex(s2, 0)}
	assertStateClose(t, sv, want)
}

func TestSingleX(t *testing.T) {
	c, err := builder.New("x", 1).X(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	want := []complex128{0, 1}
	assertStateClose(t, sv, want)
}

func TestSingleH(t *testing.T) {
	c, err := builder.New("h", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2
	want := []complex128{complex(s2, 0), complex(s2, 0)}
	assertStateClose(t, sv, want)
}

func TestHH_Identity(t *testing.T) {
	c, err := builder.New("hh", 1).H(0).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	want := []complex128{1, 0}
	assertStateClose(t, sv, want)
}

func TestQFT3_FromZero(t *testing.T) {
	// QFT on |000> should give uniform superposition.
	c, err := builder.New("qft3-zero", 3).
		H(0).
		Apply(gate.CP(math.Pi/2), 1, 0).
		Apply(gate.CP(math.Pi/4), 2, 0).
		H(1).
		Apply(gate.CP(math.Pi/2), 2, 1).
		H(2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(3)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	amp := 1.0 / math.Sqrt(8)
	for i, v := range sv {
		if math.Abs(cmplx.Abs(v)-amp) > eps {
			t.Errorf("|sv[%d]| = %f, want %f", i, cmplx.Abs(v), amp)
		}
	}
}

func TestMeasurementCounts(t *testing.T) {
	// Bell state should produce ~50% |00> and ~50% |11>.
	c, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	counts, err := sim.Run(c, 10000)
	if err != nil {
		t.Fatal(err)
	}

	// Should only have "00" and "11" entries.
	for k := range counts {
		if k != "00" && k != "11" {
			t.Errorf("unexpected measurement outcome: %q", k)
		}
	}
	// Each should be roughly 5000 (±500 for statistical noise).
	c00 := counts["00"]
	c11 := counts["11"]
	if c00 < 4000 || c00 > 6000 {
		t.Errorf("counts[00] = %d, expected ~5000", c00)
	}
	if c11 < 4000 || c11 > 6000 {
		t.Errorf("counts[11] = %d, expected ~5000", c11)
	}
}

func TestCCX_Toffoli(t *testing.T) {
	// CCX flips target when both controls are |1>.
	// Start: |110> = X(1), X(2) on 3-qubit register -> index 6
	// CCX(2,1,0) should give |111> = index 7
	c, err := builder.New("toffoli", 3).
		X(1).
		X(2).
		CCX(2, 1, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(3)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// Should be |111> = index 7
	for i, v := range sv {
		if i == 7 {
			if cmplx.Abs(v-1) > eps {
				t.Errorf("sv[7] = %v, want 1", v)
			}
		} else {
			if cmplx.Abs(v) > eps {
				t.Errorf("sv[%d] = %v, want 0", i, v)
			}
		}
	}
}

func TestSWAP(t *testing.T) {
	// Start: |01> (X on qubit 0), SWAP(0,1) should give |10>
	c, err := builder.New("swap", 2).
		X(0).
		SWAP(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// |10> = index 2
	want := []complex128{0, 0, 1, 0}
	assertStateClose(t, sv, want)
}

func TestExpectationValue(t *testing.T) {
	// |0> state: <Z> = +1
	sim := New(1)
	c, err := builder.New("z0", 1).Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	ev := sim.ExpectationValue([]int{0})
	if math.Abs(ev-1.0) > eps {
		t.Errorf("<Z>|0> = %f, want 1.0", ev)
	}

	// |1> state: <Z> = -1
	c, err = builder.New("z1", 1).X(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	ev = sim.ExpectationValue([]int{0})
	if math.Abs(ev-(-1.0)) > eps {
		t.Errorf("<Z>|1> = %f, want -1.0", ev)
	}

	// |+> state: <Z> = 0
	c, err = builder.New("z+", 1).H(0).Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	ev = sim.ExpectationValue([]int{0})
	if math.Abs(ev) > eps {
		t.Errorf("<Z>|+> = %f, want 0.0", ev)
	}
}

func assertStateClose(t *testing.T, got, want []complex128) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("state size %d, want %d", len(got), len(want))
	}
	for i := range got {
		if cmplx.Abs(got[i]-want[i]) > eps {
			t.Errorf("state[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// --- 2Q kernel correctness tests ---

func TestGate2_CNOT_NonAdjacent(t *testing.T) {
	// CNOT on non-adjacent qubits (q0=0, q1=2) in 4-qubit system.
	// |0001> (X on q0) -> CNOT(0,2) -> |0101> (q0 and q2 set)
	c, err := builder.New("cnot-nonadj", 4).
		X(0).
		CNOT(0, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(4)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// |0101> = bit0=1, bit2=1 = index 5
	want := make([]complex128, 16)
	want[5] = 1
	assertStateClose(t, sv, want)
}

func TestGate2_CZ(t *testing.T) {
	// H(0) to get |+0>, X(1) to get |+1>, CZ(0,1).
	// |+1> = 1/sqrt2 (|01> + |11>), CZ negates |11>: 1/sqrt2 (|01> - |11>) = |−1>
	c, err := builder.New("cz", 2).
		H(0).
		X(1).
		CZ(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2
	// |01> = idx 2, |11> = idx 3
	want := []complex128{0, 0, complex(s2, 0), complex(-s2, 0)}
	assertStateClose(t, sv, want)
}

func TestGate2_CY(t *testing.T) {
	// CY on |10> (control q0=1, target q1=0): should apply Y to q1.
	// |10> -> CY -> i|11>
	c, err := builder.New("cy", 2).
		X(0).
		Apply(gate.CY, 0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// CY matrix: |10> -> 1i*|11>, confirmed from matrix row 3: m[14]=1i
	want := []complex128{0, 0, 0, 1i}
	assertStateClose(t, sv, want)
}

func TestGate2_Diagonal_CP(t *testing.T) {
	// CP(pi) = CZ. Test via the diagonal kernel path.
	// Same test as CZ but using CP(pi).
	c, err := builder.New("cp-pi", 2).
		H(0).
		X(1).
		Apply(gate.CP(math.Pi), 0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2
	want := []complex128{0, 0, complex(s2, 0), complex(-s2, 0)}
	assertStateClose(t, sv, want)
}

func TestGate2_Diagonal_CRZ(t *testing.T) {
	// CRZ(pi) on |11>: control=q0, target=q1
	// |11>: CRZ applies e^{i*pi/2} to |11> = i
	c, err := builder.New("crz", 2).
		X(0).
		X(1).
		Apply(gate.CRZ(math.Pi), 0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// CRZ(pi) matrix: diag(1, 1, e^{-i*pi/2}, e^{i*pi/2}) = diag(1, 1, -i, i)
	// |11> maps to row 3 (index 11): m[15] = e^{i*pi/2} = i
	want := []complex128{0, 0, 0, 1i}
	assertStateClose(t, sv, want)
}

func TestGate2_Controlled_CRX(t *testing.T) {
	// CRX(pi) on |10> should flip target: |10> -> -i|11>
	c, err := builder.New("crx", 2).
		X(0).
		Apply(gate.CRX(math.Pi), 0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// CRX(pi): cos(pi/2)=0, sin(pi/2)=1
	// Row 2: [0, 0, 0, -i] -> |10> maps to -i|11>
	want := []complex128{0, 0, 0, -1i}
	assertStateClose(t, sv, want)
}

func TestGate2_Controlled_CRY(t *testing.T) {
	// CRY(pi) on |10>: should map to |11>
	c, err := builder.New("cry", 2).
		X(0).
		Apply(gate.CRY(math.Pi), 0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// CRY(pi): cos(pi/2)=0, sin(pi/2)=1
	// Row 3: m[14]=sin=1, so |10> -> +|11>
	want := []complex128{0, 0, 0, 1}
	assertStateClose(t, sv, want)
}

func TestGate2_Generic_MS(t *testing.T) {
	// MS gate on |00>: should produce (1/sqrt2)(|00> - i*e^{-i(phi0+phi1)}|11>)
	phi0, phi1 := 0.0, 0.0
	c, err := builder.New("ms", 2).
		Apply(gate.MS(phi0, phi1), 0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2
	// MS(0,0): 1/sqrt2 * [[1,0,0,-i],[0,1,-i,0],[0,-i,1,0],[-i,0,0,1]]
	// |00> -> 1/sqrt2 * (|00> - i|11>)
	want := []complex128{complex(s2, 0), 0, 0, complex(0, -s2)}
	assertStateClose(t, sv, want)
}

// --- 3Q kernel correctness tests ---

func TestGate3_CCX_NonAdjacent(t *testing.T) {
	// CCX on non-adjacent qubits in 5-qubit system: q0=0, q1=2, q2=4
	// Set q0=1, q2=1 -> CCX(0,2,4) should flip q4
	c, err := builder.New("ccx-nonadj", 5).
		X(0).
		X(2).
		Apply(gate.CCX, 0, 2, 4).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(5)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	// |10101> = bit0=1, bit2=1, bit4=1 = 1+4+16 = 21
	want := make([]complex128, 32)
	want[21] = 1
	assertStateClose(t, sv, want)
}

func TestGate3_CSWAP(t *testing.T) {
	// CSWAP(0,1,2): control=q0, swap q1 and q2.
	// State index: bit at position q means qubit q is set.
	// |101>: q0=1, q1=0, q2=1 -> index = (1<<0)|(1<<2) = 5
	// After CSWAP: q0=1, q1=1, q2=0 -> index = (1<<0)|(1<<1) = 3
	c, err := builder.New("cswap", 3).
		X(0).
		X(2).
		Apply(gate.CSWAP, 0, 1, 2).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	sim := New(3)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}

	sv := sim.StateVector()
	want := []complex128{0, 0, 0, 1, 0, 0, 0, 0}
	assertStateClose(t, sv, want)
}

// --- Benchmarks ---

func BenchmarkSimulate16(b *testing.B) {
	// Build a 16-qubit GHZ circuit.
	bld := builder.New("ghz16", 16)
	bld.H(0)
	for i := range 15 {
		bld.CNOT(i, i+1)
	}
	c, err := bld.Build()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		sim := New(16)
		sim.Evolve(c)
	}
}

func BenchmarkCNOT16(b *testing.B) {
	c, err := builder.New("cnot16", 16).CNOT(0, 15).Build()
	if err != nil {
		b.Fatal(err)
	}
	sim := New(16)
	b.ResetTimer()
	for range b.N {
		sim.state[0] = 1
		sim.Evolve(c)
	}
}

func BenchmarkCNOT20(b *testing.B) {
	c, err := builder.New("cnot20", 20).CNOT(0, 19).Build()
	if err != nil {
		b.Fatal(err)
	}
	sim := New(20)
	b.ResetTimer()
	for range b.N {
		sim.state[0] = 1
		sim.Evolve(c)
	}
}

func BenchmarkCP16(b *testing.B) {
	c, err := builder.New("cp16", 16).
		Apply(gate.CP(math.Pi/4), 0, 15).
		Build()
	if err != nil {
		b.Fatal(err)
	}
	sim := New(16)
	b.ResetTimer()
	for range b.N {
		sim.state[0] = 1
		sim.Evolve(c)
	}
}

func BenchmarkMS16(b *testing.B) {
	c, err := builder.New("ms16", 16).
		Apply(gate.MS(0.5, 0.3), 0, 15).
		Build()
	if err != nil {
		b.Fatal(err)
	}
	sim := New(16)
	b.ResetTimer()
	for range b.N {
		sim.state[0] = 1
		sim.Evolve(c)
	}
}

func TestRun_ZeroShots(t *testing.T) {
	c, err := builder.New("x", 1).X(0).MeasureAll().Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(1)
	counts, err := sim.Run(c, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 0 {
		t.Errorf("expected empty counts for 0 shots, got %v", counts)
	}
}

func TestEvolve_EmptyCircuit(t *testing.T) {
	c, err := builder.New("empty", 2).Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	if cmplx.Abs(sv[0]-1) > eps {
		t.Errorf("sv[0] = %v, want 1", sv[0])
	}
	for i := 1; i < len(sv); i++ {
		if cmplx.Abs(sv[i]) > eps {
			t.Errorf("sv[%d] = %v, want 0", i, sv[i])
		}
	}
}

func TestExpectPauliString_MismatchedQubits(t *testing.T) {
	c, err := builder.New("id", 2).Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	ps := pauli.NewPauliString(1.0, map[int]pauli.Pauli{0: pauli.Z}, 3)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for mismatched qubit count")
		}
	}()
	sim.ExpectPauliString(ps)
}

func TestExpectPauliSum_MismatchedQubits(t *testing.T) {
	c, err := builder.New("id", 2).Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(2)
	if err := sim.Evolve(c); err != nil {
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
	sim.ExpectPauliSum(sum)
}

func TestEvolve_Reset(t *testing.T) {
	t.Run("X_then_Reset", func(t *testing.T) {
		// X(0) puts qubit in |1>, Reset(0) should bring it back to |0>.
		c, err := builder.New("xreset", 1).X(0).Reset(0).Build()
		if err != nil {
			t.Fatal(err)
		}
		sim := New(1)
		if err := sim.Evolve(c); err != nil {
			t.Fatal(err)
		}
		sv := sim.StateVector()
		want := []complex128{1, 0}
		assertStateClose(t, sv, want)
	})

	t.Run("H_then_Reset", func(t *testing.T) {
		// H(0) puts qubit in |+> = (|0>+|1>)/sqrt(2), Reset should give |0> with norm 1.
		c, err := builder.New("hreset", 1).H(0).Reset(0).Build()
		if err != nil {
			t.Fatal(err)
		}
		sim := New(1)
		if err := sim.Evolve(c); err != nil {
			t.Fatal(err)
		}
		sv := sim.StateVector()
		want := []complex128{1, 0}
		assertStateClose(t, sv, want)
	})

	t.Run("Bell_Reset_Qubit0", func(t *testing.T) {
		// H(0), CNOT(0,1) -> (|00>+|11>)/sqrt(2)
		// Reset(0) should give: qubit 0 = |0>, qubit 1 mixed.
		// After reset: |00> with prob 1/2 and |01> with prob 1/2 -> but in Evolve
		// (deterministic), amplitude for |00> gets norm of (1/sqrt2, 0) pair = 1/sqrt2,
		// and amplitude for |01> gets norm of (0, 1/sqrt2) pair = 1/sqrt2.
		// Result: state = 1/sqrt2 |00> + 1/sqrt2 |10> (qubit 1 is bit 1).
		// Index mapping: |00>=0, |01>=1 (q0=1), |10>=2 (q1=1), |11>=3.
		// Bell state: sv[0]=1/sqrt2, sv[3]=1/sqrt2.
		// Reset qubit 0: pairs are (i0, i1) where i0 has q0=0, i1 has q0=1.
		//   pair (0, 1): a0=1/sqrt2, a1=0 -> norm=1/sqrt2 -> state[0]=1/sqrt2, state[1]=0
		//   pair (2, 3): a0=0, a1=1/sqrt2 -> norm=1/sqrt2 -> state[2]=1/sqrt2, state[3]=0
		c, err := builder.New("bellreset", 2).H(0).CNOT(0, 1).Reset(0).Build()
		if err != nil {
			t.Fatal(err)
		}
		sim := New(2)
		if err := sim.Evolve(c); err != nil {
			t.Fatal(err)
		}
		sv := sim.StateVector()
		s2 := 1.0 / math.Sqrt2
		want := []complex128{complex(s2, 0), 0, complex(s2, 0), 0}
		assertStateClose(t, sv, want)

		// Verify normalization.
		var total float64
		for _, a := range sv {
			total += real(a)*real(a) + imag(a)*imag(a)
		}
		if math.Abs(total-1.0) > eps {
			t.Errorf("norm = %f, want 1.0", total)
		}
	})

	t.Run("Reset_on_zero", func(t *testing.T) {
		// Reset on |0> should leave state unchanged.
		c, err := builder.New("reset0", 1).Reset(0).Build()
		if err != nil {
			t.Fatal(err)
		}
		sim := New(1)
		if err := sim.Evolve(c); err != nil {
			t.Fatal(err)
		}
		sv := sim.StateVector()
		want := []complex128{1, 0}
		assertStateClose(t, sv, want)
	})
}

func TestParallelThreshold_17Q(t *testing.T) {
	c, err := builder.New("par17", 17).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(17)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	s2 := 1.0 / math.Sqrt2

	// |00...0> = index 0
	if math.Abs(cmplx.Abs(sv[0])-s2) > eps {
		t.Errorf("|sv[0]| = %f, want %f", cmplx.Abs(sv[0]), s2)
	}
	// |11...0> = bit0=1, bit1=1 = index 3
	if math.Abs(cmplx.Abs(sv[3])-s2) > eps {
		t.Errorf("|sv[3]| = %f, want %f", cmplx.Abs(sv[3]), s2)
	}
}

// --- StatePrep tests ---

func TestStatePrep_FastPath_Plus(t *testing.T) {
	// Full-state prep on 1 qubit: |+> via fast path.
	s2 := 1.0 / math.Sqrt2
	c, err := builder.New("sp1", 1).
		StatePrep([]complex128{complex(s2, 0), complex(s2, 0)}, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	want := []complex128{complex(s2, 0), complex(s2, 0)}
	assertStateClose(t, sv, want)
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
	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	want := []complex128{complex(s2, 0), 0, 0, complex(s2, 0)}
	assertStateClose(t, sv, want)
}

func TestStatePrep_Decompose_1Q(t *testing.T) {
	// Test the decomposition path: create a 2-qubit circuit but only state-prep qubit 1.
	// Use builder.Apply with a 1-qubit StatePrep on qubit 1.
	s2 := 1.0 / math.Sqrt2
	g := gate.MustStatePrep([]complex128{complex(s2, 0), complex(s2, 0)})
	c, err := builder.New("sp-decompose", 2).
		Apply(g, 1).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(2)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	// qubit 0 = |0>, qubit 1 = |+> = 1/sqrt2(|0>+|1>)
	// |00> + |10> / sqrt(2): idx 0 and idx 2 (bit 1 set = qubit 1)
	want := []complex128{complex(s2, 0), 0, complex(s2, 0), 0}
	assertStateClose(t, sv, want)
}

func TestStatePrep_One(t *testing.T) {
	// Prepare |1> state.
	c, err := builder.New("sp1", 1).
		StatePrep([]complex128{0, 1}, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	want := []complex128{0, 1}
	assertStateClose(t, sv, want)
}

func TestStatePrep_Normalized(t *testing.T) {
	// Prepare an arbitrary 1-qubit state and verify normalization.
	s2 := 1.0 / math.Sqrt2
	c, err := builder.New("sp-arb", 1).
		StatePrep([]complex128{complex(s2, 0), complex(0, s2)}, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	var norm float64
	for _, v := range sv {
		norm += real(v)*real(v) + imag(v)*imag(v)
	}
	if math.Abs(norm-1.0) > eps {
		t.Errorf("norm = %f, want 1.0", norm)
	}
}

func TestEvolve_DelayIsNoOp(t *testing.T) {
	// H(0) then Delay then nothing else — state should be |+>.
	c, err := builder.New("hdelay", 1).
		H(0).
		Delay(0, 100, gate.UnitNs).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	sim := New(1)
	if err := sim.Evolve(c); err != nil {
		t.Fatal(err)
	}
	sv := sim.StateVector()
	inv := 1.0 / math.Sqrt(2)
	want := []complex128{complex(inv, 0), complex(inv, 0)}
	assertStateClose(t, sv, want)
}
