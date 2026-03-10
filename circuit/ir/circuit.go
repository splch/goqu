// Package ir defines the circuit intermediate representation.
package ir

import "github.com/splch/qgo/circuit/gate"

// Circuit is an immutable sequence of quantum operations with metadata.
type Circuit struct {
	name      string
	numQubits int
	numClbits int
	ops       []Operation
	metadata  map[string]string
}

// New creates a Circuit directly. Prefer using the builder package.
func New(name string, numQubits, numClbits int, ops []Operation, metadata map[string]string) *Circuit {
	// Copy ops to ensure immutability.
	copied := make([]Operation, len(ops))
	copy(copied, ops)
	var md map[string]string
	if metadata != nil {
		md = make(map[string]string, len(metadata))
		for k, v := range metadata {
			md[k] = v
		}
	}
	return &Circuit{
		name:      name,
		numQubits: numQubits,
		numClbits: numClbits,
		ops:       copied,
		metadata:  md,
	}
}

func (c *Circuit) Name() string              { return c.name }
func (c *Circuit) NumQubits() int             { return c.numQubits }
func (c *Circuit) NumClbits() int             { return c.numClbits }
func (c *Circuit) Ops() []Operation {
	out := make([]Operation, len(c.ops))
	copy(out, c.ops)
	return out
}
func (c *Circuit) Metadata() map[string]string {
	if c.metadata == nil {
		return nil
	}
	out := make(map[string]string, len(c.metadata))
	for k, v := range c.metadata {
		out[k] = v
	}
	return out
}

// Operation is a single step in a circuit.
type Operation struct {
	Gate      gate.Gate
	Qubits    []int      // qubit indices
	Clbits    []int      // classical bit indices (for measurement)
	Condition *Condition // optional classical conditioning
}

// Condition represents classical control flow.
type Condition struct {
	Register string
	Value    int
}

// Stats returns circuit statistics.
func (c *Circuit) Stats() Stats {
	s := Stats{GateCount: len(c.ops)}
	for _, op := range c.ops {
		if op.Gate != nil && op.Gate.Qubits() >= 2 {
			s.TwoQubitGates++
		}
		if op.Gate != nil && len(op.Gate.Params()) > 0 {
			s.Params += len(op.Gate.Params())
		}
	}
	s.Depth = c.depth()
	return s
}

// depth computes circuit depth by tracking the latest time step per qubit.
func (c *Circuit) depth() int {
	if len(c.ops) == 0 {
		return 0
	}
	layers := make([]int, c.numQubits)
	maxDepth := 0
	for _, op := range c.ops {
		// Find the maximum layer among this operation's qubits.
		opLayer := 0
		for _, q := range op.Qubits {
			if q < len(layers) && layers[q] > opLayer {
				opLayer = layers[q]
			}
		}
		// This operation goes in the next layer.
		opLayer++
		for _, q := range op.Qubits {
			if q < len(layers) {
				layers[q] = opLayer
			}
		}
		if opLayer > maxDepth {
			maxDepth = opLayer
		}
	}
	return maxDepth
}

// Stats holds circuit statistics.
type Stats struct {
	Depth        int
	GateCount    int
	TwoQubitGates int
	Params       int
}
