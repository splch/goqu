package mitigation

import (
	"fmt"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

// DDSequence selects the dynamical decoupling pulse sequence.
type DDSequence int

const (
	// DDXX inserts an X-X sequence into idle periods.
	DDXX DDSequence = iota
	// DDXY4 inserts an X-Y-X-Y sequence into idle periods.
	DDXY4
)

// DDConfig specifies parameters for dynamical decoupling insertion.
type DDConfig struct {
	// Circuit is the quantum circuit to protect.
	Circuit *ir.Circuit
	// Sequence selects the DD pulse sequence. Default: DDXX.
	Sequence DDSequence
}

// InsertDD returns a new circuit with dynamical decoupling sequences inserted
// into idle qubit periods. This is a pure circuit transform — no executor or
// noise model is needed.
func InsertDD(cfg DDConfig) (*ir.Circuit, error) {
	if cfg.Circuit == nil {
		return nil, fmt.Errorf("mitigation.InsertDD: Circuit is nil")
	}

	ops := cfg.Circuit.Ops()
	if len(ops) == 0 {
		return cfg.Circuit, nil
	}

	seq := ddPulses(cfg.Sequence)
	numQubits := cfg.Circuit.NumQubits()

	// Assign each operation to a layer.
	layers := layerAssign(ops, numQubits)

	maxLayer := 0
	for _, l := range layers {
		if l > maxLayer {
			maxLayer = l
		}
	}

	// Build per-qubit occupancy: which layers have a gate on each qubit.
	occupied := make([]map[int]bool, numQubits)
	for q := range numQubits {
		occupied[q] = make(map[int]bool)
	}
	for i, op := range ops {
		for _, q := range op.Qubits {
			occupied[q][layers[i]] = true
		}
	}

	// Collect DD insertions per (qubit, layer).
	type ddInsert struct {
		qubit int
		layer int
		gate  gate.Gate
	}
	var inserts []ddInsert

	for q := range numQubits {
		// Find idle gaps: contiguous layers where qubit q has no gate.
		layer := 1
		for layer <= maxLayer {
			if occupied[q][layer] {
				layer++
				continue
			}
			// Start of idle gap.
			gapStart := layer
			for layer <= maxLayer && !occupied[q][layer] {
				layer++
			}
			gapLen := layer - gapStart

			if gapLen < len(seq) {
				continue
			}

			// Place DD pulses evenly in the gap.
			spacing := float64(gapLen) / float64(len(seq))
			for i, g := range seq {
				insertLayer := gapStart + int(float64(i)*spacing+spacing/2)
				if insertLayer >= layer {
					insertLayer = layer - 1
				}
				inserts = append(inserts, ddInsert{qubit: q, layer: insertLayer, gate: g})
			}
		}
	}

	// Build layer-to-ops mapping for the original circuit.
	layerOps := make([][]ir.Operation, maxLayer+1)
	for i, op := range ops {
		l := layers[i]
		layerOps[l] = append(layerOps[l], op)
	}

	// Add DD insertions.
	for _, ins := range inserts {
		layerOps[ins.layer] = append(layerOps[ins.layer], ir.Operation{
			Gate:   ins.gate,
			Qubits: []int{ins.qubit},
		})
	}

	// Flatten back to sequential ops.
	var newOps []ir.Operation
	for l := 1; l <= maxLayer; l++ {
		newOps = append(newOps, layerOps[l]...)
	}

	return ir.New(cfg.Circuit.Name(), cfg.Circuit.NumQubits(),
		cfg.Circuit.NumClbits(), newOps, cfg.Circuit.Metadata()), nil
}

// ddPulses returns the gate sequence for the given DD type.
func ddPulses(seq DDSequence) []gate.Gate {
	switch seq {
	case DDXY4:
		return []gate.Gate{gate.X, gate.Y, gate.X, gate.Y}
	default: // DDXX
		return []gate.Gate{gate.X, gate.X}
	}
}

// layerAssign replicates the depth-tracking logic from ir.Circuit.depth()
// but returns per-operation layer indices (1-based).
func layerAssign(ops []ir.Operation, numQubits int) []int {
	qubitLayer := make([]int, numQubits)
	result := make([]int, len(ops))

	for i, op := range ops {
		opLayer := 0
		for _, q := range op.Qubits {
			if q < numQubits && qubitLayer[q] > opLayer {
				opLayer = qubitLayer[q]
			}
		}
		opLayer++
		for _, q := range op.Qubits {
			if q < numQubits {
				qubitLayer[q] = opLayer
			}
		}
		result[i] = opLayer
	}
	return result
}
