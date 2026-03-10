package ibm

import (
	"fmt"
	"strings"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/qasm/emitter"
)

// serializeCircuit converts a circuit IR to an OpenQASM 3.0 string
// suitable for IBM Quantum Runtime submission.
func serializeCircuit(c *ir.Circuit) (string, error) {
	qasm, err := emitter.EmitString(c)
	if err != nil {
		return "", fmt.Errorf("ibm: emit QASM: %w", err)
	}
	return qasm, nil
}

// parseResults converts IBM Quantum sampler results to a backend.Result.
// It handles the classical register sample format returned by Sampler V2.
func parseResults(resp ibmResultResponse, numQubits, shots int) (*backend.Result, error) {
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("ibm: no results in response")
	}

	// Aggregate counts from all samples across classical registers.
	counts := make(map[string]int)
	totalSamples := 0

	pub := resp.Results[0]

	// If classical register samples are present, use them.
	if len(pub.Data.CRegSamples) > 0 {
		for _, samples := range pub.Data.CRegSamples {
			for _, sample := range samples {
				bs := sampleToBitstring(sample, numQubits)
				counts[bs]++
				totalSamples++
			}
		}
	}

	if totalSamples == 0 {
		// No samples found; return empty result.
		return &backend.Result{
			Counts: counts,
			Shots:  shots,
		}, nil
	}

	return &backend.Result{
		Counts: counts,
		Shots:  totalSamples,
	}, nil
}

// sampleToBitstring converts a sample (array of bit values) to a bitstring.
// Each element in the sample is 0 or 1. The result is MSB-first.
func sampleToBitstring(sample []int, numQubits int) string {
	n := numQubits
	if len(sample) < n {
		n = len(sample)
	}
	var sb strings.Builder
	sb.Grow(numQubits)
	// Pad with leading zeros if sample is shorter than numQubits.
	for i := 0; i < numQubits-len(sample); i++ {
		sb.WriteByte('0')
	}
	for i := 0; i < n; i++ {
		if sample[i] != 0 {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}
