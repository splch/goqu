//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/circuit/ir"
	"github.com/splch/goqu/qasm/parser"
	"github.com/splch/goqu/sim/pauli"
	"github.com/splch/goqu/sim/statevector"
)

// Result types

type expectationResult struct {
	CircuitSVG  string
	Expectation float64
	Error       string
}

type sweepResult struct {
	ParamValues  []float64
	Expectations []float64
	Error        string
}

type sweep2DResult struct {
	Param1Values []float64
	Param2Values []float64
	Energies     [][]float64
	MinEnergy    float64
	MinParam1    float64
	MinParam2    float64
	Error        string
}

type gradientResult struct {
	Gradients []float64
	Error     string
}

// Helper: parse a Pauli expression string

// parsePauliExpr parses a Pauli expression that may be a single PauliString
// (e.g. "ZZI") or a sum with real coefficients (e.g. "0.5*ZZI + -0.3*XIZ").
// Returns the expectation value given a statevector.
func computePauliExpect(sv []complex128, pauliStr string, nq int) (float64, error) {
	pauliStr = strings.TrimSpace(pauliStr)
	if pauliStr == "" {
		return 0, fmt.Errorf("empty Pauli string")
	}

	// Check if it contains "+" which indicates a sum.
	if strings.Contains(pauliStr, "+") {
		return computePauliSumExpect(sv, pauliStr, nq)
	}

	// Single term, possibly with a coefficient like "0.5*ZZI".
	coeff, label, err := parsePauliTerm(pauliStr)
	if err != nil {
		return 0, err
	}
	ps, err := pauli.Parse(label)
	if err != nil {
		return 0, err
	}
	if ps.NumQubits() != nq {
		return 0, fmt.Errorf("Pauli string has %d qubits, circuit has %d", ps.NumQubits(), nq)
	}
	val := pauli.Expect(sv, ps.Scale(complex(coeff, 0)))
	return real(val), nil
}

// computePauliSumExpect handles sum expressions like "0.5*ZZI + -0.3*XIZ".
func computePauliSumExpect(sv []complex128, expr string, nq int) (float64, error) {
	parts := strings.Split(expr, "+")
	var terms []pauli.PauliString
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		coeff, label, err := parsePauliTerm(part)
		if err != nil {
			return 0, err
		}
		ps, err := pauli.Parse(label)
		if err != nil {
			return 0, err
		}
		if ps.NumQubits() != nq {
			return 0, fmt.Errorf("Pauli term %q has %d qubits, circuit has %d", part, ps.NumQubits(), nq)
		}
		terms = append(terms, ps.Scale(complex(coeff, 0)))
	}
	if len(terms) == 0 {
		return 0, fmt.Errorf("no valid Pauli terms found")
	}
	psum, err := pauli.NewPauliSum(terms)
	if err != nil {
		return 0, err
	}
	return real(pauli.ExpectSum(sv, psum)), nil
}

// parsePauliTerm parses "coeff*LABEL" or just "LABEL" (coeff defaults to 1).
func parsePauliTerm(s string) (float64, string, error) {
	s = strings.TrimSpace(s)
	if idx := strings.LastIndex(s, "*"); idx >= 0 {
		coeffStr := strings.TrimSpace(s[:idx])
		label := strings.TrimSpace(s[idx+1:])
		coeff, err := strconv.ParseFloat(coeffStr, 64)
		if err != nil {
			return 0, "", fmt.Errorf("invalid coefficient %q: %w", coeffStr, err)
		}
		return coeff, label, nil
	}
	return 1.0, s, nil
}

// Helper: linspace

func linspace(start, stop float64, count int) []float64 {
	if count <= 0 {
		return nil
	}
	if count == 1 {
		return []float64{start}
	}
	vals := make([]float64, count)
	step := (stop - start) / float64(count-1)
	for i := range count {
		vals[i] = start + float64(i)*step
	}
	return vals
}

// Helper: parse and evolve a QASM circuit, returning the statevector

func evolveQASM(qasm string) ([]complex128, *ir.Circuit, int, error) {
	circ, err := parser.ParseString(qasm)
	if err != nil {
		return nil, nil, 0, err
	}
	nq := circ.NumQubits()
	if nq > 16 {
		return nil, nil, 0, fmt.Errorf("expectation value limited to 16 qubits")
	}
	sim := statevector.New(nq)
	defer sim.Close()
	if err := sim.Evolve(circ); err != nil {
		return nil, nil, 0, err
	}
	return sim.StateVector(), circ, nq, nil
}

// computeExpectationJS
// Args: (qasm string, pauliStr string, dark? bool)

func computeExpectationJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshalExpectation(expectationResult{Error: "usage: computeExpectation(qasm, pauliStr, dark?)"})
	}
	qasm := args[0].String()
	pauliStr := args[1].String()
	dark := len(args) >= 3 && args[2].Truthy()

	sv, circ, nq, err := evolveQASM(qasm)
	if err != nil {
		return marshalExpectation(expectationResult{Error: err.Error()})
	}

	var drawOpts []draw.SVGOption
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
	}

	exp, err := computePauliExpect(sv, pauliStr, nq)
	if err != nil {
		return marshalExpectation(expectationResult{Error: err.Error()})
	}

	return marshalExpectation(expectationResult{
		CircuitSVG:  draw.SVG(circ, drawOpts...),
		Expectation: exp,
	})
}

// sweepExpectationJS
// Args: (qasmTemplate string, pauliStr string, paramName string,
//
//	start float64, stop float64, count int, dark? bool)
//

func sweepExpectationJS(_ js.Value, args []js.Value) any {
	if len(args) < 6 {
		return marshalSweep(sweepResult{Error: "usage: sweepExpectation(qasmTemplate, pauliStr, paramName, start, stop, count, dark?)"})
	}
	qasmTemplate := args[0].String()
	pauliStr := args[1].String()
	paramName := args[2].String()
	start := args[3].Float()
	stop := args[4].Float()
	count := args[5].Int()
	if count < 1 {
		count = 1
	}
	if count > 200 {
		count = 200
	}

	paramValues := linspace(start, stop, count)
	expectations := make([]float64, count)

	placeholder := "{" + paramName + "}"
	for i, pv := range paramValues {
		pvStr := strconv.FormatFloat(pv, 'g', -1, 64)
		qasm := strings.ReplaceAll(qasmTemplate, placeholder, pvStr)

		sv, _, nq, err := evolveQASM(qasm)
		if err != nil {
			return marshalSweep(sweepResult{Error: fmt.Sprintf("at param=%v: %s", pv, err.Error())})
		}
		exp, err := computePauliExpect(sv, pauliStr, nq)
		if err != nil {
			return marshalSweep(sweepResult{Error: fmt.Sprintf("at param=%v: %s", pv, err.Error())})
		}
		expectations[i] = exp
	}

	return marshalSweep(sweepResult{
		ParamValues:  paramValues,
		Expectations: expectations,
	})
}

// sweep2DJS
// Args: (qasmTemplate string, pauliStr string,
//
//	param1Name string, p1Start float64, p1Stop float64,
//	param2Name string, p2Start float64, p2Stop float64,
//	gridSize int, dark? bool)
//

func sweep2DJS(_ js.Value, args []js.Value) any {
	if len(args) < 9 {
		return marshalSweep2D(sweep2DResult{Error: "usage: sweep2D(qasmTemplate, pauliStr, param1Name, p1Start, p1Stop, param2Name, p2Start, p2Stop, gridSize, dark?)"})
	}
	qasmTemplate := args[0].String()
	pauliStr := args[1].String()
	param1Name := args[2].String()
	p1Start := args[3].Float()
	p1Stop := args[4].Float()
	param2Name := args[5].String()
	p2Start := args[6].Float()
	p2Stop := args[7].Float()
	gridSize := args[8].Int()
	if gridSize < 2 {
		gridSize = 2
	}
	if gridSize > 25 {
		gridSize = 25
	}

	p1Values := linspace(p1Start, p1Stop, gridSize)
	p2Values := linspace(p2Start, p2Stop, gridSize)

	energies := make([][]float64, gridSize)
	minEnergy := math.Inf(1)
	minP1 := p1Start
	minP2 := p2Start

	ph1 := "{" + param1Name + "}"
	ph2 := "{" + param2Name + "}"

	for i, p1 := range p1Values {
		energies[i] = make([]float64, gridSize)
		for j, p2 := range p2Values {
			p1Str := strconv.FormatFloat(p1, 'g', -1, 64)
			p2Str := strconv.FormatFloat(p2, 'g', -1, 64)
			qasm := strings.ReplaceAll(qasmTemplate, ph1, p1Str)
			qasm = strings.ReplaceAll(qasm, ph2, p2Str)

			sv, _, nq, err := evolveQASM(qasm)
			if err != nil {
				return marshalSweep2D(sweep2DResult{Error: fmt.Sprintf("at (%v, %v): %s", p1, p2, err.Error())})
			}
			exp, err := computePauliExpect(sv, pauliStr, nq)
			if err != nil {
				return marshalSweep2D(sweep2DResult{Error: fmt.Sprintf("at (%v, %v): %s", p1, p2, err.Error())})
			}
			energies[i][j] = exp
			if exp < minEnergy {
				minEnergy = exp
				minP1 = p1
				minP2 = p2
			}
		}
	}

	return marshalSweep2D(sweep2DResult{
		Param1Values: p1Values,
		Param2Values: p2Values,
		Energies:     energies,
		MinEnergy:    minEnergy,
		MinParam1:    minP1,
		MinParam2:    minP2,
	})
}

// computeGradientJS
// Args: (qasmTemplate string, pauliStr string, paramNames string,
//
//	paramValues string, method string, dark? bool)
//
// paramNames: comma-separated parameter names
// paramValues: comma-separated float64 values
// method: "parameter_shift" or "finite_difference"

func computeGradientJS(_ js.Value, args []js.Value) any {
	if len(args) < 5 {
		return marshalGradient(gradientResult{Error: "usage: computeGradient(qasmTemplate, pauliStr, paramNames, paramValues, method, dark?)"})
	}
	qasmTemplate := args[0].String()
	pauliStr := args[1].String()
	paramNamesStr := args[2].String()
	paramValuesStr := args[3].String()
	method := args[4].String()

	// Parse comma-separated parameter names.
	names := splitCSV(paramNamesStr)
	if len(names) == 0 {
		return marshalGradient(gradientResult{Error: "no parameter names provided"})
	}

	// Parse comma-separated parameter values.
	valStrs := splitCSV(paramValuesStr)
	if len(valStrs) != len(names) {
		return marshalGradient(gradientResult{Error: fmt.Sprintf("got %d names but %d values", len(names), len(valStrs))})
	}
	values := make([]float64, len(valStrs))
	for i, vs := range valStrs {
		v, err := strconv.ParseFloat(vs, 64)
		if err != nil {
			return marshalGradient(gradientResult{Error: fmt.Sprintf("invalid param value %q: %s", vs, err.Error())})
		}
		values[i] = v
	}

	// Build a cost function that substitutes all params and computes expectation.
	costFn := func(x []float64) (float64, error) {
		qasm := qasmTemplate
		for k, name := range names {
			ph := "{" + name + "}"
			qasm = strings.ReplaceAll(qasm, ph, strconv.FormatFloat(x[k], 'g', -1, 64))
		}
		sv, _, nq, err := evolveQASM(qasm)
		if err != nil {
			return 0, err
		}
		return computePauliExpect(sv, pauliStr, nq)
	}

	gradients := make([]float64, len(names))

	switch method {
	case "parameter_shift":
		// Parameter shift rule: df/dx_i = [f(x + pi/2 * e_i) - f(x - pi/2 * e_i)] / 2
		shift := 0.5 * math.Pi
		xp := make([]float64, len(values))
		xm := make([]float64, len(values))
		for i := range names {
			copy(xp, values)
			copy(xm, values)
			xp[i] += shift
			xm[i] -= shift
			fp, err := costFn(xp)
			if err != nil {
				return marshalGradient(gradientResult{Error: fmt.Sprintf("parameter_shift +shift for %s: %s", names[i], err.Error())})
			}
			fm, err := costFn(xm)
			if err != nil {
				return marshalGradient(gradientResult{Error: fmt.Sprintf("parameter_shift -shift for %s: %s", names[i], err.Error())})
			}
			gradients[i] = (fp - fm) / 2.0
		}

	default: // "finite_difference" or any other value
		h := 1e-5
		xp := make([]float64, len(values))
		xm := make([]float64, len(values))
		for i := range names {
			copy(xp, values)
			copy(xm, values)
			xp[i] += h
			xm[i] -= h
			fp, err := costFn(xp)
			if err != nil {
				return marshalGradient(gradientResult{Error: fmt.Sprintf("finite_diff +h for %s: %s", names[i], err.Error())})
			}
			fm, err := costFn(xm)
			if err != nil {
				return marshalGradient(gradientResult{Error: fmt.Sprintf("finite_diff -h for %s: %s", names[i], err.Error())})
			}
			gradients[i] = (fp - fm) / (2.0 * h)
		}
	}

	return marshalGradient(gradientResult{Gradients: gradients})
}

// splitCSV splits a comma-separated string into trimmed non-empty parts.
func splitCSV(s string) []string {
	raw := strings.Split(s, ",")
	var out []string
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}
