package ionq

import "fmt"

// PulseShapes configures custom MS gate pulse envelopes per qubit pair.
// Schema: am-v4 (amplitude modulation).
// Only supported with native gateset circuits.
//
// See https://github.com/pdanford/qiskit-ionq for schema details.
type PulseShapes struct {
	Iteration  int         // iteration counter for calibration tracking
	SeedSource string      // traceability label for pulse data origin
	Pairs      []PulsePair // per-qubit-pair pulse definitions
}

// PulsePair defines the pulse envelope for MS gates on a specific qubit pair.
type PulsePair struct {
	Q0, Q1          int        // qubit indices
	Amplitudes      []float64  // piecewise-constant envelope (arb. units)
	DurationUsec    float64    // total pulse duration in microseconds
	Scale           float64    // amplitude scaling factor [0.0, 1.0]
	NearestModesIdx [2]int     // indices of lower/upper motional modes
	RelDet          [2]float64 // detuning weights for nearest modes
	DetuningShift   float64    // optional carrier frequency shift (MHz), default 0
	Tag             string     // optional user annotation
}

// NewPulseShapes creates a validated PulseShapes configuration.
func NewPulseShapes(iteration int, seedSource string, pairs ...PulsePair) (*PulseShapes, error) {
	if len(pairs) == 0 {
		return nil, fmt.Errorf("ionq: pulse shapes require at least one pair")
	}
	for i, p := range pairs {
		if len(p.Amplitudes) == 0 {
			return nil, fmt.Errorf("ionq: pulse pair %d (%d,%d) has empty amplitudes", i, p.Q0, p.Q1)
		}
		if p.DurationUsec <= 0 {
			return nil, fmt.Errorf("ionq: pulse pair %d (%d,%d) duration must be positive, got %g", i, p.Q0, p.Q1, p.DurationUsec)
		}
		if p.Scale < 0 || p.Scale > 1 {
			return nil, fmt.Errorf("ionq: pulse pair %d (%d,%d) scale must be in [0,1], got %g", i, p.Q0, p.Q1, p.Scale)
		}
	}
	// Defensive copy.
	copied := make([]PulsePair, len(pairs))
	for i, p := range pairs {
		amps := make([]float64, len(p.Amplitudes))
		copy(amps, p.Amplitudes)
		copied[i] = p
		copied[i].Amplitudes = amps
	}
	return &PulseShapes{
		Iteration:  iteration,
		SeedSource: seedSource,
		Pairs:      copied,
	}, nil
}

// marshalPulseShapes converts PulseShapes into the runtime_options JSON map
// matching IonQ's am-v4 schema.
func marshalPulseShapes(ps *PulseShapes) (map[string]any, error) {
	if ps == nil {
		return nil, fmt.Errorf("ionq: nil pulse shapes")
	}

	shapes := map[string]any{
		"schema":      "am-v4",
		"iteration":   ps.Iteration,
		"seed_source": ps.SeedSource,
	}

	for _, p := range ps.Pairs {
		key := fmt.Sprintf("(%d,%d)", p.Q0, p.Q1)
		entry := map[string]any{
			"amplitudes":      p.Amplitudes,
			"durationUsec":    p.DurationUsec,
			"scale":           p.Scale,
			"nearestModesIdx": [2]int{p.NearestModesIdx[0], p.NearestModesIdx[1]},
			"relDet":          [2]float64{p.RelDet[0], p.RelDet[1]},
		}
		if p.DetuningShift != 0 {
			entry["detuningShift"] = p.DetuningShift
		}
		if p.Tag != "" {
			entry["tag"] = p.Tag
		}
		shapes[key] = entry
	}

	return map[string]any{
		"custom_pulse_shapes": shapes,
	}, nil
}
