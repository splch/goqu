package waveform

import (
	"fmt"
	"math"

	"github.com/splch/qgo/pulse"
)

// parametric implements pulse.Waveform using a closure-based sample function.
type parametric struct {
	name     string
	duration float64
	sampleFn func(dt float64) []complex128
}

func (p *parametric) Name() string                   { return p.name }
func (p *parametric) Duration() float64              { return p.duration }
func (p *parametric) Sample(dt float64) []complex128 { return p.sampleFn(dt) }

// Constant returns a flat-amplitude waveform.
func Constant(amplitude complex128, duration float64) (pulse.Waveform, error) {
	if duration <= 0 {
		return nil, fmt.Errorf("waveform: constant duration must be positive, got %g", duration)
	}
	return &parametric{
		name:     fmt.Sprintf("constant(%g+%gi, %g)", real(amplitude), imag(amplitude), duration),
		duration: duration,
		sampleFn: func(dt float64) []complex128 {
			n := int(math.Ceil(duration / dt))
			out := make([]complex128, n)
			for i := range out {
				out[i] = amplitude
			}
			return out
		},
	}, nil
}

// MustConstant is like Constant but panics on error.
func MustConstant(amplitude complex128, duration float64) pulse.Waveform {
	w, err := Constant(amplitude, duration)
	if err != nil {
		panic(err)
	}
	return w
}

// Gaussian returns a Gaussian-shaped waveform.
// amplitude is the peak amplitude, duration is the total length in seconds,
// and sigma is the standard deviation in seconds.
func Gaussian(amplitude float64, duration, sigma float64) (pulse.Waveform, error) {
	if duration <= 0 {
		return nil, fmt.Errorf("waveform: gaussian duration must be positive, got %g", duration)
	}
	if sigma <= 0 {
		return nil, fmt.Errorf("waveform: gaussian sigma must be positive, got %g", sigma)
	}
	return &parametric{
		name:     fmt.Sprintf("gaussian(%g, %g, %g)", amplitude, duration, sigma),
		duration: duration,
		sampleFn: func(dt float64) []complex128 {
			n := int(math.Ceil(duration / dt))
			out := make([]complex128, n)
			center := duration / 2
			for i := range out {
				t := float64(i) * dt
				d := t - center
				out[i] = complex(amplitude*math.Exp(-d*d/(2*sigma*sigma)), 0)
			}
			return out
		},
	}, nil
}

// MustGaussian is like Gaussian but panics on error.
func MustGaussian(amplitude float64, duration, sigma float64) pulse.Waveform {
	w, err := Gaussian(amplitude, duration, sigma)
	if err != nil {
		panic(err)
	}
	return w
}

// DRAG returns a Derivative Removal by Adiabatic Gate (DRAG) waveform.
// The real part is a Gaussian envelope; the imaginary part is its derivative
// scaled by beta, which corrects for leakage to non-computational states.
func DRAG(amplitude float64, duration, sigma, beta float64) (pulse.Waveform, error) {
	if duration <= 0 {
		return nil, fmt.Errorf("waveform: DRAG duration must be positive, got %g", duration)
	}
	if sigma <= 0 {
		return nil, fmt.Errorf("waveform: DRAG sigma must be positive, got %g", sigma)
	}
	return &parametric{
		name:     fmt.Sprintf("drag(%g, %g, %g, %g)", amplitude, duration, sigma, beta),
		duration: duration,
		sampleFn: func(dt float64) []complex128 {
			n := int(math.Ceil(duration / dt))
			out := make([]complex128, n)
			center := duration / 2
			for i := range out {
				t := float64(i) * dt
				d := t - center
				gauss := amplitude * math.Exp(-d*d/(2*sigma*sigma))
				deriv := -d / (sigma * sigma) * gauss
				out[i] = complex(gauss, beta*deriv)
			}
			return out
		},
	}, nil
}

// MustDRAG is like DRAG but panics on error.
func MustDRAG(amplitude float64, duration, sigma, beta float64) pulse.Waveform {
	w, err := DRAG(amplitude, duration, sigma, beta)
	if err != nil {
		panic(err)
	}
	return w
}

// GaussianSquare returns a Gaussian-square waveform: a flat-top pulse with
// Gaussian rise and fall edges. width is the flat-top duration in seconds;
// sigma controls the Gaussian edges.
func GaussianSquare(amplitude float64, duration, sigma, width float64) (pulse.Waveform, error) {
	if duration <= 0 {
		return nil, fmt.Errorf("waveform: gaussian_square duration must be positive, got %g", duration)
	}
	if sigma <= 0 {
		return nil, fmt.Errorf("waveform: gaussian_square sigma must be positive, got %g", sigma)
	}
	if width < 0 || width > duration {
		return nil, fmt.Errorf("waveform: gaussian_square width must be in [0, duration], got %g", width)
	}
	return &parametric{
		name:     fmt.Sprintf("gaussian_square(%g, %g, %g, %g)", amplitude, duration, sigma, width),
		duration: duration,
		sampleFn: func(dt float64) []complex128 {
			n := int(math.Ceil(duration / dt))
			out := make([]complex128, n)
			riseEnd := (duration - width) / 2
			fallStart := riseEnd + width
			for i := range out {
				t := float64(i) * dt
				var val float64
				switch {
				case t < riseEnd:
					d := t - riseEnd
					val = amplitude * math.Exp(-d*d/(2*sigma*sigma))
				case t <= fallStart:
					val = amplitude
				default:
					d := t - fallStart
					val = amplitude * math.Exp(-d*d/(2*sigma*sigma))
				}
				out[i] = complex(val, 0)
			}
			return out
		},
	}, nil
}

// MustGaussianSquare is like GaussianSquare but panics on error.
func MustGaussianSquare(amplitude float64, duration, sigma, width float64) pulse.Waveform {
	w, err := GaussianSquare(amplitude, duration, sigma, width)
	if err != nil {
		panic(err)
	}
	return w
}

// Arbitrary creates a waveform from pre-computed samples at the given dt.
// The samples slice is defensively copied.
func Arbitrary(samples []complex128, dt float64) (pulse.Waveform, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("waveform: arbitrary samples must not be empty")
	}
	if dt <= 0 {
		return nil, fmt.Errorf("waveform: arbitrary dt must be positive, got %g", dt)
	}
	copied := make([]complex128, len(samples))
	copy(copied, samples)
	duration := float64(len(samples)) * dt
	return &parametric{
		name:     fmt.Sprintf("arbitrary(%d samples, dt=%g)", len(samples), dt),
		duration: duration,
		sampleFn: func(requestedDt float64) []complex128 {
			if requestedDt == dt {
				out := make([]complex128, len(copied))
				copy(out, copied)
				return out
			}
			// Resample via nearest-neighbor interpolation.
			n := int(math.Ceil(duration / requestedDt))
			out := make([]complex128, n)
			for i := range out {
				t := float64(i) * requestedDt
				idx := int(t / dt)
				if idx >= len(copied) {
					idx = len(copied) - 1
				}
				out[i] = copied[idx]
			}
			return out
		},
	}, nil
}

// MustArbitrary is like Arbitrary but panics on error.
func MustArbitrary(samples []complex128, dt float64) pulse.Waveform {
	w, err := Arbitrary(samples, dt)
	if err != nil {
		panic(err)
	}
	return w
}
