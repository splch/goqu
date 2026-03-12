package operator

import (
	"math/cmplx"
	"testing"

	"github.com/splch/qgo/sim/noise"
)

func TestCompose_IdentityIdentity(t *testing.T) {
	id := identity1Q()
	c := Compose(id, id)

	// Composing two identities should give identity.
	s := KrausToSuperOp(c)
	sId := KrausToSuperOp(id)
	if !matClose(s.matrix, sId.matrix, testTol) {
		t.Error("Compose(I, I) should be I")
	}
}

func TestCompose_IdentityChannel(t *testing.T) {
	id := identity1Q()
	ch := noise.Depolarizing1Q(0.2)
	k := FromChannel(ch)

	// Compose(id, depol) should equal depol.
	c := Compose(id, k)
	sC := KrausToSuperOp(c)
	sK := KrausToSuperOp(k)
	if !matClose(sC.matrix, sK.matrix, 1e-8) {
		t.Error("Compose(I, depol) should equal depol")
	}

	// Compose(depol, id) should also equal depol.
	c2 := Compose(k, id)
	sC2 := KrausToSuperOp(c2)
	if !matClose(sC2.matrix, sK.matrix, 1e-8) {
		t.Error("Compose(depol, I) should equal depol")
	}
}

func TestCompose_Associativity(t *testing.T) {
	a := FromChannel(noise.Depolarizing1Q(0.1))
	b := FromChannel(noise.BitFlip(0.15))
	c := FromChannel(noise.PhaseFlip(0.2))

	// (a ; b) ; c should equal a ; (b ; c)
	ab := Compose(a, b)
	abc1 := Compose(ab, c)

	bc := Compose(b, c)
	abc2 := Compose(a, bc)

	s1 := KrausToSuperOp(abc1)
	s2 := KrausToSuperOp(abc2)
	if !matClose(s1.matrix, s2.matrix, 1e-7) {
		t.Error("Compose is not associative")
	}
}

func TestTensor_IdentityIdentity(t *testing.T) {
	id1 := identity1Q()
	id2 := Tensor(id1, id1)

	// Should be a 2-qubit identity channel.
	if id2.NumQubits() != 2 {
		t.Fatalf("expected 2 qubits, got %d", id2.NumQubits())
	}
	ops := id2.Operators()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operator, got %d", len(ops))
	}
	// Should be the 4x4 identity.
	id4 := identityMatrix(4)
	if !matClose(ops[0], id4, testTol) {
		t.Error("Tensor(I, I) should be I_4")
	}
}

func TestTensor_DifferentChannels(t *testing.T) {
	a := FromChannel(noise.BitFlip(0.1))
	b := FromChannel(noise.PhaseFlip(0.2))
	c := Tensor(a, b)

	if c.NumQubits() != 2 {
		t.Fatalf("expected 2 qubits, got %d", c.NumQubits())
	}
	// Verify it is CPTP.
	if !IsCPTP(c, 1e-7) {
		t.Error("Tensor of CPTP channels should be CPTP")
	}
}

func TestCompose_PanicOnMismatch(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on qubit mismatch")
		}
	}()
	a := identity1Q()
	b := NewKraus(2, [][]complex128{identityMatrix(4)})
	Compose(a, b)
}

func TestTensor_Roundtrip(t *testing.T) {
	// Verify Tensor of two channels is CPTP and produces correct SuperOp
	// by checking that applying the tensor channel to a product state
	// gives the same result as applying each channel separately.
	a := FromChannel(noise.Depolarizing1Q(0.1))
	b := FromChannel(noise.PhaseFlip(0.2))

	tab := Tensor(a, b)
	if !IsCPTP(tab, 1e-7) {
		t.Error("Tensor(a,b) should be CPTP")
	}

	// Apply tensor channel to |00><00| = |0><0| (x) |0><0|.
	rho00 := []complex128{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	dim := 4
	result := make([]complex128, dim*dim)
	for _, ek := range tab.operators {
		ekd := adjoint(ek, dim)
		tmp := matMul(ek, rho00, dim)
		prod := matMul(tmp, ekd, dim)
		for i := range result {
			result[i] += prod[i]
		}
	}

	// Compute separately: E_a(|0><0|) and E_b(|0><0|)
	rho0 := []complex128{1, 0, 0, 0}
	resA := make([]complex128, 4)
	for _, ek := range a.operators {
		ekd := adjoint(ek, 2)
		tmp := matMul(ek, rho0, 2)
		prod := matMul(tmp, ekd, 2)
		for i := range resA {
			resA[i] += prod[i]
		}
	}
	resB := make([]complex128, 4)
	for _, ek := range b.operators {
		ekd := adjoint(ek, 2)
		tmp := matMul(ek, rho0, 2)
		prod := matMul(tmp, ekd, 2)
		for i := range resB {
			resB[i] += prod[i]
		}
	}

	// Expected: resA (x) resB (Kronecker product of 2x2 matrices)
	expected := make([]complex128, 16)
	for i := range 2 {
		for j := range 2 {
			for k := range 2 {
				for l := range 2 {
					row := i*2 + k
					col := j*2 + l
					expected[row*4+col] = resA[i*2+j] * resB[k*2+l]
				}
			}
		}
	}

	for i := range expected {
		if cmplx.Abs(result[i]-expected[i]) > 1e-8 {
			t.Errorf("Tensor result mismatch at [%d]: %v vs %v",
				i, result[i], expected[i])
		}
	}
}
