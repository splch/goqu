package waveform

import (
	"math"
	"testing"
)

// FuzzWaveformSample constructs random waveforms and verifies that Sample
// never panics, returns the expected length, and produces finite samples.
func FuzzWaveformSample(f *testing.F) {
	f.Add(1.0, 1e-7, 2e-8, 0.5, 5e-8, 1e-9, uint8(0))
	f.Add(0.5, 5e-8, 1e-8, 0.0, 2e-8, 1e-9, uint8(1))
	f.Add(0.1, 2e-7, 5e-8, 1.0, 1e-7, 2e-9, uint8(2))
	f.Add(1.0, 1e-7, 3e-8, 0.3, 6e-8, 1e-9, uint8(3))
	f.Add(0.8, 1e-7, 0.0, 0.0, 0.0, 1e-9, uint8(4))

	f.Fuzz(func(t *testing.T, amplitude, duration, sigma, beta, width, dt float64, kind uint8) {
		// Filter clearly invalid inputs.
		if math.IsNaN(amplitude) || math.IsInf(amplitude, 0) || math.Abs(amplitude) > 1e10 {
			return
		}
		if duration <= 0 || duration > 1e-3 || math.IsNaN(duration) || math.IsInf(duration, 0) {
			return
		}
		if dt <= 0 || dt > duration || math.IsNaN(dt) || math.IsInf(dt, 0) {
			return
		}
		if math.IsNaN(sigma) || math.IsInf(sigma, 0) || sigma < 0 || sigma > duration {
			return
		}
		if math.IsNaN(beta) || math.IsInf(beta, 0) || math.Abs(beta) > 1e10 {
			return
		}
		if math.IsNaN(width) || math.IsInf(width, 0) || width < 0 || width > duration {
			return
		}

		var wf interface {
			Sample(float64) []complex128
			Duration() float64
		}
		var err error

		switch kind % 5 {
		case 0:
			wf, err = Constant(complex(amplitude, 0), duration)
		case 1:
			if sigma <= 0 {
				return
			}
			wf, err = Gaussian(amplitude, duration, sigma)
		case 2:
			if sigma <= 0 {
				return
			}
			wf, err = DRAG(amplitude, duration, sigma, beta)
		case 3:
			if sigma <= 0 {
				return
			}
			wf, err = GaussianSquare(amplitude, duration, sigma, width)
		case 4:
			samples := []complex128{complex(amplitude, 0)}
			wf, err = Arbitrary(samples, dt)
		}

		if err != nil {
			return
		}

		out := wf.Sample(dt)
		expectedLen := int(math.Ceil(wf.Duration() / dt))
		if len(out) != expectedLen {
			t.Errorf("Sample(%g) returned %d samples, expected %d (dur=%g)",
				dt, len(out), expectedLen, wf.Duration())
		}

		for i, s := range out {
			if math.IsNaN(real(s)) || math.IsNaN(imag(s)) ||
				math.IsInf(real(s), 0) || math.IsInf(imag(s), 0) {
				t.Errorf("sample[%d] is non-finite: %v", i, s)
			}
		}
	})
}
