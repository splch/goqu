// Package mathutil provides internal math helpers shared across qgo packages.
//
// [NormalizeAngle] wraps an angle to (−π, π]. [NearZeroMod2Pi] tests whether
// an angle is effectively zero modulo 2π. [StripParams] and
// [StripParamsAndDagger] extract base gate names for decomposition lookups.
package mathutil
