package vqc_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/ansatz"
	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/algorithm/vqc"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/statevector"
)

func TestKernel_Diagonal(t *testing.T) {
	cfg := vqc.KernelConfig{NumQubits: 2, FeatureMap: vqc.ZFeatureMap(1)}
	data := [][]float64{{0.1, 0.2}, {0.5, 0.8}, {1.0, 1.5}}

	mat, err := vqc.KernelMatrix(context.Background(), cfg, data)
	if err != nil {
		t.Fatal(err)
	}

	for i := range data {
		if math.Abs(mat[i][i]-1.0) > 1e-6 {
			t.Errorf("K[%d][%d] = %f, want 1.0", i, i, mat[i][i])
		}
	}
	// Symmetry check.
	for i := range data {
		for j := range data {
			if math.Abs(mat[i][j]-mat[j][i]) > 1e-6 {
				t.Errorf("K[%d][%d]=%f != K[%d][%d]=%f", i, j, mat[i][j], j, i, mat[j][i])
			}
		}
	}
}

func TestVQC_SimpleClassification(t *testing.T) {
	// 2-qubit classification with well-separated features scaled to [0, 2*pi].
	// Class 0 features near 0, class 1 features near pi.
	trainX := [][]float64{
		{0.2, 0.3},
		{0.3, 0.2},
		{0.1, 0.4},
		{math.Pi, math.Pi - 0.2},
		{math.Pi - 0.1, math.Pi},
		{math.Pi + 0.1, math.Pi - 0.1},
	}
	trainY := []int{0, 0, 0, 1, 1, 1}

	cfg := vqc.Config{
		NumQubits:  2,
		FeatureMap: vqc.ZFeatureMap(2),
		Ansatz:     ansatz.NewRealAmplitudes(2, 2, ansatz.Linear),
		Optimizer:  &optim.NelderMead{InitialStep: 1.0},
		TrainX:     trainX,
		TrainY:     trainY,
	}

	res, err := vqc.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should achieve at least 66% accuracy on this separable problem.
	if res.TrainAccuracy < 0.66 {
		t.Errorf("train accuracy = %f, want >= 0.66", res.TrainAccuracy)
	}
}

func TestVQC_ErrorCases(t *testing.T) {
	validFM := vqc.ZFeatureMap(1)
	validAnsatz := ansatz.NewRealAmplitudes(1, 1, ansatz.Linear)
	validOpt := &optim.NelderMead{}

	tests := []struct {
		name string
		cfg  vqc.Config
	}{
		{
			name: "zero qubits",
			cfg: vqc.Config{
				NumQubits:  0,
				FeatureMap: validFM,
				Ansatz:     validAnsatz,
				Optimizer:  validOpt,
				TrainX:     [][]float64{{0.1}},
				TrainY:     []int{0},
			},
		},
		{
			name: "nil feature map",
			cfg: vqc.Config{
				NumQubits: 1,
				Ansatz:    validAnsatz,
				Optimizer: validOpt,
				TrainX:    [][]float64{{0.1}},
				TrainY:    []int{0},
			},
		},
		{
			name: "nil ansatz",
			cfg: vqc.Config{
				NumQubits:  1,
				FeatureMap: validFM,
				Optimizer:  validOpt,
				TrainX:     [][]float64{{0.1}},
				TrainY:     []int{0},
			},
		},
		{
			name: "nil optimizer",
			cfg: vqc.Config{
				NumQubits:  1,
				FeatureMap: validFM,
				Ansatz:     validAnsatz,
				TrainX:     [][]float64{{0.1}},
				TrainY:     []int{0},
			},
		},
		{
			name: "empty training data",
			cfg: vqc.Config{
				NumQubits:  1,
				FeatureMap: validFM,
				Ansatz:     validAnsatz,
				Optimizer:  validOpt,
				TrainX:     [][]float64{},
				TrainY:     []int{},
			},
		},
		{
			name: "mismatched TrainX and TrainY lengths",
			cfg: vqc.Config{
				NumQubits:  1,
				FeatureMap: validFM,
				Ansatz:     validAnsatz,
				Optimizer:  validOpt,
				TrainX:     [][]float64{{0.1}, {0.2}},
				TrainY:     []int{0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := vqc.Run(context.Background(), tt.cfg)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestKernel_ErrorCases(t *testing.T) {
	t.Run("zero qubits", func(t *testing.T) {
		cfg := vqc.KernelConfig{NumQubits: 0, FeatureMap: vqc.ZFeatureMap(1)}
		_, err := vqc.KernelMatrix(context.Background(), cfg, [][]float64{{0.1}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("nil feature map", func(t *testing.T) {
		cfg := vqc.KernelConfig{NumQubits: 1}
		_, err := vqc.KernelMatrix(context.Background(), cfg, [][]float64{{0.1}})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("empty data", func(t *testing.T) {
		cfg := vqc.KernelConfig{NumQubits: 1, FeatureMap: vqc.ZFeatureMap(1)}
		_, err := vqc.KernelMatrix(context.Background(), cfg, [][]float64{})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestKernel_ZZFeatureMap(t *testing.T) {
	cfg := vqc.KernelConfig{NumQubits: 2, FeatureMap: vqc.ZZFeatureMap(1)}
	data := [][]float64{{0.1, 0.2}, {0.5, 0.8}}

	mat, err := vqc.KernelMatrix(context.Background(), cfg, data)
	if err != nil {
		t.Fatal(err)
	}

	// Diagonal must be 1.
	for i := range data {
		if math.Abs(mat[i][i]-1.0) > 1e-6 {
			t.Errorf("K[%d][%d] = %f, want 1.0", i, i, mat[i][i])
		}
	}
	// Off-diagonal must be in [0, 1].
	if mat[0][1] < 0 || mat[0][1] > 1 {
		t.Errorf("K[0][1] = %f, want in [0, 1]", mat[0][1])
	}
}

func TestAngleEmbedding(t *testing.T) {
	rotations := []struct {
		name string
		rot  vqc.RotationType
	}{
		{"RX", vqc.RotRX},
		{"RY", vqc.RotRY},
		{"RZ", vqc.RotRZ},
	}
	for _, r := range rotations {
		t.Run(r.name, func(t *testing.T) {
			cfg := vqc.KernelConfig{NumQubits: 2, FeatureMap: vqc.AngleEmbedding(r.rot)}
			data := [][]float64{{0.1, 0.2}, {0.5, 0.8}, {1.0, 1.5}}

			mat, err := vqc.KernelMatrix(context.Background(), cfg, data)
			if err != nil {
				t.Fatal(err)
			}

			// Diagonal must be 1.
			for i := range data {
				if math.Abs(mat[i][i]-1.0) > 1e-6 {
					t.Errorf("K[%d][%d] = %f, want 1.0", i, i, mat[i][i])
				}
			}
			// Symmetry.
			for i := range data {
				for j := range data {
					if math.Abs(mat[i][j]-mat[j][i]) > 1e-6 {
						t.Errorf("K[%d][%d]=%f != K[%d][%d]=%f", i, j, mat[i][j], j, i, mat[j][i])
					}
				}
			}
		})
	}
}

func TestAmplitudeEmbedding(t *testing.T) {
	t.Run("basis_state", func(t *testing.T) {
		// Embedding [1, 0, 0, 0] on 2 qubits should give |00⟩.
		fm := vqc.AmplitudeEmbedding()
		b := builder.New("test", 2)
		fm(b, []float64{1, 0, 0, 0}, []int{0, 1})
		circ, err := b.Build()
		if err != nil {
			t.Fatal(err)
		}
		sim := statevector.New(2)
		if err := sim.Evolve(circ); err != nil {
			t.Fatal(err)
		}
		sv := sim.StateVector()
		// |00⟩ should have amplitude ≈ 1.
		if math.Abs(real(sv[0])-1.0) > 1e-6 || math.Abs(imag(sv[0])) > 1e-6 {
			t.Errorf("sv[0] = %v, want 1+0i", sv[0])
		}
	})

	t.Run("uniform", func(t *testing.T) {
		// Embedding [1, 1, 1, 1] should normalize to [0.5, 0.5, 0.5, 0.5].
		fm := vqc.AmplitudeEmbedding()
		b := builder.New("test", 2)
		fm(b, []float64{1, 1, 1, 1}, []int{0, 1})
		circ, err := b.Build()
		if err != nil {
			t.Fatal(err)
		}
		sim := statevector.New(2)
		if err := sim.Evolve(circ); err != nil {
			t.Fatal(err)
		}
		sv := sim.StateVector()
		// Each amplitude should be ≈ 0.5.
		for i, a := range sv {
			if math.Abs(real(a)-0.5) > 1e-6 {
				t.Errorf("sv[%d] = %v, want ≈ 0.5", i, a)
			}
		}
	})

	t.Run("padding", func(t *testing.T) {
		// Fewer features than 2^n: should zero-pad and normalize.
		fm := vqc.AmplitudeEmbedding()
		b := builder.New("test", 2)
		fm(b, []float64{3, 4}, []int{0, 1}) // norm = 5, normalized = [0.6, 0.8, 0, 0]
		circ, err := b.Build()
		if err != nil {
			t.Fatal(err)
		}
		sim := statevector.New(2)
		if err := sim.Evolve(circ); err != nil {
			t.Fatal(err)
		}
		sv := sim.StateVector()
		if math.Abs(real(sv[0])-0.6) > 1e-6 {
			t.Errorf("sv[0] = %v, want ≈ 0.6", sv[0])
		}
		if math.Abs(real(sv[1])-0.8) > 1e-6 {
			t.Errorf("sv[1] = %v, want ≈ 0.8", sv[1])
		}
	})
}
