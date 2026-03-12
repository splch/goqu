// Package parser implements a hand-written recursive descent parser for OpenQASM 3.0.
package parser

import (
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/splch/qgo/circuit/gate"
	"github.com/splch/qgo/circuit/ir"
	"github.com/splch/qgo/qasm/token"
)

// Option configures parser behavior.
type Option func(*config)

type config struct {
	strict bool
}

// WithStrictMode rejects unknown gate names instead of treating them as opaque.
func WithStrictMode(strict bool) Option {
	return func(c *config) { c.strict = strict }
}

// Parse reads OpenQASM 3.0 source and returns a Circuit.
func Parse(source io.Reader, opts ...Option) (*ir.Circuit, error) {
	data, err := io.ReadAll(source)
	if err != nil {
		return nil, fmt.Errorf("qasm: read error: %w", err)
	}
	return ParseString(string(data), opts...)
}

// ParseString parses OpenQASM 3.0 source from a string.
func ParseString(source string, opts ...Option) (*ir.Circuit, error) {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	tokens := newLexer(source).tokenize()
	p := &parser{tokens: tokens, cfg: cfg, gates: builtinGates()}
	return p.parseProgram()
}

// register tracks a named qubit or classical register's offset and size.
type register struct {
	start int
	size  int
}

type parser struct {
	tokens []token.Token
	pos    int
	cfg    *config

	// Circuit state accumulated during parsing.
	name      string
	numQubits int
	numClbits int
	ops       []ir.Operation
	metadata  map[string]string

	// Named registers: name → register.
	qregs map[string]register
	cregs map[string]register

	// Gate definitions: name → gatedef.
	gates map[string]*gatedef
}

type gatedef struct {
	params []string // parameter names
	qubits []string // qubit parameter names
	body   []token.Token
}

func (p *parser) cur() token.Token {
	if p.pos >= len(p.tokens) {
		return token.Token{Type: token.EOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) peek() token.Type {
	return p.cur().Type
}

func (p *parser) advance() token.Token {
	t := p.cur()
	p.pos++
	return t
}

func (p *parser) expect(typ token.Type) (token.Token, error) {
	t := p.cur()
	if t.Type != typ {
		return t, fmt.Errorf("line %d:%d: expected %s, got %s (%q)", t.Line, t.Col, typ, t.Type, t.Literal)
	}
	p.pos++
	return t, nil
}

func (p *parser) parseProgram() (*ir.Circuit, error) {
	p.qregs = make(map[string]register)
	p.cregs = make(map[string]register)
	p.metadata = make(map[string]string)

	// Optional version header.
	if p.peek() == token.OPENQASM {
		if err := p.parseVersion(); err != nil {
			return nil, err
		}
	}

	for p.peek() != token.EOF {
		if err := p.parseStatement(); err != nil {
			return nil, err
		}
	}

	return ir.New(p.name, p.numQubits, p.numClbits, p.ops, p.metadata), nil
}

func (p *parser) parseVersion() error {
	p.advance() // consume OPENQASM
	t := p.advance()
	if t.Type != token.INT && t.Type != token.FLOAT {
		return fmt.Errorf("line %d:%d: expected version number, got %s", t.Line, t.Col, t.Type)
	}
	p.metadata["openqasm_version"] = t.Literal
	_, err := p.expect(token.SEMICOLON)
	return err
}

func (p *parser) parseStatement() error {
	switch p.peek() {
	case token.INCLUDE:
		return p.parseInclude()
	case token.GATE:
		return p.parseGateDecl()
	case token.QUBIT:
		return p.parseQubitDecl()
	case token.BIT:
		return p.parseBitDecl()
	case token.QREG:
		return p.parseQregDecl()
	case token.CREG:
		return p.parseCregDecl()
	case token.MEASURE:
		return p.parseMeasure()
	case token.RESET:
		return p.parseReset()
	case token.BARRIER:
		return p.parseBarrier()
	case token.IF:
		return p.parseIf()
	case token.IDENT:
		// Check if it's an assignment "ident = measure ..." or "ident[i] = measure ..."
		if p.isAssignment() {
			return p.parseAssignment()
		}
		return p.parseGateCall()
	case token.U:
		return p.parseGateCall()
	case token.CTRL, token.NEGCTRL, token.INV, token.POW:
		return p.parseGateCall()
	case token.GPHASE:
		return p.parseGateCall()
	default:
		t := p.cur()
		return fmt.Errorf("line %d:%d: unexpected token %s (%q)", t.Line, t.Col, t.Type, t.Literal)
	}
}

func (p *parser) parseInclude() error {
	p.advance() // consume 'include'
	t, err := p.expect(token.STRING)
	if err != nil {
		return err
	}
	_, err = p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}
	// stdgates.inc is handled by preloading builtin gates.
	if t.Literal == "stdgates.inc" {
		return nil
	}
	// For other includes, we'd need file I/O. Skip for now.
	return nil
}

func (p *parser) parseQubitDecl() error {
	p.advance() // consume 'qubit'
	size := 1
	if p.peek() == token.LBRACKET {
		var err error
		size, err = p.parseDesignator()
		if err != nil {
			return err
		}
	}
	name, err := p.expect(token.IDENT)
	if err != nil {
		return err
	}
	_, err = p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}
	p.qregs[name.Literal] = register{start: p.numQubits, size: size}
	p.numQubits += size
	return nil
}

func (p *parser) parseBitDecl() error {
	p.advance() // consume 'bit'
	size := 1
	if p.peek() == token.LBRACKET {
		var err error
		size, err = p.parseDesignator()
		if err != nil {
			return err
		}
	}
	name, err := p.expect(token.IDENT)
	if err != nil {
		return err
	}
	_, err = p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}
	p.cregs[name.Literal] = register{start: p.numClbits, size: size}
	p.numClbits += size
	return nil
}

func (p *parser) parseQregDecl() error {
	p.advance() // consume 'qreg'
	name, err := p.expect(token.IDENT)
	if err != nil {
		return err
	}
	size, err := p.parseDesignator()
	if err != nil {
		return err
	}
	_, err = p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}
	p.qregs[name.Literal] = register{start: p.numQubits, size: size}
	p.numQubits += size
	return nil
}

func (p *parser) parseCregDecl() error {
	p.advance() // consume 'creg'
	name, err := p.expect(token.IDENT)
	if err != nil {
		return err
	}
	size, err := p.parseDesignator()
	if err != nil {
		return err
	}
	_, err = p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}
	p.cregs[name.Literal] = register{start: p.numClbits, size: size}
	p.numClbits += size
	return nil
}

func (p *parser) parseDesignator() (int, error) {
	_, err := p.expect(token.LBRACKET)
	if err != nil {
		return 0, err
	}
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	_, err = p.expect(token.RBRACKET)
	if err != nil {
		return 0, err
	}
	n := int(v)
	if float64(n) != v || n <= 0 {
		return 0, fmt.Errorf("designator must be a positive integer, got %v", v)
	}
	return n, nil
}

func (p *parser) parseGateDecl() error {
	p.advance() // consume 'gate'
	name, err := p.expect(token.IDENT)
	if err != nil {
		return err
	}

	var params []string
	if p.peek() == token.LPAREN {
		p.advance() // consume (
		for p.peek() != token.RPAREN && p.peek() != token.EOF {
			t, err := p.expect(token.IDENT)
			if err != nil {
				return err
			}
			params = append(params, t.Literal)
			if p.peek() == token.COMMA {
				p.advance()
			}
		}
		_, err = p.expect(token.RPAREN)
		if err != nil {
			return err
		}
	}

	var qubits []string
	for p.peek() != token.LBRACE && p.peek() != token.EOF {
		t, err := p.expect(token.IDENT)
		if err != nil {
			return err
		}
		qubits = append(qubits, t.Literal)
		if p.peek() == token.COMMA {
			p.advance()
		}
	}

	// Collect body tokens.
	_, err = p.expect(token.LBRACE)
	if err != nil {
		return err
	}
	depth := 1
	bodyStart := p.pos
	for depth > 0 && p.peek() != token.EOF {
		switch p.peek() {
		case token.LBRACE:
			depth++
		case token.RBRACE:
			depth--
		}
		if depth > 0 {
			p.advance()
		}
	}
	bodyTokens := make([]token.Token, p.pos-bodyStart)
	copy(bodyTokens, p.tokens[bodyStart:p.pos])
	_, err = p.expect(token.RBRACE)
	if err != nil {
		return err
	}

	p.gates[name.Literal] = &gatedef{
		params: params,
		qubits: qubits,
		body:   bodyTokens,
	}
	return nil
}

func (p *parser) parseMeasure() error {
	t := p.advance() // consume 'measure'
	qubits, err := p.parseQubitArgs()
	if err != nil {
		return err
	}

	if p.peek() == token.ARROW {
		p.advance() // consume ->
		clbits, err := p.parseClbitArgs()
		if err != nil {
			return err
		}
		_, err = p.expect(token.SEMICOLON)
		if err != nil {
			return err
		}
		if len(qubits) != len(clbits) {
			return fmt.Errorf("line %d:%d: measure qubit/clbit count mismatch", t.Line, t.Col)
		}
		for i := range qubits {
			p.ops = append(p.ops, ir.Operation{Qubits: []int{qubits[i]}, Clbits: []int{clbits[i]}})
		}
		return nil
	}

	_, err = p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}
	// Implicit classical bits: each qubit maps to the same-index classical bit.
	for _, q := range qubits {
		if q >= p.numClbits {
			return fmt.Errorf("line %d:%d: implicit measurement on q[%d] requires c[%d] but only %d classical bits declared",
				t.Line, t.Col, q, q, p.numClbits)
		}
		p.ops = append(p.ops, ir.Operation{Qubits: []int{q}, Clbits: []int{q}})
	}
	return nil
}

// isAssignment performs lookahead to check if current statement is "ident = measure" or "ident[i] = measure".
func (p *parser) isAssignment() bool {
	i := p.pos
	if i >= len(p.tokens) || p.tokens[i].Type != token.IDENT {
		return false
	}
	i++ // skip ident
	// Optional index: [expr]
	if i < len(p.tokens) && p.tokens[i].Type == token.LBRACKET {
		depth := 1
		i++
		for i < len(p.tokens) && depth > 0 {
			switch p.tokens[i].Type {
			case token.LBRACKET:
				depth++
			case token.RBRACKET:
				depth--
			}
			i++
		}
	}
	return i < len(p.tokens) && p.tokens[i].Type == token.EQUALS
}

func (p *parser) parseAssignment() error {
	// c = measure q; or c[0] = measure q[0];
	clbits, err := p.parseClbitArgs()
	if err != nil {
		return err
	}
	_, err = p.expect(token.EQUALS)
	if err != nil {
		return err
	}
	_, err = p.expect(token.MEASURE)
	if err != nil {
		return err
	}
	qubits, err := p.parseQubitArgs()
	if err != nil {
		return err
	}
	_, err = p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}
	if len(qubits) != len(clbits) {
		return fmt.Errorf("measure qubit/clbit count mismatch")
	}
	for i := range qubits {
		p.ops = append(p.ops, ir.Operation{Qubits: []int{qubits[i]}, Clbits: []int{clbits[i]}})
	}
	return nil
}

func (p *parser) parseReset() error {
	p.advance() // consume 'reset'
	qubits, err := p.parseQubitArgs()
	if err != nil {
		return err
	}
	_, err = p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}
	// Reset is represented using the shared gate.Reset pseudo-gate.
	for _, q := range qubits {
		p.ops = append(p.ops, ir.Operation{
			Gate:   gate.Reset,
			Qubits: []int{q},
		})
	}
	return nil
}

func (p *parser) parseBarrier() error {
	p.advance() // consume 'barrier'
	var qubits []int
	if p.peek() != token.SEMICOLON {
		var err error
		qubits, err = p.parseQubitArgs()
		if err != nil {
			return err
		}
	} else {
		// Barrier on all qubits.
		for i := range p.numQubits {
			qubits = append(qubits, i)
		}
	}
	_, err := p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}
	p.ops = append(p.ops, ir.Operation{
		Gate:   barrierGate{n: len(qubits)},
		Qubits: qubits,
	})
	return nil
}

func (p *parser) parseIf() error {
	p.advance() // consume 'if'
	_, err := p.expect(token.LPAREN)
	if err != nil {
		return err
	}

	// Parse condition: ident == value or ident[idx] == value.
	condReg, err := p.expect(token.IDENT)
	if err != nil {
		return err
	}
	regName := condReg.Literal

	// Optional index.
	bitIdx := -1
	if p.peek() == token.LBRACKET {
		p.advance() // consume [
		v, err2 := p.parseExpr()
		if err2 != nil {
			return err2
		}
		bitIdx = int(v)
		if _, err2 = p.expect(token.RBRACKET); err2 != nil {
			return err2
		}
	}

	_, err = p.expect(token.EQEQ)
	if err != nil {
		return err
	}
	val, err := p.parseExpr()
	if err != nil {
		return err
	}
	_, err = p.expect(token.RPAREN)
	if err != nil {
		return err
	}

	// Resolve classical bit index.
	clbit := 0
	reg, ok := p.cregs[regName]
	if ok {
		if bitIdx >= 0 {
			clbit = reg.start + bitIdx
		} else {
			clbit = reg.start
		}
	}

	cond := &ir.Condition{Clbit: clbit, Value: int(val), Register: regName}

	// Parse body — single statement or block.
	if p.peek() == token.LBRACE {
		p.advance() // consume {
		for p.peek() != token.RBRACE && p.peek() != token.EOF {
			opsBefore := len(p.ops)
			if err := p.parseStatement(); err != nil {
				return err
			}
			// Attach condition to newly added ops.
			for i := opsBefore; i < len(p.ops); i++ {
				p.ops[i].Condition = cond
			}
		}
		_, err = p.expect(token.RBRACE)
		return err
	}

	// Single statement.
	opsBefore := len(p.ops)
	if err := p.parseStatement(); err != nil {
		return err
	}
	for i := opsBefore; i < len(p.ops); i++ {
		p.ops[i].Condition = cond
	}
	return nil
}

func (p *parser) parseGateCall() error {
	// Handle gate modifiers: ctrl @, negctrl @, inv @, pow(k) @
	ctrlCount := 0
	negctrlCount := 0
	invCount := 0
	powK := 0
	hasPow := false
	// Track which control qubit positions (0-indexed from start of qubit list)
	// are negative controls, to sandwich with X gates.
	var negctrlPositions []int
	totalCtrlSoFar := 0
	for p.peek() == token.CTRL || p.peek() == token.NEGCTRL || p.peek() == token.INV || p.peek() == token.POW {
		modTok := p.advance() // consume modifier keyword
		switch modTok.Type {
		case token.CTRL:
			n := 1
			if p.peek() == token.LPAREN {
				p.advance()
				v, err := p.parseExpr()
				if err != nil {
					return err
				}
				n = int(v)
				_, err = p.expect(token.RPAREN)
				if err != nil {
					return err
				}
			}
			ctrlCount += n
			totalCtrlSoFar += n
		case token.NEGCTRL:
			n := 1
			if p.peek() == token.LPAREN {
				p.advance()
				v, err := p.parseExpr()
				if err != nil {
					return err
				}
				n = int(v)
				_, err = p.expect(token.RPAREN)
				if err != nil {
					return err
				}
			}
			for i := range n {
				negctrlPositions = append(negctrlPositions, totalCtrlSoFar+i)
			}
			negctrlCount += n
			totalCtrlSoFar += n
		case token.INV:
			invCount++
		case token.POW:
			if p.peek() != token.LPAREN {
				return fmt.Errorf("line %d: pow modifier requires parenthesized exponent", modTok.Line)
			}
			p.advance()
			v, err := p.parseExpr()
			if err != nil {
				return err
			}
			_, err = p.expect(token.RPAREN)
			if err != nil {
				return err
			}
			k := int(math.Round(v))
			if math.Abs(v-float64(k)) > 1e-10 {
				return fmt.Errorf("line %d: pow modifier requires integer exponent, got %v", modTok.Line, v)
			}
			hasPow = true
			powK = k
		}
		_, err := p.expect(token.AT)
		if err != nil {
			return err
		}
	}

	nameToken := p.advance()
	gateName := nameToken.Literal

	// Parse optional parameters.
	var params []float64
	if p.peek() == token.LPAREN {
		p.advance() // consume (
		for p.peek() != token.RPAREN && p.peek() != token.EOF {
			v, err := p.parseExpr()
			if err != nil {
				return err
			}
			params = append(params, v)
			if p.peek() == token.COMMA {
				p.advance()
			}
		}
		_, err := p.expect(token.RPAREN)
		if err != nil {
			return err
		}
	}

	// Parse qubit arguments.
	qubits, err := p.parseQubitArgs()
	if err != nil {
		return err
	}
	_, err = p.expect(token.SEMICOLON)
	if err != nil {
		return err
	}

	g, err := p.resolveGate(gateName, params)
	if err != nil {
		if p.cfg.strict {
			return err
		}
		// Non-strict: create an opaque gate.
		innerQubits := len(qubits) - ctrlCount
		if innerQubits < 1 {
			innerQubits = 1
		}
		g = opaqueGate{name: gateName, n: innerQubits, params: params}
	}

	// Apply pow modifier: raise gate to integer power.
	if hasPow {
		g = gate.Pow(g, powK)
	}

	// Apply ctrl + negctrl modifier: wrap with total control qubits.
	totalControls := ctrlCount + negctrlCount
	if totalControls > 0 {
		g = gate.Controlled(g, totalControls)
	}

	// Apply inv modifier: take inverse for each inv @ encountered.
	for range invCount {
		g = g.Inverse()
	}

	// Emit X gates on negctrl qubits before the operation.
	for _, pos := range negctrlPositions {
		p.ops = append(p.ops, ir.Operation{Gate: gate.X, Qubits: []int{qubits[pos]}})
	}

	p.ops = append(p.ops, ir.Operation{Gate: g, Qubits: qubits})

	// Emit X gates on negctrl qubits after the operation.
	for _, pos := range negctrlPositions {
		p.ops = append(p.ops, ir.Operation{Gate: gate.X, Qubits: []int{qubits[pos]}})
	}

	return nil
}

func (p *parser) resolveGate(name string, params []float64) (gate.Gate, error) {
	switch name {
	// Standard fixed gates.
	case "id", "I":
		return gate.I, nil
	case "h", "H":
		return gate.H, nil
	case "x", "X":
		return gate.X, nil
	case "y", "Y":
		return gate.Y, nil
	case "z", "Z":
		return gate.Z, nil
	case "s", "S":
		return gate.S, nil
	case "sdg":
		return gate.Sdg, nil
	case "t", "T":
		return gate.T, nil
	case "tdg":
		return gate.Tdg, nil
	case "sx":
		return gate.SX, nil
	case "cx", "CX", "cnot", "CNOT":
		return gate.CNOT, nil
	case "cz":
		return gate.CZ, nil
	case "cy":
		return gate.CY, nil
	case "swap":
		return gate.SWAP, nil
	case "ccx":
		return gate.CCX, nil
	case "cswap":
		return gate.CSWAP, nil

	// Parameterized gates.
	case "rx":
		if len(params) != 1 {
			return nil, fmt.Errorf("rx requires 1 parameter, got %d", len(params))
		}
		return gate.RX(params[0]), nil
	case "ry":
		if len(params) != 1 {
			return nil, fmt.Errorf("ry requires 1 parameter, got %d", len(params))
		}
		return gate.RY(params[0]), nil
	case "rz":
		if len(params) != 1 {
			return nil, fmt.Errorf("rz requires 1 parameter, got %d", len(params))
		}
		return gate.RZ(params[0]), nil
	case "p", "phase":
		if len(params) != 1 {
			return nil, fmt.Errorf("p/phase requires 1 parameter, got %d", len(params))
		}
		return gate.Phase(params[0]), nil
	case "U", "u3":
		if len(params) != 3 {
			return nil, fmt.Errorf("U/u3 requires 3 parameters, got %d", len(params))
		}
		return gate.U3(params[0], params[1], params[2]), nil
	case "u1":
		if len(params) != 1 {
			return nil, fmt.Errorf("u1 requires 1 parameter, got %d", len(params))
		}
		return gate.Phase(params[0]), nil
	case "cp", "cphase":
		if len(params) != 1 {
			return nil, fmt.Errorf("cp requires 1 parameter, got %d", len(params))
		}
		return gate.CP(params[0]), nil
	case "crx":
		if len(params) != 1 {
			return nil, fmt.Errorf("crx requires 1 parameter, got %d", len(params))
		}
		return gate.CRX(params[0]), nil
	case "cry":
		if len(params) != 1 {
			return nil, fmt.Errorf("cry requires 1 parameter, got %d", len(params))
		}
		return gate.CRY(params[0]), nil
	case "crz":
		if len(params) != 1 {
			return nil, fmt.Errorf("crz requires 1 parameter, got %d", len(params))
		}
		return gate.CRZ(params[0]), nil
	case "rxx":
		if len(params) != 1 {
			return nil, fmt.Errorf("rxx requires 1 parameter, got %d", len(params))
		}
		return gate.RXX(params[0]), nil
	case "ryy":
		if len(params) != 1 {
			return nil, fmt.Errorf("ryy requires 1 parameter, got %d", len(params))
		}
		return gate.RYY(params[0]), nil
	case "rzz":
		if len(params) != 1 {
			return nil, fmt.Errorf("rzz requires 1 parameter, got %d", len(params))
		}
		return gate.RZZ(params[0]), nil
	case "gphase":
		// Global phase — no qubits, just a parameter. Treat as identity for IR.
		return gate.I, nil
	}

	// Check user-defined gates.
	if gd, ok := p.gates[name]; ok {
		// User-defined gate — we don't expand it, just create an opaque reference.
		return opaqueGate{name: name, n: len(gd.qubits), params: params}, nil
	}

	return nil, fmt.Errorf("unknown gate: %s", name)
}

// parseQubitArgs parses a list of qubit arguments: q, q[0], q[0:2], etc.
func (p *parser) parseQubitArgs() ([]int, error) {
	var result []int
	for {
		qubits, err := p.parseSingleQubitArg()
		if err != nil {
			return nil, err
		}
		result = append(result, qubits...)
		if p.peek() != token.COMMA {
			break
		}
		p.advance() // consume comma
	}
	return result, nil
}

func (p *parser) parseSingleQubitArg() ([]int, error) {
	name, err := p.expect(token.IDENT)
	if err != nil {
		return nil, err
	}
	reg, ok := p.qregs[name.Literal]
	if !ok {
		return nil, fmt.Errorf("line %d:%d: undefined qubit register %q", name.Line, name.Col, name.Literal)
	}
	start, size := reg.start, reg.size

	if p.peek() == token.LBRACKET {
		p.advance() // consume [
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		_, err = p.expect(token.RBRACKET)
		if err != nil {
			return nil, err
		}
		idx := int(v)
		if idx < 0 || idx >= size {
			return nil, fmt.Errorf("line %d:%d: qubit index %d out of range [0, %d)", name.Line, name.Col, idx, size)
		}
		return []int{start + idx}, nil
	}

	// Entire register.
	qubits := make([]int, size)
	for i := range size {
		qubits[i] = start + i
	}
	return qubits, nil
}

func (p *parser) parseClbitArgs() ([]int, error) {
	var result []int
	for {
		clbits, err := p.parseSingleClbitArg()
		if err != nil {
			return nil, err
		}
		result = append(result, clbits...)
		if p.peek() != token.COMMA {
			break
		}
		p.advance()
	}
	return result, nil
}

func (p *parser) parseSingleClbitArg() ([]int, error) {
	name, err := p.expect(token.IDENT)
	if err != nil {
		return nil, err
	}
	reg, ok := p.cregs[name.Literal]
	if !ok {
		return nil, fmt.Errorf("line %d:%d: undefined classical register %q", name.Line, name.Col, name.Literal)
	}
	start, size := reg.start, reg.size

	if p.peek() == token.LBRACKET {
		p.advance()
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		_, err = p.expect(token.RBRACKET)
		if err != nil {
			return nil, err
		}
		idx := int(v)
		if idx < 0 || idx >= size {
			return nil, fmt.Errorf("line %d:%d: classical bit index %d out of range [0, %d)", name.Line, name.Col, idx, size)
		}
		return []int{start + idx}, nil
	}

	clbits := make([]int, size)
	for i := range size {
		clbits[i] = start + i
	}
	return clbits, nil
}

// Expression parser (precedence climbing).
func (p *parser) parseExpr() (float64, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (float64, error) {
	return p.parseAdd()
}

func (p *parser) parseAdd() (float64, error) {
	left, err := p.parseMul()
	if err != nil {
		return 0, err
	}
	for p.peek() == token.PLUS || p.peek() == token.MINUS {
		op := p.advance()
		right, err := p.parseMul()
		if err != nil {
			return 0, err
		}
		if op.Type == token.PLUS {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *parser) parseMul() (float64, error) {
	left, err := p.parsePow()
	if err != nil {
		return 0, err
	}
	for p.peek() == token.STAR || p.peek() == token.SLASH || p.peek() == token.PERCENT {
		op := p.advance()
		right, err := p.parsePow()
		if err != nil {
			return 0, err
		}
		switch op.Type {
		case token.STAR:
			left *= right
		case token.SLASH:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		case token.PERCENT:
			left = math.Mod(left, right)
		}
	}
	return left, nil
}

func (p *parser) parsePow() (float64, error) {
	base, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	if p.peek() == token.POWER {
		p.advance()
		exp, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *parser) parseUnary() (float64, error) {
	if p.peek() == token.MINUS {
		p.advance()
		v, err := p.parseUnary()
		return -v, err
	}
	if p.peek() == token.PLUS {
		p.advance()
		return p.parseUnary()
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (float64, error) {
	t := p.cur()
	switch t.Type {
	case token.INT:
		p.advance()
		return strconv.ParseFloat(t.Literal, 64)
	case token.FLOAT:
		p.advance()
		return strconv.ParseFloat(t.Literal, 64)
	case token.PI:
		p.advance()
		return math.Pi, nil
	case token.EULER:
		p.advance()
		return math.E, nil
	case token.TAU:
		p.advance()
		return 2 * math.Pi, nil
	case token.LPAREN:
		p.advance() // consume (
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		_, err = p.expect(token.RPAREN)
		return v, err
	case token.IDENT:
		// Built-in functions: sin, cos, tan, sqrt, exp, log, arccos, etc.
		p.advance()
		fname := t.Literal
		_, err := p.expect(token.LPAREN)
		if err != nil {
			return 0, err
		}
		arg, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		_, err = p.expect(token.RPAREN)
		if err != nil {
			return 0, err
		}
		switch fname {
		case "sin":
			return math.Sin(arg), nil
		case "cos":
			return math.Cos(arg), nil
		case "tan":
			return math.Tan(arg), nil
		case "sqrt":
			return math.Sqrt(arg), nil
		case "exp":
			return math.Exp(arg), nil
		case "log", "ln":
			return math.Log(arg), nil
		case "acos", "arccos":
			return math.Acos(arg), nil
		case "asin", "arcsin":
			return math.Asin(arg), nil
		case "atan", "arctan":
			return math.Atan(arg), nil
		default:
			return 0, fmt.Errorf("line %d:%d: unknown function %q", t.Line, t.Col, fname)
		}
	default:
		return 0, fmt.Errorf("line %d:%d: unexpected token %s (%q) in expression", t.Line, t.Col, t.Type, t.Literal)
	}
}

// builtinGates returns the gate definition map, initially empty.
// User-defined gates are added during parsing.
func builtinGates() map[string]*gatedef {
	return make(map[string]*gatedef)
}

// Pseudo-gates for barrier and reset.
type barrierGate struct{ n int }

func (g barrierGate) Name() string                     { return "barrier" }
func (g barrierGate) Qubits() int                      { return g.n }
func (g barrierGate) Matrix() []complex128             { return nil }
func (g barrierGate) Params() []float64                { return nil }
func (g barrierGate) Inverse() gate.Gate               { return g }
func (g barrierGate) Decompose(_ []int) []gate.Applied { return nil }

type opaqueGate struct {
	name   string
	n      int
	params []float64
}

func (g opaqueGate) Name() string                     { return g.name }
func (g opaqueGate) Qubits() int                      { return g.n }
func (g opaqueGate) Matrix() []complex128             { return nil }
func (g opaqueGate) Params() []float64                { return g.params }
func (g opaqueGate) Inverse() gate.Gate               { return g }
func (g opaqueGate) Decompose(_ []int) []gate.Applied { return nil }
