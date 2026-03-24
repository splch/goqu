package param

import (
	"fmt"

	"github.com/splch/goqu/internal/piformat"
)

// Expr represents a symbolic expression over parameters.
type Expr interface {
	// Eval evaluates the expression with the given bindings.
	// Returns an error if any required parameter is unbound.
	Eval(bindings map[string]float64) (float64, error)

	// Parameters returns all free parameters in the expression.
	Parameters() []*Parameter

	// String returns a human-readable representation.
	String() string

	// IsNumeric returns true if this expression has no free parameters.
	IsNumeric() bool
}

// Literal creates a constant numeric expression.
func Literal(v float64) Expr {
	return &literal{v: v}
}

// Add returns a + b.
func Add(a, b Expr) Expr { return &binOp{op: '+', left: a, right: b} }

// Sub returns a - b.
func Sub(a, b Expr) Expr { return &binOp{op: '-', left: a, right: b} }

// Mul returns a * b.
func Mul(a, b Expr) Expr { return &binOp{op: '*', left: a, right: b} }

// Div returns a / b.
func Div(a, b Expr) Expr { return &binOp{op: '/', left: a, right: b} }

// Neg returns -a.
func Neg(a Expr) Expr { return &negExpr{inner: a} }

// literal

type literal struct{ v float64 }

func (l *literal) Eval(_ map[string]float64) (float64, error) { return l.v, nil }
func (l *literal) Parameters() []*Parameter                   { return nil }
func (l *literal) IsNumeric() bool                            { return true }

func (l *literal) String() string {
	return piformat.FormatUnicode(l.v)
}

// paramRef

type paramRef struct{ param *Parameter }

func (p *paramRef) Eval(bindings map[string]float64) (float64, error) {
	name := p.param.Name()
	v, ok := bindings[name]
	if !ok {
		return 0, fmt.Errorf("param: unbound parameter %q", name)
	}
	return v, nil
}

func (p *paramRef) Parameters() []*Parameter { return []*Parameter{p.param} }
func (p *paramRef) IsNumeric() bool          { return false }
func (p *paramRef) String() string           { return p.param.Name() }

// binOp

type binOp struct {
	op    byte // '+', '-', '*', '/'
	left  Expr
	right Expr
}

func (b *binOp) Eval(bindings map[string]float64) (float64, error) {
	l, err := b.left.Eval(bindings)
	if err != nil {
		return 0, err
	}
	r, err := b.right.Eval(bindings)
	if err != nil {
		return 0, err
	}
	switch b.op {
	case '+':
		return l + r, nil
	case '-':
		return l - r, nil
	case '*':
		return l * r, nil
	case '/':
		if r == 0 {
			return 0, fmt.Errorf("param: division by zero")
		}
		return l / r, nil
	}
	return 0, fmt.Errorf("param: unknown op %c", b.op)
}

func (b *binOp) Parameters() []*Parameter {
	return mergeParams(b.left.Parameters(), b.right.Parameters())
}

func (b *binOp) IsNumeric() bool {
	return b.left.IsNumeric() && b.right.IsNumeric()
}

func (b *binOp) String() string {
	return fmt.Sprintf("(%s %c %s)", b.left.String(), b.op, b.right.String())
}

// negExpr

type negExpr struct{ inner Expr }

func (n *negExpr) Eval(bindings map[string]float64) (float64, error) {
	v, err := n.inner.Eval(bindings)
	if err != nil {
		return 0, err
	}
	return -v, nil
}

func (n *negExpr) Parameters() []*Parameter { return n.inner.Parameters() }
func (n *negExpr) IsNumeric() bool          { return n.inner.IsNumeric() }
func (n *negExpr) String() string           { return fmt.Sprintf("(-%s)", n.inner.String()) }

// helpers

// mergeParams deduplicates parameters by pointer identity.
func mergeParams(a, b []*Parameter) []*Parameter {
	seen := make(map[*Parameter]bool, len(a)+len(b))
	var result []*Parameter
	for _, p := range a {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	for _, p := range b {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}
