package noise

import (
	"github.com/splch/qgo/transpile/target"
)

// CalibrationData contains hardware calibration parameters for building device noise models.
type CalibrationData struct {
	// GateTimes maps gate name to duration in nanoseconds.
	GateTimes map[string]float64
	// T1 maps qubit index to T1 relaxation time in nanoseconds.
	T1 map[int]float64
	// T2 maps qubit index to T2 dephasing time in nanoseconds.
	T2 map[int]float64
	// ReadoutErrors maps qubit index to (P(1|0), P(0|1)).
	ReadoutErrors map[int][2]float64
}

// FromTarget builds a NoiseModel from a target's gate fidelities and optional calibration data.
// Gate fidelities are converted to depolarizing error rates.
// If cal is non-nil, ThermalRelaxation channels are added per-qubit using T1/T2/gate times,
// and readout errors are added.
func FromTarget(t target.Target, cal *CalibrationData) *NoiseModel {
	m := New()

	for gateName, fidelity := range t.GateFidelities {
		if fidelity >= 1.0 {
			continue // perfect gate, no noise
		}
		if isGate2Q(gateName) {
			// 2Q depolarizing: p = (1 - F) * 16/15
			p := (1 - fidelity) * 16.0 / 15.0
			if p > 1 {
				p = 1
			}
			m.AddGateError(gateName, Depolarizing2Q(p))
		} else {
			// 1Q depolarizing: p = (1 - F) * 4/3
			p := (1 - fidelity) * 4.0 / 3.0
			if p > 1 {
				p = 1
			}
			m.AddGateError(gateName, Depolarizing1Q(p))
		}
	}

	if cal != nil {
		// Add ThermalRelaxation per gate+qubit where we have all needed data
		for gateName, gateTime := range cal.GateTimes {
			for qubit, t1 := range cal.T1 {
				t2, hasT2 := cal.T2[qubit]
				if !hasT2 {
					continue
				}
				if t1 <= 0 || t2 <= 0 || gateTime <= 0 {
					continue
				}
				// Ensure physical constraint t2 <= 2*t1
				if t2 > 2*t1 {
					t2 = 2 * t1
				}
				ch := ThermalRelaxation(t1, t2, gateTime)
				m.AddGateQubitError(gateName, []int{qubit}, ch)
			}
		}

		// Add readout errors
		for qubit, re := range cal.ReadoutErrors {
			m.AddReadoutError(qubit, NewReadoutError(re[0], re[1]))
		}
	}

	return m
}

// FromTargetSimple builds a NoiseModel from gate fidelities only (no calibration data).
func FromTargetSimple(t target.Target) *NoiseModel {
	return FromTarget(t, nil)
}

// isGate2Q returns true for known 2-qubit gate names.
func isGate2Q(name string) bool {
	switch name {
	case "CX", "CZ", "CNOT", "SWAP", "CY", "CP", "CRZ", "CRX", "CRY",
		"RXX", "RYY", "RZZ", "MS", "ECR":
		return true
	}
	return false
}
