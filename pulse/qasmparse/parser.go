package qasmparse

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/splch/qgo/pulse"
	"github.com/splch/qgo/pulse/waveform"
)

// WaveformConstructor builds a waveform from numeric arguments.
type WaveformConstructor func(args []float64) (pulse.Waveform, error)

// config holds parser options.
type config struct {
	defaultDt     float64
	waveformFuncs map[string]WaveformConstructor
}

// Option configures the parser.
type Option func(*config)

// WithDefaultDt sets the port time resolution (default 1e-9 = 1ns).
func WithDefaultDt(dt float64) Option {
	return func(c *config) { c.defaultDt = dt }
}

// WithWaveform registers a custom waveform constructor by name.
func WithWaveform(name string, fn WaveformConstructor) Option {
	return func(c *config) { c.waveformFuncs[name] = fn }
}

// Parse reads OpenPulse QASM from r and returns a pulse Program.
func Parse(r io.Reader, opts ...Option) (*pulse.Program, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("qasmparse: read source: %w", err)
	}
	return ParseString(string(data), opts...)
}

// ParseString parses OpenPulse QASM source text into a pulse Program.
func ParseString(source string, opts ...Option) (*pulse.Program, error) {
	cfg := &config{
		defaultDt:     1e-9,
		waveformFuncs: make(map[string]WaveformConstructor),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	p := &parser{
		lex:    newLexer(source),
		cfg:    cfg,
		ports:  make(map[string]pulse.Port),
		frames: make(map[string]pulse.Frame),
	}
	p.advance() // prime the first token

	return p.parseProgram()
}

// parser is a recursive descent parser for OpenPulse QASM.
type parser struct {
	lex    *lexer
	cfg    *config
	cur    token
	ports  map[string]pulse.Port
	frames map[string]pulse.Frame

	portList  []pulse.Port
	frameList []pulse.Frame
	instrs    []pulse.Instruction
}

func (p *parser) advance() {
	p.cur = p.lex.next()
}

func (p *parser) expect(typ tokenType) (token, error) {
	if p.cur.typ != typ {
		return p.cur, p.errorf("expected %d, got %q", typ, p.cur.literal)
	}
	tok := p.cur
	p.advance()
	return tok, nil
}

func (p *parser) errorf(format string, args ...any) error {
	prefix := fmt.Sprintf("qasmparse:%d:%d: ", p.cur.line, p.cur.col)
	return fmt.Errorf(prefix+format, args...)
}

func (p *parser) parseProgram() (*pulse.Program, error) {
	// Optional version declaration.
	if p.cur.typ == tokOPENQASM {
		if err := p.parseVersion(); err != nil {
			return nil, err
		}
	}

	// Port declarations.
	for p.cur.typ == tokEXTERN {
		if err := p.parsePortDecl(); err != nil {
			return nil, err
		}
	}

	// Cal block(s).
	for p.cur.typ == tokCAL {
		if err := p.parseCalBlock(); err != nil {
			return nil, err
		}
	}

	if p.cur.typ != tokEOF {
		return nil, p.errorf("unexpected token %q after cal block", p.cur.literal)
	}

	if len(p.instrs) == 0 {
		return nil, fmt.Errorf("qasmparse: no instructions found")
	}

	prog := pulse.NewProgram("parsed",
		p.portList,
		p.frameList,
		p.instrs,
		nil,
	)
	return prog, nil
}

func (p *parser) parseVersion() error {
	p.advance() // consume OPENQASM
	// Consume version number (e.g., "3.0").
	if p.cur.typ != tokNUMBER {
		return p.errorf("expected version number after OPENQASM")
	}
	p.advance()
	// Optional ".0" part.
	if p.cur.typ == tokNUMBER {
		p.advance()
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}
	return nil
}

func (p *parser) parsePortDecl() error {
	p.advance() // consume 'extern'
	if _, err := p.expect(tokPORT); err != nil {
		return err
	}
	nameTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}

	port, err := pulse.NewPort(nameTok.literal, p.cfg.defaultDt)
	if err != nil {
		return p.errorf("invalid port: %w", err)
	}
	p.ports[nameTok.literal] = port
	p.portList = append(p.portList, port)
	return nil
}

func (p *parser) parseCalBlock() error {
	p.advance() // consume 'cal'
	if _, err := p.expect(tokLBRACE); err != nil {
		return err
	}

	for p.cur.typ != tokRBRACE && p.cur.typ != tokEOF {
		if err := p.parseCalStmt(); err != nil {
			return err
		}
	}

	if _, err := p.expect(tokRBRACE); err != nil {
		return err
	}
	return nil
}

func (p *parser) parseCalStmt() error {
	switch p.cur.typ {
	case tokFRAME:
		return p.parseFrameDecl()
	case tokPLAY:
		return p.parsePlayStmt()
	case tokDELAY:
		return p.parseDelayStmt()
	case tokSET_PHASE:
		return p.parseSetPhaseStmt()
	case tokSHIFT_PHASE:
		return p.parseShiftPhaseStmt()
	case tokSET_FREQUENCY:
		return p.parseSetFrequencyStmt()
	case tokSHIFT_FREQUENCY:
		return p.parseShiftFrequencyStmt()
	case tokBARRIER:
		return p.parseBarrierStmt()
	case tokCAPTURE_V0:
		return p.parseCaptureStmt()
	default:
		return p.errorf("unexpected token %q in cal block", p.cur.literal)
	}
}

func (p *parser) parseFrameDecl() error {
	p.advance() // consume 'frame'
	nameTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokEQUAL); err != nil {
		return err
	}
	if _, err := p.expect(tokNEWFRAME); err != nil {
		return err
	}
	if _, err := p.expect(tokLPAREN); err != nil {
		return err
	}

	portTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokCOMMA); err != nil {
		return err
	}
	freq, err := p.parseExpr()
	if err != nil {
		return err
	}
	if _, err := p.expect(tokCOMMA); err != nil {
		return err
	}
	phase, err := p.parseExpr()
	if err != nil {
		return err
	}
	if _, err := p.expect(tokRPAREN); err != nil {
		return err
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}

	port, ok := p.ports[portTok.literal]
	if !ok {
		// Auto-create port if not declared (some QASM doesn't have extern port).
		var portErr error
		port, portErr = pulse.NewPort(portTok.literal, p.cfg.defaultDt)
		if portErr != nil {
			return p.errorf("invalid port %q: %w", portTok.literal, portErr)
		}
		p.ports[portTok.literal] = port
		p.portList = append(p.portList, port)
	}

	frame, err := pulse.NewFrame(nameTok.literal, port, freq, phase)
	if err != nil {
		return p.errorf("invalid frame: %w", err)
	}
	p.frames[nameTok.literal] = frame
	p.frameList = append(p.frameList, frame)
	return nil
}

func (p *parser) parsePlayStmt() error {
	p.advance() // consume 'play'
	if _, err := p.expect(tokLPAREN); err != nil {
		return err
	}
	frameTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokCOMMA); err != nil {
		return err
	}

	wf, err := p.parseWaveformCall()
	if err != nil {
		return err
	}

	if _, err := p.expect(tokRPAREN); err != nil {
		return err
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}

	frame, ok := p.frames[frameTok.literal]
	if !ok {
		return p.errorf("undefined frame %q", frameTok.literal)
	}
	p.instrs = append(p.instrs, pulse.Play{Frame: frame, Waveform: wf})
	return nil
}

func (p *parser) parseDelayStmt() error {
	p.advance() // consume 'delay'
	if _, err := p.expect(tokLBRACKET); err != nil {
		return err
	}
	dur, err := p.parseDuration()
	if err != nil {
		return err
	}
	if _, err := p.expect(tokRBRACKET); err != nil {
		return err
	}

	frameTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}

	frame, ok := p.frames[frameTok.literal]
	if !ok {
		return p.errorf("undefined frame %q", frameTok.literal)
	}
	p.instrs = append(p.instrs, pulse.Delay{Frame: frame, Duration: dur})
	return nil
}

func (p *parser) parseSetPhaseStmt() error {
	p.advance() // consume 'set_phase'
	if _, err := p.expect(tokLPAREN); err != nil {
		return err
	}
	frameTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokCOMMA); err != nil {
		return err
	}
	val, err := p.parseExpr()
	if err != nil {
		return err
	}
	if _, err := p.expect(tokRPAREN); err != nil {
		return err
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}

	frame, ok := p.frames[frameTok.literal]
	if !ok {
		return p.errorf("undefined frame %q", frameTok.literal)
	}
	p.instrs = append(p.instrs, pulse.SetPhase{Frame: frame, Phase: val})
	return nil
}

func (p *parser) parseShiftPhaseStmt() error {
	p.advance() // consume 'shift_phase'
	if _, err := p.expect(tokLPAREN); err != nil {
		return err
	}
	frameTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokCOMMA); err != nil {
		return err
	}
	val, err := p.parseExpr()
	if err != nil {
		return err
	}
	if _, err := p.expect(tokRPAREN); err != nil {
		return err
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}

	frame, ok := p.frames[frameTok.literal]
	if !ok {
		return p.errorf("undefined frame %q", frameTok.literal)
	}
	p.instrs = append(p.instrs, pulse.ShiftPhase{Frame: frame, Delta: val})
	return nil
}

func (p *parser) parseSetFrequencyStmt() error {
	p.advance() // consume 'set_frequency'
	if _, err := p.expect(tokLPAREN); err != nil {
		return err
	}
	frameTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokCOMMA); err != nil {
		return err
	}
	val, err := p.parseExpr()
	if err != nil {
		return err
	}
	if _, err := p.expect(tokRPAREN); err != nil {
		return err
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}

	frame, ok := p.frames[frameTok.literal]
	if !ok {
		return p.errorf("undefined frame %q", frameTok.literal)
	}
	p.instrs = append(p.instrs, pulse.SetFrequency{Frame: frame, Frequency: val})
	return nil
}

func (p *parser) parseShiftFrequencyStmt() error {
	p.advance() // consume 'shift_frequency'
	if _, err := p.expect(tokLPAREN); err != nil {
		return err
	}
	frameTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokCOMMA); err != nil {
		return err
	}
	val, err := p.parseExpr()
	if err != nil {
		return err
	}
	if _, err := p.expect(tokRPAREN); err != nil {
		return err
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}

	frame, ok := p.frames[frameTok.literal]
	if !ok {
		return p.errorf("undefined frame %q", frameTok.literal)
	}
	p.instrs = append(p.instrs, pulse.ShiftFrequency{Frame: frame, Delta: val})
	return nil
}

func (p *parser) parseBarrierStmt() error {
	p.advance() // consume 'barrier'

	var bFrames []pulse.Frame
	for {
		frameTok, err := p.expect(tokIDENT)
		if err != nil {
			return err
		}
		frame, ok := p.frames[frameTok.literal]
		if !ok {
			return p.errorf("undefined frame %q", frameTok.literal)
		}
		bFrames = append(bFrames, frame)
		if p.cur.typ != tokCOMMA {
			break
		}
		p.advance() // consume ','
	}

	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}
	p.instrs = append(p.instrs, pulse.Barrier{Frames: bFrames})
	return nil
}

func (p *parser) parseCaptureStmt() error {
	p.advance() // consume 'capture_v0'
	if _, err := p.expect(tokLPAREN); err != nil {
		return err
	}
	frameTok, err := p.expect(tokIDENT)
	if err != nil {
		return err
	}
	if _, err := p.expect(tokCOMMA); err != nil {
		return err
	}
	dur, err := p.parseDuration()
	if err != nil {
		return err
	}
	if _, err := p.expect(tokRPAREN); err != nil {
		return err
	}
	if _, err := p.expect(tokSEMICOLON); err != nil {
		return err
	}

	frame, ok := p.frames[frameTok.literal]
	if !ok {
		return p.errorf("undefined frame %q", frameTok.literal)
	}
	p.instrs = append(p.instrs, pulse.Capture{Frame: frame, Duration: dur})
	return nil
}

// parseDuration parses a duration literal (NUMBERs) or an expression followed by 's'.
func (p *parser) parseDuration() (float64, error) {
	if p.cur.typ == tokDURATION {
		val, err := strconv.ParseFloat(p.cur.literal, 64)
		if err != nil {
			return 0, p.errorf("invalid duration %q: %w", p.cur.literal, err)
		}
		p.advance()
		return val, nil
	}
	// Allow "NUMBER s" as two tokens (the emitter uses %gs which may not have 's' attached).
	val, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	// Consume trailing 's' if present as an identifier.
	if p.cur.typ == tokIDENT && p.cur.literal == "s" {
		p.advance()
	}
	return val, nil
}

// parseWaveformCall parses: IDENT "(" expr_or_complex ("," expr)* ")"
func (p *parser) parseWaveformCall() (pulse.Waveform, error) {
	nameTok, err := p.expect(tokIDENT)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokLPAREN); err != nil {
		return nil, err
	}

	var args []float64
	var complexArg *complex128

	// For "constant", try to parse the first argument as a complex literal
	// (REAL +/- IMAG i) before falling back to a regular expression.
	if nameTok.literal == "constant" {
		if c, ok := p.tryParseComplex(); ok {
			complexArg = &c
		} else {
			firstVal, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, firstVal)
		}
	} else {
		firstVal, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, firstVal)
	}

	// Remaining arguments.
	for p.cur.typ == tokCOMMA {
		p.advance()
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, val)
	}

	if _, err := p.expect(tokRPAREN); err != nil {
		return nil, err
	}

	return p.resolveWaveform(nameTok.literal, args, complexArg)
}

// tryParseComplex attempts to parse a complex literal: [sign] NUMBER (+|-) NUMBER i.
// Returns (value, true) on success, or (0, false) if the tokens don't match,
// restoring the parser to its original position on failure.
func (p *parser) tryParseComplex() (complex128, bool) {
	// Save state.
	lexState := p.lex.save()
	savedCur := p.cur

	// Optional sign for real part.
	realSign := 1.0
	if p.cur.typ == tokMINUS {
		realSign = -1.0
		p.advance()
	} else if p.cur.typ == tokPLUS {
		p.advance()
	}

	if p.cur.typ != tokNUMBER {
		p.lex.restore(lexState)
		p.cur = savedCur
		return 0, false
	}
	realVal, err := strconv.ParseFloat(p.cur.literal, 64)
	if err != nil {
		p.lex.restore(lexState)
		p.cur = savedCur
		return 0, false
	}
	realVal *= realSign
	p.advance()

	// Must have + or -.
	if p.cur.typ != tokPLUS && p.cur.typ != tokMINUS {
		p.lex.restore(lexState)
		p.cur = savedCur
		return 0, false
	}
	imagSign := 1.0
	if p.cur.typ == tokMINUS {
		imagSign = -1.0
	}
	p.advance()

	// Must have a number.
	if p.cur.typ != tokNUMBER {
		p.lex.restore(lexState)
		p.cur = savedCur
		return 0, false
	}
	imagVal, err := strconv.ParseFloat(p.cur.literal, 64)
	if err != nil {
		p.lex.restore(lexState)
		p.cur = savedCur
		return 0, false
	}
	imagVal *= imagSign
	p.advance()

	// Must have 'i'.
	if p.cur.typ != tokIDENT || p.cur.literal != "i" {
		p.lex.restore(lexState)
		p.cur = savedCur
		return 0, false
	}
	p.advance() // consume 'i'

	return complex(realVal, imagVal), true
}

func (p *parser) resolveWaveform(name string, args []float64, complexArg *complex128) (pulse.Waveform, error) {
	switch name {
	case "constant":
		if complexArg != nil {
			if len(args) != 1 {
				return nil, p.errorf("constant(complex, duration) expects 1 extra arg, got %d", len(args))
			}
			return waveform.Constant(*complexArg, args[0])
		}
		if len(args) != 2 {
			return nil, p.errorf("constant(amplitude, duration) expects 2 args, got %d", len(args))
		}
		return waveform.Constant(complex(args[0], 0), args[1])

	case "gaussian":
		if len(args) != 3 {
			return nil, p.errorf("gaussian(amp, duration, sigma) expects 3 args, got %d", len(args))
		}
		return waveform.Gaussian(args[0], args[1], args[2])

	case "drag":
		if len(args) != 4 {
			return nil, p.errorf("drag(amp, duration, sigma, beta) expects 4 args, got %d", len(args))
		}
		return waveform.DRAG(args[0], args[1], args[2], args[3])

	case "gaussian_square":
		if len(args) != 4 {
			return nil, p.errorf("gaussian_square(amp, duration, sigma, width) expects 4 args, got %d", len(args))
		}
		return waveform.GaussianSquare(args[0], args[1], args[2], args[3])

	default:
		if fn, ok := p.cfg.waveformFuncs[name]; ok {
			return fn(args)
		}
		return nil, p.errorf("unknown waveform %q", name)
	}
}

// Expression parser: precedence climbing.
// expr = addExpr
// addExpr = mulExpr (('+' | '-') mulExpr)*
// mulExpr = unaryExpr (('*' | '/') unaryExpr)*
// unaryExpr = ('-' | '+') unaryExpr | primary
// primary = NUMBER | '(' expr ')' | 'pi' | IDENT

func (p *parser) parseExpr() (float64, error) {
	return p.parseAddExpr()
}

func (p *parser) parseAddExpr() (float64, error) {
	left, err := p.parseMulExpr()
	if err != nil {
		return 0, err
	}
	for p.cur.typ == tokPLUS || p.cur.typ == tokMINUS {
		op := p.cur.typ
		p.advance()
		right, err := p.parseMulExpr()
		if err != nil {
			return 0, err
		}
		if op == tokPLUS {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *parser) parseMulExpr() (float64, error) {
	left, err := p.parseUnaryExpr()
	if err != nil {
		return 0, err
	}
	for p.cur.typ == tokSTAR || p.cur.typ == tokSLASH {
		op := p.cur.typ
		p.advance()
		right, err := p.parseUnaryExpr()
		if err != nil {
			return 0, err
		}
		if op == tokSTAR {
			left *= right
		} else {
			if right == 0 {
				return 0, p.errorf("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

func (p *parser) parseUnaryExpr() (float64, error) {
	if p.cur.typ == tokMINUS {
		p.advance()
		val, err := p.parseUnaryExpr()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}
	if p.cur.typ == tokPLUS {
		p.advance()
		return p.parseUnaryExpr()
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (float64, error) {
	switch p.cur.typ {
	case tokNUMBER:
		val, err := strconv.ParseFloat(p.cur.literal, 64)
		if err != nil {
			return 0, p.errorf("invalid number %q: %w", p.cur.literal, err)
		}
		p.advance()
		return val, nil

	case tokDURATION:
		val, err := strconv.ParseFloat(p.cur.literal, 64)
		if err != nil {
			return 0, p.errorf("invalid duration %q: %w", p.cur.literal, err)
		}
		p.advance()
		return val, nil

	case tokIDENT:
		if strings.EqualFold(p.cur.literal, "pi") {
			p.advance()
			return math.Pi, nil
		}
		return 0, p.errorf("unexpected identifier %q in expression", p.cur.literal)

	case tokLPAREN:
		p.advance()
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if _, err := p.expect(tokRPAREN); err != nil {
			return 0, err
		}
		return val, nil

	default:
		return 0, p.errorf("unexpected token %q in expression", p.cur.literal)
	}
}
