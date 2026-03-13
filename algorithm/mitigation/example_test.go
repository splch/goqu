package mitigation_test

import (
	"context"
	"fmt"

	"github.com/splch/goqu/algorithm/mitigation"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/noise"
	"github.com/splch/goqu/sim/pauli"
)

func ExampleRunZNE() {
	// Build a Bell circuit.
	circ, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		panic(err)
	}

	// Observable: Z on qubit 0.
	hamiltonian, err := pauli.NewPauliSum([]pauli.PauliString{
		pauli.ZOn([]int{0}, 2),
	})
	if err != nil {
		panic(err)
	}

	// Create a noisy executor.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.02))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.04))
	exec := mitigation.DensityMatrixExecutor(hamiltonian, nm)

	// Run ZNE with linear extrapolation.
	result, err := mitigation.RunZNE(context.Background(), mitigation.ZNEConfig{
		Circuit:      circ,
		Executor:     exec,
		ScaleFactors: []float64{1, 3, 5},
		Extrapolator: mitigation.LinearExtrapolator,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("mitigated value: %.4f\n", result.MitigatedValue)
	fmt.Printf("scale factors: %v\n", result.ScaleFactors)
	fmt.Printf("noisy values: [")
	for i, v := range result.NoisyValues {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%.4f", v)
	}
	fmt.Println("]")
	// Output will vary with noise parameters, but mitigated value
	// should be closer to the ideal (0.0 for Bell state Z0) than
	// the raw noisy value at scale factor 1.
}

func ExampleCalibrateReadout() {
	numQubits := 2
	shots := 10000

	// Mock basis executor with known readout errors.
	exec := mockBasisExecutor(numQubits, 0.05, 0.03)

	cal, err := mitigation.CalibrateReadout(context.Background(), numQubits, shots, exec)
	if err != nil {
		panic(err)
	}

	// Simulate noisy measurement of |00⟩.
	noisyCounts, err := exec(context.Background(), 0, shots)
	if err != nil {
		panic(err)
	}

	corrected := cal.CorrectCounts(noisyCounts)

	fmt.Printf("corrected counts have %d entries\n", len(corrected))
	// Verify |00⟩ dominates.
	total := 0
	for _, c := range corrected {
		total += c
	}
	if total > 0 {
		fmt.Printf("|00⟩ fraction: %.2f\n", float64(corrected["00"])/float64(total))
	}
}

func ExampleInsertDD() {
	// Build a circuit with idle qubit periods.
	circ, err := builder.New("dd_example", 3).
		H(0).
		X(0).
		H(0).
		X(0).
		H(0).
		CNOT(0, 1).
		CNOT(0, 2).
		Build()
	if err != nil {
		panic(err)
	}

	// Insert XX dynamical decoupling.
	ddCirc, err := mitigation.InsertDD(mitigation.DDConfig{
		Circuit:  circ,
		Sequence: mitigation.DDXX,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("original gates: %d\n", len(circ.Ops()))
	fmt.Printf("DD gates: %d\n", len(ddCirc.Ops()))
	fmt.Printf("DD added %d pulses\n", len(ddCirc.Ops())-len(circ.Ops()))
}

func ExampleRunTwirl() {
	// Build a Bell circuit.
	circ, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		panic(err)
	}

	// Observable: ZZ correlation.
	zz := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zz})
	if err != nil {
		panic(err)
	}

	// Noisy executor.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.02))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.04))
	exec := mitigation.DensityMatrixExecutor(ham, nm)

	result, err := mitigation.RunTwirl(context.Background(), mitigation.TwirlConfig{
		Circuit:  circ,
		Executor: exec,
		Samples:  50,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("twirled value: %.4f (averaged over %d samples)\n",
		result.MitigatedValue, len(result.RawValues))
}

func ExampleRunPEC() {
	// Build a Bell circuit.
	circ, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		Build()
	if err != nil {
		panic(err)
	}

	// Observable: ZZ.
	zz := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zz})
	if err != nil {
		panic(err)
	}

	// Depolarizing noise model (required for PEC).
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.02))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.04))
	exec := mitigation.DensityMatrixExecutor(ham, nm)

	result, err := mitigation.RunPEC(context.Background(), mitigation.PECConfig{
		Circuit:    circ,
		Executor:   exec,
		NoiseModel: nm,
		Samples:    500,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("PEC mitigated: %.4f (overhead: %.2f)\n",
		result.MitigatedValue, result.Overhead)
}

func ExampleRunCDR() {
	// Circuit with non-Clifford T gates.
	ops := []ir.Operation{
		{Gate: gate.H, Qubits: []int{0}},
		{Gate: gate.T, Qubits: []int{0}},
		{Gate: gate.CNOT, Qubits: []int{0, 1}},
		{Gate: gate.T, Qubits: []int{1}},
	}
	circ := ir.New("t_circuit", 2, 0, ops, nil)

	// Observable: ZZ.
	zz := pauli.NewPauliString(1, map[int]pauli.Pauli{0: pauli.Z, 1: pauli.Z}, 2)
	ham, err := pauli.NewPauliSum([]pauli.PauliString{zz})
	if err != nil {
		panic(err)
	}

	// Noisy executor.
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(0.02))
	nm.AddDefaultError(2, noise.Depolarizing2Q(0.04))
	exec := mitigation.DensityMatrixExecutor(ham, nm)

	result, err := mitigation.RunCDR(context.Background(), mitigation.CDRConfig{
		Circuit:     circ,
		Executor:    exec,
		Hamiltonian: ham,
		NumTraining: 20,
		Fraction:    0.75,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("CDR mitigated: %.4f (fit: y = %.4f·x + %.4f)\n",
		result.MitigatedValue, result.FitA, result.FitB)
}

func ExampleRunTREX() {
	// Build a Bell circuit with measurements.
	circ, err := builder.New("bell", 2).
		H(0).
		CNOT(0, 1).
		MeasureAll().
		Build()
	if err != nil {
		panic(err)
	}

	// Mock shot runner with readout errors.
	runner := mockShotRunner(0.05, 0.03)

	result, err := mitigation.RunTREX(context.Background(), mitigation.TREXConfig{
		Circuit:    circ,
		Runner:     runner,
		Shots:      1000,
		Samples:    5,
		CalibShots: 10000,
	})
	if err != nil {
		panic(err)
	}

	total := 0
	for _, c := range result.Counts {
		total += c
	}
	fmt.Printf("TREX total counts: %d\n", total)
}
