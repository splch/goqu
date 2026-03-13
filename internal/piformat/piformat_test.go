package piformat

import (
	"math"
	"testing"
)

func TestFormatUnicode(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{math.Pi, "π"},
		{-math.Pi, "-π"},
		{math.Pi / 2, "π/2"},
		{math.Pi / 4, "π/4"},
		{3 * math.Pi / 4, "3*π/4"},
		{0.123, "0.123"},
	}
	for _, tt := range tests {
		got := FormatUnicode(tt.v)
		if got != tt.want {
			t.Errorf("FormatUnicode(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestFormatASCII(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{math.Pi, "pi"},
		{-math.Pi, "-pi"},
		{math.Pi / 2, "pi/2"},
		{0, "0"},
		{1e-12, "0"},
		{0.5, "0.5"},
	}
	for _, tt := range tests {
		got := FormatASCII(tt.v)
		if got != tt.want {
			t.Errorf("FormatASCII(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestFormatLaTeX(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{math.Pi, `\pi`},
		{-math.Pi, `-\pi`},
		{math.Pi / 2, `\frac{\pi}{2}`},
		{math.Pi / 4, `\frac{\pi}{4}`},
		{3 * math.Pi / 4, `\frac{3\pi}{4}`},
		{-math.Pi / 4, `-\frac{\pi}{4}`},
		{-3 * math.Pi / 4, `-\frac{3\pi}{4}`},
		{0, "0"},
		{1e-12, "0"},
		{0.123, "0.123"},
	}
	for _, tt := range tests {
		got := FormatLaTeX(tt.v)
		if got != tt.want {
			t.Errorf("FormatLaTeX(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestFormatQASM(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{math.Pi, "pi"},
		{math.Pi / 4, "pi/4"},
		{0.123456789, "0.123456789"},
	}
	for _, tt := range tests {
		got := FormatQASM(tt.v)
		if got != tt.want {
			t.Errorf("FormatQASM(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}
