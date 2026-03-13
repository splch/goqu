package mitigation

import (
	"context"
	"fmt"
	"math"

	"github.com/splch/goqu/circuit/ir"
)

// ZNEConfig specifies the parameters for zero-noise extrapolation.
type ZNEConfig struct {
	// Circuit is the quantum circuit to mitigate.
	Circuit *ir.Circuit
	// Executor evaluates a circuit and returns an expectation value.
	Executor Executor
	// ScaleFactors are the noise scale factors. Must be positive odd integers.
	// Default: [1, 3, 5].
	ScaleFactors []float64
	// ScaleMethod selects the noise-scaling strategy. Default: UnitaryFolding.
	ScaleMethod ScaleMethod
	// Extrapolator selects the extrapolation method. Default: LinearExtrapolator.
	Extrapolator Extrapolator
}

// ZNEResult holds the output of zero-noise extrapolation.
type ZNEResult struct {
	// MitigatedValue is the extrapolated zero-noise expectation value.
	MitigatedValue float64
	// NoisyValues are the raw expectation values at each scale factor.
	NoisyValues []float64
	// ScaleFactors are the scale factors that were used.
	ScaleFactors []float64
}

// RunZNE performs zero-noise extrapolation.
//
// It folds the circuit at each scale factor, executes each folded circuit,
// and extrapolates the results to the zero-noise limit.
func RunZNE(ctx context.Context, cfg ZNEConfig) (*ZNEResult, error) {
	if cfg.Circuit == nil {
		return nil, fmt.Errorf("mitigation.RunZNE: Circuit is nil")
	}
	if cfg.Executor == nil {
		return nil, fmt.Errorf("mitigation.RunZNE: Executor is nil")
	}

	scaleFactors := cfg.ScaleFactors
	if len(scaleFactors) == 0 {
		scaleFactors = []float64{1, 3, 5}
	}

	// Validate scale factors are positive odd integers.
	for _, sf := range scaleFactors {
		rounded := math.Round(sf)
		if sf != rounded || rounded < 1 || int(rounded)%2 == 0 {
			return nil, fmt.Errorf("mitigation.RunZNE: scale factor %v must be a positive odd integer", sf)
		}
	}

	values := make([]float64, len(scaleFactors))
	for i, sf := range scaleFactors {
		folded, err := FoldCircuit(cfg.Circuit, int(sf), cfg.ScaleMethod)
		if err != nil {
			return nil, fmt.Errorf("mitigation.RunZNE: fold at scale %v: %w", sf, err)
		}

		val, err := cfg.Executor(ctx, folded)
		if err != nil {
			return nil, fmt.Errorf("mitigation.RunZNE: execute at scale %v: %w", sf, err)
		}
		values[i] = val
	}

	mitigated, err := Extrapolate(scaleFactors, values, cfg.Extrapolator)
	if err != nil {
		return nil, fmt.Errorf("mitigation.RunZNE: extrapolate: %w", err)
	}

	return &ZNEResult{
		MitigatedValue: mitigated,
		NoisyValues:    values,
		ScaleFactors:   scaleFactors,
	}, nil
}
