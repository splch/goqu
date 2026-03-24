//go:build js && wasm

package main

import (
	"context"
	"strings"
	"syscall/js"

	"github.com/splch/goqu/algorithm/walk"
)

// walkResult holds the quantum walk output.
type walkResult struct {
	Classical []float64
	Quantum   []float64
	Error     string
}

// quantumWalkJS runs a discrete-time quantum walk for the given number of steps.
// Args: (steps int)
// Returns JSON with classical and quantum probability distributions.
func quantumWalkJS(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return marshalWalk(walkResult{Error: "usage: quantumWalk(steps)"})
	}
	steps := args[0].Int()
	steps = max(steps, 1)
	if steps > 1000 {
		return marshalWalk(walkResult{Error: "steps must be <= 1000"})
	}

	res, err := walk.Run(context.Background(), walk.Config{Steps: steps})
	if err != nil {
		return marshalWalk(walkResult{Error: err.Error()})
	}
	return marshalWalk(walkResult{
		Classical: res.Classical,
		Quantum:   res.Quantum,
	})
}

// marshalWalk marshals the walk result to JSON.
func marshalWalk(r walkResult) string {
	var b strings.Builder
	b.WriteByte('{')
	n := 0

	if r.Error != "" {
		jsonKey(&b, "error", &n)
		jsonStr(&b, r.Error)
		b.WriteByte('}')
		return b.String()
	}

	jsonKey(&b, "classical", &n)
	jsonFloats(&b, r.Classical)

	jsonKey(&b, "quantum", &n)
	jsonFloats(&b, r.Quantum)

	b.WriteByte('}')
	return b.String()
}
