// Package sweep provides parameter sweep types for variational quantum circuits.
package sweep

// Sweep defines a set of parameter bindings to evaluate a circuit over.
type Sweep interface {
	// Len returns the number of parameter points in the sweep.
	Len() int

	// Params returns the parameter names covered by this sweep.
	Params() []string

	// Resolve expands the sweep into concrete binding maps.
	Resolve() []map[string]float64
}

// Linspace sweeps a single parameter over evenly-spaced values from Start to Stop (inclusive).
type Linspace struct {
	Key   string
	Start float64
	Stop  float64
	Count int
}

func (l Linspace) Len() int { return max(l.Count, 0) }

func (l Linspace) Params() []string { return []string{l.Key} }

func (l Linspace) Resolve() []map[string]float64 {
	n := l.Len()
	if n == 0 {
		return nil
	}
	out := make([]map[string]float64, n)
	if n == 1 {
		out[0] = map[string]float64{l.Key: l.Start}
		return out
	}
	step := (l.Stop - l.Start) / float64(n-1)
	for i := range n {
		out[i] = map[string]float64{l.Key: l.Start + float64(i)*step}
	}
	return out
}

// Points holds explicit values for a single parameter.
type Points struct {
	key    string
	values []float64
}

// NewPoints creates a Points sweep with a defensive copy of the values slice.
func NewPoints(key string, values []float64) Points {
	cp := make([]float64, len(values))
	copy(cp, values)
	return Points{key: key, values: cp}
}

func (p Points) Len() int { return len(p.values) }

func (p Points) Params() []string { return []string{p.key} }

func (p Points) Resolve() []map[string]float64 {
	if len(p.values) == 0 {
		return nil
	}
	out := make([]map[string]float64, len(p.values))
	for i, v := range p.values {
		out[i] = map[string]float64{p.key: v}
	}
	return out
}

// UnitSweep is a single-point sweep with fixed bindings.
type UnitSweep struct {
	bindings map[string]float64
}

// Single creates a UnitSweep with a defensive copy of the bindings map.
func Single(bindings map[string]float64) UnitSweep {
	cp := make(map[string]float64, len(bindings))
	for k, v := range bindings {
		cp[k] = v
	}
	return UnitSweep{bindings: cp}
}

func (u UnitSweep) Len() int { return 1 }

func (u UnitSweep) Params() []string {
	keys := make([]string, 0, len(u.bindings))
	for k := range u.bindings {
		keys = append(keys, k)
	}
	return keys
}

func (u UnitSweep) Resolve() []map[string]float64 {
	cp := make(map[string]float64, len(u.bindings))
	for k, v := range u.bindings {
		cp[k] = v
	}
	return []map[string]float64{cp}
}
