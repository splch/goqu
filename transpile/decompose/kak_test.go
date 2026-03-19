package decompose

import (
	"math"
	"math/cmplx"
	"math/rand/v2"
	"testing"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

const kakTol = 1e-6

// kakCircuitUnitary computes the 4x4 unitary for a sequence of KAK output
// operations on a 2-qubit system (qubits q0 and q1).
func kakCircuitUnitary(ops []ir.Operation, q0, q1 int) []complex128 {
	return OpsToUnitary4(ops, q0, q1)
}

func countCNOTs(ops []ir.Operation) int {
	n := 0
	for _, op := range ops {
		if op.Gate == gate.CNOT {
			n++
		}
	}
	return n
}

func TestKAK_Identity(t *testing.T) {
	m := Eye(4)
	ops := KAK(m, 0, 1)
	if len(ops) != 0 {
		t.Errorf("KAK(Identity): expected 0 ops, got %d", len(ops))
	}
}

func TestKAK_CNOT(t *testing.T) {
	m := gate.CNOT.Matrix()
	ops := KAK(m, 0, 1)
	if ops == nil {
		t.Fatal("KAK(CNOT): returned nil")
	}
	cx := countCNOTs(ops)
	if cx != 1 {
		t.Errorf("KAK(CNOT): expected 1 CNOT, got %d", cx)
	}
	got := kakCircuitUnitary(ops, 0, 1)
	if _, ok := GlobalPhase(got, m, kakTol); !ok {
		t.Error("KAK(CNOT): reconstructed unitary does not match CNOT up to global phase")
	}
}

func TestKAK_SWAP(t *testing.T) {
	m := gate.SWAP.Matrix()
	ops := KAK(m, 0, 1)
	if ops == nil {
		t.Fatal("KAK(SWAP): returned nil")
	}
	cx := countCNOTs(ops)
	if cx != 3 {
		t.Errorf("KAK(SWAP): expected 3 CNOTs, got %d", cx)
	}
	got := kakCircuitUnitary(ops, 0, 1)
	if _, ok := GlobalPhase(got, m, kakTol); !ok {
		t.Error("KAK(SWAP): reconstructed unitary does not match SWAP up to global phase")
	}
}

func TestKAK_CZ(t *testing.T) {
	m := gate.CZ.Matrix()
	ops := KAK(m, 0, 1)
	if ops == nil {
		t.Fatal("KAK(CZ): returned nil")
	}
	cx := countCNOTs(ops)
	if cx > 3 {
		t.Errorf("KAK(CZ): expected at most 3 CNOTs, got %d", cx)
	}
	got := kakCircuitUnitary(ops, 0, 1)
	if _, ok := GlobalPhase(got, m, kakTol); !ok {
		t.Logf("original: %v", m)
		t.Logf("reconstructed: %v", got)
		t.Error("KAK(CZ): reconstructed unitary does not match CZ up to global phase")
	}
}

func TestKAK_CP(t *testing.T) {
	phi := math.Pi / 4
	m := gate.CP(phi).Matrix()
	ops := KAK(m, 0, 1)
	if ops == nil {
		t.Fatal("KAK(CP(pi/4)): returned nil")
	}
	cx := countCNOTs(ops)
	if cx > 3 {
		t.Errorf("KAK(CP(pi/4)): expected at most 3 CNOTs, got %d", cx)
	}
	got := kakCircuitUnitary(ops, 0, 1)
	if _, ok := GlobalPhase(got, m, kakTol); !ok {
		t.Error("KAK(CP(pi/4)): reconstructed unitary does not match up to global phase")
	}
}

func TestKAK_CY(t *testing.T) {
	m := gate.CY.Matrix()
	ops := KAK(m, 0, 1)
	if ops == nil {
		t.Fatal("KAK(CY): returned nil")
	}
	cx := countCNOTs(ops)
	if cx > 3 {
		t.Errorf("KAK(CY): expected at most 3 CNOTs, got %d", cx)
	}
	got := kakCircuitUnitary(ops, 0, 1)
	if _, ok := GlobalPhase(got, m, kakTol); !ok {
		t.Error("KAK(CY): reconstructed unitary does not match up to global phase")
	}
}

func TestKAK_LocalUnitary(t *testing.T) {
	// H⊗X should need 0 CNOTs.
	m := Tensor(gate.H.Matrix(), 2, gate.X.Matrix(), 2)
	ops := KAK(m, 0, 1)
	cx := countCNOTs(ops)
	if cx != 0 {
		t.Errorf("KAK(H⊗X): expected 0 CNOTs, got %d", cx)
	}
	if len(ops) > 0 {
		got := kakCircuitUnitary(ops, 0, 1)
		if _, ok := GlobalPhase(got, m, kakTol); !ok {
			t.Error("KAK(H⊗X): reconstructed unitary does not match up to global phase")
		}
	}
}

func TestKAK_MaxCNOTCount(t *testing.T) {
	gates := []struct {
		name string
		mat  []complex128
	}{
		{"CNOT", gate.CNOT.Matrix()},
		{"SWAP", gate.SWAP.Matrix()},
		{"CZ", gate.CZ.Matrix()},
		{"CY", gate.CY.Matrix()},
		{"CP(pi/4)", gate.CP(math.Pi / 4).Matrix()},
		{"CRZ(pi/3)", gate.CRZ(math.Pi / 3).Matrix()},
	}
	for _, tc := range gates {
		t.Run(tc.name, func(t *testing.T) {
			ops := KAK(tc.mat, 0, 1)
			cx := countCNOTs(ops)
			if cx > 3 {
				t.Errorf("KAK(%s): used %d CNOTs, max should be 3", tc.name, cx)
			}
		})
	}
}

func TestKAK_DifferentQubitIndices(t *testing.T) {
	m := gate.CNOT.Matrix()
	ops := KAK(m, 2, 5)
	if ops == nil {
		t.Fatal("KAK(CNOT, q0=2, q1=5): returned nil")
	}
	for i, op := range ops {
		for _, q := range op.Qubits {
			if q != 2 && q != 5 {
				t.Errorf("op[%d] references unexpected qubit %d", i, q)
			}
		}
	}
}

func TestKAK_IdentityScaled(t *testing.T) {
	m := MatScale(Eye(4), complex(0, 1)) // i * I
	ops := KAK(m, 0, 1)
	if len(ops) != 0 {
		cx := countCNOTs(ops)
		if cx != 0 {
			t.Errorf("KAK(i*I): expected 0 CNOTs, got %d", cx)
		}
	}
}

func TestKAK_IsGlobalPhaseOf(t *testing.T) {
	m := gate.CNOT.Matrix()
	if !isGlobalPhaseOf(m, m, 1e-10) {
		t.Error("isGlobalPhaseOf(CNOT, CNOT): expected true")
	}
	if isGlobalPhaseOf(m, gate.SWAP.Matrix(), 1e-10) {
		t.Error("isGlobalPhaseOf(CNOT, SWAP): expected false")
	}
	scaled := MatScale(Eye(4), complex(0, 1))
	if !isGlobalPhaseOf(scaled, Eye(4), 1e-10) {
		t.Error("isGlobalPhaseOf(i*I, I): expected true")
	}
}

// randomHaarUnitary4 generates a Haar-random 4×4 unitary via QR decomposition
// of a random complex Gaussian matrix (Gram-Schmidt orthogonalization).
func randomHaarUnitary4(rng *rand.Rand) []complex128 {
	n := 4
	a := make([]complex128, n*n)
	for i := range a {
		a[i] = complex(rng.NormFloat64(), rng.NormFloat64())
	}
	q := make([]complex128, n*n)
	copy(q, a)
	for j := range n {
		for k := range j {
			dot := complex(0, 0)
			for i := range n {
				dot += cmplx.Conj(q[i*n+k]) * q[i*n+j]
			}
			for i := range n {
				q[i*n+j] -= dot * q[i*n+k]
			}
		}
		norm := 0.0
		for i := range n {
			norm += real(q[i*n+j] * cmplx.Conj(q[i*n+j]))
		}
		norm = math.Sqrt(norm)
		if norm < 1e-15 {
			norm = 1
		}
		for i := range n {
			q[i*n+j] /= complex(norm, 0)
		}
	}
	return q
}

func TestKAK_RandomUnitaries(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	const numTests = 200
	const tol = 1e-5

	for i := range numTests {
		m := randomHaarUnitary4(rng)
		ops := KAK(m, 0, 1)

		cx := countCNOTs(ops)
		if cx > 3 {
			t.Errorf("random[%d]: used %d CNOTs, max should be 3", i, cx)
		}

		if len(ops) > 0 {
			got := kakCircuitUnitary(ops, 0, 1)
			if _, ok := GlobalPhase(got, m, tol); !ok {
				t.Errorf("random[%d]: reconstructed unitary does not match original (up to global phase)", i)
			}
		}
	}
}

func TestKAK_ThreeCNOT_KnownGates(t *testing.T) {
	iswap := []complex128{1, 0, 0, 0, 0, 0, 1i, 0, 0, 1i, 0, 0, 0, 0, 0, 1}
	sqrtswap := []complex128{
		1, 0, 0, 0,
		0, complex(0.5, 0.5), complex(0.5, -0.5), 0,
		0, complex(0.5, -0.5), complex(0.5, 0.5), 0,
		0, 0, 0, 1,
	}

	cases := []struct {
		name    string
		mat     []complex128
		maxCNOT int
	}{
		{"iSWAP", iswap, 3},
		{"sqrt(SWAP)", sqrtswap, 3},
		{"CP(pi/7)", gate.CP(math.Pi / 7).Matrix(), 3},
		{"CRZ(pi/5)", gate.CRZ(math.Pi / 5).Matrix(), 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := KAK(tc.mat, 0, 1)
			if ops == nil {
				t.Fatal("returned nil")
			}
			cx := countCNOTs(ops)
			if cx > tc.maxCNOT {
				t.Errorf("expected at most %d CNOTs, got %d", tc.maxCNOT, cx)
			}
			got := kakCircuitUnitary(ops, 0, 1)
			if _, ok := GlobalPhase(got, tc.mat, 1e-6); !ok {
				t.Error("reconstructed unitary does not match up to global phase")
			}
		})
	}
}

func TestKAK_DiagonalCP_SmallAngle(t *testing.T) {
	// CP with very small angle - tests numerical stability near identity.
	m := gate.CP(0.001).Matrix()
	ops := KAK(m, 0, 1)
	cx := countCNOTs(ops)
	if cx > 3 {
		t.Errorf("KAK(CP(0.001)): used %d CNOTs, max should be 3", cx)
	}
	if len(ops) > 0 {
		got := kakCircuitUnitary(ops, 0, 1)
		if _, ok := GlobalPhase(got, m, kakTol); !ok {
			t.Error("KAK(CP(0.001)): reconstructed unitary does not match")
		}
	}
}
