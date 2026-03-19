package parser

import "github.com/splch/goqu/qir/token"

// lexer tokenizes the QIR subset of LLVM IR source.
type lexer struct {
	input  string
	pos    int
	line   int
	col    int
	tokens []token.Token
}

func newLexer(input string) *lexer {
	return &lexer{input: input, line: 1, col: 1}
}

func (l *lexer) tokenize() []token.Token {
	for {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			l.tokens = append(l.tokens, token.Token{Type: token.EOF, Line: l.line, Col: l.col})
			break
		}

		ch := l.input[l.pos]

		// Line comments: ; to end of line.
		if ch == ';' {
			l.skipLineComment()
			continue
		}

		startLine, startCol := l.line, l.col

		switch {
		case ch == '(':
			l.emit(token.LPAREN, "(", startLine, startCol)
			l.advance()
		case ch == ')':
			l.emit(token.RPAREN, ")", startLine, startCol)
			l.advance()
		case ch == '{':
			l.emit(token.LBRACE, "{", startLine, startCol)
			l.advance()
		case ch == '}':
			l.emit(token.RBRACE, "}", startLine, startCol)
			l.advance()
		case ch == '[':
			l.emit(token.LBRACKET, "[", startLine, startCol)
			l.advance()
		case ch == ']':
			l.emit(token.RBRACKET, "]", startLine, startCol)
			l.advance()
		case ch == ',':
			l.emit(token.COMMA, ",", startLine, startCol)
			l.advance()
		case ch == '=':
			l.emit(token.EQUALS, "=", startLine, startCol)
			l.advance()
		case ch == '!':
			l.advance()
			if l.pos < len(l.input) && l.input[l.pos] == '"' {
				// Metadata string: !"..."
				l.advance() // skip opening quote
				start := l.pos
				for l.pos < len(l.input) && l.input[l.pos] != '"' {
					l.advance()
				}
				lit := `!"` + l.input[start:l.pos] + `"`
				if l.pos < len(l.input) {
					l.advance() // skip closing quote
				}
				l.emit(token.STRING_LIT, lit, startLine, startCol)
			} else {
				l.emit(token.BANG, "!", startLine, startCol)
			}
		case ch == '#':
			l.emit(token.HASH, "#", startLine, startCol)
			l.advance()
		case ch == '*':
			l.emit(token.STAR, "*", startLine, startCol)
			l.advance()
		case ch == '@':
			l.readGlobal(startLine, startCol)
		case ch == '%':
			l.readLocal(startLine, startCol)
		case ch == '"':
			l.readString(startLine, startCol)
		case ch == 'c' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '"':
			l.readCString(startLine, startCol)
		case ch == '-' || isDigit(ch):
			l.readNumber(startLine, startCol)
		case isIdentStart(ch):
			l.readIdentOrKeyword(startLine, startCol)
		default:
			l.emit(token.ILLEGAL, string(ch), startLine, startCol)
			l.advance()
		}
	}
	return l.tokens
}

func (l *lexer) advance() {
	if l.pos < len(l.input) {
		if l.input[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *lexer) emit(typ token.Type, lit string, line, col int) {
	l.tokens = append(l.tokens, token.Token{Type: typ, Literal: lit, Line: line, Col: col})
}

func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *lexer) skipLineComment() {
	for l.pos < len(l.input) && l.input[l.pos] != '\n' {
		l.advance()
	}
}

func (l *lexer) readGlobal(line, col int) {
	l.advance() // skip @
	start := l.pos
	// Can be a name or number.
	for l.pos < len(l.input) && isIdentContinue(l.input[l.pos]) {
		l.advance()
	}
	l.emit(token.GLOBAL, "@"+l.input[start:l.pos], line, col)
}

func (l *lexer) readLocal(line, col int) {
	l.advance() // skip %
	start := l.pos
	for l.pos < len(l.input) && isIdentContinue(l.input[l.pos]) {
		l.advance()
	}
	l.emit(token.LOCAL, "%"+l.input[start:l.pos], line, col)
}

func (l *lexer) readString(line, col int) {
	l.advance() // skip opening quote
	start := l.pos
	for l.pos < len(l.input) && l.input[l.pos] != '"' {
		if l.input[l.pos] == '\\' {
			l.advance() // skip escape
		}
		l.advance()
	}
	lit := l.input[start:l.pos]
	if l.pos < len(l.input) {
		l.advance() // skip closing quote
	}
	l.emit(token.STRING_LIT, lit, line, col)
}

func (l *lexer) readCString(line, col int) {
	l.advance() // skip 'c'
	l.advance() // skip opening quote
	start := l.pos
	for l.pos < len(l.input) && l.input[l.pos] != '"' {
		if l.input[l.pos] == '\\' {
			l.advance() // skip escape
		}
		l.advance()
	}
	lit := l.input[start:l.pos]
	if l.pos < len(l.input) {
		l.advance() // skip closing quote
	}
	l.emit(token.CSTRING, lit, line, col)
}

func (l *lexer) readNumber(line, col int) {
	start := l.pos
	isFloat := false

	// Handle negative sign.
	if l.input[l.pos] == '-' {
		l.advance()
	}

	for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
		l.advance()
	}

	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		isFloat = true
		l.advance()
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.advance()
		}
	}

	// Scientific notation.
	if l.pos < len(l.input) && (l.input[l.pos] == 'e' || l.input[l.pos] == 'E') {
		isFloat = true
		l.advance()
		if l.pos < len(l.input) && (l.input[l.pos] == '+' || l.input[l.pos] == '-') {
			l.advance()
		}
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.advance()
		}
	}

	lit := l.input[start:l.pos]
	if isFloat {
		l.emit(token.FLOAT, lit, line, col)
	} else {
		l.emit(token.INT, lit, line, col)
	}
}

func (l *lexer) readIdentOrKeyword(line, col int) {
	start := l.pos
	for l.pos < len(l.input) && isIdentContinue(l.input[l.pos]) {
		l.advance()
	}
	lit := l.input[start:l.pos]
	typ := token.LookupIdent(lit)
	l.emit(typ, lit, line, col)
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentContinue(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch) || ch == '.'
}
