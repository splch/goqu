// Package waveform provides standard pulse envelope shapes for the
// [pulse] package.
//
// Five waveform constructors are provided: [Constant] (flat amplitude),
// [Gaussian] (bell curve), [DRAG] (Derivative Removal by Adiabatic Gate),
// [GaussianSquare] (flat-top with Gaussian edges), and [Arbitrary]
// (user-provided samples).
//
// Each constructor returns a [pulse.Waveform] that lazily samples at
// the requested time resolution. Each also has a Must variant that
// panics on error.
package waveform
