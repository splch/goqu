package waveform

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestConstant(t *testing.T) {
	w, err := Constant(0.5+0.1i, 1e-6)
	if err != nil {
		t.Fatal(err)
	}
	if w.Duration() != 1e-6 {
		t.Errorf("Duration() = %g, want %g", w.Duration(), 1e-6)
	}

	dt := 1e-9
	samples := w.Sample(dt)
	expected := int(math.Ceil(1e-6 / dt))
	if len(samples) != expected {
		t.Errorf("len(samples) = %d, want %d", len(samples), expected)
	}
	for i, s := range samples {
		if cmplx.Abs(s-(0.5+0.1i)) > 1e-12 {
			t.Errorf("sample[%d] = %v, want %v", i, s, 0.5+0.1i)
			break
		}
	}
}

func TestConstantError(t *testing.T) {
	_, err := Constant(1, 0)
	if err == nil {
		t.Error("expected error for zero duration")
	}
	_, err = Constant(1, -1)
	if err == nil {
		t.Error("expected error for negative duration")
	}
}

func TestGaussian(t *testing.T) {
	w, err := Gaussian(1.0, 1e-7, 2e-8)
	if err != nil {
		t.Fatal(err)
	}

	dt := 1e-9
	samples := w.Sample(dt)
	// Peak should be at center
	center := len(samples) / 2
	peak := real(samples[center])
	if math.Abs(peak-1.0) > 0.01 {
		t.Errorf("peak amplitude = %g, want ~1.0", peak)
	}

	// Edges should be near zero
	edge := real(samples[0])
	if edge > 0.1 {
		t.Errorf("edge amplitude = %g, want near zero", edge)
	}
}

func TestGaussianErrors(t *testing.T) {
	tests := []struct {
		amp, dur, sigma float64
	}{
		{1, 0, 1e-8},     // zero duration
		{1, 1e-7, 0},     // zero sigma
		{1, -1e-7, 1e-8}, // negative duration
		{1, 1e-7, -1e-8}, // negative sigma
	}
	for _, tt := range tests {
		_, err := Gaussian(tt.amp, tt.dur, tt.sigma)
		if err == nil {
			t.Errorf("Gaussian(%g, %g, %g) expected error", tt.amp, tt.dur, tt.sigma)
		}
	}
}

func TestDRAG(t *testing.T) {
	w, err := DRAG(1.0, 1e-7, 2e-8, 0.5)
	if err != nil {
		t.Fatal(err)
	}

	dt := 1e-9
	samples := w.Sample(dt)
	center := len(samples) / 2

	// Real part at center should be ~1.0 (peak of Gaussian)
	if math.Abs(real(samples[center])-1.0) > 0.01 {
		t.Errorf("real part at center = %g, want ~1.0", real(samples[center]))
	}

	// Imaginary part at center should be ~0 (derivative is zero at peak)
	if math.Abs(imag(samples[center])) > 0.01 {
		t.Errorf("imag part at center = %g, want ~0", imag(samples[center]))
	}

	// Imaginary part before center should be nonzero (positive derivative)
	if imag(samples[center/2]) == 0 {
		t.Error("imag part before center should be nonzero")
	}
}

func TestDRAGErrors(t *testing.T) {
	_, err := DRAG(1, 0, 1e-8, 0.5)
	if err == nil {
		t.Error("expected error for zero duration")
	}
	_, err = DRAG(1, 1e-7, 0, 0.5)
	if err == nil {
		t.Error("expected error for zero sigma")
	}
}

func TestGaussianSquare(t *testing.T) {
	dur := 1e-7
	sigma := 1e-8
	width := 5e-8
	w, err := GaussianSquare(1.0, dur, sigma, width)
	if err != nil {
		t.Fatal(err)
	}

	dt := 1e-9
	samples := w.Sample(dt)

	// Center should be at peak amplitude (flat top)
	center := len(samples) / 2
	if math.Abs(real(samples[center])-1.0) > 1e-10 {
		t.Errorf("flat top = %g, want 1.0", real(samples[center]))
	}
}

func TestGaussianSquareErrors(t *testing.T) {
	_, err := GaussianSquare(1, 0, 1e-8, 5e-8)
	if err == nil {
		t.Error("expected error for zero duration")
	}
	_, err = GaussianSquare(1, 1e-7, 0, 5e-8)
	if err == nil {
		t.Error("expected error for zero sigma")
	}
	_, err = GaussianSquare(1, 1e-7, 1e-8, 2e-7)
	if err == nil {
		t.Error("expected error for width > duration")
	}
	_, err = GaussianSquare(1, 1e-7, 1e-8, -1)
	if err == nil {
		t.Error("expected error for negative width")
	}
}

func TestArbitrary(t *testing.T) {
	original := []complex128{1, 2, 3, 4, 5}
	dt := 1e-9
	w, err := Arbitrary(original, dt)
	if err != nil {
		t.Fatal(err)
	}

	expectedDur := 5e-9
	if math.Abs(w.Duration()-expectedDur) > 1e-15 {
		t.Errorf("Duration() = %g, want %g", w.Duration(), expectedDur)
	}

	// Sample at same dt should return identical samples.
	samples := w.Sample(dt)
	if len(samples) != 5 {
		t.Fatalf("len(samples) = %d, want 5", len(samples))
	}
	for i, s := range samples {
		if s != original[i] {
			t.Errorf("sample[%d] = %v, want %v", i, s, original[i])
		}
	}

	// Verify defensive copy - mutating original doesn't affect waveform.
	original[0] = 999
	samples2 := w.Sample(dt)
	if samples2[0] != 1 {
		t.Error("Arbitrary should defensively copy samples")
	}
}

func TestArbitraryResample(t *testing.T) {
	original := []complex128{1, 2, 3, 4}
	dt := 1e-9
	w, err := Arbitrary(original, dt)
	if err != nil {
		t.Fatal(err)
	}

	// Sample at 2x dt - should get 2 samples via nearest-neighbor.
	samples := w.Sample(2e-9)
	if len(samples) != 2 {
		t.Fatalf("len(samples) = %d, want 2", len(samples))
	}
	if samples[0] != 1 {
		t.Errorf("sample[0] = %v, want 1", samples[0])
	}
	if samples[1] != 3 {
		t.Errorf("sample[1] = %v, want 3", samples[1])
	}
}

func TestArbitraryErrors(t *testing.T) {
	_, err := Arbitrary(nil, 1e-9)
	if err == nil {
		t.Error("expected error for nil samples")
	}
	_, err = Arbitrary([]complex128{1}, 0)
	if err == nil {
		t.Error("expected error for zero dt")
	}
}

func TestMustVariants(t *testing.T) {
	// MustConstant
	w := MustConstant(1, 1e-6)
	if w.Duration() != 1e-6 {
		t.Error("MustConstant failed")
	}

	// MustGaussian
	w = MustGaussian(1, 1e-7, 2e-8)
	if w.Duration() != 1e-7 {
		t.Error("MustGaussian failed")
	}

	// MustDRAG
	w = MustDRAG(1, 1e-7, 2e-8, 0.5)
	if w.Duration() != 1e-7 {
		t.Error("MustDRAG failed")
	}

	// MustGaussianSquare
	w = MustGaussianSquare(1, 1e-7, 1e-8, 5e-8)
	if w.Duration() != 1e-7 {
		t.Error("MustGaussianSquare failed")
	}

	// MustArbitrary
	w = MustArbitrary([]complex128{1, 2, 3}, 1e-9)
	if math.Abs(w.Duration()-3e-9) > 1e-20 {
		t.Error("MustArbitrary failed")
	}
}

func TestMustPanics(t *testing.T) {
	panics := func(fn func()) (panicked bool) {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		fn()
		return false
	}

	if !panics(func() { MustConstant(1, 0) }) {
		t.Error("MustConstant(1, 0) should panic")
	}
	if !panics(func() { MustGaussian(1, 0, 1e-8) }) {
		t.Error("MustGaussian should panic")
	}
	if !panics(func() { MustDRAG(1, 0, 1e-8, 0.5) }) {
		t.Error("MustDRAG should panic")
	}
	if !panics(func() { MustGaussianSquare(1, 0, 1e-8, 5e-8) }) {
		t.Error("MustGaussianSquare should panic")
	}
	if !panics(func() { MustArbitrary(nil, 1e-9) }) {
		t.Error("MustArbitrary should panic")
	}
}
