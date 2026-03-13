package mitigation

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

// ShotRunner executes a circuit with a given number of shots and returns
// measurement counts. This is the callback type for shot-based mitigation.
type ShotRunner func(ctx context.Context, circuit *ir.Circuit, shots int) (map[string]int, error)

// TREXCalibration holds per-qubit readout flip rates for TREX correction.
type TREXCalibration struct {
	numQubits int
	flipRates []float64 // per-qubit flip rate f_q
}

// CalibrateTREX estimates per-qubit readout flip rates by running a trivial
// circuit with randomized X insertions before measurement. Uses the all-zero
// state so any measured 1s (after classical correction) indicate readout errors.
func CalibrateTREX(ctx context.Context, numQubits, calibShots int, runner ShotRunner) (*TREXCalibration, error) {
	if numQubits < 1 {
		return nil, fmt.Errorf("mitigation.CalibrateTREX: numQubits must be >= 1")
	}
	if calibShots < 1 {
		return nil, fmt.Errorf("mitigation.CalibrateTREX: calibShots must be >= 1")
	}

	// Build a trivial circuit: just measure all qubits.
	ops := make([]ir.Operation, numQubits)
	for q := range numQubits {
		ops[q] = ir.Operation{
			Gate:   nil,
			Qubits: []int{q},
			Clbits: []int{q},
		}
	}
	baseCirc := ir.New("trex_calib", numQubits, numQubits, ops, nil)

	// Run multiple twirled copies and accumulate per-qubit one-counts.
	nSamples := 10
	rng := rand.New(rand.NewSource(rand.Int63()))
	oneCounts := make([]int, numQubits)
	totalShots := 0

	for range nSamples {
		twirled, flipMask := twirlReadoutCircuit(baseCirc, rng)
		counts, err := runner(ctx, twirled, calibShots)
		if err != nil {
			return nil, fmt.Errorf("mitigation.CalibrateTREX: %w", err)
		}

		corrected := flipCounts(counts, flipMask, numQubits)
		for bs, c := range corrected {
			totalShots += c
			for i, ch := range bs {
				q := len(bs) - 1 - i // MSB-first
				if ch == '1' {
					oneCounts[q] += c
				}
			}
		}
	}

	flipRates := make([]float64, numQubits)
	if totalShots > 0 {
		for q := range numQubits {
			flipRates[q] = float64(oneCounts[q]) / float64(totalShots)
		}
	}

	return &TREXCalibration{
		numQubits: numQubits,
		flipRates: flipRates,
	}, nil
}

// CorrectExpectation corrects a raw expectation value of a Z-type observable
// on the given qubits using the calibrated flip rates.
// The correction divides by ∏(1 - 2·f_q) for each qubit in the observable.
func (cal *TREXCalibration) CorrectExpectation(rawValue float64, qubits []int) float64 {
	factor := 1.0
	for _, q := range qubits {
		if q >= 0 && q < len(cal.flipRates) {
			factor *= 1 - 2*cal.flipRates[q]
		}
	}
	if factor == 0 {
		return rawValue
	}
	return rawValue / factor
}

// TREXConfig specifies parameters for TREX readout error mitigation.
type TREXConfig struct {
	// Circuit is the quantum circuit to run.
	Circuit *ir.Circuit
	// Runner executes a circuit with shots and returns counts.
	Runner ShotRunner
	// Shots is the number of measurement shots per sample.
	Shots int
	// Samples is the number of twirled copies. Default: 10.
	Samples int
	// Calibration is a pre-computed calibration. If nil, auto-calibrate.
	Calibration *TREXCalibration
	// CalibShots is the number of shots for auto-calibration. Default: 1000.
	CalibShots int
}

// TREXResult holds the output of TREX readout error mitigation.
type TREXResult struct {
	// Counts are the merged, corrected measurement counts.
	Counts map[string]int
	// Calibration is the calibration data (computed or provided).
	Calibration *TREXCalibration
}

// RunTREX performs Twirled Readout Error eXtinction.
//
// It inserts random X gates before measurements, classically undoes the
// bit flips, and merges results across multiple twirled copies. This mitigates
// readout errors with O(n) overhead instead of O(2^n) for confusion-matrix methods.
func RunTREX(ctx context.Context, cfg TREXConfig) (*TREXResult, error) {
	if cfg.Circuit == nil {
		return nil, fmt.Errorf("mitigation.RunTREX: Circuit is nil")
	}
	if cfg.Runner == nil {
		return nil, fmt.Errorf("mitigation.RunTREX: Runner is nil")
	}
	if cfg.Shots < 1 {
		return nil, fmt.Errorf("mitigation.RunTREX: Shots must be >= 1")
	}

	samples := cfg.Samples
	if samples <= 0 {
		samples = 10
	}

	cal := cfg.Calibration
	if cal == nil {
		calibShots := cfg.CalibShots
		if calibShots <= 0 {
			calibShots = 1000
		}
		var err error
		cal, err = CalibrateTREX(ctx, cfg.Circuit.NumQubits(), calibShots, cfg.Runner)
		if err != nil {
			return nil, fmt.Errorf("mitigation.RunTREX: auto-calibrate: %w", err)
		}
	}

	rng := rand.New(rand.NewSource(rand.Int63()))
	merged := make(map[string]int)

	for range samples {
		twirled, flipMask := twirlReadoutCircuit(cfg.Circuit, rng)
		counts, err := cfg.Runner(ctx, twirled, cfg.Shots)
		if err != nil {
			return nil, fmt.Errorf("mitigation.RunTREX: %w", err)
		}

		corrected := flipCounts(counts, flipMask, cfg.Circuit.NumQubits())
		for bs, c := range corrected {
			merged[bs] += c
		}
	}

	return &TREXResult{
		Counts:      merged,
		Calibration: cal,
	}, nil
}

// twirlReadoutCircuit inserts random X gates before each measurement operation
// and returns the modified circuit along with a bitmask of flipped qubits.
func twirlReadoutCircuit(circuit *ir.Circuit, rng *rand.Rand) (*ir.Circuit, int) {
	ops := circuit.Ops()
	flipMask := 0
	var newOps []ir.Operation

	for _, op := range ops {
		// Detect measurement: Gate==nil and Clbits non-empty.
		if op.Gate == nil && len(op.Clbits) > 0 {
			// For each measured qubit, randomly insert X before measurement.
			for _, q := range op.Qubits {
				if rng.Intn(2) == 1 {
					newOps = append(newOps, ir.Operation{
						Gate:   gate.X,
						Qubits: []int{q},
					})
					flipMask ^= 1 << q
				}
			}
		}
		newOps = append(newOps, op)
	}

	circ := ir.New(circuit.Name(), circuit.NumQubits(), circuit.NumClbits(),
		newOps, circuit.Metadata())
	return circ, flipMask
}

// flipCounts classically undoes the X insertions by XORing each bitstring
// with the flip mask.
func flipCounts(counts map[string]int, flipMask, numQubits int) map[string]int {
	if flipMask == 0 {
		return counts
	}
	result := make(map[string]int, len(counts))
	for bs, c := range counts {
		idx := bitstringToInt(bs)
		idx ^= flipMask
		result[intToBitstring(idx, numQubits)] += c
	}
	return result
}
