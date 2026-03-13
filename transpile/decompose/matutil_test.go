package decompose

import (
	"math"
	"math/cmplx"
	"testing"
)

const tol = 1e-12

// assertClose fails the test if |a-b| > tol for any element.
func assertClose(t *testing.T, label string, got, want []complex128) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length mismatch: got %d, want %d", label, len(got), len(want))
	}
	for i := range got {
		if cmplx.Abs(got[i]-want[i]) > tol {
			t.Errorf("%s[%d]: got %v, want %v", label, i, got[i], want[i])
		}
	}
}

// --- MatMul tests ---

func TestMatMul_Identity(t *testing.T) {
	id := Eye(2)
	a := []complex128{1, 2, 3, 4}
	result := MatMul(id, a, 2)
	assertClose(t, "I*A", result, a)

	result2 := MatMul(a, id, 2)
	assertClose(t, "A*I", result2, a)
}

func TestMatMul_HadamardSquared(t *testing.T) {
	// H * H = I
	s2 := 1.0 / math.Sqrt2
	h := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}
	result := MatMul(h, h, 2)
	assertClose(t, "H*H", result, Eye(2))
}

func TestMatMul_PauliXSquared(t *testing.T) {
	// X * X = I
	x := []complex128{0, 1, 1, 0}
	result := MatMul(x, x, 2)
	assertClose(t, "X*X", result, Eye(2))
}

func TestMatMul_PauliXY(t *testing.T) {
	// X * Y = iZ
	x := []complex128{0, 1, 1, 0}
	y := []complex128{0, -1i, 1i, 0}
	result := MatMul(x, y, 2)
	// iZ = [[i, 0], [0, -i]]
	want := []complex128{1i, 0, 0, -1i}
	assertClose(t, "X*Y", result, want)
}

func TestMatMul_3x3(t *testing.T) {
	// 3x3 identity multiplication.
	id3 := Eye(3)
	a := []complex128{1, 2, 3, 4, 5, 6, 7, 8, 9}
	result := MatMul(id3, a, 3)
	assertClose(t, "I3*A", result, a)
}

// --- MatAdj tests ---

func TestMatAdj_Identity(t *testing.T) {
	id := Eye(2)
	result := MatAdj(id, 2)
	assertClose(t, "adj(I)", result, id)
}

func TestMatAdj_Hermitian(t *testing.T) {
	// Hadamard is Hermitian: H† = H.
	s2 := 1.0 / math.Sqrt2
	h := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}
	result := MatAdj(h, 2)
	assertClose(t, "adj(H)", result, h)
}

func TestMatAdj_Complex(t *testing.T) {
	// [[1, 2+i], [3-i, 4]] -> [[1, 3+i], [2-i, 4]]
	m := []complex128{1, complex(2, 1), complex(3, -1), 4}
	want := []complex128{1, complex(3, 1), complex(2, -1), 4}
	result := MatAdj(m, 2)
	assertClose(t, "adj(M)", result, want)
}

func TestMatAdj_UnitaryInverse(t *testing.T) {
	// For unitary U, U†*U = I. Use the S gate: diag(1, i).
	s := []complex128{1, 0, 0, 1i}
	sAdj := MatAdj(s, 2)
	result := MatMul(sAdj, s, 2)
	assertClose(t, "S†*S", result, Eye(2))
}

// --- Tensor tests ---

func TestTensor_IxH(t *testing.T) {
	// I ⊗ H should be a 4x4 matrix.
	s2 := 1.0 / math.Sqrt2
	id := Eye(2)
	h := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}
	result := Tensor(id, 2, h, 2)
	if len(result) != 16 {
		t.Fatalf("Tensor(I,H) length = %d, want 16", len(result))
	}
	// I ⊗ H = [[H, 0], [0, H]]
	// Top-left 2x2 block should be H.
	want := []complex128{
		complex(s2, 0), complex(s2, 0), 0, 0,
		complex(s2, 0), complex(-s2, 0), 0, 0,
		0, 0, complex(s2, 0), complex(s2, 0),
		0, 0, complex(s2, 0), complex(-s2, 0),
	}
	assertClose(t, "I⊗H", result, want)
}

func TestTensor_HxI(t *testing.T) {
	s2 := 1.0 / math.Sqrt2
	id := Eye(2)
	h := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}
	result := Tensor(h, 2, id, 2)
	// H ⊗ I = [[s2*I, s2*I], [s2*I, -s2*I]]
	want := []complex128{
		complex(s2, 0), 0, complex(s2, 0), 0,
		0, complex(s2, 0), 0, complex(s2, 0),
		complex(s2, 0), 0, complex(-s2, 0), 0,
		0, complex(s2, 0), 0, complex(-s2, 0),
	}
	assertClose(t, "H⊗I", result, want)
}

func TestTensor_IxI(t *testing.T) {
	id := Eye(2)
	result := Tensor(id, 2, id, 2)
	assertClose(t, "I⊗I", result, Eye(4))
}

func TestTensor_XxZ(t *testing.T) {
	x := []complex128{0, 1, 1, 0}
	z := []complex128{1, 0, 0, -1}
	result := Tensor(x, 2, z, 2)
	// X ⊗ Z = [[0*Z, 1*Z], [1*Z, 0*Z]] = [[0,0,1,0],[0,0,0,-1],[1,0,0,0],[0,-1,0,0]]
	want := []complex128{
		0, 0, 1, 0,
		0, 0, 0, -1,
		1, 0, 0, 0,
		0, -1, 0, 0,
	}
	assertClose(t, "X⊗Z", result, want)
}

func TestTensor_ScalarLike(t *testing.T) {
	// 1x1 ⊗ 2x2 should give the same 2x2.
	one := []complex128{complex(3, 0)}
	m := []complex128{1, 2, 3, 4}
	result := Tensor(one, 1, m, 2)
	want := []complex128{3, 6, 9, 12}
	assertClose(t, "3⊗M", result, want)
}

// --- Det2x2 tests ---

func TestDet2x2_Identity(t *testing.T) {
	id := Eye(2)
	d := Det2x2(id)
	if cmplx.Abs(d-1) > tol {
		t.Errorf("det(I) = %v, want 1", d)
	}
}

func TestDet2x2_PauliX(t *testing.T) {
	x := []complex128{0, 1, 1, 0}
	d := Det2x2(x)
	if cmplx.Abs(d-(-1)) > tol {
		t.Errorf("det(X) = %v, want -1", d)
	}
}

func TestDet2x2_PauliY(t *testing.T) {
	y := []complex128{0, -1i, 1i, 0}
	d := Det2x2(y)
	// det(Y) = ad - bc = (0)(0) - (-i)(i) = 0 - (-i^2) = 0 - 1 = -1.
	if cmplx.Abs(d-(-1)) > tol {
		t.Errorf("det(Y) = %v, want -1", d)
	}
}

func TestDet2x2_Hadamard(t *testing.T) {
	s2 := 1.0 / math.Sqrt2
	h := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}
	d := Det2x2(h)
	// det(H) = s2*(-s2) - s2*s2 = -0.5 - 0.5 = -1
	if cmplx.Abs(d-(-1)) > tol {
		t.Errorf("det(H) = %v, want -1", d)
	}
}

func TestDet2x2_SGate(t *testing.T) {
	s := []complex128{1, 0, 0, 1i}
	d := Det2x2(s)
	// det(S) = 1*i - 0*0 = i
	if cmplx.Abs(d-1i) > tol {
		t.Errorf("det(S) = %v, want i", d)
	}
}

// --- ToSU2 tests ---

func TestToSU2_Identity(t *testing.T) {
	id := Eye(2)
	su2 := ToSU2(id)
	d := Det2x2(su2)
	if cmplx.Abs(d-1) > tol {
		t.Errorf("det(ToSU2(I)) = %v, want 1", d)
	}
}

func TestToSU2_Hadamard(t *testing.T) {
	s2 := 1.0 / math.Sqrt2
	h := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}
	su2 := ToSU2(h)
	d := Det2x2(su2)
	if cmplx.Abs(d-1) > tol {
		t.Errorf("det(ToSU2(H)) = %v, want 1", d)
	}
	// ToSU2(H) should still be unitary: U†U = I.
	adj := MatAdj(su2, 2)
	product := MatMul(adj, su2, 2)
	if !IsIdentity(product, 2, tol) {
		t.Error("ToSU2(H) is not unitary")
	}
}

func TestToSU2_SGate(t *testing.T) {
	s := []complex128{1, 0, 0, 1i}
	su2 := ToSU2(s)
	d := Det2x2(su2)
	if cmplx.Abs(d-1) > tol {
		t.Errorf("det(ToSU2(S)) = %v, want 1", d)
	}
}

func TestToSU2_PauliX(t *testing.T) {
	x := []complex128{0, 1, 1, 0}
	su2 := ToSU2(x)
	d := Det2x2(su2)
	if cmplx.Abs(d-1) > tol {
		t.Errorf("det(ToSU2(X)) = %v, want 1", d)
	}
}

func TestToSU2_PreservesUnitary(t *testing.T) {
	// RZ(pi/4) is unitary. ToSU2 should preserve unitarity.
	theta := math.Pi / 4
	rz := []complex128{
		cmplx.Exp(complex(0, -theta/2)), 0,
		0, cmplx.Exp(complex(0, theta/2)),
	}
	su2 := ToSU2(rz)
	adj := MatAdj(su2, 2)
	product := MatMul(adj, su2, 2)
	if !IsIdentity(product, 2, tol) {
		t.Error("ToSU2(RZ) is not unitary")
	}
	d := Det2x2(su2)
	if cmplx.Abs(d-1) > tol {
		t.Errorf("det(ToSU2(RZ)) = %v, want 1", d)
	}
}

// --- IsIdentity tests ---

func TestIsIdentity_True(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4} {
		if !IsIdentity(Eye(n), n, tol) {
			t.Errorf("IsIdentity(Eye(%d)) = false, want true", n)
		}
	}
}

func TestIsIdentity_False(t *testing.T) {
	x := []complex128{0, 1, 1, 0}
	if IsIdentity(x, 2, tol) {
		t.Error("IsIdentity(X) = true, want false")
	}

	h := []complex128{
		complex(1/math.Sqrt2, 0), complex(1/math.Sqrt2, 0),
		complex(1/math.Sqrt2, 0), complex(-1/math.Sqrt2, 0),
	}
	if IsIdentity(h, 2, tol) {
		t.Error("IsIdentity(H) = true, want false")
	}
}

func TestIsIdentity_NearIdentity(t *testing.T) {
	// Slightly perturbed identity should still pass with suitable tolerance.
	m := []complex128{
		complex(1+1e-14, 0), complex(0, 1e-14),
		complex(-1e-14, 0), complex(1, -1e-14),
	}
	if !IsIdentity(m, 2, 1e-12) {
		t.Error("IsIdentity(near-I, tol=1e-12) = false, want true")
	}
	// Should fail with tight tolerance.
	if IsIdentity(m, 2, 1e-15) {
		t.Error("IsIdentity(near-I, tol=1e-15) = true, want false")
	}
}

// --- MatClose tests ---

func TestMatClose_Equal(t *testing.T) {
	a := []complex128{1, 2i, complex(3, 4), 5}
	b := []complex128{1, 2i, complex(3, 4), 5}
	if !MatClose(a, b, tol) {
		t.Error("MatClose(a, a) = false, want true")
	}
}

func TestMatClose_Different(t *testing.T) {
	a := []complex128{1, 0, 0, 1}
	b := []complex128{1, 0, 0, -1}
	if MatClose(a, b, tol) {
		t.Error("MatClose(I, Z) = true, want false")
	}
}

func TestMatClose_DifferentLengths(t *testing.T) {
	a := []complex128{1, 0}
	b := []complex128{1, 0, 0, 1}
	if MatClose(a, b, tol) {
		t.Error("MatClose with different lengths should be false")
	}
}

func TestMatClose_WithinTolerance(t *testing.T) {
	a := []complex128{complex(1, 0), complex(0, 1e-13)}
	b := []complex128{complex(1, 0), 0}
	if !MatClose(a, b, 1e-12) {
		t.Error("MatClose with small difference should be true")
	}
}

// --- GlobalPhase tests ---

func TestGlobalPhase_SameMatrix(t *testing.T) {
	a := Eye(2)
	phi, ok := GlobalPhase(a, a, tol)
	if !ok {
		t.Fatal("GlobalPhase(I, I) returned false")
	}
	if math.Abs(phi) > tol {
		t.Errorf("GlobalPhase(I, I) phase = %v, want 0", phi)
	}
}

func TestGlobalPhase_MinusI(t *testing.T) {
	a := Eye(2)
	b := MatScale(Eye(2), -1) // b = -I
	phi, ok := GlobalPhase(a, b, tol)
	if !ok {
		t.Fatal("GlobalPhase(I, -I) returned false")
	}
	// a/b element-wise = 1/(-1) = -1, so phase = pi.
	if math.Abs(math.Abs(phi)-math.Pi) > tol*100 {
		t.Errorf("GlobalPhase(I, -I) phase = %v, want ±pi", phi)
	}
}

func TestGlobalPhase_PhaseFactorI(t *testing.T) {
	// a = i * b
	b := []complex128{1, 0, 0, 1}
	a := MatScale(b, 1i)
	phi, ok := GlobalPhase(a, b, tol)
	if !ok {
		t.Fatal("GlobalPhase(i*I, I) returned false")
	}
	// a = e^{i*pi/2} * b
	if math.Abs(phi-math.Pi/2) > tol*100 {
		t.Errorf("GlobalPhase(i*I, I) phase = %v, want pi/2", phi)
	}
}

func TestGlobalPhase_NoRelation(t *testing.T) {
	// X and Z are not related by a global phase.
	x := []complex128{0, 1, 1, 0}
	z := []complex128{1, 0, 0, -1}
	_, ok := GlobalPhase(x, z, tol)
	if ok {
		t.Error("GlobalPhase(X, Z) should return false")
	}
}

func TestGlobalPhase_ZeroMatrices(t *testing.T) {
	a := []complex128{0, 0, 0, 0}
	b := []complex128{0, 0, 0, 0}
	phi, ok := GlobalPhase(a, b, tol)
	if !ok {
		t.Fatal("GlobalPhase(0, 0) returned false")
	}
	if phi != 0 {
		t.Errorf("GlobalPhase(0, 0) = %v, want 0", phi)
	}
}

func TestGlobalPhase_DifferentLengths(t *testing.T) {
	a := []complex128{1, 0}
	b := []complex128{1, 0, 0, 1}
	_, ok := GlobalPhase(a, b, tol)
	if ok {
		t.Error("GlobalPhase with different lengths should return false")
	}
}

// --- Eye tests ---

func TestEye_1x1(t *testing.T) {
	result := Eye(1)
	want := []complex128{1}
	assertClose(t, "Eye(1)", result, want)
}

func TestEye_2x2(t *testing.T) {
	result := Eye(2)
	want := []complex128{1, 0, 0, 1}
	assertClose(t, "Eye(2)", result, want)
}

func TestEye_4x4(t *testing.T) {
	result := Eye(4)
	if len(result) != 16 {
		t.Fatalf("Eye(4) length = %d, want 16", len(result))
	}
	for i := range 4 {
		for j := range 4 {
			want := complex(0, 0)
			if i == j {
				want = 1
			}
			if result[i*4+j] != want {
				t.Errorf("Eye(4)[%d][%d] = %v, want %v", i, j, result[i*4+j], want)
			}
		}
	}
}

// --- MatScale tests ---

func TestMatScale(t *testing.T) {
	m := []complex128{1, 2, 3, 4}
	result := MatScale(m, 2i)
	want := []complex128{2i, 4i, 6i, 8i}
	assertClose(t, "MatScale", result, want)
}

func TestMatScale_One(t *testing.T) {
	m := []complex128{1, 2i, complex(3, 4), 5}
	result := MatScale(m, 1)
	assertClose(t, "MatScale(m,1)", result, m)
}

// --- Combined / integration tests ---

func TestUnitaryProperty_HadamardAdj(t *testing.T) {
	// H is self-adjoint and unitary: H*H†=I.
	s2 := 1.0 / math.Sqrt2
	h := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}
	hAdj := MatAdj(h, 2)
	product := MatMul(h, hAdj, 2)
	if !IsIdentity(product, 2, tol) {
		t.Error("H*H† is not identity")
	}
}

func TestTensorProduct_Unitarity(t *testing.T) {
	// If U and V are unitary, U ⊗ V should also be unitary.
	s2 := 1.0 / math.Sqrt2
	h := []complex128{
		complex(s2, 0), complex(s2, 0),
		complex(s2, 0), complex(-s2, 0),
	}
	x := []complex128{0, 1, 1, 0}
	hx := Tensor(h, 2, x, 2)
	hxAdj := MatAdj(hx, 4)
	product := MatMul(hx, hxAdj, 4)
	if !IsIdentity(product, 4, tol) {
		t.Error("(H⊗X)(H⊗X)† is not identity")
	}
}
