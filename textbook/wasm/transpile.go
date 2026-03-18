//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"

	"github.com/splch/goqu/circuit/draw"
	"github.com/splch/goqu/qasm/parser"
	qasmemitter "github.com/splch/goqu/qasm/emitter"
	quilemitter "github.com/splch/goqu/quil/emitter"
	"github.com/splch/goqu/transpile/pipeline"
	"github.com/splch/goqu/transpile/target"
)

// transpileStats holds gate/depth statistics for before/after comparison.
type transpileStats struct {
	Gates        int
	TwoQubitGates int
	Depth        int
}

// transpileResult holds the full output from transpileQASMJS.
type transpileResult struct {
	BeforeCircuitSVG string
	AfterCircuitSVG  string
	TranspiledQASM   string
	TranspiledQuil   string
	BeforeStats      transpileStats
	AfterStats       transpileStats
	Error            string
}

// targetInfoResult holds target metadata from getTargetInfoJS.
type targetInfoResult struct {
	Name       string
	NumQubits  int
	BasisGates []string
	Edges      [][2]int
	Error      string
}

// lookupTarget maps a JS target name string to a predefined target.Target.
func lookupTarget(name string) (target.Target, error) {
	switch name {
	case "ibm_brisbane":
		return target.IBMBrisbane, nil
	case "ibm_sherbrooke":
		return target.IBMSherbrooke, nil
	case "ibm_eagle":
		return target.IBMEagle, nil
	case "ionq_aria":
		return target.IonQAria, nil
	case "ionq_forte":
		return target.IonQForte, nil
	case "quantinuum_h1":
		return target.QuantinuumH1, nil
	case "quantinuum_h2":
		return target.QuantinuumH2, nil
	case "google_sycamore":
		return target.GoogleSycamore, nil
	case "google_willow":
		return target.GoogleWillow, nil
	case "rigetti_ankaa":
		return target.RigettiAnkaa, nil
	case "simulator":
		return target.Simulator, nil
	default:
		return target.Target{}, fmt.Errorf("unknown target: %s", name)
	}
}

// transpileQASMJS transpiles a QASM circuit to a hardware target.
// Args: (qasm string, targetName string, level int, dark? bool)
func transpileQASMJS(_ js.Value, args []js.Value) any {
	if len(args) < 3 {
		return marshalTranspile(transpileResult{Error: "usage: transpileQASM(qasm, targetName, level, dark?)"})
	}
	qasm := args[0].String()
	targetName := args[1].String()
	level := args[2].Int()
	dark := len(args) >= 4 && args[3].Truthy()

	// Cap level at 2 (Level 3 has no benefit in single-threaded WASM).
	if level > 2 {
		level = 2
	}
	if level < 0 {
		level = 0
	}

	circ, err := parser.ParseString(qasm)
	if err != nil {
		return marshalTranspile(transpileResult{Error: err.Error()})
	}

	tgt, err := lookupTarget(targetName)
	if err != nil {
		return marshalTranspile(transpileResult{Error: err.Error()})
	}

	// Collect before stats.
	before := circ.Stats()
	beforeStats := transpileStats{
		Gates:        before.GateCount,
		TwoQubitGates: before.TwoQubitGates,
		Depth:        before.Depth,
	}

	// Draw options for SVG rendering.
	var drawOpts []draw.SVGOption
	if dark {
		drawOpts = append(drawOpts, draw.WithStyle(draw.DarkStyle()))
	}

	r := transpileResult{
		BeforeCircuitSVG: draw.SVG(circ, drawOpts...),
		BeforeStats:      beforeStats,
	}

	// Run the transpilation pipeline.
	result, err := pipeline.DefaultPipeline(pipeline.Level(level))(circ, tgt)
	if err != nil {
		r.Error = err.Error()
		return marshalTranspile(r)
	}

	// Collect after stats.
	after := result.Stats()
	r.AfterStats = transpileStats{
		Gates:        after.GateCount,
		TwoQubitGates: after.TwoQubitGates,
		Depth:        after.Depth,
	}

	r.AfterCircuitSVG = draw.SVG(result, drawOpts...)

	// Emit transpiled QASM.
	qasmStr, err := qasmemitter.EmitString(result)
	if err != nil {
		r.Error = fmt.Sprintf("qasm emit: %v", err)
		return marshalTranspile(r)
	}
	r.TranspiledQASM = qasmStr

	// Emit transpiled Quil.
	quilStr, err := quilemitter.EmitString(result)
	if err != nil {
		// Quil emission may fail for some gate sets; include partial result.
		r.TranspiledQuil = fmt.Sprintf("// Quil emission error: %v", err)
	} else {
		r.TranspiledQuil = quilStr
	}

	return marshalTranspile(r)
}

// getTargetInfoJS returns metadata for a named hardware target.
// Args: (targetName string)
func getTargetInfoJS(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalTargetInfo(targetInfoResult{Error: "usage: getTargetInfo(targetName)"})
	}
	targetName := args[0].String()

	tgt, err := lookupTarget(targetName)
	if err != nil {
		return marshalTargetInfo(targetInfoResult{Error: err.Error()})
	}

	r := targetInfoResult{
		Name:       tgt.Name,
		NumQubits:  tgt.NumQubits,
		BasisGates: tgt.BasisGates,
	}

	// Convert connectivity pairs to [2]int edges.
	if tgt.Connectivity != nil {
		r.Edges = make([][2]int, len(tgt.Connectivity))
		for i, p := range tgt.Connectivity {
			r.Edges[i] = [2]int{p.Q0, p.Q1}
		}
	}

	return marshalTargetInfo(r)
}
