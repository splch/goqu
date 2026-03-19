// Package token defines token types for the QIR subset of LLVM IR.
package token

// Type represents a token type.
type Type int

const (
	// Special.
	EOF Type = iota
	ILLEGAL

	// Literals.
	IDENT      // identifier (alphanumeric + underscore)
	GLOBAL     // @name or @0 (global identifier)
	LOCAL      // %name or %0 (local identifier)
	INT        // integer literal
	FLOAT      // floating point literal
	CSTRING    // c"...\00" (C string constant)
	STRING_LIT // "string"

	// Punctuation.
	LPAREN  // (
	RPAREN  // )
	LBRACE  // {
	RBRACE  // }
	LBRACKET // [
	RBRACKET // ]
	COMMA    // ,
	EQUALS   // =
	BANG     // !
	HASH     // #
	STAR     // *

	// Keywords.
	DEFINE
	DECLARE
	CALL
	BR
	RET
	SWITCH
	PHI
	ICMP
	ADD
	ZEXT
	INTTOPTR
	NULL
	LABEL
	TO
	VOID
	PTR
	TYPE
	OPAQUE
	I1
	I8
	I32
	I64
	DOUBLE
	INTERNAL
	CONSTANT
	GLOBAL_KW // "global"
	WRITEONLY
	READONLY
	NONNULL
	TRUE
	FALSE
	ATTRIBUTES
	LOCAL_UNNAMED_ADDR
	TAIL
	SLT
	X_TOKEN // "x" in array types like [3 x i8]
)

var tokenNames = map[Type]string{
	EOF: "EOF", ILLEGAL: "ILLEGAL",
	IDENT: "IDENT", GLOBAL: "GLOBAL", LOCAL: "LOCAL",
	INT: "INT", FLOAT: "FLOAT", CSTRING: "CSTRING", STRING_LIT: "STRING",
	LPAREN: "(", RPAREN: ")", LBRACE: "{", RBRACE: "}",
	LBRACKET: "[", RBRACKET: "]", COMMA: ",", EQUALS: "=",
	BANG: "!", HASH: "#", STAR: "*",
	DEFINE: "define", DECLARE: "declare", CALL: "call",
	BR: "br", RET: "ret", SWITCH: "switch",
	PHI: "phi", ICMP: "icmp", ADD: "add", ZEXT: "zext",
	INTTOPTR: "inttoptr", NULL: "null", LABEL: "label",
	TO: "to", VOID: "void", PTR: "ptr",
	TYPE: "type", OPAQUE: "opaque",
	I1: "i1", I8: "i8", I32: "i32", I64: "i64", DOUBLE: "double",
	INTERNAL: "internal", CONSTANT: "constant",
	GLOBAL_KW: "global", WRITEONLY: "writeonly", READONLY: "readonly",
	NONNULL: "nonnull", TRUE: "true", FALSE: "false",
	ATTRIBUTES: "attributes", LOCAL_UNNAMED_ADDR: "local_unnamed_addr",
	TAIL: "tail", SLT: "slt", X_TOKEN: "x",
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
	"define":             DEFINE,
	"declare":            DECLARE,
	"call":               CALL,
	"br":                 BR,
	"ret":                RET,
	"switch":             SWITCH,
	"phi":                PHI,
	"icmp":               ICMP,
	"add":                ADD,
	"zext":               ZEXT,
	"inttoptr":           INTTOPTR,
	"null":               NULL,
	"label":              LABEL,
	"to":                 TO,
	"void":               VOID,
	"ptr":                PTR,
	"type":               TYPE,
	"opaque":             OPAQUE,
	"i1":                 I1,
	"i8":                 I8,
	"i32":                I32,
	"i64":                I64,
	"double":             DOUBLE,
	"internal":           INTERNAL,
	"constant":           CONSTANT,
	"global":             GLOBAL_KW,
	"writeonly":          WRITEONLY,
	"readonly":           READONLY,
	"nonnull":            NONNULL,
	"true":               TRUE,
	"false":              FALSE,
	"attributes":         ATTRIBUTES,
	"local_unnamed_addr": LOCAL_UNNAMED_ADDR,
	"tail":               TAIL,
	"slt":                SLT,
	"x":                  X_TOKEN,
}

// LookupIdent returns the token type for an identifier, checking keywords first.
func LookupIdent(ident string) Type {
	if tok, ok := Keywords[ident]; ok {
		return tok
	}
	return IDENT
}
