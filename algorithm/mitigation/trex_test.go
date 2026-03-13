package mitigation_test

import (
	"context"
	"math"
	"testing"

	"github.com/splch/goqu/algorithm/mitigation"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/ir"
)

// mockShotRunner returns a ShotRunner that produces deterministic counts
// with optional readout error simulation.
func mockShotRunner(p01, p10 float64) mitigation.ShotRunner {
	return func(_ context.Context, circuit *ir.Circuit, shots int) (map[string]int, error) {
		numQubits := circuit.NumQubits()

		// Run the "circuit" by looking at which X gates are present to
		// determine the prepared state. For simplicity, start in |0...0⟩
		// and flip qubits that have an X gate applied.
		state := 0
		for _, op := range circuit.Ops() {
			if op.Gate != nil && op.Gate.Name() == "X" && len(op.Qubits) == 1 {
				state ^= 1 << op.Qubits[0]
			}
		}

		// Apply readout errors deterministically.
		dim := 1 << numQubits
		counts := make(map[string]int)
		bs := make([]byte, numQubits)

		for measured := range dim {
			prob := 1.0
			for q := range numQubits {
				prepBit := (state >> q) & 1
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

func TestCalibrateTREX(t *testing.T) {
	p01, p10 := 0.05, 0.03

	cal, err := mitigation.CalibrateTREX(context.Background(), 2, 10000, mockShotRunner(p01, p10))
	if err != nil {
		t.Fatal(err)
	}

	// The twirl calibration measures f_q from the all-zero state.
	// After twirled readout, roughly p01/2 of the time we get the wrong answer.

	// Just check the correction formula works.
	rawValue := 0.9 // some noisy expectation
	corrected := cal.CorrectExpectation(rawValue, []int{0})

	// Corrected should be larger (closer to ideal).
	if corrected <= rawValue {
		t.Errorf("expected correction to increase value: raw=%f, corrected=%f", rawValue, corrected)
	}
}

func TestTREXCorrectExpectation_AnalyticalFormula(t *testing.T) {
	// Manually construct a calibration with known flip rates.
	cal, err := mitigation.CalibrateTREX(context.Background(), 2, 10000, mockShotRunner(0, 0))
	if err != nil {
		t.Fatal(err)
	}

	// With zero readout error, correction should be identity.
	raw := 0.8
	corrected := cal.CorrectExpectation(raw, []int{0, 1})
	if math.Abs(corrected-raw) > 0.05 {
		t.Errorf("zero-error correction: expected ~%f, got %f", raw, corrected)
	}
}

func TestRunTREX(t *testing.T) {
	p01, p10 := 0.05, 0.03

	circ, err := builder.New("test", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	result, err := mitigation.RunTREX(context.Background(), mitigation.TREXConfig{
		Circuit:    circ,
		Runner:     mockShotRunner(p01, p10),
		Shots:      1000,
		Samples:    5,
		CalibShots: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Counts) == 0 {
		t.Error("expected non-empty counts")
	}
	if result.Calibration == nil {
		t.Error("expected calibration to be returned")
	}

	// Verify total shots.
	total := 0
	for _, c := range result.Counts {
		total += c
	}
	if total == 0 {
		t.Error("total counts is 0")
	}
}

func TestRunTREX_Errors(t *testing.T) {
	runner := mockShotRunner(0, 0)

	t.Run("nil circuit", func(t *testing.T) {
		_, err := mitigation.RunTREX(context.Background(), mitigation.TREXConfig{
			Runner: runner,
			Shots:  100,
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("nil runner", func(t *testing.T) {
		circ, _ := builder.New("test", 1).H(0).MeasureAll().Build()
		_, err := mitigation.RunTREX(context.Background(), mitigation.TREXConfig{
			Circuit: circ,
			Shots:   100,
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("zero shots", func(t *testing.T) {
		circ, _ := builder.New("test", 1).H(0).MeasureAll().Build()
		_, err := mitigation.RunTREX(context.Background(), mitigation.TREXConfig{
			Circuit: circ,
			Runner:  runner,
			Shots:   0,
		})
		if err == nil {
			t.Error("expected error")
		}
	})
}
