package ionq

import (
	"encoding/json"
	"testing"
)

func TestNewPulseShapes(t *testing.T) {
	pair := PulsePair{
		Q0:              0,
		Q1:              1,
		Amplitudes:      []float64{0.1, 0.5, 0.9, 0.5, 0.1},
		DurationUsec:    40.0,
		Scale:           1.0,
		NearestModesIdx: [2]int{4, 5},
		RelDet:          [2]float64{1, 0},
	}
	ps, err := NewPulseShapes(0, "reference", pair)
	if err != nil {
		t.Fatal(err)
	}
	if ps.Iteration != 0 {
		t.Errorf("Iteration = %d, want 0", ps.Iteration)
	}
	if ps.SeedSource != "reference" {
		t.Errorf("SeedSource = %q, want %q", ps.SeedSource, "reference")
	}
	if len(ps.Pairs) != 1 {
		t.Fatalf("len(Pairs) = %d, want 1", len(ps.Pairs))
	}
}

func TestNewPulseShapesDefensiveCopy(t *testing.T) {
	amps := []float64{0.1, 0.5, 0.9}
	pair := PulsePair{
		Amplitudes:   amps,
		DurationUsec: 40.0,
		Scale:        1.0,
	}
	ps, err := NewPulseShapes(0, "test", pair)
	if err != nil {
		t.Fatal(err)
	}
	// Mutate original.
	amps[0] = 999
	if ps.Pairs[0].Amplitudes[0] != 0.1 {
		t.Error("PulseShapes should defensively copy amplitudes")
	}
}

func TestNewPulseShapesErrors(t *testing.T) {
	// No pairs.
	_, err := NewPulseShapes(0, "test")
	if err == nil {
		t.Error("expected error for no pairs")
	}

	// Empty amplitudes.
	_, err = NewPulseShapes(0, "test", PulsePair{DurationUsec: 40, Scale: 1})
	if err == nil {
		t.Error("expected error for empty amplitudes")
	}

	// Non-positive duration.
	_, err = NewPulseShapes(0, "test", PulsePair{
		Amplitudes:   []float64{0.1},
		DurationUsec: 0,
		Scale:        1,
	})
	if err == nil {
		t.Error("expected error for zero duration")
	}

	// Scale out of range.
	_, err = NewPulseShapes(0, "test", PulsePair{
		Amplitudes:   []float64{0.1},
		DurationUsec: 40,
		Scale:        1.5,
	})
	if err == nil {
		t.Error("expected error for scale > 1")
	}

	_, err = NewPulseShapes(0, "test", PulsePair{
		Amplitudes:   []float64{0.1},
		DurationUsec: 40,
		Scale:        -0.1,
	})
	if err == nil {
		t.Error("expected error for scale < 0")
	}
}

func TestMarshalPulseShapes(t *testing.T) {
	pair := PulsePair{
		Q0:              0,
		Q1:              1,
		Amplitudes:      []float64{0.1, 0.5, 0.9, 0.5, 0.1},
		DurationUsec:    40.0,
		Scale:           1.0,
		NearestModesIdx: [2]int{4, 5},
		RelDet:          [2]float64{1, 0},
		Tag:             "test_pulse",
	}
	ps, err := NewPulseShapes(0, "reference", pair)
	if err != nil {
		t.Fatal(err)
	}

	result, err := marshalPulseShapes(ps)
	if err != nil {
		t.Fatal(err)
	}

	// Verify structure by marshaling to JSON and back.
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	shapes, ok := decoded["custom_pulse_shapes"].(map[string]any)
	if !ok {
		t.Fatal("missing custom_pulse_shapes key")
	}

	if shapes["schema"] != "am-v4" {
		t.Errorf("schema = %v, want am-v4", shapes["schema"])
	}
	if shapes["seed_source"] != "reference" {
		t.Errorf("seed_source = %v, want reference", shapes["seed_source"])
	}

	pairData, ok := shapes["(0,1)"].(map[string]any)
	if !ok {
		t.Fatal("missing (0,1) pair key")
	}
	if pairData["durationUsec"].(float64) != 40.0 {
		t.Errorf("durationUsec = %v, want 40", pairData["durationUsec"])
	}
	if pairData["tag"] != "test_pulse" {
		t.Errorf("tag = %v, want test_pulse", pairData["tag"])
	}
}

func TestMarshalPulseShapesNil(t *testing.T) {
	_, err := marshalPulseShapes(nil)
	if err == nil {
		t.Error("expected error for nil pulse shapes")
	}
}

func TestMarshalPulseShapesDetuningShift(t *testing.T) {
	pair := PulsePair{
		Q0:            0,
		Q1:            1,
		Amplitudes:    []float64{0.5},
		DurationUsec:  40.0,
		Scale:         0.8,
		DetuningShift: 1.5,
	}
	ps, err := NewPulseShapes(1, "calibration", pair)
	if err != nil {
		t.Fatal(err)
	}

	result, err := marshalPulseShapes(ps)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	shapes := decoded["custom_pulse_shapes"].(map[string]any)
	pairData := shapes["(0,1)"].(map[string]any)
	if pairData["detuningShift"].(float64) != 1.5 {
		t.Errorf("detuningShift = %v, want 1.5", pairData["detuningShift"])
	}
}

func TestMarshalPulseShapesNoOptionals(t *testing.T) {
	pair := PulsePair{
		Q0:           2,
		Q1:           3,
		Amplitudes:   []float64{0.5},
		DurationUsec: 30.0,
		Scale:        1.0,
	}
	ps, err := NewPulseShapes(0, "test", pair)
	if err != nil {
		t.Fatal(err)
	}

	result, err := marshalPulseShapes(ps)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	shapes := decoded["custom_pulse_shapes"].(map[string]any)
	pairData := shapes["(2,3)"].(map[string]any)
	// DetuningShift should be omitted when zero.
	if _, ok := pairData["detuningShift"]; ok {
		t.Error("detuningShift should be omitted when zero")
	}
	// Tag should be omitted when empty.
	if _, ok := pairData["tag"]; ok {
		t.Error("tag should be omitted when empty")
	}
}
