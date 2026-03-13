// Package piformat formats angles as pi fractions for display.
package piformat

import (
	"fmt"
	"math"
)

const tol = 1e-10

// piFraction detects if v is a simple fraction of pi (denom 1–16).
// Returns numerator, denominator, and ok=true if found.
func piFraction(v float64) (num, denom int, ok bool) {
	ratio := v / math.Pi
	// Check ±pi.
	if math.Abs(ratio-1) < tol {
		return 1, 1, true
	}
	if math.Abs(ratio+1) < tol {
		return -1, 1, true
	}
	// Check fractions pi/2 through pi/16.
	for d := 2; d <= 16; d++ {
		n := ratio * float64(d)
		if math.Abs(n-math.Round(n)) < tol {
			return int(math.Round(n)), d, true
		}
	}
	return 0, 0, false
}

// FormatUnicode formats an angle using the "π" symbol and "%.4g" fallback.
func FormatUnicode(v float64) string {
	num, denom, ok := piFraction(v)
	if !ok {
		return fmt.Sprintf("%.4g", v)
	}
	return formatFraction(num, denom, "π")
}

// FormatASCII formats an angle using "pi", includes a near-zero check,
// and uses "%.4g" as fallback.
func FormatASCII(v float64) string {
	if math.Abs(v) < tol {
		return "0"
	}
	num, denom, ok := piFraction(v)
	if !ok {
		return fmt.Sprintf("%.4g", v)
	}
	return formatFraction(num, denom, "pi")
}

// FormatQASM formats an angle using "pi" with "%.10g" fallback (higher precision).
func FormatQASM(v float64) string {
	num, denom, ok := piFraction(v)
	if !ok {
		return fmt.Sprintf("%.10g", v)
	}
	return formatFraction(num, denom, "pi")
}

// FormatLaTeX formats an angle using LaTeX math notation for pi fractions.
// Produces \pi, \frac{\pi}{4}, \frac{3\pi}{4}, etc.
// Falls back to "%.4g" for non-pi-fraction values.
func FormatLaTeX(v float64) string {
	if math.Abs(v) < tol {
		return "0"
	}
	num, denom, ok := piFraction(v)
	if !ok {
		return fmt.Sprintf("%.4g", v)
	}
	return formatLaTeXFraction(num, denom)
}

// formatLaTeXFraction renders a pi fraction in LaTeX math notation.
func formatLaTeXFraction(num, denom int) string {
	if denom == 1 {
		if num == 1 {
			return `\pi`
		}
		if num == -1 {
			return `-\pi`
		}
	}
	sign := ""
	if num < 0 {
		sign = "-"
		num = -num
	}
	numerator := `\pi`
	if num != 1 {
		numerator = fmt.Sprintf(`%d\pi`, num)
	}
	return fmt.Sprintf(`%s\frac{%s}{%d}`, sign, numerator, denom)
}

// formatFraction renders n*sym/d as a human-readable string.
func formatFraction(num, denom int, sym string) string {
	if denom == 1 {
		if num == 1 {
			return sym
		}
		if num == -1 {
			return "-" + sym
		}
	}
	if num == 1 {
		return fmt.Sprintf("%s/%d", sym, denom)
	}
	if num == -1 {
		return fmt.Sprintf("-%s/%d", sym, denom)
	}
	return fmt.Sprintf("%d*%s/%d", num, sym, denom)
}
