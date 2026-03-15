package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/splch/goqu/algorithm/grover"
	"github.com/splch/goqu/algorithm/qpe"
	"github.com/splch/goqu/algorithm/textbook"
	"github.com/splch/goqu/circuit/builder"
	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/circuit/gate"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/sim/densitymatrix"
	"github.com/splch/goqu/sim/noise"
	"github.com/splch/goqu/sim/statevector"
	"github.com/splch/goqu/transpile/pipeline"
	"github.com/splch/goqu/transpile/target"
)

// DemoResult holds the output of a demo circuit execution.
type DemoResult struct {
	SVG       string
	Counts    map[string]int
	StateInfo string
	Circuit   *ir.Circuit
}

// runDemo builds a circuit, draws it, and simulates it.
func runDemo(c *ir.Circuit, shots int) (*DemoResult, error) {
	svg := draw.SVG(c)
	sim := statevector.New(c.NumQubits())
	counts, err := sim.Run(c, shots)
	if err != nil {
		return nil, err
	}
	return &DemoResult{SVG: svg, Counts: counts, Circuit: c}, nil
}

// histogramHTML generates a simple CSS bar chart from measurement counts.
func histogramHTML(counts map[string]int) string {
	if len(counts) == 0 {
		return "<p>No measurement results</p>"
	}

	// Sort keys
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	total := 0
	maxCount := 0
	for _, v := range counts {
		total += v
		if v > maxCount {
			maxCount = v
		}
	}

	var sb strings.Builder
	sb.WriteString(`<div class="histogram">`)
	for _, k := range keys {
		v := counts[k]
		pct := float64(v) / float64(total) * 100
		barWidth := float64(v) / float64(maxCount) * 100
		sb.WriteString(fmt.Sprintf(
			`<div class="hist-row">`+
				`<span class="hist-label">|%s⟩</span>`+
				`<div class="hist-bar-bg"><div class="hist-bar" style="width:%.1f%%"></div></div>`+
				`<span class="hist-value">%d (%.1f%%)</span>`+
				`</div>`,
			k, barWidth, v, pct))
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// --- Lesson 2: Qubit demos ---

func demoSuperposition(gateName string, shots int) (*DemoResult, error) {
	b := builder.New("superposition", 1)
	switch gateName {
	case "H":
		b.H(0)
	case "X":
		b.X(0)
	case "Y":
		b.Y(0)
	case "Z":
		b.Z(0)
	default:
		b.H(0)
	}
	b.MeasureAll()
	c, err := b.Build()
	if err != nil {
		return nil, err
	}
	return runDemo(c, shots)
}

// --- Lesson 3: Gate demos ---

func demoGate(gateName string, shots int) (*DemoResult, error) {
	b := builder.New("gate-demo", 1)
	switch gateName {
	case "H":
		b.H(0)
	case "X":
		b.X(0)
	case "Y":
		b.Y(0)
	case "Z":
		b.Z(0)
	case "S":
		b.S(0)
	case "T":
		b.T(0)
	case "RX":
		b.RX(math.Pi/4, 0)
	case "RY":
		b.RY(math.Pi/4, 0)
	case "RZ":
		b.RZ(math.Pi/4, 0)
	default:
		b.H(0)
	}
	b.MeasureAll()
	c, err := b.Build()
	if err != nil {
		return nil, err
	}
	return runDemo(c, shots)
}

// --- Lesson 4: Multi-gate circuits ---

func demoCircuit(gates string, shots int) (*DemoResult, error) {
	b := builder.New("circuit-demo", 2)
	for _, g := range strings.Split(gates, ",") {
		switch strings.TrimSpace(g) {
		case "H0":
			b.H(0)
		case "H1":
			b.H(1)
		case "X0":
			b.X(0)
		case "X1":
			b.X(1)
		case "CNOT":
			b.CNOT(0, 1)
		case "SWAP":
			b.SWAP(0, 1)
		}
	}
	b.MeasureAll()
	c, err := b.Build()
	if err != nil {
		return nil, err
	}
	return runDemo(c, shots)
}

// --- Lesson 5: Entanglement / Bell states ---

func demoBellState(variant string, shots int) (*DemoResult, error) {
	b := builder.New("bell-state", 2)
	switch variant {
	case "phi+":
		b.H(0).CNOT(0, 1)
	case "phi-":
		b.X(0).H(0).CNOT(0, 1)
	case "psi+":
		b.H(0).CNOT(0, 1).X(1)
	case "psi-":
		b.X(0).H(0).CNOT(0, 1).X(1)
	default:
		b.H(0).CNOT(0, 1)
	}
	b.MeasureAll()
	c, err := b.Build()
	if err != nil {
		return nil, err
	}
	return runDemo(c, shots)
}

// --- Lesson 6: Deutsch-Jozsa ---

func demoDeutschJozsa(oracleType string, shots int) (*DemoResult, error) {
	var oracle textbook.DJOracle
	switch oracleType {
	case "constant0":
		oracle = func(b *builder.Builder, inputs []int, ancilla int) {
			// f(x) = 0 for all x — do nothing
		}
	case "constant1":
		oracle = func(b *builder.Builder, inputs []int, ancilla int) {
			b.X(ancilla) // f(x) = 1 for all x
		}
	case "balanced":
		oracle = func(b *builder.Builder, inputs []int, ancilla int) {
			b.CNOT(inputs[0], ancilla) // f(x) = x_0
		}
	default:
		oracle = func(b *builder.Builder, inputs []int, ancilla int) {
			b.CNOT(inputs[0], ancilla)
		}
	}

	result, err := textbook.DeutschJozsa(context.Background(), textbook.DJConfig{
		NumQubits: 3,
		Oracle:    oracle,
		Shots:     shots,
	})
	if err != nil {
		return nil, err
	}

	svg := draw.SVG(result.Circuit)
	info := "constant"
	if !result.IsConstant {
		info = "balanced"
	}

	return &DemoResult{
		SVG:       svg,
		Counts:    result.Counts,
		StateInfo: fmt.Sprintf("The function is: %s", info),
		Circuit:   result.Circuit,
	}, nil
}

// --- Lesson 7: Grover ---

func demoGrover(markedState int, numQubits int, shots int) (*DemoResult, error) {
	if numQubits < 2 || numQubits > 5 {
		numQubits = 3
	}

	oracle := func(b *builder.Builder, qubits []int) {
		// Phase-flip the marked state
		n := len(qubits)
		// Flip qubits that should be |0⟩ in the marked state
		for i := 0; i < n; i++ {
			if (markedState>>i)&1 == 0 {
				b.X(qubits[i])
			}
		}
		// Multi-controlled Z (using MCZ pattern: H-MCX-H on last qubit)
		if n == 2 {
			b.CZ(qubits[0], qubits[1])
		} else if n == 3 {
			b.CCZ(qubits[0], qubits[1], qubits[2])
		} else {
			// For larger: H-MCX-H
			last := qubits[n-1]
			b.H(last)
			controls := make([]int, n-1)
			copy(controls, qubits[:n-1])
			b.MCX(controls, last)
			b.H(last)
		}
		// Undo flips
		for i := 0; i < n; i++ {
			if (markedState>>i)&1 == 0 {
				b.X(qubits[i])
			}
		}
	}

	res, err := grover.Run(context.Background(), grover.Config{
		NumQubits:    numQubits,
		Oracle:       oracle,
		NumSolutions: 1,
		Shots:        shots,
	})
	if err != nil {
		return nil, err
	}

	svg := draw.SVG(res.Circuit)
	return &DemoResult{
		SVG:       svg,
		Counts:    res.Counts,
		StateInfo: fmt.Sprintf("Searched for |%s⟩, found: |%s⟩ (%d iterations)", formatBits(markedState, numQubits), res.TopResult, res.NumIters),
		Circuit:   res.Circuit,
	}, nil
}

func formatBits(val, n int) string {
	s := fmt.Sprintf("%b", val)
	for len(s) < n {
		s = "0" + s
	}
	return s
}

// --- Lesson 8: QPE ---

func demoQPE(phase float64, numBits int, shots int) (*DemoResult, error) {
	if numBits < 2 || numBits > 6 {
		numBits = 3
	}

	// Create a phase gate with the given phase
	phaseGate := gate.Phase(2 * math.Pi * phase)

	// Prepare eigenstate |1⟩
	eigenPrep, err := builder.New("eigen", 1).X(0).Build()
	if err != nil {
		return nil, err
	}

	res, err := qpe.Run(context.Background(), qpe.Config{
		Unitary:      phaseGate,
		NumPhaseBits: numBits,
		EigenState:   eigenPrep,
		Shots:        shots,
	})
	if err != nil {
		return nil, err
	}

	svg := draw.SVG(res.Circuit)
	return &DemoResult{
		SVG:       svg,
		Counts:    res.Counts,
		StateInfo: fmt.Sprintf("Input phase: %.4f, Estimated phase: %.4f", phase, res.Phase),
		Circuit:   res.Circuit,
	}, nil
}

// --- Lesson 9: Shor (circuit display only for small N) ---

func demoShorCircuit(shots int) (*DemoResult, error) {
	// Build a period-finding circuit for a=2, N=15 (classic textbook example)
	// We show the circuit structure rather than running full Shor's
	b := builder.New("period-finding", 8)

	// Phase register: qubits 0-3 (4 counting qubits)
	for q := 0; q < 4; q++ {
		b.H(q)
	}

	// Work register: qubit 4 initialized to |1⟩
	b.X(4)

	// Controlled modular exponentiation (simplified for a=2 mod 15)
	// 2^1 mod 15: SWAP work qubits controlled on counting qubit 3
	b.CNOT(3, 5).CNOT(3, 6)
	// 2^2 mod 15 = 4: controlled on counting qubit 2
	b.CNOT(2, 6).CNOT(2, 7)
	// 2^4 mod 15 = 1: identity (controlled on qubit 1) — no gates needed
	// 2^8 mod 15 = 1: identity (controlled on qubit 0) — no gates needed

	// Inverse QFT on phase register
	for q := 0; q < 2; q++ {
		b.SWAP(q, 3-q)
	}
	for q := 0; q < 4; q++ {
		b.H(q)
		for k := q + 1; k < 4; k++ {
			b.MCP(-math.Pi/float64(int(1)<<(k-q)), []int{k}, q)
		}
	}

	// Measure phase register
	for q := 0; q < 4; q++ {
		b.Measure(q, q)
	}

	c, err := b.Build()
	if err != nil {
		return nil, err
	}

	svg := draw.SVG(c)
	sim := statevector.New(c.NumQubits())
	counts, err := sim.Run(c, shots)
	if err != nil {
		return nil, err
	}

	return &DemoResult{
		SVG:       svg,
		Counts:    counts,
		StateInfo: "Period-finding circuit for a=2, N=15. Peaks at multiples of 2^n/r reveal the period r=4, giving factors 3 and 5.",
		Circuit:   c,
	}, nil
}

// --- Lesson 10: Variational (simple parameter sweep demo) ---

func demoVariational(theta float64, shots int) (*DemoResult, error) {
	// Simple 2-qubit variational circuit
	b := builder.New("variational", 2)
	b.RY(theta, 0)
	b.RY(theta*0.5, 1)
	b.CNOT(0, 1)
	b.RY(theta*0.3, 0)
	b.MeasureAll()

	c, err := b.Build()
	if err != nil {
		return nil, err
	}

	svg := draw.SVG(c)
	sim := statevector.New(2)
	counts, err := sim.Run(c, shots)
	if err != nil {
		return nil, err
	}

	// Compute Z expectation value
	expZ := 0.0
	total := 0
	for bitstr, count := range counts {
		val := 1.0
		for _, ch := range bitstr {
			if ch == '1' {
				val *= -1
			}
		}
		expZ += val * float64(count)
		total += count
	}
	expZ /= float64(total)

	return &DemoResult{
		SVG:       svg,
		Counts:    counts,
		StateInfo: fmt.Sprintf("θ = %.2f rad, ⟨ZZ⟩ = %.4f", theta, expZ),
		Circuit:   c,
	}, nil
}

// --- Lesson 11: Noise ---

func demoNoise(noiseLevel float64, shots int) (*DemoResult, error) {
	// Bell state circuit
	b := builder.New("noisy-bell", 2)
	b.H(0).CNOT(0, 1).MeasureAll()
	c, err := b.Build()
	if err != nil {
		return nil, err
	}

	svg := draw.SVG(c)

	// Ideal simulation
	idealSim := statevector.New(2)
	idealCounts, err := idealSim.Run(c, shots)
	if err != nil {
		return nil, err
	}

	// Noisy simulation
	nm := noise.New()
	nm.AddDefaultError(1, noise.Depolarizing1Q(noiseLevel))
	nm.AddDefaultError(2, noise.Depolarizing2Q(noiseLevel))

	noisySim := densitymatrix.New(2).WithNoise(nm)
	noisyCounts, err := noisySim.Run(c, shots)
	if err != nil {
		return nil, err
	}

	return &DemoResult{
		SVG:    svg,
		Counts: noisyCounts,
		StateInfo: fmt.Sprintf("Noise level: %.2f | Ideal: %v | Noisy: %v",
			noiseLevel, formatCounts(idealCounts), formatCounts(noisyCounts)),
		Circuit: c,
	}, nil
}

func formatCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("|%s⟩:%d", k, counts[k])
	}
	return strings.Join(parts, " ")
}

// --- Lesson 12: Transpilation ---

func demoTranspile(targetName string) (*DemoResult, error) {
	// Build a circuit with non-native gates
	b := builder.New("transpile-demo", 3)
	b.H(0).CNOT(0, 1).CNOT(1, 2).T(0).S(1)
	c, err := b.Build()
	if err != nil {
		return nil, err
	}

	originalSVG := draw.SVG(c)
	originalStats := c.Stats()

	var t target.Target
	switch targetName {
	case "ionq":
		t = target.IonQForte
	case "ibm":
		t = target.IBMEagle
	case "rigetti":
		t = target.RigettiAnkaa
	default:
		t = target.IonQForte
	}

	compiled, err := pipeline.Run(context.Background(), c, t, pipeline.LevelFull)
	if err != nil {
		return nil, err
	}

	compiledSVG := draw.SVG(compiled)
	compiledStats := compiled.Stats()

	return &DemoResult{
		SVG: originalSVG,
		Counts: map[string]int{
			"original_depth":       originalStats.Depth,
			"original_gates":       originalStats.GateCount,
			"original_2q":          originalStats.TwoQubitGates,
			"compiled_depth":       compiledStats.Depth,
			"compiled_gates":       compiledStats.GateCount,
			"compiled_2q":          compiledStats.TwoQubitGates,
		},
		StateInfo: fmt.Sprintf("Target: %s | Basis: %v | Original: %d gates (depth %d) → Compiled: %d gates (depth %d)\n%s",
			t.Name, t.BasisGates,
			originalStats.GateCount, originalStats.Depth,
			compiledStats.GateCount, compiledStats.Depth,
			compiledSVG),
	}, nil
}

// --- Sandbox ---

func demoSandbox(numQubits int, gateList []string, shots int) (*DemoResult, error) {
	if numQubits < 1 || numQubits > 6 {
		numQubits = 2
	}

	b := builder.New("sandbox", numQubits)
	for _, g := range gateList {
		parts := strings.Split(g, ":")
		if len(parts) < 2 {
			continue
		}
		gateName := parts[0]
		qubitIdx := 0
		fmt.Sscanf(parts[1], "%d", &qubitIdx)

		if qubitIdx >= numQubits {
			continue
		}

		switch gateName {
		case "H":
			b.H(qubitIdx)
		case "X":
			b.X(qubitIdx)
		case "Y":
			b.Y(qubitIdx)
		case "Z":
			b.Z(qubitIdx)
		case "S":
			b.S(qubitIdx)
		case "T":
			b.T(qubitIdx)
		case "CNOT":
			if len(parts) >= 3 {
				target := 0
				fmt.Sscanf(parts[2], "%d", &target)
				if target < numQubits && target != qubitIdx {
					b.CNOT(qubitIdx, target)
				}
			}
		case "CZ":
			if len(parts) >= 3 {
				target := 0
				fmt.Sscanf(parts[2], "%d", &target)
				if target < numQubits && target != qubitIdx {
					b.CZ(qubitIdx, target)
				}
			}
		case "SWAP":
			if len(parts) >= 3 {
				target := 0
				fmt.Sscanf(parts[2], "%d", &target)
				if target < numQubits && target != qubitIdx {
					b.SWAP(qubitIdx, target)
				}
			}
		case "RX":
			b.RX(math.Pi/4, qubitIdx)
		case "RY":
			b.RY(math.Pi/4, qubitIdx)
		case "RZ":
			b.RZ(math.Pi/4, qubitIdx)
		}
	}
	b.MeasureAll()
	c, err := b.Build()
	if err != nil {
		return nil, err
	}

	return runDemo(c, shots)
}
