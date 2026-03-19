package qir

import "github.com/splch/goqu/circuit/ir"

// Profile identifies a QIR target profile.
type Profile int

const (
	// BaseProfile represents static quantum circuits with no mid-circuit
	// measurement, no classical branching, and no dynamic qubit management.
	// All measurements occur at the end, and results are recorded via
	// __quantum__rt__result_record_output. This is the most widely supported
	// profile across QIR-consuming platforms.
	BaseProfile Profile = iota

	// AdaptiveProfile extends BaseProfile with mid-circuit measurement,
	// classical branching based on measurement results, reset operations,
	// and structured control flow (if/else, while, for, switch).
	AdaptiveProfile
)

// String returns the QIR profile attribute value.
func (p Profile) String() string {
	switch p {
	case BaseProfile:
		return "base_profile"
	case AdaptiveProfile:
		return "adaptive_profile"
	default:
		return "base_profile"
	}
}

// DetectProfile returns the appropriate QIR profile for the given circuit.
// Dynamic circuits (mid-circuit measurement, classical conditioning, reset,
// or control flow) require [AdaptiveProfile]; all others use [BaseProfile].
func DetectProfile(c *ir.Circuit) Profile {
	if c.IsDynamic() {
		return AdaptiveProfile
	}
	return BaseProfile
}
