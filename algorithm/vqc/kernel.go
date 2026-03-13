package vqc

import (
	"context"
	"fmt"
	"math/cmplx"

	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/sim/statevector"
)

// KernelConfig specifies the quantum kernel parameters.
type KernelConfig struct {
	// NumQubits is the number of qubits used by the feature map.
	NumQubits int
	// FeatureMap encodes classical features into a quantum state.
	FeatureMap FeatureMap
}

// KernelMatrix computes the pairwise quantum kernel matrix for the given data.
// K[i][j] = |<0|V†(x_j)·V(x_i)|0>|² where V is the feature map circuit.
// The matrix is symmetric: K[i][j] = K[j][i].
func KernelMatrix(ctx context.Context, cfg KernelConfig, dataX [][]float64) ([][]float64, error) {
	if cfg.NumQubits < 1 {
		return nil, fmt.Errorf("kernel: NumQubits must be >= 1, got %d", cfg.NumQubits)
	}
	if cfg.FeatureMap == nil {
		return nil, fmt.Errorf("kernel: FeatureMap is required")
	}
	if len(dataX) == 0 {
		return nil, fmt.Errorf("kernel: dataX must be non-empty")
	}

	n := len(dataX)
	mat := make([][]float64, n)
	for i := range n {
		mat[i] = make([]float64, n)
	}

	for i := range n {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		mat[i][i] = 1.0 // K(x, x) = 1 by construction
		for j := i + 1; j < n; j++ {
			k, err := KernelEntry(cfg, dataX[i], dataX[j])
			if err != nil {
				return nil, fmt.Errorf("kernel: entry [%d][%d]: %w", i, j, err)
			}
			mat[i][j] = k
			mat[j][i] = k // symmetry
		}
	}

	return mat, nil
}

// KernelEntry computes the fidelity |<0|V†(x2)·V(x1)|0>|² for two data points.
func KernelEntry(cfg KernelConfig, x1, x2 []float64) (float64, error) {
	n := cfg.NumQubits
	qubits := make([]int, n)
	for i := range n {
		qubits[i] = i
	}

	// Build V(x1).
	b1 := builder.New("fm1", n)
	cfg.FeatureMap(b1, x1, qubits)
	c1, err := b1.Build()
	if err != nil {
		return 0, fmt.Errorf("kernel: feature map x1: %w", err)
	}

	// Build V(x2).
	b2 := builder.New("fm2", n)
	cfg.FeatureMap(b2, x2, qubits)
	c2, err := b2.Build()
	if err != nil {
		return 0, fmt.Errorf("kernel: feature map x2: %w", err)
	}

	// Build V(x1) then V†(x2).
	cb := builder.New("kernel", n)
	idMap := make(map[int]int, n)
	for i := range n {
		idMap[i] = i
	}
	cb.Compose(c1, idMap)
	cb.ComposeInverse(c2, idMap)
	circ, err := cb.Build()
	if err != nil {
		return 0, fmt.Errorf("kernel: compose: %w", err)
	}

	sim := statevector.New(n)
	if err := sim.Evolve(circ); err != nil {
		return 0, fmt.Errorf("kernel: evolve: %w", err)
	}
	sv := sim.StateVector()

	return real(sv[0] * cmplx.Conj(sv[0])), nil
}
