package statevector

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/qgo/circuit/builder"
	"github.com/splch/qgo/circuit/gate"
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
	// CZ on |1,+>: H(1) X(0) -> |1,+>, then CZ(0,1) -> |1,->
	// Start: |00>, X(0) -> |01> (qubit 0 = 1), H(1) -> |0>(1/sqrt2)(|0>+|1>)
	// Actually, let's do: H(0) to get |+0>, X(1) to get |+1>, CZ(0,1)
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
	// CSWAP(0,1,2): control=q0, swap q1 and q2
	// |101> (q0=1, q2=1) -> CSWAP -> |011> (q0=1, q1=1)
	// Wait, CSWAP matrix: when control (bit2=q0) is set, swap q1 and q2.
	// Let's set up |101>: X(0), X(2) -> index = 1+4 = 5
	// CSWAP(0,1,2): control=q0(bit2), q1(bit1), q2(bit0)
	// |101> has q0=1,q1=0,q2=1 -> swap q1,q2 -> q0=1,q1=1,q2=0 = |110> = idx 3
	// Actually in our convention: bit2(MSB)=q0, bit1=q1, bit0(LSB)=q2
	// But state index: bit at position q means qubit q is set.
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
