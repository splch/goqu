package mitigation

import (
	"context"
	"fmt"
	"math"
	"math/rand"

	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// CDRConfig specifies parameters for Clifford Data Regression.
type CDRConfig struct {
	// Circuit is the quantum circuit to mitigate.
	Circuit *ir.Circuit
	// Executor is the noisy executor.
	Executor Executor
	// Hamiltonian is the observable for ideal expectation computation.
	Hamiltonian pauli.PauliSum
	// NumTraining is the number of near-Clifford training circuits. Default: 20.
	NumTraining int
	// Fraction is the fraction of non-Clifford gates to replace. Default: 0.75.
	Fraction float64
}

// CDRResult holds the output of Clifford Data Regression.
type CDRResult struct {
	// MitigatedValue is the corrected expectation value.
	MitigatedValue float64
	// TrainingNoisy are the noisy expectation values for training circuits.
	TrainingNoisy []float64
	// TrainingIdeal are the ideal expectation values for training circuits.
	TrainingIdeal []float64
	// FitA is the slope of the affine fit y = a·x + b.
	FitA float64
	// FitB is the intercept of the affine fit.
	FitB float64
}

// RunCDR performs Clifford Data Regression.
//
// It generates near-Clifford training circuits by replacing a fraction of
// non-Clifford gates with their nearest Clifford equivalents, runs both
// noisy and ideal simulations on these training circuits, fits an affine
// correction model, and applies it to the original noisy result.
func RunCDR(ctx context.Context, cfg CDRConfig) (*CDRResult, error) {
	if cfg.Circuit == nil {
		return nil, fmt.Errorf("mitigation.RunCDR: Circuit is nil")
	}
	if cfg.Executor == nil {
		return nil, fmt.Errorf("mitigation.RunCDR: Executor is nil")
	}

	numTraining := cfg.NumTraining
	if numTraining <= 0 {
		numTraining = 20
	}
	fraction := cfg.Fraction
	if fraction <= 0 || fraction > 1 {
		fraction = 0.75
	}

	rng := rand.New(rand.NewSource(rand.Int63()))

	// Get the noisy value for the original circuit.
	noisyOriginal, err := cfg.Executor(ctx, cfg.Circuit)
	if err != nil {
		return nil, fmt.Errorf("mitigation.RunCDR: execute original: %w", err)
	}

	// Generate training circuits and collect noisy + ideal values.
	trainingNoisy := make([]float64, numTraining)
	trainingIdeal := make([]float64, numTraining)

	for i := range numTraining {
		training := generateTrainingCircuit(cfg.Circuit, fraction, rng)

		noisyVal, err := cfg.Executor(ctx, training)
		if err != nil {
			return nil, fmt.Errorf("mitigation.RunCDR: execute training %d: %w", i, err)
		}

		idealVal := idealExpectation(training, cfg.Hamiltonian)

		trainingNoisy[i] = noisyVal
		trainingIdeal[i] = idealVal
	}

	// Fit affine model: ideal = a * noisy + b.
	a, b, err := affineFit(trainingNoisy, trainingIdeal)
	if err != nil {
		return nil, fmt.Errorf("mitigation.RunCDR: affine fit: %w", err)
	}

	mitigated := a*noisyOriginal + b

	return &CDRResult{
		MitigatedValue: mitigated,
		TrainingNoisy:  trainingNoisy,
		TrainingIdeal:  trainingIdeal,
		FitA:           a,
		FitB:           b,
	}, nil
}

// generateTrainingCircuit creates a near-Clifford circuit by replacing a
// fraction of non-Clifford gates with their nearest Clifford equivalents.
func generateTrainingCircuit(circuit *ir.Circuit, fraction float64, rng *rand.Rand) *ir.Circuit {
	ops := circuit.Ops()

	// Find non-Clifford gate indices.
	var nonCliffordIdx []int
	for i, op := range ops {
		if op.Gate != nil && !isCliffordGate(op.Gate) {
			nonCliffordIdx = append(nonCliffordIdx, i)
		}
	}

	if len(nonCliffordIdx) == 0 {
		return circuit
	}

	// Randomly select gates to replace.
	nReplace := int(math.Round(fraction * float64(len(nonCliffordIdx))))
	if nReplace > len(nonCliffordIdx) {
		nReplace = len(nonCliffordIdx)
	}
	if nReplace < 1 {
		nReplace = 1
	}

	// Fisher-Yates shuffle and take first nReplace.
	perm := make([]int, len(nonCliffordIdx))
	copy(perm, nonCliffordIdx)
	for i := len(perm) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		perm[i], perm[j] = perm[j], perm[i]
	}
	replaceSet := make(map[int]bool, nReplace)
	for _, idx := range perm[:nReplace] {
		replaceSet[idx] = true
	}

	// Build new ops.
	newOps := make([]ir.Operation, len(ops))
	for i, op := range ops {
		if replaceSet[i] {
			cliff := nearestClifford(op.Gate)
			newOps[i] = ir.Operation{
				Gate:      cliff,
				Qubits:    op.Qubits,
				Clbits:    op.Clbits,
				Condition: op.Condition,
			}
		} else {
			newOps[i] = op
		}
	}

	return ir.New(circuit.Name(), circuit.NumQubits(), circuit.NumClbits(),
		newOps, circuit.Metadata())
}

// isCliffordGate returns true if the gate is a Clifford gate.
func isCliffordGate(g gate.Gate) bool {
	// Check pointer equality against known Clifford singletons.
	switch g {
	case gate.I, gate.H, gate.X, gate.Y, gate.Z, gate.S, gate.Sdg, gate.SX,
		gate.CNOT, gate.CZ, gate.SWAP, gate.CY:
		return true
	}

	// Fallback: check by name.
	switch g.Name() {
	case "I", "H", "X", "Y", "Z", "S", "S†", "SX",
		"CNOT", "CZ", "SWAP", "CY":
		return true
	}

	// For known single-axis rotations, check if the parameter is a multiple of π/2.
	params := g.Params()
	if len(params) == 1 {
		name := g.Name()
		isRotation := (len(name) >= 2 && (name[:2] == "RX" || name[:2] == "RY" || name[:2] == "RZ")) || name == "Phase"
		if isRotation {
			normalized := params[0] / (math.Pi / 2)
			return math.Abs(normalized-math.Round(normalized)) <= 1e-10
		}
	}

	return false
}

// nearestClifford returns the nearest Clifford gate for a given gate.
func nearestClifford(g gate.Gate) gate.Gate {
	name := g.Name()

	// Handle known non-Clifford singletons.
	switch g {
	case gate.T:
		return gate.S
	case gate.Tdg:
		return gate.Sdg
	}
	switch name {
	case "T":
		return gate.S
	case "T†":
		return gate.Sdg
	}

	params := g.Params()
	if len(params) == 0 {
		return g // already Clifford or unknown
	}

	// Round first parameter to nearest multiple of π/2.
	theta := params[0]
	k := int(math.Round(theta / (math.Pi / 2)))
	k = ((k % 4) + 4) % 4 // normalize to 0..3

	switch {
	case len(name) >= 2 && name[:2] == "RZ":
		return rzClifford(k)
	case len(name) >= 2 && name[:2] == "RX":
		return rxClifford(k)
	case len(name) >= 2 && name[:2] == "RY":
		return ryClifford(k)
	case name[0] == 'P': // Phase gate
		return rzClifford(k)
	default:
		return g // unrecognized parameterized gate, return as-is
	}
}

// rzClifford returns the Clifford gate equivalent to RZ(k·π/2).
func rzClifford(k int) gate.Gate {
	switch k {
	case 0:
		return gate.I
	case 1:
		return gate.S
	case 2:
		return gate.Z
	case 3:
		return gate.Sdg
	default:
		return gate.I
	}
}

// rxClifford returns the Clifford gate equivalent to RX(k·π/2).
func rxClifford(k int) gate.Gate {
	switch k {
	case 0:
		return gate.I
	case 1:
		return gate.SX
	case 2:
		return gate.X
	case 3:
		// RX(3π/2) = RX(-π/2) = SX†; no named gate, use parameterized form.
		return gate.RX(3 * math.Pi / 2)
	default:
		return gate.I
	}
}

// ryClifford returns the Clifford gate equivalent to RY(k·π/2).
func ryClifford(k int) gate.Gate {
	switch k {
	case 0:
		return gate.I
	case 1:
		return gate.RY(math.Pi / 2)
	case 2:
		return gate.Y
	case 3:
		return gate.RY(3 * math.Pi / 2)
	default:
		return gate.I
	}
}

// idealExpectation computes ⟨ψ|H|ψ⟩ for a circuit using ideal statevector sim.
func idealExpectation(circuit *ir.Circuit, hamiltonian pauli.PauliSum) float64 {
	sim := statevector.New(circuit.NumQubits())
	if err := sim.Evolve(circuit); err != nil {
		return 0
	}
	return sim.ExpectPauliSum(hamiltonian)
}

// affineFit performs least-squares linear regression: y = a·x + b.
// Returns (a, b, error).
func affineFit(x, y []float64) (float64, float64, error) {
	n := len(x)
	if n != len(y) || n < 2 {
		return 0, 0, fmt.Errorf("need at least 2 data points, got %d", n)
	}

	sumX, sumY, sumXX, sumXY := 0.0, 0.0, 0.0, 0.0
	for i := range n {
		sumX += x[i]
		sumY += y[i]
		sumXX += x[i] * x[i]
		sumXY += x[i] * y[i]
	}

	nf := float64(n)
	denom := nf*sumXX - sumX*sumX
	if math.Abs(denom) < 1e-15 {
		// All x values are the same; return mean of y.
		return 0, sumY / nf, nil
	}

	a := (nf*sumXY - sumX*sumY) / denom
	b := (sumY - a*sumX) / nf

	return a, b, nil
}
