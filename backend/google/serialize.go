package google

import (
	"fmt"
	"math"
	"strings"

	"github.com/splch/goqu/backend"
	"github.com/splch/goqu/circuit/ir"
)

// serializeCircuit converts a circuit IR to a Cirq JSON program using
// Google's native gate set (PhasedXZ + CZ + measure).
//
// Gate parameters are converted from radians to half-turns (exponent = radians / pi).
// CNOT is decomposed to H(target) + CZ + H(target).
// SWAP is decomposed to 3x CNOT, each further decomposed to CZ+H.
func serializeCircuit(c *ir.Circuit) (*cirqProgram, error) {
	ops := c.Ops()
	// Convert all operations to native ops (PhasedXZ, CZ, measure).
	var nativeOps []nativeOp
	for _, op := range ops {
		if op.ControlFlow != nil {
			return nil, fmt.Errorf("google: control flow operations not supported")
		}
		native, err := convertOp(op)
		if err != nil {
			return nil, err
		}
		nativeOps = append(nativeOps, native...)
	}

	// Schedule operations into moments based on qubit conflicts.
	moments := scheduleMoments(nativeOps, c.NumQubits())

	// Build qubit list.
	qubits := make([]cirqQubit, c.NumQubits())
	for i := range qubits {
		qubits[i] = cirqQubit{Type: "LineQubit", X: i}
	}

	return &cirqProgram{
		Type:    "Circuit",
		Moments: moments,
		Qubits:  qubits,
	}, nil
}

// nativeOp is an intermediate representation of a gate in Google's native set.
type nativeOp struct {
	kind    string // "phasedxz", "cz", "measure"
	qubits  []int
	x       float64 // PhasedXZ x_exponent
	z       float64 // PhasedXZ z_exponent
	a       float64 // PhasedXZ axis_phase_exponent
	exp     float64 // CZ exponent
	measKey string  // measurement key
}

// convertOp converts an IR operation to one or more native ops.
func convertOp(op ir.Operation) ([]nativeOp, error) {
	// Measurement: gate is nil, clbits present.
	if op.Gate == nil && len(op.Clbits) > 0 {
		key := "m"
		if len(op.Qubits) > 0 {
			key = fmt.Sprintf("m%d", op.Qubits[0])
		}
		return []nativeOp{{
			kind:    "measure",
			qubits:  op.Qubits,
			measKey: key,
		}}, nil
	}
	if op.Gate == nil {
		return nil, nil // skip unknown nil-gate ops
	}

	name := op.Gate.Name()
	params := op.Gate.Params()

	switch {
	// Identity — skip (no-op on hardware).
	case name == "I":
		return nil, nil

	// Barrier and delay — skip (not physical gates).
	case name == "barrier", name == "delay":
		return nil, nil

	// H: PhasedXZ(x=1, z=1, a=0)
	case name == "H":
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 1.0, z: 1.0, a: 0.0}}, nil

	// X: PhasedXZ(x=1, z=0, a=0)
	case name == "X":
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 1.0, z: 0.0, a: 0.0}}, nil

	// Y: PhasedXZ(x=1, z=0, a=0.5)
	case name == "Y":
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 1.0, z: 0.0, a: 0.5}}, nil

	// Z: PhasedXZ(x=0, z=1, a=0)
	case name == "Z":
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 0.0, z: 1.0, a: 0.0}}, nil

	// S: PhasedXZ(x=0, z=0.5, a=0)
	case name == "S":
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 0.0, z: 0.5, a: 0.0}}, nil

	// Sdg: PhasedXZ(x=0, z=-0.5, a=0)
	case name == "S†":
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 0.0, z: -0.5, a: 0.0}}, nil

	// T: PhasedXZ(x=0, z=0.25, a=0)
	case name == "T":
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 0.0, z: 0.25, a: 0.0}}, nil

	// Tdg: PhasedXZ(x=0, z=-0.25, a=0)
	case name == "T†":
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 0.0, z: -0.25, a: 0.0}}, nil

	// SX: PhasedXZ(x=0.5, z=0, a=0)
	case name == "SX":
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 0.5, z: 0.0, a: 0.0}}, nil

	// RX(θ): PhasedXZ(x=θ/π, z=0, a=0)
	case strings.HasPrefix(name, "RX"):
		if len(params) < 1 {
			return nil, fmt.Errorf("google: RX gate missing parameter")
		}
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: params[0] / math.Pi, z: 0.0, a: 0.0}}, nil

	// RY(θ): PhasedXZ(x=θ/π, z=0, a=0.25)
	case strings.HasPrefix(name, "RY"):
		if len(params) < 1 {
			return nil, fmt.Errorf("google: RY gate missing parameter")
		}
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: params[0] / math.Pi, z: 0.0, a: 0.25}}, nil

	// RZ(θ): PhasedXZ(x=0, z=θ/π, a=0)
	case strings.HasPrefix(name, "RZ"):
		if len(params) < 1 {
			return nil, fmt.Errorf("google: RZ gate missing parameter")
		}
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 0.0, z: params[0] / math.Pi, a: 0.0}}, nil

	// Phase(θ): equivalent to RZ, PhasedXZ(x=0, z=θ/π, a=0)
	case strings.HasPrefix(name, "Phase"):
		if len(params) < 1 {
			return nil, fmt.Errorf("google: Phase gate missing parameter")
		}
		return []nativeOp{{kind: "phasedxz", qubits: op.Qubits, x: 0.0, z: params[0] / math.Pi, a: 0.0}}, nil

	// CZ: native
	case name == "CZ":
		return []nativeOp{{kind: "cz", qubits: op.Qubits, exp: 1.0}}, nil

	// CNOT: decompose to H(target) + CZ + H(target)
	case name == "CNOT":
		ctrl, tgt := op.Qubits[0], op.Qubits[1]
		return []nativeOp{
			{kind: "phasedxz", qubits: []int{tgt}, x: 1.0, z: 1.0, a: 0.0}, // H
			{kind: "cz", qubits: []int{ctrl, tgt}, exp: 1.0},                 // CZ
			{kind: "phasedxz", qubits: []int{tgt}, x: 1.0, z: 1.0, a: 0.0}, // H
		}, nil

	// CY: decompose to Sdg(target) + CNOT + S(target)
	// -> Sdg(t) + H(t) + CZ(c,t) + H(t) + S(t)
	case name == "CY":
		ctrl, tgt := op.Qubits[0], op.Qubits[1]
		return []nativeOp{
			{kind: "phasedxz", qubits: []int{tgt}, x: 0.0, z: -0.5, a: 0.0}, // Sdg
			{kind: "phasedxz", qubits: []int{tgt}, x: 1.0, z: 1.0, a: 0.0},   // H
			{kind: "cz", qubits: []int{ctrl, tgt}, exp: 1.0},                   // CZ
			{kind: "phasedxz", qubits: []int{tgt}, x: 1.0, z: 1.0, a: 0.0},   // H
			{kind: "phasedxz", qubits: []int{tgt}, x: 0.0, z: 0.5, a: 0.0},   // S
		}, nil

	// SWAP: decompose to 3x CNOT -> each CNOT decomposes to H+CZ+H
	case name == "SWAP":
		q0, q1 := op.Qubits[0], op.Qubits[1]
		return []nativeOp{
			// CNOT(q0, q1) = H(q1) + CZ(q0,q1) + H(q1)
			{kind: "phasedxz", qubits: []int{q1}, x: 1.0, z: 1.0, a: 0.0},
			{kind: "cz", qubits: []int{q0, q1}, exp: 1.0},
			{kind: "phasedxz", qubits: []int{q1}, x: 1.0, z: 1.0, a: 0.0},
			// CNOT(q1, q0) = H(q0) + CZ(q1,q0) + H(q0)
			{kind: "phasedxz", qubits: []int{q0}, x: 1.0, z: 1.0, a: 0.0},
			{kind: "cz", qubits: []int{q1, q0}, exp: 1.0},
			{kind: "phasedxz", qubits: []int{q0}, x: 1.0, z: 1.0, a: 0.0},
			// CNOT(q0, q1) = H(q1) + CZ(q0,q1) + H(q1)
			{kind: "phasedxz", qubits: []int{q1}, x: 1.0, z: 1.0, a: 0.0},
			{kind: "cz", qubits: []int{q0, q1}, exp: 1.0},
			{kind: "phasedxz", qubits: []int{q1}, x: 1.0, z: 1.0, a: 0.0},
		}, nil

	// Reset — not supported on Google hardware.
	case name == "reset":
		return nil, fmt.Errorf("google: reset gate is not supported")

	default:
		return nil, fmt.Errorf("google: unsupported gate %q", name)
	}
}

// scheduleMoments groups native ops into moments based on qubit conflicts.
// Operations that share no qubits go into the same moment.
func scheduleMoments(ops []nativeOp, numQubits int) []cirqMoment {
	if len(ops) == 0 {
		return nil
	}

	layers := make([]int, numQubits) // next available layer per qubit
	type scheduled struct {
		op    nativeOp
		layer int
	}
	var items []scheduled

	for _, op := range ops {
		// Find the earliest layer where all qubits are available.
		layer := 0
		for _, q := range op.qubits {
			if q < numQubits && layers[q] > layer {
				layer = layers[q]
			}
		}
		items = append(items, scheduled{op: op, layer: layer})
		// Mark qubits as occupied through this layer.
		for _, q := range op.qubits {
			if q < numQubits {
				layers[q] = layer + 1
			}
		}
	}

	// Find max layer.
	maxLayer := 0
	for _, item := range items {
		if item.layer > maxLayer {
			maxLayer = item.layer
		}
	}

	// Build moments.
	moments := make([]cirqMoment, maxLayer+1)
	for i := range moments {
		moments[i] = cirqMoment{Type: "Moment"}
	}
	for _, item := range items {
		moments[item.layer].Operations = append(
			moments[item.layer].Operations,
			nativeOpToCirq(item.op),
		)
	}
	return moments
}

// nativeOpToCirq converts a native op to a Cirq JSON operation.
func nativeOpToCirq(op nativeOp) cirqOperation {
	qubits := make([]cirqQubitRef, len(op.qubits))
	for i, q := range op.qubits {
		qubits[i] = cirqQubitRef{Type: "LineQubit", X: q}
	}

	switch op.kind {
	case "phasedxz":
		return cirqOperation{
			Type: "GateOperation",
			Gate: cirqGate{
				Type:         "PhasedXZGate",
				Exponent:     op.x,
				PhaseExp:     op.z,
				AxisPhaseExp: op.a,
			},
			Qubits: qubits,
		}
	case "cz":
		return cirqOperation{
			Type: "GateOperation",
			Gate: cirqGate{
				Type:     "CZPowGate",
				Exponent: op.exp,
			},
			Qubits: qubits,
		}
	case "measure":
		return cirqOperation{
			Type: "GateOperation",
			Gate: cirqGate{
				Type: "MeasurementGate",
				Key:  op.measKey,
			},
			Qubits: qubits,
		}
	default:
		return cirqOperation{Type: "GateOperation", Qubits: qubits}
	}
}

// parseResults converts Google Quantum measurement results to a backend.Result.
func parseResults(cr cirqResult, shots int) (*backend.Result, error) {
	counts := make(map[string]int)
	totalSamples := 0

	if len(cr.MeasurementResults) == 0 {
		return &backend.Result{Counts: counts, Shots: shots}, nil
	}

	// Aggregate all measurement results. For a standard circuit,
	// there's typically one measurement result with all qubits.
	// Each repetition produces a bitstring.
	mr := cr.MeasurementResults[0]
	for _, sample := range mr.Results {
		bs := sampleToBitstring(sample)
		counts[bs]++
		totalSamples++
	}

	if totalSamples == 0 {
		return &backend.Result{Counts: counts, Shots: shots}, nil
	}

	return &backend.Result{
		Counts: counts,
		Shots:  totalSamples,
	}, nil
}

// sampleToBitstring converts a measurement sample to a bitstring (MSB-first).
func sampleToBitstring(sample []int) string {
	var sb strings.Builder
	sb.Grow(len(sample))
	for _, v := range sample {
		if v != 0 {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}
