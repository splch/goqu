//go:build js && wasm

package main

import (
	"strconv"
	"strings"
)

// Manual JSON marshaling to avoid importing encoding/json and its heavy
// reflect dependency, which adds ~1 MB to the WASM binary.

func marshalRun(r runResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.Circuit != "" {
		jsonKey(&b, "circuit", &n)
		jsonStr(&b, r.Circuit)
	}
	if r.Histogram != "" {
		jsonKey(&b, "histogram", &n)
		jsonStr(&b, r.Histogram)
	}
	if r.Bloch != "" {
		jsonKey(&b, "bloch", &n)
		jsonStr(&b, r.Bloch)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalState(r stateResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.Amplitudes) > 0 {
		jsonKey(&b, "amplitudes", &n)
		b.WriteByte('[')
		for i, a := range r.Amplitudes {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"re":`)
			jsonFloat(&b, a.Re)
			b.WriteString(`,"im":`)
			jsonFloat(&b, a.Im)
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	if len(r.Probabilities) > 0 {
		jsonKey(&b, "probabilities", &n)
		jsonFloats(&b, r.Probabilities)
	}
	if len(r.BlochVectors) > 0 {
		jsonKey(&b, "blochVectors", &n)
		b.WriteByte('[')
		for i, v := range r.BlochVectors {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"x":`)
			jsonFloat(&b, v.X)
			b.WriteString(`,"y":`)
			jsonFloat(&b, v.Y)
			b.WriteString(`,"z":`)
			jsonFloat(&b, v.Z)
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalProb(r probResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.Probabilities) > 0 {
		jsonKey(&b, "probabilities", &n)
		jsonFloats(&b, r.Probabilities)
	}
	if len(r.Labels) > 0 {
		jsonKey(&b, "labels", &n)
		b.WriteByte('[')
		for i, s := range r.Labels {
			if i > 0 {
				b.WriteByte(',')
			}
			jsonStr(&b, s)
		}
		b.WriteByte(']')
	}
	if r.Histogram != "" {
		jsonKey(&b, "histogram", &n)
		jsonStr(&b, r.Histogram)
	}
	if r.Circuit != "" {
		jsonKey(&b, "circuit", &n)
		jsonStr(&b, r.Circuit)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

// jsonKey writes ,"key": (with leading comma if not first field).
func jsonKey(b *strings.Builder, key string, n *int) {
	if *n > 0 {
		b.WriteByte(',')
	}
	*n++
	b.WriteByte('"')
	b.WriteString(key)
	b.WriteString(`":`)
}

// jsonStr writes a JSON-escaped string value.
func jsonStr(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte("0123456789abcdef"[r>>4])
				b.WriteByte("0123456789abcdef"[r&0xf])
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// jsonFloat writes a float64 as a JSON number.
func jsonFloat(b *strings.Builder, f float64) {
	b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
}

// jsonFloats writes a JSON array of float64 values.
func jsonFloats(b *strings.Builder, fs []float64) {
	b.WriteByte('[')
	for i, f := range fs {
		if i > 0 {
			b.WriteByte(',')
		}
		jsonFloat(b, f)
	}
	b.WriteByte(']')
}

// jsonInt writes an int as a JSON number.
func jsonInt(b *strings.Builder, v int) {
	b.WriteString(strconv.Itoa(v))
}

// jsonBool writes a bool as a JSON boolean.
func jsonBool(b *strings.Builder, v bool) {
	if v {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
}

// jsonInts writes a JSON array of int values.
func jsonInts(b *strings.Builder, vs []int) {
	b.WriteByte('[')
	for i, v := range vs {
		if i > 0 {
			b.WriteByte(',')
		}
		jsonInt(b, v)
	}
	b.WriteByte(']')
}

// jsonStrs writes a JSON array of string values.
func jsonStrs(b *strings.Builder, vs []string) {
	b.WriteByte('[')
	for i, s := range vs {
		if i > 0 {
			b.WriteByte(',')
		}
		jsonStr(b, s)
	}
	b.WriteByte(']')
}

// json2DFloats writes a JSON 2D array of float64 values.
func json2DFloats(b *strings.Builder, vs [][]float64) {
	b.WriteByte('[')
	for i, row := range vs {
		if i > 0 {
			b.WriteByte(',')
		}
		jsonFloats(b, row)
	}
	b.WriteByte(']')
}

// noisyResult holds the output from runNoisyQASMJS.
type noisyResult struct {
	CircuitSVG   string
	HistogramSVG string
	StateCitySVG string
	Purity       float64
	Fidelity     float64
	Error        string
}

// compareResult holds the output from compareIdealNoisyJS.
type compareResult struct {
	CircuitSVG        string
	IdealHistogramSVG string
	NoisyHistogramSVG string
	StateCitySVG      string
	Purity            float64
	Fidelity          float64
	Error             string
}

func marshalNoisy(r noisyResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.CircuitSVG != "" {
		jsonKey(&b, "circuitSVG", &n)
		jsonStr(&b, r.CircuitSVG)
	}
	if r.HistogramSVG != "" {
		jsonKey(&b, "histogramSVG", &n)
		jsonStr(&b, r.HistogramSVG)
	}
	if r.StateCitySVG != "" {
		jsonKey(&b, "stateCitySVG", &n)
		jsonStr(&b, r.StateCitySVG)
	}
	if r.Purity != 0 {
		jsonKey(&b, "purity", &n)
		jsonFloat(&b, r.Purity)
	}
	if r.Fidelity != 0 {
		jsonKey(&b, "fidelity", &n)
		jsonFloat(&b, r.Fidelity)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalCompare(r compareResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.CircuitSVG != "" {
		jsonKey(&b, "circuitSVG", &n)
		jsonStr(&b, r.CircuitSVG)
	}
	if r.IdealHistogramSVG != "" {
		jsonKey(&b, "idealHistogramSVG", &n)
		jsonStr(&b, r.IdealHistogramSVG)
	}
	if r.NoisyHistogramSVG != "" {
		jsonKey(&b, "noisyHistogramSVG", &n)
		jsonStr(&b, r.NoisyHistogramSVG)
	}
	if r.StateCitySVG != "" {
		jsonKey(&b, "stateCitySVG", &n)
		jsonStr(&b, r.StateCitySVG)
	}
	if r.Purity != 0 {
		jsonKey(&b, "purity", &n)
		jsonFloat(&b, r.Purity)
	}
	if r.Fidelity != 0 {
		jsonKey(&b, "fidelity", &n)
		jsonFloat(&b, r.Fidelity)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalClifford(r cliffordResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.CircuitSVG != "" {
		jsonKey(&b, "circuitSVG", &n)
		jsonStr(&b, r.CircuitSVG)
	}
	if r.HistogramSVG != "" {
		jsonKey(&b, "histogramSVG", &n)
		jsonStr(&b, r.HistogramSVG)
	}
	if len(r.Stabilizers) > 0 {
		jsonKey(&b, "stabilizers", &n)
		jsonStrs(&b, r.Stabilizers)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalCliffordSteps(r cliffordStepsResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.Steps) > 0 {
		jsonKey(&b, "steps", &n)
		b.WriteByte('[')
		for i, step := range r.Steps {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"gate":`)
			jsonStr(&b, step.Gate)
			b.WriteString(`,"qubits":`)
			jsonInts(&b, step.Qubits)
			b.WriteString(`,"stabilizers":`)
			jsonStrs(&b, step.Stabilizers)
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	if r.TotalSteps > 0 {
		jsonKey(&b, "totalSteps", &n)
		jsonInt(&b, r.TotalSteps)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

// marshalTranspileStats writes a transpileStats as a JSON object.
func marshalTranspileStats(b *strings.Builder, s transpileStats) {
	b.WriteString(`{"gates":`)
	jsonInt(b, s.Gates)
	b.WriteString(`,"twoQubitGates":`)
	jsonInt(b, s.TwoQubitGates)
	b.WriteString(`,"depth":`)
	jsonInt(b, s.Depth)
	b.WriteByte('}')
}

func marshalTranspile(r transpileResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.BeforeCircuitSVG != "" {
		jsonKey(&b, "beforeCircuitSVG", &n)
		jsonStr(&b, r.BeforeCircuitSVG)
	}
	if r.AfterCircuitSVG != "" {
		jsonKey(&b, "afterCircuitSVG", &n)
		jsonStr(&b, r.AfterCircuitSVG)
	}
	if r.TranspiledQASM != "" {
		jsonKey(&b, "transpiledQASM", &n)
		jsonStr(&b, r.TranspiledQASM)
	}
	if r.TranspiledQuil != "" {
		jsonKey(&b, "transpiledQuil", &n)
		jsonStr(&b, r.TranspiledQuil)
	}
	// Always emit stats objects (they contain meaningful zero values).
	jsonKey(&b, "beforeStats", &n)
	marshalTranspileStats(&b, r.BeforeStats)
	jsonKey(&b, "afterStats", &n)
	marshalTranspileStats(&b, r.AfterStats)
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalTargetInfo(r targetInfoResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.Name != "" {
		jsonKey(&b, "name", &n)
		jsonStr(&b, r.Name)
	}
	jsonKey(&b, "numQubits", &n)
	jsonInt(&b, r.NumQubits)
	if len(r.BasisGates) > 0 {
		jsonKey(&b, "basisGates", &n)
		jsonStrs(&b, r.BasisGates)
	}
	if len(r.Edges) > 0 {
		jsonKey(&b, "edges", &n)
		b.WriteByte('[')
		for i, e := range r.Edges {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteByte('[')
			jsonInt(&b, e[0])
			b.WriteByte(',')
			jsonInt(&b, e[1])
			b.WriteByte(']')
		}
		b.WriteByte(']')
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

// ---------------------------------------------------------------------------
// Pauli expectation value marshalers
// ---------------------------------------------------------------------------

func marshalExpectation(r expectationResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.CircuitSVG != "" {
		jsonKey(&b, "circuitSVG", &n)
		jsonStr(&b, r.CircuitSVG)
	}
	jsonKey(&b, "expectation", &n)
	jsonFloat(&b, r.Expectation)
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalSweep(r sweepResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.ParamValues) > 0 {
		jsonKey(&b, "paramValues", &n)
		jsonFloats(&b, r.ParamValues)
	}
	if len(r.Expectations) > 0 {
		jsonKey(&b, "expectations", &n)
		jsonFloats(&b, r.Expectations)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalSweep2D(r sweep2DResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.Param1Values) > 0 {
		jsonKey(&b, "param1Values", &n)
		jsonFloats(&b, r.Param1Values)
	}
	if len(r.Param2Values) > 0 {
		jsonKey(&b, "param2Values", &n)
		jsonFloats(&b, r.Param2Values)
	}
	if len(r.Energies) > 0 {
		jsonKey(&b, "energies", &n)
		json2DFloats(&b, r.Energies)
	}
	jsonKey(&b, "minEnergy", &n)
	jsonFloat(&b, r.MinEnergy)
	jsonKey(&b, "minParam1", &n)
	jsonFloat(&b, r.MinParam1)
	jsonKey(&b, "minParam2", &n)
	jsonFloat(&b, r.MinParam2)
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalGradient(r gradientResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.Gradients) > 0 {
		jsonKey(&b, "gradients", &n)
		jsonFloats(&b, r.Gradients)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

// ---------------------------------------------------------------------------
// QAOA marshaler
// ---------------------------------------------------------------------------

func marshalQAOA(r qaoaResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.CircuitSVG != "" {
		jsonKey(&b, "circuitSVG", &n)
		jsonStr(&b, r.CircuitSVG)
	}
	if r.HistogramSVG != "" {
		jsonKey(&b, "histogramSVG", &n)
		jsonStr(&b, r.HistogramSVG)
	}
	if r.QASMCode != "" {
		jsonKey(&b, "qasmCode", &n)
		jsonStr(&b, r.QASMCode)
	}
	if r.BestBitstring != "" {
		jsonKey(&b, "bestBitstring", &n)
		jsonStr(&b, r.BestBitstring)
	}
	jsonKey(&b, "energy", &n)
	jsonFloat(&b, r.Energy)
	if len(r.History) > 0 {
		jsonKey(&b, "history", &n)
		jsonFloats(&b, r.History)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

// ---------------------------------------------------------------------------
// VQE marshaler
// ---------------------------------------------------------------------------

func marshalVQE(r vqeResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.CircuitSVG != "" {
		jsonKey(&b, "circuitSVG", &n)
		jsonStr(&b, r.CircuitSVG)
	}
	jsonKey(&b, "energy", &n)
	jsonFloat(&b, r.Energy)
	if len(r.History) > 0 {
		jsonKey(&b, "history", &n)
		jsonFloats(&b, r.History)
	}
	jsonKey(&b, "numIterations", &n)
	jsonInt(&b, r.NumIterations)
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

// ---------------------------------------------------------------------------
// Mitigation marshalers
// ---------------------------------------------------------------------------

func marshalZNE(r zneResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.ScaleFactors) > 0 {
		jsonKey(&b, "scaleFactors", &n)
		jsonFloats(&b, r.ScaleFactors)
	}
	if len(r.NoisyValues) > 0 {
		jsonKey(&b, "noisyValues", &n)
		jsonFloats(&b, r.NoisyValues)
	}
	jsonKey(&b, "linearExtrapolated", &n)
	jsonFloat(&b, r.LinearExtrapolated)
	jsonKey(&b, "ideal", &n)
	jsonFloat(&b, r.Ideal)
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalDD(r ddResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.BeforeCircuitSVG != "" {
		jsonKey(&b, "beforeCircuitSVG", &n)
		jsonStr(&b, r.BeforeCircuitSVG)
	}
	if r.AfterCircuitSVG != "" {
		jsonKey(&b, "afterCircuitSVG", &n)
		jsonStr(&b, r.AfterCircuitSVG)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalTwirl(r twirlResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.OriginalCircuitSVG != "" {
		jsonKey(&b, "originalCircuitSVG", &n)
		jsonStr(&b, r.OriginalCircuitSVG)
	}
	if r.TwirledCircuitSVG != "" {
		jsonKey(&b, "twirledCircuitSVG", &n)
		jsonStr(&b, r.TwirledCircuitSVG)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

// ---------------------------------------------------------------------------
// Algorithm marshalers
// ---------------------------------------------------------------------------

func marshalAlgorithm(r algorithmResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.CircuitSVG != "" {
		jsonKey(&b, "circuitSVG", &n)
		jsonStr(&b, r.CircuitSVG)
	}
	if r.HistogramSVG != "" {
		jsonKey(&b, "histogramSVG", &n)
		jsonStr(&b, r.HistogramSVG)
	}
	if r.QASMCode != "" {
		jsonKey(&b, "qasmCode", &n)
		jsonStr(&b, r.QASMCode)
	}
	if r.Result != "" {
		jsonKey(&b, "result", &n)
		jsonStr(&b, r.Result)
	}
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalGroverSteps(r groverStepResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if len(r.Steps) > 0 {
		jsonKey(&b, "steps", &n)
		b.WriteByte('[')
		for i, step := range r.Steps {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"iteration":`)
			jsonInt(&b, step.Iteration)
			b.WriteString(`,"probabilities":`)
			jsonFloats(&b, step.Probabilities)
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	jsonKey(&b, "optimalIterations", &n)
	jsonInt(&b, r.OptimalIterations)
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

func marshalAnsatz(r ansatzResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.CircuitSVG != "" {
		jsonKey(&b, "circuitSVG", &n)
		jsonStr(&b, r.CircuitSVG)
	}
	if r.QASMCode != "" {
		jsonKey(&b, "qasmCode", &n)
		jsonStr(&b, r.QASMCode)
	}
	jsonKey(&b, "numParams", &n)
	jsonInt(&b, r.NumParams)
	jsonKey(&b, "depth", &n)
	jsonInt(&b, r.Depth)
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

// ---------------------------------------------------------------------------
// Trotter marshaler
// ---------------------------------------------------------------------------

func marshalTrotter(r trotterResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0
	if r.CircuitSVG != "" {
		jsonKey(&b, "circuitSVG", &n)
		jsonStr(&b, r.CircuitSVG)
	}
	if r.QASMCode != "" {
		jsonKey(&b, "qasmCode", &n)
		jsonStr(&b, r.QASMCode)
	}
	jsonKey(&b, "expectation", &n)
	jsonFloat(&b, r.Expectation)
	jsonKey(&b, "steps", &n)
	jsonInt(&b, r.Steps)
	jsonKey(&b, "order", &n)
	jsonInt(&b, r.Order)
	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
	}
	b.WriteByte('}')
	return b.String()
}

