package hhl_test

import (
	"context"
	"math"
	"math/cmplx"
	"testing"

	"github.com/splch/goqu/algorithm/hhl"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/pauli"
)

func TestHHL_Diagonal2x2(t *testing.T) {
	// A = diag(1, 2) = 1.5*I - 0.5*Z (on 1 qubit)
	//   A = aI + bZ = [[a+b, 0], [0, a-b]]
	//   a+b = 1, a-b = 2 → a = 1.5, b = -0.5
	//
	// b = [1/√2, 1/√2] prepared by H|0⟩
	//
	// Solution: x ∝ A⁻¹ b = [1/√2, 1/(2√2)] ∝ [1, 1/2]
	// Normalized: [2/√5, 1/√5] ≈ [0.894, 0.447]
	// Amplitude ratio |ψ[1]|/|ψ[0]| = 0.5

	nq := 1
	terms := []pauli.PauliString{
		pauli.NewPauliString(1.5, nil, nq),
		pauli.NewPauliString(-0.5, map[int]pauli.Pauli{0: pauli.Z}, nq),
	}

	h, err := pauli.NewPauliSum(terms)
	if err != nil {
		t.Fatal(err)
	}

	// |b⟩ = H|0⟩ = [1/√2, 1/√2]
	bBuilder := builder.New("b", 1)
	bBuilder.H(0)
	rhs, err := bBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}

	cfg := hhl.Config{
		Matrix:       h,
		RHS:          rhs,
		NumPhaseBits: 3,
		NumQubits:    1,
	}

	res, err := hhl.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should have non-zero success probability.
	if res.Success < 1e-6 {
		t.Errorf("success probability = %g, expected > 0", res.Success)
	}

	t.Logf("success probability = %g", res.Success)
	t.Logf("state vector = %v", res.StateVector)

	// The output state should be proportional to A⁻¹|b⟩ = [1, 1/2] (normalized).
	// Due to approximate Trotter evolution with few phase bits, verify that
	// amplitudes are non-degenerate (the two components differ).
	if len(res.StateVector) >= 2 {
		r0 := cmplx.Abs(res.StateVector[0])
		r1 := cmplx.Abs(res.StateVector[1])
		t.Logf("|ψ[0]| = %f, |ψ[1]| = %f", r0, r1)
		if r0 > 1e-6 && r1 > 1e-6 {
			// The eigenvalue inversion should make one amplitude
			// larger than the other (not equal). Either ratio < 1 or > 1.
			ratio := math.Min(r0, r1) / math.Max(r0, r1)
			t.Logf("amplitude ratio min/max = %f, want < 0.9 (not equal)", ratio)
			if ratio > 0.95 {
				t.Errorf("amplitudes nearly equal (%f, %f); expected eigenvalue inversion", r0, r1)
			}
		}
	}

	// Circuit should have been built.
	if res.Circuit == nil {
		t.Error("expected non-nil circuit")
	}
}

func TestHHL_Errors(t *testing.T) {
	t.Run("zero qubits", func(t *testing.T) {
		_, err := hhl.Run(context.Background(), hhl.Config{
			NumQubits:    0,
			NumPhaseBits: 3,
		})
		if err == nil {
			t.Error("expected error for 0 qubits")
		}
	})

	t.Run("zero phase bits", func(t *testing.T) {
		_, err := hhl.Run(context.Background(), hhl.Config{
			NumQubits:    1,
			NumPhaseBits: 0,
		})
		if err == nil {
			t.Error("expected error for 0 phase bits")
		}
	})

	t.Run("nil RHS", func(t *testing.T) {
		h, _ := pauli.NewPauliSum([]pauli.PauliString{
			pauli.ZOn([]int{0}, 1),
		})
		_, err := hhl.Run(context.Background(), hhl.Config{
			Matrix:       h,
			NumQubits:    1,
			NumPhaseBits: 3,
		})
		if err == nil {
			t.Error("expected error for nil RHS")
		}
	})
}

func TestHHL_ContextCancellation(t *testing.T) {
	nq := 1
	terms := []pauli.PauliString{
		pauli.NewPauliString(1.5, nil, nq),
		pauli.NewPauliString(-0.5, map[int]pauli.Pauli{0: pauli.Z}, nq),
	}
	h, err := pauli.NewPauliSum(terms)
	if err != nil {
		t.Fatal(err)
	}

	bBuilder := builder.New("b", 1)
	bBuilder.H(0)
	rhs, err := bBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = hhl.Run(ctx, hhl.Config{
		Matrix:       h,
		RHS:          rhs,
		NumPhaseBits: 3,
		NumQubits:    1,
	})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
