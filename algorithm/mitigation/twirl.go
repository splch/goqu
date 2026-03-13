package mitigation

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
)

// TwirlConfig specifies parameters for Pauli twirling.
type TwirlConfig struct {
	// Circuit is the quantum circuit to twirl.
	Circuit *ir.Circuit
	// Executor evaluates a circuit and returns an expectation value.
	Executor Executor
	// Samples is the number of twirled circuit copies to average. Default: 100.
	Samples int
}

// TwirlResult holds the output of Pauli twirling.
type TwirlResult struct {
	// MitigatedValue is the average expectation over all twirled circuits.
	MitigatedValue float64
	// RawValues are the individual expectation values per sample.
	RawValues []float64
}

// RunTwirl performs Pauli twirling on 2-qubit gates (CNOT and CZ).
//
// It generates multiple randomly twirled copies of the circuit, executes each,
// and averages the results. This converts coherent errors into stochastic
// Pauli errors, improving the accuracy of other mitigation techniques.
func RunTwirl(ctx context.Context, cfg TwirlConfig) (*TwirlResult, error) {
	if cfg.Circuit == nil {
		return nil, fmt.Errorf("mitigation.RunTwirl: Circuit is nil")
	}
	if cfg.Executor == nil {
		return nil, fmt.Errorf("mitigation.RunTwirl: Executor is nil")
	}

	samples := cfg.Samples
	if samples <= 0 {
		samples = 100
	}

	rng := rand.New(rand.NewSource(rand.Int63()))
	values := make([]float64, samples)

	for i := range samples {
		twirled, err := TwirlCircuit(cfg.Circuit, rng)
		if err != nil {
			return nil, fmt.Errorf("mitigation.RunTwirl: sample %d: %w", i, err)
		}
		val, err := cfg.Executor(ctx, twirled)
		if err != nil {
			return nil, fmt.Errorf("mitigation.RunTwirl: execute sample %d: %w", i, err)
		}
		values[i] = val
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return &TwirlResult{
		MitigatedValue: sum / float64(samples),
		RawValues:      values,
	}, nil
}

// TwirlCircuit returns a new circuit with random Pauli gates inserted around
// each CNOT and CZ gate. Returns an error if unsupported 2-qubit gates are
// encountered.
func TwirlCircuit(circuit *ir.Circuit, rng *rand.Rand) (*ir.Circuit, error) {
	ops := circuit.Ops()
	var newOps []ir.Operation

	for _, op := range ops {
		if op.Gate == nil || op.Gate.Qubits() != 2 {
			newOps = append(newOps, op)
			continue
		}

		name := op.Gate.Name()
		var table *[16]twirlEntry
		switch name {
		case "CNOT":
			table = &cnotTwirlTable
		case "CZ":
			table = &czTwirlTable
		default:
			return nil, fmt.Errorf("mitigation.TwirlCircuit: unsupported 2-qubit gate %q", name)
		}

		// Pick a random twirl pair.
		idx := rng.Intn(16)
		entry := table[idx]

		// Insert before Paulis.
		if entry.before[0] != gate.I {
			newOps = append(newOps, ir.Operation{Gate: entry.before[0], Qubits: []int{op.Qubits[0]}})
		}
		if entry.before[1] != gate.I {
			newOps = append(newOps, ir.Operation{Gate: entry.before[1], Qubits: []int{op.Qubits[1]}})
		}

		// Original gate.
		newOps = append(newOps, op)

		// Insert after Paulis.
		if entry.after[0] != gate.I {
			newOps = append(newOps, ir.Operation{Gate: entry.after[0], Qubits: []int{op.Qubits[0]}})
		}
		if entry.after[1] != gate.I {
			newOps = append(newOps, ir.Operation{Gate: entry.after[1], Qubits: []int{op.Qubits[1]}})
		}
	}

	return ir.New(circuit.Name(), circuit.NumQubits(), circuit.NumClbits(),
		newOps, circuit.Metadata()), nil
}

// twirlEntry holds the Pauli gates to insert before and after a 2Q gate.
type twirlEntry struct {
	before [2]gate.Gate // [control, target]
	after  [2]gate.Gate
}

// paulis maps index 0–3 to I, X, Y, Z.
var paulis = [4]gate.Gate{gate.I, gate.X, gate.Y, gate.Z}

// Pauli conjugation tables for CNOT and CZ.
// For each of 16 two-qubit Paulis (Pa⊗Pb), the table stores {before, after}
// such that: (before[0]⊗before[1]) · G · (after[0]⊗after[1]) = G
// i.e., after = conjugation of before through G.
var cnotTwirlTable [16]twirlEntry
var czTwirlTable [16]twirlEntry

func init() {
	// CNOT conjugation table: CNOT · (Pa⊗Pb) · CNOT = (Pc⊗Pd)
	// Heisenberg picture rules:
	//   X_ctrl → X_ctrl ⊗ X_tgt,  Z_ctrl → Z_ctrl
	//   X_tgt  → X_tgt,           Z_tgt  → Z_ctrl ⊗ Z_tgt
	//   Y_ctrl → Y_ctrl ⊗ X_tgt,  Y_tgt  → Z_ctrl ⊗ Y_tgt
	// For twirling: (Pa⊗Pb) · CNOT · (Pc⊗Pd) = CNOT
	// requires (Pc⊗Pd) = CNOT · (Pa⊗Pb) · CNOT (both self-adjoint).
	cnotConj := [4][4][2]int{
		// before: I⊗{I,X,Y,Z} → after
		{{0, 0}, {0, 1}, {3, 2}, {3, 3}}, // I⊗{I,X,Y,Z}
		// X⊗{I,X,Y,Z}
		{{1, 1}, {1, 0}, {2, 3}, {2, 2}}, // X⊗I→X⊗X, X⊗X→X⊗I, X⊗Y→Y⊗Z, X⊗Z→-Y⊗Y
		// Y⊗{I,X,Y,Z}
		{{2, 1}, {2, 0}, {1, 3}, {1, 2}}, // Y⊗I→Y⊗X, Y⊗X→Y⊗I, Y⊗Y→-X⊗Z, Y⊗Z→X⊗Y
		// Z⊗{I,X,Y,Z}
		{{3, 0}, {3, 1}, {0, 2}, {0, 3}}, // Z⊗I→Z⊗I, Z⊗X→Z⊗X, Z⊗Y→I⊗Y, Z⊗Z→I⊗Z
	}

	idx := 0
	for a := range 4 {
		for b := range 4 {
			c := cnotConj[a][b]
			cnotTwirlTable[idx] = twirlEntry{
				before: [2]gate.Gate{paulis[a], paulis[b]},
				after:  [2]gate.Gate{paulis[c[0]], paulis[c[1]]},
			}
			idx++
		}
	}

	// CZ conjugation table: CZ · (Pa⊗Pb) · CZ = (Pc⊗Pd)
	// Heisenberg picture rules (CZ is symmetric):
	//   X_0 → X_0 ⊗ Z_1,  Z_0 → Z_0
	//   X_1 → Z_0 ⊗ X_1,  Z_1 → Z_1
	//   Y_0 → Y_0 ⊗ Z_1,  Y_1 → Z_0 ⊗ Y_1
	// Products derived by composing single-qubit rules (phases absorbed).
	czConj := [4][4][2]int{
		{{0, 0}, {3, 1}, {3, 2}, {0, 3}}, // I⊗I→I⊗I, I⊗X→Z⊗X, I⊗Y→Z⊗Y, I⊗Z→I⊗Z
		{{1, 3}, {2, 2}, {2, 1}, {1, 0}}, // X⊗I→X⊗Z, X⊗X→Y⊗Y, X⊗Y→Y⊗X, X⊗Z→X⊗I
		{{2, 3}, {1, 2}, {1, 1}, {2, 0}}, // Y⊗I→Y⊗Z, Y⊗X→X⊗Y, Y⊗Y→X⊗X, Y⊗Z→Y⊗I
		{{3, 0}, {0, 1}, {0, 2}, {3, 3}}, // Z⊗I→Z⊗I, Z⊗X→I⊗X, Z⊗Y→I⊗Y, Z⊗Z→Z⊗Z
	}

	idx = 0
	for a := range 4 {
		for b := range 4 {
			c := czConj[a][b]
			czTwirlTable[idx] = twirlEntry{
				before: [2]gate.Gate{paulis[a], paulis[b]},
				after:  [2]gate.Gate{paulis[c[0]], paulis[c[1]]},
			}
			idx++
		}
	}
}
