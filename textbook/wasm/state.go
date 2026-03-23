//go:build js && wasm

package main

import (
	"math"
	"math/cmplx"
	"strings"
	"syscall/js"

	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/qasm/parser"
	"github.com/splch/goqu/sim/statevector"
	"github.com/splch/goqu/viz"
)

type complexNum struct {
	Re float64
	Im float64
}

type blochVector struct {
	X float64
	Y float64
	Z float64
}

type stateResult struct {
	Amplitudes    []complexNum
	Probabilities []float64
	BlochVectors  []blochVector
	Error         string
}

type probResult struct {
	Probabilities []float64
	Labels        []string
	Histogram     string
	Circuit       string
	Error         string
}

// getStateVectorJS returns the full statevector after circuit evolution (no measurement).
func getStateVectorJS(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalState(stateResult{Error: "usage: getStateVector(qasm)"})
	}
	qasm := args[0].String()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalState(stateResult{Error: err.Error()})
	}

	nq := circ.NumQubits()
	sim := statevector.New(nq)
	defer sim.Close()

	if err := sim.Evolve(circ); err != nil {
		return marshalState(stateResult{Error: err.Error()})
	}

	sv := sim.StateVector()
	r := stateResult{
		Amplitudes:    make([]complexNum, len(sv)),
		Probabilities: make([]float64, len(sv)),
	}

	for i, amp := range sv {
		r.Amplitudes[i] = complexNum{Re: real(amp), Im: imag(amp)}
		r.Probabilities[i] = real(amp)*real(amp) + imag(amp)*imag(amp)
	}

	// Compute per-qubit Bloch vectors via partial trace for small systems.
	if nq <= 6 {
		r.BlochVectors = make([]blochVector, nq)
		for q := range nq {
			bx, by, bz := qubitBlochCoords(sv, nq, q)
			r.BlochVectors[q] = blochVector{X: bx, Y: by, Z: bz}
		}
	}

	return marshalState(r)
}

// getProbabilitiesJS returns exact probabilities and a histogram SVG.
func getProbabilitiesJS(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalProb(probResult{Error: "usage: getProbabilities(qasm, dark?)"})
	}
	qasm := args[0].String()
	dark := len(args) >= 2 && args[1].Truthy()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalProb(probResult{Error: err.Error()})
	}

	var drawOpts []draw.SVGOption
	var vizOpts []viz.Option
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
		vizOpts = append(vizOpts, viz.WithStyle(viz.DarkStyle()))
	}

	nq := circ.NumQubits()
	sim := statevector.New(nq)
	defer sim.Close()

	if err := sim.Evolve(circ); err != nil {
		return marshalProb(probResult{Error: err.Error()})
	}

	sv := sim.StateVector()
	probs := make([]float64, len(sv))
	labels := make([]string, len(sv))
	probMap := make(map[string]float64)

	for i, amp := range sv {
		p := real(amp)*real(amp) + imag(amp)*imag(amp)
		probs[i] = p
		label := formatBinary(i, nq)
		labels[i] = label
		if p > 1e-10 {
			probMap[label] = p
		}
	}

	r := probResult{
		Probabilities: probs,
		Labels:        labels,
		Histogram:     viz.HistogramProb(probMap, vizOpts...),
		Circuit:       draw.SVG(circ, drawOpts...),
	}

	return marshalProb(r)
}

// qubitBlochCoords computes the Bloch vector for qubit q in a multi-qubit statevector
// by tracing out all other qubits.
func qubitBlochCoords(sv []complex128, nq, q int) (x, y, z float64) {
	dim := 1 << nq
	var rho00, rho11 float64
	var rho01 complex128

	for i := range dim {
		bit := (i >> (nq - 1 - q)) & 1
		paired := i ^ (1 << (nq - 1 - q))
		amp := sv[i]
		prob := real(amp)*real(amp) + imag(amp)*imag(amp)

		if bit == 0 {
			rho00 += prob
			rho01 += amp * cmplx.Conj(sv[paired])
		} else {
			rho11 += prob
		}
	}

	x = 2 * real(rho01)
	y = -2 * imag(rho01)
	z = rho00 - rho11
	return
}

// formatBinary returns i as an nq-bit binary string.
func formatBinary(i, nq int) string {
	s := make([]byte, nq)
	for b := range nq {
		if (i>>(nq-1-b))&1 == 1 {
			s[b] = '1'
		} else {
			s[b] = '0'
		}
	}
	return string(s)
}

// Entanglement entropy

// entropyResult holds the entanglement entropy output.
type entropyResult struct {
	Entropy             float64
	SchmidtCoefficients []float64
	SchmidtRank         int
	Error               string
}

// entanglementEntropyJS computes the von Neumann entropy of a subsystem.
// Args: (qasm string, qubitIndices []int)
// qubitIndices is a JS array of qubit indices defining subsystem A.
func entanglementEntropyJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshalEntropy(entropyResult{Error: "usage: entanglementEntropy(qasm, qubitIndices)"})
	}
	qasm := args[0].String()
	jsArr := args[1]
	if jsArr.Type() != js.TypeObject || jsArr.Length() == 0 {
		return marshalEntropy(entropyResult{Error: "qubitIndices must be a non-empty array"})
	}

	nA := jsArr.Length()
	qubitIndices := make([]int, nA)
	for i := range nA {
		qubitIndices[i] = jsArr.Index(i).Int()
	}

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalEntropy(entropyResult{Error: err.Error()})
	}

	nq := circ.NumQubits()
	if nq > 8 {
		return marshalEntropy(entropyResult{Error: "circuit exceeds 8-qubit limit for entropy computation"})
	}

	// Validate qubit indices: bounds and duplicates.
	aSet := make(map[int]bool, nA)
	for _, q := range qubitIndices {
		if q < 0 || q >= nq {
			return marshalEntropy(entropyResult{Error: "qubit index out of range [0, nq)"})
		}
		if aSet[q] {
			return marshalEntropy(entropyResult{Error: "duplicate qubit index"})
		}
		aSet[q] = true
	}

	sim := statevector.New(nq)
	defer sim.Close()

	if err := sim.Evolve(circ); err != nil {
		return marshalEntropy(entropyResult{Error: err.Error()})
	}

	sv := sim.StateVector()
	dimA := 1 << nA
	dimB := 1 << (nq - nA)

	// Build ordered lists: A qubits and B qubits (complement).
	aQubits := qubitIndices
	bQubits := make([]int, 0, nq-nA)
	for q := range nq {
		if !aSet[q] {
			bQubits = append(bQubits, q)
		}
	}

	// Reshape the statevector into a dimA x dimB matrix.
	// psi[iA][iB] where iA indexes over A-qubit basis states and iB indexes over B-qubit basis states.
	psi := make([]complex128, dimA*dimB)
	dim := 1 << nq
	for idx := range dim {
		// Extract the A-index and B-index from the full index.
		iA := 0
		for a, q := range aQubits {
			bit := (idx >> (nq - 1 - q)) & 1
			iA |= bit << (nA - 1 - a)
		}
		iB := 0
		for b, q := range bQubits {
			bit := (idx >> (nq - 1 - q)) & 1
			iB |= bit << (len(bQubits) - 1 - b)
		}
		psi[iA*dimB+iB] = sv[idx]
	}

	// Compute reduced density matrix rhoA = psi * psi-dagger (tracing out B).
	// rhoA[i][j] = sum_k psi[i][k] * conj(psi[j][k])
	rhoA := make([]complex128, dimA*dimA)
	for i := range dimA {
		for j := range dimA {
			var sum complex128
			for k := range dimB {
				sum += psi[i*dimB+k] * cmplx.Conj(psi[j*dimB+k])
			}
			rhoA[i*dimA+j] = sum
		}
	}

	// Eigendecompose rhoA to get Schmidt coefficients (eigenvalues).
	eigenvalues := hermitianEigvals(rhoA, dimA)

	// Compute von Neumann entropy: S = -sum(p * log2(p)) for p > 0.
	entropy := 0.0
	var schmidtCoeffs []float64
	schmidtRank := 0
	for _, ev := range eigenvalues {
		if ev > 1e-14 {
			schmidtCoeffs = append(schmidtCoeffs, math.Sqrt(ev))
			schmidtRank++
			entropy -= ev * math.Log2(ev)
		}
	}

	return marshalEntropy(entropyResult{
		Entropy:             entropy,
		SchmidtCoefficients: schmidtCoeffs,
		SchmidtRank:         schmidtRank,
	})
}

// marshalEntropy marshals the entropy result to JSON.
func marshalEntropy(r entropyResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0

	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
		b.WriteByte('}')
		return b.String()
	}

	jsonKey(&b, "entropy", &n)
	jsonFloat(&b, r.Entropy)

	jsonKey(&b, "schmidtCoefficients", &n)
	jsonFloats(&b, r.SchmidtCoefficients)

	jsonKey(&b, "schmidtRank", &n)
	jsonInt(&b, r.SchmidtRank)

	b.WriteByte('}')
	return b.String()
}

// Partial trace

// partialTraceResult holds the partial trace output.
type partialTraceResult struct {
	DensityMatrix [][2]float64 // flat row-major {re, im}
	Dim           int
	Purity        float64
	Error         string
}

// partialTraceJS computes the partial trace over specified qubits.
// Args: (qasm string, traceOutQubits []int)
func partialTraceJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshalPartialTrace(partialTraceResult{Error: "usage: partialTrace(qasm, traceOutQubits)"})
	}
	qasm := args[0].String()
	jsArr := args[1]
	if jsArr.Type() != js.TypeObject || jsArr.Length() == 0 {
		return marshalPartialTrace(partialTraceResult{Error: "traceOutQubits must be a non-empty array"})
	}

	nTrace := jsArr.Length()
	traceQubits := make([]int, nTrace)
	for i := range nTrace {
		traceQubits[i] = jsArr.Index(i).Int()
	}

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalPartialTrace(partialTraceResult{Error: err.Error()})
	}

	nq := circ.NumQubits()
	if nq > 8 {
		return marshalPartialTrace(partialTraceResult{Error: "circuit exceeds 8-qubit limit for partial trace"})
	}

	// Validate: must keep at least one qubit.
	if nTrace >= nq {
		return marshalPartialTrace(partialTraceResult{Error: "must keep at least one qubit after tracing"})
	}

	// Validate qubit indices: bounds and duplicates.
	traceSet := make(map[int]bool, nTrace)
	for _, q := range traceQubits {
		if q < 0 || q >= nq {
			return marshalPartialTrace(partialTraceResult{Error: "qubit index out of range [0, nq)"})
		}
		if traceSet[q] {
			return marshalPartialTrace(partialTraceResult{Error: "duplicate qubit index"})
		}
		traceSet[q] = true
	}

	sim := statevector.New(nq)
	defer sim.Close()

	if err := sim.Evolve(circ); err != nil {
		return marshalPartialTrace(partialTraceResult{Error: err.Error()})
	}

	sv := sim.StateVector()

	// Kept qubits (complement of traceQubits).
	nKept := nq - nTrace
	keptQubits := make([]int, 0, nKept)
	for q := range nq {
		if !traceSet[q] {
			keptQubits = append(keptQubits, q)
		}
	}

	dimKept := 1 << nKept
	dimTraced := 1 << nTrace
	dim := 1 << nq

	// Reshape statevector into a dimKept x dimTraced matrix.
	psi := make([]complex128, dimKept*dimTraced)
	for idx := range dim {
		iK := 0
		for k, q := range keptQubits {
			bit := (idx >> (nq - 1 - q)) & 1
			iK |= bit << (nKept - 1 - k)
		}
		iT := 0
		for t, q := range traceQubits {
			bit := (idx >> (nq - 1 - q)) & 1
			iT |= bit << (nTrace - 1 - t)
		}
		psi[iK*dimTraced+iT] = sv[idx]
	}

	// Compute rho_kept = Tr_traced(|psi><psi|) via matrix multiply.
	rhoKept := make([]complex128, dimKept*dimKept)
	for i := range dimKept {
		for j := range dimKept {
			var sum complex128
			for k := range dimTraced {
				sum += psi[i*dimTraced+k] * cmplx.Conj(psi[j*dimTraced+k])
			}
			rhoKept[i*dimKept+j] = sum
		}
	}

	// Compute purity Tr(rho^2).
	var purity float64
	for i := range dimKept {
		for j := range dimKept {
			v := rhoKept[i*dimKept+j]
			// Tr(rho^2) = sum_ij |rho_ij|^2 for Hermitian rho.
			purity += real(v)*real(v) + imag(v)*imag(v)
		}
	}

	// Convert to output format.
	dmFlat := make([][2]float64, dimKept*dimKept)
	for i, c := range rhoKept {
		dmFlat[i] = [2]float64{real(c), imag(c)}
	}

	return marshalPartialTrace(partialTraceResult{
		DensityMatrix: dmFlat,
		Dim:           dimKept,
		Purity:        purity,
	})
}

// marshalPartialTrace marshals the partial trace result to JSON.
func marshalPartialTrace(r partialTraceResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0

	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
		b.WriteByte('}')
		return b.String()
	}

	jsonKey(&b, "densityMatrix", &n)
	b.WriteByte('[')
	for i, c := range r.DensityMatrix {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"re":`)
		jsonFloat(&b, c[0])
		b.WriteString(`,"im":`)
		jsonFloat(&b, c[1])
		b.WriteByte('}')
	}
	b.WriteByte(']')

	jsonKey(&b, "dim", &n)
	jsonInt(&b, r.Dim)

	jsonKey(&b, "purity", &n)
	jsonFloat(&b, r.Purity)

	b.WriteByte('}')
	return b.String()
}

// Marginal distribution

// marginalResult holds the marginal distribution output.
type marginalResult struct {
	Probabilities []float64
	Bloch         blochVector
	Error         string
}

// marginalDistributionJS computes the marginal probability distribution
// and Bloch vector for a single qubit after circuit evolution.
// Args: (qasm string, qubitIndex int)
func marginalDistributionJS(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return marshalMarginal(marginalResult{Error: "usage: marginalDistribution(qasm, qubitIndex)"})
	}
	qasm := args[0].String()
	qubitIndex := args[1].Int()

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalMarginal(marginalResult{Error: err.Error()})
	}

	nq := circ.NumQubits()
	if qubitIndex < 0 || qubitIndex >= nq {
		return marshalMarginal(marginalResult{Error: "qubitIndex out of range"})
	}

	sim := statevector.New(nq)
	defer sim.Close()

	if err := sim.Evolve(circ); err != nil {
		return marshalMarginal(marginalResult{Error: err.Error()})
	}

	sv := sim.StateVector()

	// Compute marginal probabilities for the specified qubit.
	var p0, p1 float64
	dim := 1 << nq
	for i := range dim {
		prob := real(sv[i])*real(sv[i]) + imag(sv[i])*imag(sv[i])
		bit := (i >> (nq - 1 - qubitIndex)) & 1
		if bit == 0 {
			p0 += prob
		} else {
			p1 += prob
		}
	}

	// Compute Bloch vector for this qubit.
	bx, by, bz := qubitBlochCoords(sv, nq, qubitIndex)

	return marshalMarginal(marginalResult{
		Probabilities: []float64{p0, p1},
		Bloch:         blochVector{X: bx, Y: by, Z: bz},
	})
}

// marshalMarginal marshals the marginal distribution result to JSON.
func marshalMarginal(r marginalResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0

	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
		b.WriteByte('}')
		return b.String()
	}

	jsonKey(&b, "probabilities", &n)
	jsonFloats(&b, r.Probabilities)

	jsonKey(&b, "bloch", &n)
	b.WriteString(`{"x":`)
	jsonFloat(&b, r.Bloch.X)
	b.WriteString(`,"y":`)
	jsonFloat(&b, r.Bloch.Y)
	b.WriteString(`,"z":`)
	jsonFloat(&b, r.Bloch.Z)
	b.WriteByte('}')

	b.WriteByte('}')
	return b.String()
}

// Hermitian eigenvalue decomposition (minimal, for entropy computation)

// hermitianEigvals computes eigenvalues of an n x n Hermitian matrix
// using Jacobi iteration. Returns eigenvalues in arbitrary order.
// This is a lightweight implementation for small matrices (WASM context).
func hermitianEigvals(m []complex128, n int) []float64 {
	if n <= 1 {
		eigenvalues := make([]float64, n)
		for i := range n {
			eigenvalues[i] = real(m[i*n+i])
		}
		return eigenvalues
	}

	const tol = 1e-12
	const maxIter = 500

	a := make([]complex128, n*n)
	copy(a, m)

	for range maxIter {
		// Find largest off-diagonal element.
		maxVal := 0.0
		p, q := 0, 1
		for i := range n {
			for j := i + 1; j < n; j++ {
				val := cmplx.Abs(a[i*n+j])
				if val > maxVal {
					maxVal = val
					p = i
					q = j
				}
			}
		}
		if maxVal < tol {
			break
		}

		// Jacobi rotation to zero out a[p,q].
		app := real(a[p*n+p])
		aqq := real(a[q*n+q])
		apq := a[p*n+q]

		absApq := cmplx.Abs(apq)
		if absApq < tol {
			continue
		}
		phase := apq / complex(absApq, 0)

		diff := app - aqq
		var t float64
		if math.Abs(diff) < tol {
			t = 1.0
		} else {
			tau := diff / (2 * absApq)
			t = 1.0 / (math.Abs(tau) + math.Sqrt(1+tau*tau))
			if tau < 0 {
				t = -t
			}
		}

		c := 1.0 / math.Sqrt(1+t*t)
		s := t * c
		cc := complex(c, 0)
		sPhase := complex(s, 0) * phase
		sPhaseConj := cmplx.Conj(sPhase)

		for j := range n {
			if j == p || j == q {
				continue
			}
			ajp := a[j*n+p]
			ajq := a[j*n+q]
			a[j*n+p] = cc*ajp + sPhaseConj*ajq
			a[j*n+q] = -sPhase*ajp + cc*ajq
			a[p*n+j] = cmplx.Conj(a[j*n+p])
			a[q*n+j] = cmplx.Conj(a[j*n+q])
		}

		a[p*n+p] = complex(c*c*app+2*c*s*absApq+s*s*aqq, 0)
		a[q*n+q] = complex(s*s*app-2*c*s*absApq+c*c*aqq, 0)
		a[p*n+q] = 0
		a[q*n+p] = 0
	}

	eigenvalues := make([]float64, n)
	for i := range n {
		eigenvalues[i] = real(a[i*n+i])
	}
	return eigenvalues
}
