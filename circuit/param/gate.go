package param

import (
	"fmt"
	"strings"

	"github.com/splch/qgo/circuit/gate"
)

// Ensure symbolicGate satisfies gate.Bindable.
var _ gate.Bindable = (*symbolicGate)(nil)

// symbolicGate wraps a gate constructor with symbolic expressions.
type symbolicGate struct {
	baseName    string // e.g., "RX", "RZ", "U3"
	nQubits     int
	exprs       []Expr
	constructor func(params []float64) gate.Gate
}

func (g *symbolicGate) Name() string {
	parts := make([]string, len(g.exprs))
	for i, e := range g.exprs {
		parts[i] = e.String()
	}
	return fmt.Sprintf("%s({%s})", g.baseName, strings.Join(parts, ","))
}

func (g *symbolicGate) Qubits() int { return g.nQubits }

func (g *symbolicGate) Matrix() []complex128 {
	panic(fmt.Sprintf("param: cannot compute matrix for unbound symbolic gate %s; call Bind() first", g.Name()))
}

func (g *symbolicGate) Params() []float64 { return nil }

func (g *symbolicGate) Inverse() gate.Gate {
	// Negate all expressions.
	negExprs := make([]Expr, len(g.exprs))
	for i, e := range g.exprs {
		negExprs[i] = Neg(e)
	}
	return &symbolicGate{
		baseName:    g.baseName + "†",
		nQubits:     g.nQubits,
		exprs:       negExprs,
		constructor: g.constructor,
	}
}

func (g *symbolicGate) Decompose(_ []int) []gate.Applied { return nil }

// Bind substitutes parameters and returns a concrete gate.
func (g *symbolicGate) Bind(bindings map[string]float64) (gate.Gate, error) {
	vals := make([]float64, len(g.exprs))
	for i, e := range g.exprs {
		v, err := e.Eval(bindings)
		if err != nil {
			return nil, fmt.Errorf("param: binding gate %s: %w", g.baseName, err)
		}
		vals[i] = v
	}
	return g.constructor(vals), nil
}

// FreeParameters returns names of all free parameters.
func (g *symbolicGate) FreeParameters() []string {
	seen := make(map[string]bool)
	var names []string
	for _, e := range g.exprs {
		for _, p := range e.Parameters() {
			n := p.Name()
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	return names
}

// IsBound returns true if all expressions are numeric (no free params).
func (g *symbolicGate) IsBound() bool {
	for _, e := range g.exprs {
		if !e.IsNumeric() {
			return false
		}
	}
	return true
}

// SymRX creates a symbolic RX gate.
func SymRX(theta Expr) gate.Gate {
	return &symbolicGate{
		baseName: "RX",
		nQubits:  1,
		exprs:    []Expr{theta},
		constructor: func(p []float64) gate.Gate {
			return gate.RX(p[0])
		},
	}
}

// SymRY creates a symbolic RY gate.
func SymRY(theta Expr) gate.Gate {
	return &symbolicGate{
		baseName: "RY",
		nQubits:  1,
		exprs:    []Expr{theta},
		constructor: func(p []float64) gate.Gate {
			return gate.RY(p[0])
		},
	}
}

// SymRZ creates a symbolic RZ gate.
func SymRZ(theta Expr) gate.Gate {
	return &symbolicGate{
		baseName: "RZ",
		nQubits:  1,
		exprs:    []Expr{theta},
		constructor: func(p []float64) gate.Gate {
			return gate.RZ(p[0])
		},
	}
}

// SymPhase creates a symbolic Phase gate.
func SymPhase(phi Expr) gate.Gate {
	return &symbolicGate{
		baseName: "P",
		nQubits:  1,
		exprs:    []Expr{phi},
		constructor: func(p []float64) gate.Gate {
			return gate.Phase(p[0])
		},
	}
}

// SymU3 creates a symbolic U3 gate.
func SymU3(theta, phi, lambda Expr) gate.Gate {
	return &symbolicGate{
		baseName: "U3",
		nQubits:  1,
		exprs:    []Expr{theta, phi, lambda},
		constructor: func(p []float64) gate.Gate {
			return gate.U3(p[0], p[1], p[2])
		},
	}
}

// SymCP creates a symbolic controlled-phase gate.
func SymCP(phi Expr) gate.Gate {
	return &symbolicGate{
		baseName: "CP",
		nQubits:  2,
		exprs:    []Expr{phi},
		constructor: func(p []float64) gate.Gate {
			return gate.CP(p[0])
		},
	}
}

// SymRXX creates a symbolic Ising XX gate.
func SymRXX(theta Expr) gate.Gate {
	return &symbolicGate{
		baseName: "RXX",
		nQubits:  2,
		exprs:    []Expr{theta},
		constructor: func(p []float64) gate.Gate {
			return gate.RXX(p[0])
		},
	}
}

// SymRYY creates a symbolic Ising YY gate.
func SymRYY(theta Expr) gate.Gate {
	return &symbolicGate{
		baseName: "RYY",
		nQubits:  2,
		exprs:    []Expr{theta},
		constructor: func(p []float64) gate.Gate {
			return gate.RYY(p[0])
		},
	}
}

// SymRZZ creates a symbolic Ising ZZ gate.
func SymRZZ(theta Expr) gate.Gate {
	return &symbolicGate{
		baseName: "RZZ",
		nQubits:  2,
		exprs:    []Expr{theta},
		constructor: func(p []float64) gate.Gate {
			return gate.RZZ(p[0])
		},
	}
}
