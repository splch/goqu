package braket

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/qasm/emitter"
)

// braketProgram is the Braket OpenQASM IR schema wrapper.
type braketProgram struct {
	Header braketHeader `json:"braketSchemaHeader"`
	Source string       `json:"source"`
}

// braketHeader identifies the schema for the Braket action payload.
type braketHeader struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// serializeCircuit converts a circuit IR to a Braket action JSON string.
func serializeCircuit(c *ir.Circuit) (string, error) {
	qasm, err := emitter.EmitString(c)
	if err != nil {
		return "", fmt.Errorf("braket: emit qasm: %w", err)
	}
	prog := braketProgram{
		Header: braketHeader{
			Name:    "braket.ir.openqasm.program",
			Version: "1",
		},
		Source: qasm,
	}
	data, err := json.Marshal(prog)
	if err != nil {
		return "", fmt.Errorf("braket: marshal program: %w", err)
	}
	return string(data), nil
}

// braketResults is the structure of the results.json file stored in S3.
type braketResults struct {
	Counts         map[string]int     `json:"measurementCounts"`
	Probabilities  map[string]float64 `json:"measurementProbabilities"`
	MeasuredQubits []int              `json:"measuredQubits"`
}

// serializePulseProgram converts a pulse Program to OpenQASM 3.0 with
// OpenPulse cal {} block, wrapped in the Braket IR JSON schema.
func serializePulseProgram(p *pulse.Program) (string, error) {
	if p == nil {
		return "", fmt.Errorf("braket: nil pulse program")
	}

	var sb strings.Builder
	sb.WriteString("OPENQASM 3.0;\n")

	// Port declarations.
	for _, port := range p.Ports() {
		fmt.Fprintf(&sb, "extern port %s;\n", port.Name())
	}

	sb.WriteString("cal {\n")

	// Frame declarations.
	for _, f := range p.Frames() {
		fmt.Fprintf(&sb, "  frame %s = newframe(%s, %g, %g);\n",
			f.Name(), f.Port().Name(), f.Frequency(), f.Phase())
	}

	// Instructions.
	for _, inst := range p.Instructions() {
		switch v := inst.(type) {
		case pulse.Play:
			fmt.Fprintf(&sb, "  play(%s, %s);\n", v.Frame.Name(), v.Waveform.Name())
		case pulse.Delay:
			fmt.Fprintf(&sb, "  delay[%gs] %s;\n", v.Duration, v.Frame.Name())
		case pulse.SetPhase:
			fmt.Fprintf(&sb, "  set_phase(%s, %g);\n", v.Frame.Name(), v.Phase)
		case pulse.ShiftPhase:
			fmt.Fprintf(&sb, "  shift_phase(%s, %g);\n", v.Frame.Name(), v.Delta)
		case pulse.SetFrequency:
			fmt.Fprintf(&sb, "  set_frequency(%s, %g);\n", v.Frame.Name(), v.Frequency)
		case pulse.ShiftFrequency:
			fmt.Fprintf(&sb, "  shift_frequency(%s, %g);\n", v.Frame.Name(), v.Delta)
		case pulse.Barrier:
			names := make([]string, len(v.Frames))
			for i, f := range v.Frames {
				names[i] = f.Name()
			}
			fmt.Fprintf(&sb, "  barrier %s;\n", strings.Join(names, ", "))
		case pulse.Capture:
			fmt.Fprintf(&sb, "  capture_v0(%s, %gs);\n", v.Frame.Name(), v.Duration)
		}
	}

	sb.WriteString("}\n")

	prog := braketProgram{
		Header: braketHeader{
			Name:    "braket.ir.openqasm.program",
			Version: "1",
		},
		Source: sb.String(),
	}
	data, err := json.Marshal(prog)
	if err != nil {
		return "", fmt.Errorf("braket: marshal pulse program: %w", err)
	}
	return string(data), nil
}

// parseResults converts Braket S3 result data into a backend.Result.
func parseResults(data []byte, shots int) (*backend.Result, error) {
	var br braketResults
	if err := json.Unmarshal(data, &br); err != nil {
		return nil, fmt.Errorf("braket: parse results: %w", err)
	}
	return &backend.Result{
		Counts:        br.Counts,
		Probabilities: br.Probabilities,
		Shots:         shots,
	}, nil
}
