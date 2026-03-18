//go:build js && wasm

package main

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/splch/goqu/algorithm/ansatz"
	"github.com/splch/goqu/algorithm/grover"
	"github.com/splch/goqu/algorithm/qpe"
	"github.com/splch/goqu/algorithm/shor"
	"github.com/splch/goqu/algorithm/textbook"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	qasmemitter "github.com/splch/goqu/qasm/emitter"
	"github.com/splch/goqu/sim/statevector"
	"github.com/splch/goqu/viz"
)

// ---------------------------------------------------------------------------
// Result types
// ---------------------------------------------------------------------------

type algorithmResult struct {
	CircuitSVG   string
	HistogramSVG string
	QASMCode     string
	Result       string
	Error        string
}

type groverStepResult struct {
	Steps             []groverStep
	OptimalIterations int
	Error             string
}

type groverStep struct {
	Iteration     int
	Probabilities []float64
}

type ansatzResult struct {
	CircuitSVG string
	QASMCode   string
	NumParams  int
	Depth      int
	Error      string
}

// ---------------------------------------------------------------------------
// Entanglement helper
// ---------------------------------------------------------------------------

func parseEntanglement(s string) ansatz.Entanglement {
	switch s {
	case "full":
		return ansatz.Full
	case "circular":
		return ansatz.Circular
	default:
		return ansatz.Linear
	}
}

// ---------------------------------------------------------------------------
// drawAndEmit is a common helper to generate SVG, QASM, and histogram
// from a circuit and measurement counts.
// ---------------------------------------------------------------------------

func drawAndEmit(circ *ir.Circuit, counts map[string]int, dark bool) (circSVG, histSVG, qasmCode string) {
	var drawOpts []draw.SVGOption
	var vizOpts []viz.Option
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
		vizOpts = append(vizOpts, viz.WithStyle(viz.DarkStyle()))
	}

	circSVG = draw.SVG(circ, drawOpts...)

	if counts != nil {
		histSVG = viz.Histogram(counts, vizOpts...)
	}

	qs, err := qasmemitter.EmitString(circ)
	if err == nil {
		qasmCode = qs
	}
	return
}

// ---------------------------------------------------------------------------
// runOracleAlgorithmJS
//
// Args for deutsch_jozsa:    (algorithmName, nQubitsStr, oracleTypeStr, shots, dark?)
// Args for bernstein_vazirani: (algorithmName, nQubitsStr, secretStr, shots, dark?)
// Args for simon:             (algorithmName, nQubitsStr, secretStr, shots, dark?)
// ---------------------------------------------------------------------------

func runOracleAlgorithmJS(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return marshalAlgorithm(algorithmResult{Error: "usage: runOracleAlgorithm(algorithm, nQubits, param, shots, dark?)"})
	}

	alg := args[0].String()
	nQubitsStr := args[1].String()
	paramStr := args[2].String()
	shots := args[3].Int()
	dark := len(args) >= 5 && args[4].Truthy()

	nQubits, err := strconv.Atoi(nQubitsStr)
	if err != nil || nQubits < 1 {
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("invalid nQubits: %s", nQubitsStr)})
	}
	if nQubits > 10 {
		return marshalAlgorithm(algorithmResult{Error: "algorithm simulation limited to 10 qubits"})
	}
	if shots < 1 {
		shots = 1024
	}

	ctx := context.Background()

	switch alg {
	case "deutsch_jozsa":
		return runDeutschJozsa(ctx, nQubits, paramStr, shots, dark)
	case "bernstein_vazirani":
		return runBernsteinVazirani(ctx, nQubits, paramStr, shots, dark)
	case "simon":
		return runSimon(ctx, nQubits, paramStr, shots, dark)
	default:
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("unknown algorithm: %s", alg)})
	}
}

func runDeutschJozsa(ctx context.Context, nQubits int, oracleType string, shots int, dark bool) string {
	var oracle textbook.DJOracle
	switch oracleType {
	case "constant":
		oracle = textbook.ConstantOracle(1)
	case "balanced":
		// Use a mask with the lower nQubits/2 bits set for a balanced function.
		mask := (1 << (nQubits / 2)) - 1
		if mask == 0 {
			mask = 1
		}
		oracle = textbook.BalancedOracle(mask)
	default:
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("unknown oracle type: %s (use 'constant' or 'balanced')", oracleType)})
	}

	result, err := textbook.DeutschJozsa(ctx, textbook.DJConfig{
		NumQubits: nQubits,
		Oracle:    oracle,
		Shots:     shots,
	})
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: err.Error()})
	}

	circSVG, histSVG, qasmCode := drawAndEmit(result.Circuit, result.Counts, dark)

	verdict := "balanced"
	if result.IsConstant {
		verdict = "constant"
	}
	desc := fmt.Sprintf("Deutsch-Jozsa: f(x) is %s (oracle=%s, n=%d)", verdict, oracleType, nQubits)

	return marshalAlgorithm(algorithmResult{
		CircuitSVG:   circSVG,
		HistogramSVG: histSVG,
		QASMCode:     qasmCode,
		Result:       desc,
	})
}

func runBernsteinVazirani(ctx context.Context, nQubits int, secretStr string, shots int, dark bool) string {
	secret, err := strconv.Atoi(secretStr)
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("invalid secret: %s", secretStr)})
	}
	if secret < 0 || secret >= (1<<nQubits) {
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("secret %d out of range [0, %d)", secret, 1<<nQubits)})
	}

	result, err := textbook.BernsteinVazirani(ctx, textbook.BVConfig{
		NumQubits: nQubits,
		Secret:    secret,
		Shots:     shots,
	})
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: err.Error()})
	}

	circSVG, histSVG, qasmCode := drawAndEmit(result.Circuit, result.Counts, dark)

	// Format recovered secret as binary.
	secretBin := strconv.FormatInt(int64(result.Secret), 2)
	for len(secretBin) < nQubits {
		secretBin = "0" + secretBin
	}
	desc := fmt.Sprintf("Bernstein-Vazirani: recovered secret s=%s (decimal %d, n=%d)", secretBin, result.Secret, nQubits)

	return marshalAlgorithm(algorithmResult{
		CircuitSVG:   circSVG,
		HistogramSVG: histSVG,
		QASMCode:     qasmCode,
		Result:       desc,
	})
}

func runSimon(ctx context.Context, nQubits int, secretStr string, shots int, dark bool) string {
	secret, err := strconv.Atoi(secretStr)
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("invalid secret: %s", secretStr)})
	}
	if secret < 0 || secret >= (1<<nQubits) {
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("secret %d out of range [0, %d)", secret, 1<<nQubits)})
	}

	oracle := textbook.TwoToOneOracle(secret, nQubits)
	result, err := textbook.Simon(ctx, textbook.SimonConfig{
		NumQubits: nQubits,
		Oracle:    oracle,
		Shots:     shots,
	})
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: err.Error()})
	}

	// Simon's result circuit is the last round circuit. Build counts from equations.
	// We don't have aggregated counts, so run the last circuit for a histogram.
	nTotal := 2 * nQubits
	sim := statevector.New(nTotal)
	defer sim.Close()
	counts, runErr := sim.Run(result.Circuit, shots)
	if runErr != nil {
		counts = nil
	}

	circSVG, histSVG, qasmCode := drawAndEmit(result.Circuit, counts, dark)

	secretBin := strconv.FormatInt(int64(result.Period), 2)
	for len(secretBin) < nQubits {
		secretBin = "0" + secretBin
	}
	desc := fmt.Sprintf("Simon: recovered period s=%s (decimal %d, n=%d)", secretBin, result.Period, nQubits)

	return marshalAlgorithm(algorithmResult{
		CircuitSVG:   circSVG,
		HistogramSVG: histSVG,
		QASMCode:     qasmCode,
		Result:       desc,
	})
}

// ---------------------------------------------------------------------------
// runSearchAlgorithmJS
//
// Args: (algorithmName string, nQubits int, targetsStr string, shots int, dark? bool)
//   targetsStr is comma-separated, e.g. "3,5,7"
// ---------------------------------------------------------------------------

func runSearchAlgorithmJS(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return marshalAlgorithm(algorithmResult{Error: "usage: runSearchAlgorithm(algorithm, nQubits, targets, shots, dark?)"})
	}

	alg := args[0].String()
	nQubits := args[1].Int()
	targetsStr := args[2].String()
	shots := args[3].Int()
	dark := len(args) >= 5 && args[4].Truthy()

	if nQubits < 1 || nQubits > 10 {
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("nQubits must be 1-10, got %d", nQubits)})
	}
	if shots < 1 {
		shots = 1024
	}

	targets, err := parseTargets(targetsStr, nQubits)
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: err.Error()})
	}

	switch alg {
	case "grover":
		return runGrover(nQubits, targets, shots, dark)
	default:
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("unknown search algorithm: %s", alg)})
	}
}

func parseTargets(s string, nQubits int) ([]int, error) {
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
		return nil, fmt.Errorf("no targets specified")
	}
	maxVal := 1 << nQubits
	targets := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid target %q: %w", p, err)
		}
		if v < 0 || v >= maxVal {
			return nil, fmt.Errorf("target %d out of range [0, %d)", v, maxVal)
		}
		targets = append(targets, v)
	}
	return targets, nil
}

func runGrover(nQubits int, targets []int, shots int, dark bool) string {
	ctx := context.Background()
	oracle := grover.PhaseOracle(targets, nQubits)

	result, err := grover.Run(ctx, grover.Config{
		NumQubits:    nQubits,
		Oracle:       oracle,
		NumSolutions: len(targets),
		Shots:        shots,
	})
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: err.Error()})
	}

	circSVG, histSVG, qasmCode := drawAndEmit(result.Circuit, result.Counts, dark)

	desc := fmt.Sprintf("Grover search: found %s after %d iterations (n=%d, %d targets)",
		result.TopResult, result.NumIters, nQubits, len(targets))

	return marshalAlgorithm(algorithmResult{
		CircuitSVG:   circSVG,
		HistogramSVG: histSVG,
		QASMCode:     qasmCode,
		Result:       desc,
	})
}

// ---------------------------------------------------------------------------
// groverStepThroughJS
//
// Args: (nQubits int, targetsStr string)
// Returns probability distributions for iterations 0..optimal.
// ---------------------------------------------------------------------------

func groverStepThroughJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshalGroverSteps(groverStepResult{Error: "usage: groverStepThrough(nQubits, targets)"})
	}

	nQubits := args[0].Int()
	targetsStr := args[1].String()

	if nQubits < 1 || nQubits > 10 {
		return marshalGroverSteps(groverStepResult{Error: fmt.Sprintf("nQubits must be 1-10, got %d", nQubits)})
	}

	targets, err := parseTargets(targetsStr, nQubits)
	if err != nil {
		return marshalGroverSteps(groverStepResult{Error: err.Error()})
	}

	// Compute optimal iteration count.
	n := nQubits
	m := len(targets)
	if m < 1 {
		m = 1
	}
	optimalIters := max(1, int(math.Floor(math.Pi/4*math.Sqrt(float64(int(1)<<n)/float64(m)))))

	// Cap at a reasonable maximum to avoid excessive computation.
	if optimalIters > 20 {
		optimalIters = 20
	}

	oracle := grover.PhaseOracle(targets, nQubits)
	qubits := make([]int, nQubits)
	for i := range nQubits {
		qubits[i] = i
	}

	steps := make([]groverStep, 0, optimalIters+1)

	for iter := 0; iter <= optimalIters; iter++ {
		// Build a circuit with exactly `iter` Grover iterations.
		b := builder.New("GroverStep", nQubits)

		// Initial superposition.
		for q := range nQubits {
			b.H(q)
		}

		// Apply `iter` rounds of oracle + diffusion.
		for range iter {
			// Oracle.
			oracle(b, qubits)

			// Diffusion: H, X, MCZ, X, H.
			for q := range nQubits {
				b.H(q)
			}
			for q := range nQubits {
				b.X(q)
			}
			if nQubits == 1 {
				b.Z(0)
			} else {
				controls := make([]int, nQubits-1)
				for i := range nQubits - 1 {
					controls[i] = i
				}
				b.MCZ(controls, nQubits-1)
			}
			for q := range nQubits {
				b.X(q)
			}
			for q := range nQubits {
				b.H(q)
			}
		}

		circ, err := b.Build()
		if err != nil {
			return marshalGroverSteps(groverStepResult{Error: fmt.Sprintf("build iter %d: %v", iter, err)})
		}

		// Evolve with statevector and extract probabilities.
		sim := statevector.New(nQubits)
		if err := sim.Evolve(circ); err != nil {
			sim.Close()
			return marshalGroverSteps(groverStepResult{Error: fmt.Sprintf("evolve iter %d: %v", iter, err)})
		}

		sv := sim.StateVector()
		probs := make([]float64, len(sv))
		for i, amp := range sv {
			probs[i] = real(amp)*real(amp) + imag(amp)*imag(amp)
		}
		sim.Close()

		steps = append(steps, groverStep{
			Iteration:     iter,
			Probabilities: probs,
		})
	}

	return marshalGroverSteps(groverStepResult{
		Steps:             steps,
		OptimalIterations: optimalIters,
	})
}

// ---------------------------------------------------------------------------
// runQFTJS
//
// Args: (nQubits int, dark? bool)
// ---------------------------------------------------------------------------

func runQFTJS(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalAlgorithm(algorithmResult{Error: "usage: runQFT(nQubits, dark?)"})
	}

	nQubits := args[0].Int()
	dark := len(args) >= 2 && args[1].Truthy()

	if nQubits < 1 || nQubits > 10 {
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("nQubits must be 1-10, got %d", nQubits)})
	}

	circ, err := qpe.QFT(nQubits)
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: err.Error()})
	}

	var drawOpts []draw.SVGOption
	var vizOpts []viz.Option
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
		vizOpts = append(vizOpts, viz.WithStyle(viz.DarkStyle()))
	}

	circSVG := draw.SVG(circ, drawOpts...)

	qasmCode, _ := qasmemitter.EmitString(circ)

	// Run statevector for exact probabilities.
	sim := statevector.New(nQubits)
	defer sim.Close()
	if err := sim.Evolve(circ); err != nil {
		return marshalAlgorithm(algorithmResult{Error: err.Error()})
	}

	sv := sim.StateVector()
	probMap := make(map[string]float64)
	for i, amp := range sv {
		p := real(amp)*real(amp) + imag(amp)*imag(amp)
		if p > 1e-10 {
			probMap[formatBinary(i, nQubits)] = p
		}
	}
	histSVG := viz.HistogramProb(probMap, vizOpts...)

	return marshalAlgorithm(algorithmResult{
		CircuitSVG:   circSVG,
		HistogramSVG: histSVG,
		QASMCode:     qasmCode,
		Result:       fmt.Sprintf("QFT on %d qubits", nQubits),
	})
}

// ---------------------------------------------------------------------------
// buildAnsatzJS
//
// Args: (ansatzName string, nQubits int, layers int, entanglement string, dark? bool)
// For UCCSD, layers is reinterpreted as nElectrons.
// ---------------------------------------------------------------------------

func buildAnsatzJS(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return marshalAnsatz(ansatzResult{Error: "usage: buildAnsatz(name, nQubits, layers, entanglement, dark?)"})
	}

	name := args[0].String()
	nQubits := args[1].Int()
	layers := args[2].Int()
	entStr := args[3].String()
	dark := len(args) >= 5 && args[4].Truthy()

	if nQubits < 1 || nQubits > 12 {
		return marshalAnsatz(ansatzResult{Error: fmt.Sprintf("nQubits must be 1-12, got %d", nQubits)})
	}
	if layers < 1 {
		layers = 1
	}

	var a ansatz.Ansatz
	switch name {
	case "real_amplitudes":
		a = ansatz.NewRealAmplitudes(nQubits, layers, parseEntanglement(entStr))
	case "efficient_su2":
		a = ansatz.NewEfficientSU2(nQubits, layers, parseEntanglement(entStr))
	case "basic_entangler":
		a = ansatz.NewBasicEntanglerLayers(nQubits, layers)
	case "strongly_entangling":
		a = ansatz.NewStronglyEntanglingLayers(nQubits, layers)
	case "uccsd":
		// For UCCSD, layers is reinterpreted as nElectrons.
		nElectrons := layers
		if nElectrons >= nQubits {
			return marshalAnsatz(ansatzResult{Error: fmt.Sprintf("nElectrons (%d) must be < nQubits (%d)", nElectrons, nQubits)})
		}
		a = ansatz.NewUCCSD(nQubits, nElectrons)
	default:
		return marshalAnsatz(ansatzResult{Error: fmt.Sprintf("unknown ansatz: %s", name)})
	}

	circ, err := a.Circuit()
	if err != nil {
		return marshalAnsatz(ansatzResult{Error: err.Error()})
	}

	var drawOpts []draw.SVGOption
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
	}

	circSVG := draw.SVG(circ, drawOpts...)
	qasmCode, _ := qasmemitter.EmitString(circ)
	depth := circ.Stats().Depth

	return marshalAnsatz(ansatzResult{
		CircuitSVG: circSVG,
		QASMCode:   qasmCode,
		NumParams:  a.NumParams(),
		Depth:      depth,
	})
}

// ---------------------------------------------------------------------------
// runQPEJS
//
// Args: (gateType string, numPhaseBits int, shots int, dark? bool)
//   gateType: "T", "S", "Z", or a float angle for RZ
// ---------------------------------------------------------------------------

func runQPEJS(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return marshalAlgorithm(algorithmResult{Error: "usage: runQPE(gateType, numPhaseBits, shots, dark?)"})
	}
	gateType := args[0].String()
	numPhaseBits := args[1].Int()
	shots := args[2].Int()
	dark := len(args) >= 4 && args[3].Truthy()

	if numPhaseBits < 2 || numPhaseBits > 6 {
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("numPhaseBits must be 2-6, got %d", numPhaseBits)})
	}
	if shots < 1 {
		shots = 1024
	}

	// Select the unitary gate.
	var g gate.Gate
	var expectedPhase string
	switch gateType {
	case "T":
		g = gate.T
		expectedPhase = "1/8"
	case "S":
		g = gate.S
		expectedPhase = "1/4"
	case "Z":
		g = gate.Z
		expectedPhase = "1/2"
	default:
		// Try to parse as an angle for RZ.
		angle, err := strconv.ParseFloat(gateType, 64)
		if err != nil {
			return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("unknown gate type: %s (use T, S, Z, or a float angle)", gateType)})
		}
		g = gate.RZ(angle)
		expectedPhase = fmt.Sprintf("%.4f/(2pi)", angle/(2*math.Pi))
	}

	// Build eigenstate preparation: |1> (eigenstate for all these gates).
	eigenPrep := ir.New("eigenprep", 1, 0, []ir.Operation{
		{Gate: gate.X, Qubits: []int{0}},
	}, nil)

	ctx := context.Background()
	result, err := qpe.Run(ctx, qpe.Config{
		Unitary:      g,
		NumPhaseBits: numPhaseBits,
		EigenState:   eigenPrep,
		Shots:        shots,
	})
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: err.Error()})
	}

	circSVG, histSVG, qasmCode := drawAndEmit(result.Circuit, result.Counts, dark)

	desc := fmt.Sprintf("QPE: estimated phase = %.6f (expected %s, precision = %d bits)", result.Phase, expectedPhase, numPhaseBits)

	return marshalAlgorithm(algorithmResult{
		CircuitSVG:   circSVG,
		HistogramSVG: histSVG,
		QASMCode:     qasmCode,
		Result:       desc,
	})
}

// ---------------------------------------------------------------------------
// runShorJS
//
// Args: (N int, shots int, dark? bool)
// ---------------------------------------------------------------------------

func runShorJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshalAlgorithm(algorithmResult{Error: "usage: runShor(N, shots, dark?)"})
	}
	N := args[0].Int()
	shots := args[1].Int()
	dark := len(args) >= 3 && args[2].Truthy()

	// Restrict to small composites that are feasible in WASM.
	allowed := map[int]bool{15: true, 21: true, 33: true, 35: true, 55: true, 77: true}
	if !allowed[N] {
		return marshalAlgorithm(algorithmResult{Error: fmt.Sprintf("N must be one of 15, 21, 33, 35, 55, 77 (got %d)", N)})
	}
	if shots < 1 {
		shots = 1024
	}

	ctx := context.Background()
	result, err := shor.Run(ctx, shor.Config{
		N:           N,
		Shots:       shots,
		MaxAttempts: 5,
	})
	if err != nil {
		return marshalAlgorithm(algorithmResult{Error: err.Error()})
	}

	var circSVG, histSVG, qasmCode string
	if result.Circuit != nil {
		circSVG, histSVG, qasmCode = drawAndEmit(result.Circuit, nil, dark)
	}

	desc := fmt.Sprintf("Shor: %d = %d x %d (base=%d, period=%d, attempts=%d)",
		N, result.Factors[0], result.Factors[1], result.Base, result.Period, result.Attempts)

	return marshalAlgorithm(algorithmResult{
		CircuitSVG:   circSVG,
		HistogramSVG: histSVG,
		QASMCode:     qasmCode,
		Result:       desc,
	})
}

