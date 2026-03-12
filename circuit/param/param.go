// Package param provides symbolic parameters and expressions for variational circuits.
package param

import "fmt"

// Parameter is a named symbolic parameter.
type Parameter struct {
	name  string
	index int     // index within vector, or -1 for standalone
	vec   *Vector // parent vector, or nil for standalone
}

// New creates a standalone Parameter.
func New(name string) *Parameter {
	return &Parameter{name: name, index: -1}
}

// Name returns the parameter name.
func (p *Parameter) Name() string {
	if p.vec != nil {
		return fmt.Sprintf("%s[%d]", p.vec.name, p.index)
	}
	return p.name
}

// Expr returns this parameter as an expression.
func (p *Parameter) Expr() Expr {
	return &paramRef{param: p}
}

// Vector is a named collection of related parameters.
type Vector struct {
	name   string
	params []*Parameter
}

// NewVector creates a parameter vector of the given size.
func NewVector(name string, size int) *Vector {
	v := &Vector{name: name, params: make([]*Parameter, size)}
	for i := range size {
		v.params[i] = &Parameter{
			name:  fmt.Sprintf("%s[%d]", name, i),
			index: i,
			vec:   v,
		}
	}
	return v
}

// Name returns the vector name.
func (v *Vector) Name() string { return v.name }

// Size returns the number of parameters.
func (v *Vector) Size() int { return len(v.params) }

// At returns the i-th parameter.
func (v *Vector) At(i int) *Parameter {
	if i < 0 || i >= len(v.params) {
		panic(fmt.Sprintf("param.Vector.At: index %d out of range [0, %d)", i, len(v.params)))
	}
	return v.params[i]
}
