// Package transpile provides the quantum circuit transpilation framework.
package transpile

import (
	"strings"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/transpile/target"
)

// Pass transforms a circuit for a given target.
type Pass func(c *ir.Circuit, t target.Target) (*ir.Circuit, error)

// Pipeline composes passes into a single sequential pass.
func Pipeline(passes ...Pass) Pass {
	return func(c *ir.Circuit, t target.Target) (*ir.Circuit, error) {
		var err error
		for _, p := range passes {
			c, err = p(c, t)
			if err != nil {
				return nil, err
			}
		}
		return c, nil
	}
}

// BasisName returns the canonical basis gate name for a gate.
// Strips parameters: "RZ(0.78)" -> "RZ", "CNOT" -> "CX".
func BasisName(g gate.Gate) string {
	name := g.Name()

	// Strip dagger suffix for inverse gates.
	name = strings.TrimSuffix(name, "†")

	// Strip parameter parenthetical.
	if idx := strings.Index(name, "("); idx >= 0 {
		name = name[:idx]
	}

	// Canonical aliases.
	switch name {
	case "CNOT":
		return "CX"
	default:
		return name
	}
}
