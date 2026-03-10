package braket

import (
	"encoding/json"
	"fmt"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/circuit/ir"
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
