// Package vqc implements the Variational Quantum Classifier.
//
// VQC trains a parameterized quantum circuit to classify classical data.
// Each sample is encoded via a feature map, then processed by a variational
// ansatz whose parameters are optimized to minimize classification loss.
package vqc

import (
	"context"
	"fmt"
	"math"
	"math/cmplx"

	"github.com/splch/goqu/algorithm/ansatz"
	"github.com/splch/goqu/algorithm/optim"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/statevector"
)

// FeatureMap encodes classical data into a quantum state.
type FeatureMap func(b *builder.Builder, features []float64, qubits []int)

// Config specifies the VQC problem and solver.
type Config struct {
	// NumQubits is the number of qubits in the circuit.
	NumQubits int
	// FeatureMap encodes classical features into the quantum state.
	FeatureMap FeatureMap
	// Ansatz is the parameterized circuit template.
	Ansatz ansatz.Ansatz
	// Optimizer is the classical optimization method.
	Optimizer optim.Optimizer
	// Gradient is the gradient function. Nil means gradient-free.
	Gradient optim.GradientFunc
	// TrainX holds the training feature vectors.
	TrainX [][]float64
	// TrainY holds the training labels (0 or 1).
	TrainY []int
	// InitialParams are the starting parameters. Nil means zeros.
	InitialParams []float64
}

// Result holds VQC output.
type Result struct {
	OptimalParams []float64
	TrainAccuracy float64
	NumIters      int
	Converged     bool
}

// Run executes the VQC training loop.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.NumQubits < 1 {
		return nil, fmt.Errorf("vqc: NumQubits must be >= 1, got %d", cfg.NumQubits)
	}
	if cfg.FeatureMap == nil {
		return nil, fmt.Errorf("vqc: FeatureMap is required")
	}
	if cfg.Ansatz == nil {
		return nil, fmt.Errorf("vqc: Ansatz is required")
	}
	if cfg.Optimizer == nil {
		return nil, fmt.Errorf("vqc: Optimizer is required")
	}
	if len(cfg.TrainX) == 0 {
		return nil, fmt.Errorf("vqc: TrainX must be non-empty")
	}
	if len(cfg.TrainX) != len(cfg.TrainY) {
		return nil, fmt.Errorf("vqc: TrainX and TrainY must have equal length (%d != %d)",
			len(cfg.TrainX), len(cfg.TrainY))
	}

	ansatzCirc, err := cfg.Ansatz.Circuit()
	if err != nil {
		return nil, fmt.Errorf("vqc: ansatz circuit: %w", err)
	}

	paramNames := ir.FreeParameters(ansatzCirc)

	// Cost function: MSE over the training set.
	cost := optim.ObjectiveFunc(func(params []float64) float64 {
		bindings := make(map[string]float64, len(paramNames))
		for i, name := range paramNames {
			bindings[name] = params[i]
		}

		var totalLoss float64
		for idx, x := range cfg.TrainX {
			p1, err := classProb(cfg, ansatzCirc, bindings, x)
			if err != nil {
				return math.Inf(1)
			}
			y := float64(cfg.TrainY[idx])
			totalLoss += (p1 - y) * (p1 - y)
		}
		return totalLoss / float64(len(cfg.TrainX))
	})

	x0 := cfg.InitialParams
	if x0 == nil {
		x0 = make([]float64, len(paramNames))
	}

	res, err := cfg.Optimizer.Minimize(ctx, cost, x0, cfg.Gradient, nil)
	if err != nil {
		return nil, fmt.Errorf("vqc: optimization: %w", err)
	}

	// Compute training accuracy.
	preds, err := Predict(cfg, res.X, cfg.TrainX)
	if err != nil {
		return nil, fmt.Errorf("vqc: predict: %w", err)
	}
	correct := 0
	for i, p := range preds {
		if p == cfg.TrainY[i] {
			correct++
		}
	}

	return &Result{
		OptimalParams: res.X,
		TrainAccuracy: float64(correct) / float64(len(cfg.TrainY)),
		NumIters:      res.Iterations,
		Converged:     res.Converged,
	}, nil
}

// Predict classifies each sample in dataX using trained parameters.
// Returns a slice of predicted labels (0 or 1).
func Predict(cfg Config, params []float64, dataX [][]float64) ([]int, error) {
	ansatzCirc, err := cfg.Ansatz.Circuit()
	if err != nil {
		return nil, fmt.Errorf("vqc: ansatz circuit: %w", err)
	}

	paramNames := ir.FreeParameters(ansatzCirc)
	bindings := make(map[string]float64, len(paramNames))
	for i, name := range paramNames {
		bindings[name] = params[i]
	}

	labels := make([]int, len(dataX))
	for idx, x := range dataX {
		p1, err := classProb(cfg, ansatzCirc, bindings, x)
		if err != nil {
			return nil, err
		}
		if p1 >= 0.5 {
			labels[idx] = 1
		} else {
			labels[idx] = 0
		}
	}
	return labels, nil
}

// classProb computes P(class 1) = 1 - |<0|psi>|^2 for a single sample.
func classProb(cfg Config, ansatzCirc *ir.Circuit, bindings map[string]float64, x []float64) (float64, error) {
	n := cfg.NumQubits
	qubits := make([]int, n)
	for i := range n {
		qubits[i] = i
	}

	// Build feature map circuit.
	fb := builder.New("fm", n)
	cfg.FeatureMap(fb, x, qubits)
	fmCirc, err := fb.Build()
	if err != nil {
		return 0, fmt.Errorf("vqc: feature map: %w", err)
	}

	// Bind ansatz parameters.
	boundAnsatz, err := ir.Bind(ansatzCirc, bindings)
	if err != nil {
		return 0, fmt.Errorf("vqc: bind ansatz: %w", err)
	}

	// Compose: feature map + ansatz.
	cb := builder.New("vqc", n)
	idMap := make(map[int]int, n)
	for i := range n {
		idMap[i] = i
	}
	cb.Compose(fmCirc, idMap)
	cb.Compose(boundAnsatz, idMap)
	circ, err := cb.Build()
	if err != nil {
		return 0, fmt.Errorf("vqc: compose: %w", err)
	}

	sim := statevector.New(n)
	if err := sim.Evolve(circ); err != nil {
		return 0, fmt.Errorf("vqc: evolve: %w", err)
	}
	sv := sim.StateVector()

	// P(class 1) = 1 - |<0...0|psi>|^2
	p0 := real(sv[0] * cmplx.Conj(sv[0]))
	return 1 - p0, nil
}

// ZFeatureMap returns a feature map that applies H and RZ(feature) on each qubit.
// The encoding is repeated depth times.
func ZFeatureMap(depth int) FeatureMap {
	return func(b *builder.Builder, features []float64, qubits []int) {
		for range depth {
			for i, q := range qubits {
				if i < len(features) {
					b.H(q)
					b.RZ(features[i], q)
				}
			}
		}
	}
}

// ZZFeatureMap returns a feature map like ZFeatureMap but with entangling
// RZZ(f_i * f_j) gates between adjacent qubit pairs.
func ZZFeatureMap(depth int) FeatureMap {
	return func(b *builder.Builder, features []float64, qubits []int) {
		for range depth {
			for i, q := range qubits {
				if i < len(features) {
					b.H(q)
					b.RZ(features[i], q)
				}
			}
			// Entangling: RZZ(f_i * f_j) on adjacent pairs.
			for i := 0; i < len(qubits)-1; i++ {
				fi, fj := 0.0, 0.0
				if i < len(features) {
					fi = features[i]
				}
				if i+1 < len(features) {
					fj = features[i+1]
				}
				b.Apply(gate.RZZ(fi*fj), qubits[i], qubits[i+1])
			}
		}
	}
}
