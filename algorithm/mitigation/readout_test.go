package mitigation_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/mitigation"
)

// mockBasisExecutor creates a BasisExecutor that applies known readout errors.
func mockBasisExecutor(numQubits int, p01, p10 float64) mitigation.BasisExecutor {
	return func(_ context.Context, basisState int, shots int) (map[string]int, error) {
		// For each qubit, independently apply readout error.
		counts := make(map[string]int)
		bs := make([]byte, numQubits)

		// Simple deterministic approach: compute expected counts.
		dim := 1 << numQubits
		for measured := range dim {
			prob := 1.0
			for q := range numQubits {
				prepBit := (basisState >> q) & 1
				measBit := (measured >> q) & 1
				switch {
				case prepBit == 0 && measBit == 0:
					prob *= (1 - p01)
				case prepBit == 0 && measBit == 1:
					prob *= p01
				case prepBit == 1 && measBit == 0:
					prob *= p10
				case prepBit == 1 && measBit == 1:
					prob *= (1 - p10)
				}
			}
			c := int(math.Round(prob * float64(shots)))
			if c > 0 {
				for i := range numQubits {
					if (measured>>i)&1 == 1 {
						bs[numQubits-1-i] = '1'
					} else {
						bs[numQubits-1-i] = '0'
					}
				}
				counts[string(bs)] = c
			}
		}
		return counts, nil
	}
}

func TestCalibrateReadout_CorrectCounts(t *testing.T) {
	numQubits := 2
	p01, p10 := 0.05, 0.03
	shots := 10000

	exec := mockBasisExecutor(numQubits, p01, p10)
	cal, err := mitigation.CalibrateReadout(context.Background(), numQubits, shots, exec)
	if err != nil {
		t.Fatal(err)
	}

	// Create noisy counts: ideal distribution is "00" = 100%.
	// Apply readout errors.
	noisyCounts, err := exec(context.Background(), 0, shots)
	if err != nil {
		t.Fatal(err)
	}

	corrected := cal.CorrectCounts(noisyCounts)

	// After correction, "00" should dominate.
	total := 0
	for _, c := range corrected {
		total += c
	}
	if total == 0 {
		t.Fatal("corrected counts sum to 0")
	}

	prob00 := float64(corrected["00"]) / float64(total)
	if prob00 < 0.95 {
		t.Errorf("corrected P(00) = %f, want > 0.95", prob00)
	}
}

func TestCalibrateReadout_CorrectProbabilities(t *testing.T) {
	numQubits := 1
	p01, p10 := 0.1, 0.05
	shots := 100000

	exec := mockBasisExecutor(numQubits, p01, p10)
	cal, err := mitigation.CalibrateReadout(context.Background(), numQubits, shots, exec)
	if err != nil {
		t.Fatal(err)
	}

	// Noisy probabilities for |0⟩: P(0) = 1-p01, P(1) = p01.
	noisyProbs := map[string]float64{
		"0": 1 - p01,
		"1": p01,
	}

	corrected := cal.CorrectProbabilities(noisyProbs)

	// After correction, P(0) should be close to 1.0.
	if math.Abs(corrected["0"]-1.0) > 0.02 {
		t.Errorf("corrected P(0) = %f, want ~1.0", corrected["0"])
	}
}

func TestCalibrateReadoutPerQubit(t *testing.T) {
	numQubits := 2
	p01, p10 := 0.05, 0.03
	shots := 100000

	exec := mockBasisExecutor(numQubits, p01, p10)
	cal, err := mitigation.CalibrateReadoutPerQubit(context.Background(), numQubits, shots, exec)
	if err != nil {
		t.Fatal(err)
	}

	// Apply readout errors to |10⟩ and correct.
	noisyCounts, err := exec(context.Background(), 2, shots) // |10⟩ = index 2
	if err != nil {
		t.Fatal(err)
	}

	corrected := cal.CorrectCounts(noisyCounts)

	total := 0
	for _, c := range corrected {
		total += c
	}
	if total == 0 {
		t.Fatal("corrected counts sum to 0")
	}

	prob10 := float64(corrected["10"]) / float64(total)
	if prob10 < 0.90 {
		t.Errorf("corrected P(10) = %f, want > 0.90", prob10)
	}
}

func TestCalibrateReadout_Errors(t *testing.T) {
	exec := mockBasisExecutor(1, 0, 0)

	t.Run("zero qubits", func(t *testing.T) {
		_, err := mitigation.CalibrateReadout(context.Background(), 0, 1000, exec)
		if err == nil {
			t.Error("expected error for 0 qubits")
		}
	})

	t.Run("zero shots", func(t *testing.T) {
		_, err := mitigation.CalibrateReadout(context.Background(), 1, 0, exec)
		if err == nil {
			t.Error("expected error for 0 shots")
		}
	})
}
