// Package ir defines the circuit intermediate representation.
package ir

import (
	"fmt"

	"github.com/splch/goqu/circuit/gate"
)

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

func (c *Circuit) Name() string   { return c.name }
func (c *Circuit) NumQubits() int { return c.numQubits }
func (c *Circuit) NumClbits() int { return c.numClbits }
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

// Condition represents classical control flow (single-bit equality).
type Condition struct {
	Clbit    int    // classical bit index (authoritative for simulation)
	Value    int    // expected value (0 or 1)
	Register string // QASM register name (for emitter round-trip only)
}

// Stats returns circuit statistics.
func (c *Circuit) Stats() Stats {
	s := Stats{GateCount: len(c.ops)}
	for _, op := range c.ops {
		if op.Gate == nil && len(op.Clbits) > 0 {
			s.Measurements++
		}
		if op.Gate != nil {
			if op.Gate.Qubits() >= 2 {
				s.TwoQubitGates++
			}
			if len(op.Gate.Params()) > 0 {
				s.Params += len(op.Gate.Params())
			}
			switch op.Gate.Name() {
			case "reset":
				s.Resets++
			case "delay":
				s.Delays++
			}
		}
		if op.Condition != nil {
			s.ConditionalGates++
		}
	}
	s.Depth = c.depth()
	s.Dynamic = c.IsDynamic()
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

// Bind substitutes symbolic parameters with concrete values, returning a new Circuit.
// Gates implementing gate.Bindable are bound; all others are copied as-is.
// Returns an error if any symbolic gate has unbound parameters.
func Bind(c *Circuit, bindings map[string]float64) (*Circuit, error) {
	ops := c.Ops()
	result := make([]Operation, len(ops))
	for i, op := range ops {
		if op.Gate == nil {
			result[i] = op
			continue
		}
		if b, ok := op.Gate.(gate.Bindable); ok {
			bound, err := b.Bind(bindings)
			if err != nil {
				return nil, fmt.Errorf("ir.Bind: op %d: %w", i, err)
			}
			result[i] = Operation{
				Gate:      bound,
				Qubits:    op.Qubits,
				Clbits:    op.Clbits,
				Condition: op.Condition,
			}
		} else {
			result[i] = op
		}
	}
	return New(c.Name(), c.NumQubits(), c.NumClbits(), result, c.Metadata()), nil
}

// FreeParameters returns the names of all unbound symbolic parameters in the circuit.
func FreeParameters(c *Circuit) []string {
	seen := make(map[string]bool)
	var names []string
	for _, op := range c.Ops() {
		if op.Gate == nil {
			continue
		}
		if b, ok := op.Gate.(gate.Bindable); ok {
			for _, name := range b.FreeParameters() {
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}
	return names
}

// Stats holds circuit statistics.
type Stats struct {
	Depth            int
	GateCount        int
	TwoQubitGates    int
	Params           int
	Measurements     int
	Resets           int
	Delays           int
	ConditionalGates int
	Dynamic          bool
}

// IsDynamic returns true if the circuit contains mid-circuit measurements,
// conditioned gates, or reset operations.
func (c *Circuit) IsDynamic() bool {
	lastGateIdx := -1
	for i := len(c.ops) - 1; i >= 0; i-- {
		if c.ops[i].Gate != nil && c.ops[i].Gate.Name() != "barrier" && c.ops[i].Gate.Name() != "reset" {
			lastGateIdx = i
			break
		}
	}
	for i, op := range c.ops {
		if op.Condition != nil {
			return true
		}
		if op.Gate != nil && op.Gate.Name() == "reset" {
			return true
		}
		// Measurement before the last gate = mid-circuit measurement.
		if op.Gate == nil && len(op.Clbits) > 0 && i < lastGateIdx {
			return true
		}
	}
	return false
}
