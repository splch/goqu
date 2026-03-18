//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/qasm/parser"
	"github.com/splch/goqu/sim/densitymatrix"
	"github.com/splch/goqu/sim/noise"
	"github.com/splch/goqu/sim/statevector"
	"github.com/splch/goqu/viz"
)

// buildNoiseModel constructs a NoiseModel from the given model name and parameter.
func buildNoiseModel(noiseModel string, p float64) (*noise.NoiseModel, error) {
	nm := noise.New()
	switch noiseModel {
	case "depolarizing":
		nm.AddDefaultError(1, noise.Depolarizing1Q(p))
		nm.AddDefaultError(2, noise.Depolarizing2Q(p))
	case "amplitude_damping":
		nm.AddDefaultError(1, noise.AmplitudeDamping(p))
	case "phase_damping":
		nm.AddDefaultError(1, noise.PhaseDamping(p))
	case "bit_flip":
		nm.AddDefaultError(1, noise.BitFlip(p))
	case "phase_flip":
		nm.AddDefaultError(1, noise.PhaseFlip(p))
	case "thermal":
		// Intuitive slider mapping: t1=50, t2=30, time=p*50
		nm.AddDefaultError(1, noise.ThermalRelaxation(50, 30, p*50))
	default:
		return nil, fmt.Errorf("unknown noise model: %s", noiseModel)
	}
	return nm, nil
}

// runNoisyQASMJS runs a QASM circuit with a density matrix simulator and noise.
// Args: (qasm string, shots int, noiseModel string, noiseParam float64, dark? bool)
func runNoisyQASMJS(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return marshalNoisy(noisyResult{Error: "usage: runNoisyQASM(qasm, shots, noiseModel, noiseParam, dark?)"})
	}
	qasm := args[0].String()
	shots := args[1].Int()
	if shots < 1 {
		shots = 1024
	}
	noiseModel := args[2].String()
	noiseParam := args[3].Float()
	dark := len(args) >= 5 && args[4].Truthy()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalNoisy(noisyResult{Error: err.Error()})
	}

	nq := circ.NumQubits()
	if nq > 8 {
		return marshalNoisy(noisyResult{Error: "noisy simulation limited to 8 qubits"})
	}

	nm, err := buildNoiseModel(noiseModel, noiseParam)
	if err != nil {
		return marshalNoisy(noisyResult{Error: err.Error()})
	}

	var drawOpts []draw.SVGOption
	var vizOpts []viz.Option
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
		vizOpts = append(vizOpts, viz.WithStyle(viz.DarkStyle()))
	}

	r := noisyResult{CircuitSVG: draw.SVG(circ, drawOpts...)}

	// Noisy simulation for histogram.
	sim := densitymatrix.New(nq).WithNoise(nm)
	defer sim.Close()

	counts, err := sim.Run(circ, shots)
	if err != nil {
		return marshalNoisy(noisyResult{Error: err.Error()})
	}
	r.HistogramSVG = viz.Histogram(counts, vizOpts...)

	// Re-evolve for purity (Run already evolved, but we need a clean evolve).
	sim.Reset()
	if err := sim.Evolve(circ); err != nil {
		return marshalNoisy(noisyResult{Error: err.Error()})
	}
	r.Purity = sim.Purity()

	// StateCity for small systems.
	if nq <= 4 {
		dim := 1 << nq
		r.StateCitySVG = viz.StateCity(sim.DensityMatrix(), dim, vizOpts...)
	}

	// Ideal statevector for fidelity computation.
	idealSim := statevector.New(nq)
	defer idealSim.Close()
	if err := idealSim.Evolve(circ); err != nil {
		return marshalNoisy(noisyResult{Error: err.Error()})
	}
	r.Fidelity = sim.Fidelity(idealSim.StateVector())

	return marshalNoisy(r)
}

// compareIdealNoisyJS runs both ideal and noisy simulations for comparison.
// Args: (qasm string, shots int, noiseModel string, noiseParam float64, dark? bool)
func compareIdealNoisyJS(_ js.Value, args []js.Value) any {
	if len(args) < 4 {
		return marshalCompare(compareResult{Error: "usage: compareIdealNoisy(qasm, shots, noiseModel, noiseParam, dark?)"})
	}
	qasm := args[0].String()
	shots := args[1].Int()
	if shots < 1 {
		shots = 1024
	}
	noiseModel := args[2].String()
	noiseParam := args[3].Float()
	dark := len(args) >= 5 && args[4].Truthy()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalCompare(compareResult{Error: err.Error()})
	}

	nq := circ.NumQubits()
	if nq > 8 {
		return marshalCompare(compareResult{Error: "noisy simulation limited to 8 qubits"})
	}

	nm, err := buildNoiseModel(noiseModel, noiseParam)
	if err != nil {
		return marshalCompare(compareResult{Error: err.Error()})
	}

	var drawOpts []draw.SVGOption
	var vizOpts []viz.Option
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
		vizOpts = append(vizOpts, viz.WithStyle(viz.DarkStyle()))
	}

	r := compareResult{CircuitSVG: draw.SVG(circ, drawOpts...)}

	// Ideal simulation: exact probabilities.
	idealSim := statevector.New(nq)
	defer idealSim.Close()
	if err := idealSim.Evolve(circ); err != nil {
		return marshalCompare(compareResult{Error: err.Error()})
	}

	sv := idealSim.StateVector()
	probMap := make(map[string]float64)
	for i, amp := range sv {
		p := real(amp)*real(amp) + imag(amp)*imag(amp)
		if p > 1e-10 {
			probMap[formatBinary(i, nq)] = p
		}
	}
	r.IdealHistogramSVG = viz.HistogramProb(probMap, vizOpts...)

	// Noisy simulation: shot counts.
	noisySim := densitymatrix.New(nq).WithNoise(nm)
	defer noisySim.Close()

	counts, err := noisySim.Run(circ, shots)
	if err != nil {
		return marshalCompare(compareResult{Error: err.Error()})
	}
	r.NoisyHistogramSVG = viz.Histogram(counts, vizOpts...)

	// Re-evolve for purity and fidelity.
	noisySim.Reset()
	if err := noisySim.Evolve(circ); err != nil {
		return marshalCompare(compareResult{Error: err.Error()})
	}
	r.Purity = noisySim.Purity()
	r.Fidelity = noisySim.Fidelity(sv)

	// StateCity for small systems.
	if nq <= 4 {
		dim := 1 << nq
		r.StateCitySVG = viz.StateCity(noisySim.DensityMatrix(), dim, vizOpts...)
	}

	return marshalCompare(r)
}
