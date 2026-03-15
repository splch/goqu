// Package token defines token types for the OpenQASM 3.0 lexer.
package token

// Type represents a token type.
type Type int

const (
	// Special
	EOF Type = iota
	ILLEGAL
	COMMENT

	// Literals
	IDENT  // identifier
	INT    // integer literal
	FLOAT  // floating point literal
	STRING // string literal

	// Operators & delimiters
	LPAREN    // (
	RPAREN    // )
	LBRACKET  // [
	RBRACKET  // ]
	LBRACE    // {
	RBRACE    // }
	SEMICOLON // ;
	COMMA     // ,
	COLON     // :
	DOT       // .
	AT        // @
	ARROW     // ->
	EQUALS    // =
	EQEQ      // ==
	NEQ       // !=
	PLUS      // +
	MINUS     // -
	STAR      // *
	SLASH     // /
	PERCENT   // %
	POWER     // **
	AMP       // &
	PIPE      // |
	CARET     // ^
	TILDE     // ~
	BANG      // !
	LT        // <
	GT        // >
	LE        // <=
	GE        // >=
	LSHIFT    // <<
	RSHIFT    // >>
	AMPAMP    // &&
	PIPEPIPE  // ||

	// Keywords
	OPENQASM
	INCLUDE
	GATE
	QUBIT
	BIT
	QREG
	CREG
	MEASURE
	RESET
	BARRIER
	DELAY
	IF
	ELSE
	FOR
	WHILE
	SWITCH
	CASE
	DEFAULT
	IN
	RETURN
	DEF
	CONST
	INPUT
	OUTPUT
	CTRL
	NEGCTRL
	INV
	POW
	GPHASE
	U     // built-in U gate
	PI    // pi constant
	EULER // euler constant
	TAU   // tau constant
	TRUE
	FALSE
	LET
	INT_TYPE
	UINT_TYPE
	FLOAT_TYPE
	ANGLE_TYPE
	BOOL_TYPE
	COMPLEX_TYPE
	DURATION
	STRETCH
)

var tokenNames = map[Type]string{
	EOF: "EOF", ILLEGAL: "ILLEGAL", COMMENT: "COMMENT",
	IDENT: "IDENT", INT: "INT", FLOAT: "FLOAT", STRING: "STRING",
	LPAREN: "(", RPAREN: ")", LBRACKET: "[", RBRACKET: "]",
	LBRACE: "{", RBRACE: "}", SEMICOLON: ";", COMMA: ",",
	COLON: ":", DOT: ".", AT: "@", ARROW: "->",
	EQUALS: "=", EQEQ: "==", NEQ: "!=",
	PLUS: "+", MINUS: "-", STAR: "*", SLASH: "/", PERCENT: "%", POWER: "**",
	AMP: "&", PIPE: "|", CARET: "^", TILDE: "~", BANG: "!",
	LT: "<", GT: ">", LE: "<=", GE: ">=",
	LSHIFT: "<<", RSHIFT: ">>", AMPAMP: "&&", PIPEPIPE: "||",
	OPENQASM: "OPENQASM", INCLUDE: "include", GATE: "gate",
	QUBIT: "qubit", BIT: "bit", QREG: "qreg", CREG: "creg",
	MEASURE: "measure", RESET: "reset", BARRIER: "barrier", DELAY: "delay",
	IF: "if", ELSE: "else", FOR: "for", WHILE: "while", SWITCH: "switch", CASE: "case", DEFAULT: "default",
	IN: "in", RETURN: "return", DEF: "def",
	CONST: "const", INPUT: "input", OUTPUT: "output",
	CTRL: "ctrl", NEGCTRL: "negctrl", INV: "inv", POW: "pow",
	GPHASE: "gphase", U: "U", PI: "pi", EULER: "euler", TAU: "tau",
	TRUE: "true", FALSE: "false", LET: "let",
	INT_TYPE: "int", UINT_TYPE: "uint", FLOAT_TYPE: "float",
	ANGLE_TYPE: "angle", BOOL_TYPE: "bool", COMPLEX_TYPE: "complex",
	DURATION: "duration", STRETCH: "stretch",
}

func (t Type) String() string {
	if s, ok := tokenNames[t]; ok {
		return s
	}
	return "UNKNOWN"
}

// Token represents a lexical token.
type Token struct {
	Type    Type
	Literal string
	Line    int
	Col     int
}

// Keywords maps keyword strings to token types.
var Keywords = map[string]Type{
	"OPENQASM": OPENQASM,
	"include":  INCLUDE,
	"gate":     GATE,
	"qubit":    QUBIT,
	"bit":      BIT,
	"qreg":     QREG,
	"creg":     CREG,
	"measure":  MEASURE,
	"reset":    RESET,
	"barrier":  BARRIER,
	"delay":    DELAY,
	"if":       IF,
	"else":     ELSE,
	"for":      FOR,
	"while":    WHILE,
	"switch":   SWITCH,
	"case":     CASE,
	"default":  DEFAULT,
	"in":       IN,
	"return":   RETURN,
	"def":      DEF,
	"const":    CONST,
	"input":    INPUT,
	"output":   OUTPUT,
	"ctrl":     CTRL,
	"negctrl":  NEGCTRL,
	"inv":      INV,
	"pow":      POW,
	"gphase":   GPHASE,
	"U":        U,
	"pi":       PI,
	"π":        PI,
	"euler":    EULER,
	"ℇ":        EULER,
	"tau":      TAU,
	"τ":        TAU,
	"true":     TRUE,
	"false":    FALSE,
	"let":      LET,
	"int":      INT_TYPE,
	"uint":     UINT_TYPE,
	"float":    FLOAT_TYPE,
	"angle":    ANGLE_TYPE,
	"bool":     BOOL_TYPE,
	"complex":  COMPLEX_TYPE,
	"duration": DURATION,
	"stretch":  STRETCH,
}

// LookupIdent returns the token type for an identifier, checking keywords first.
func LookupIdent(ident string) Type {
	if tok, ok := Keywords[ident]; ok {
		return tok
	}
	return IDENT
}
