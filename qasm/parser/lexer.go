package parser

import (
	"unicode"
	"unicode/utf8"

	"github.com/splch/qgo/qasm/token"
)

// lexer tokenizes OpenQASM 3.0 source.
type lexer struct {
	input   string
	pos     int // current byte position
	line    int
	col     int
	tokens  []token.Token
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

		// Line comments
		if ch == '/' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '/' {
			l.skipLineComment()
			continue
		}
		// Block comments
		if ch == '/' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '*' {
			l.skipBlockComment()
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
		case ch == '[':
			l.emit(token.LBRACKET, "[", startLine, startCol)
			l.advance()
		case ch == ']':
			l.emit(token.RBRACKET, "]", startLine, startCol)
			l.advance()
		case ch == '{':
			l.emit(token.LBRACE, "{", startLine, startCol)
			l.advance()
		case ch == '}':
			l.emit(token.RBRACE, "}", startLine, startCol)
			l.advance()
		case ch == ';':
			l.emit(token.SEMICOLON, ";", startLine, startCol)
			l.advance()
		case ch == ',':
			l.emit(token.COMMA, ",", startLine, startCol)
			l.advance()
		case ch == ':':
			l.emit(token.COLON, ":", startLine, startCol)
			l.advance()
		case ch == '.':
			l.emit(token.DOT, ".", startLine, startCol)
			l.advance()
		case ch == '@':
			l.emit(token.AT, "@", startLine, startCol)
			l.advance()
		case ch == '~':
			l.emit(token.TILDE, "~", startLine, startCol)
			l.advance()
		case ch == '+':
			l.emit(token.PLUS, "+", startLine, startCol)
			l.advance()
		case ch == '%':
			l.emit(token.PERCENT, "%", startLine, startCol)
			l.advance()
		case ch == '^':
			l.emit(token.CARET, "^", startLine, startCol)
			l.advance()
		case ch == '-':
			l.advance()
			if l.pos < len(l.input) && l.input[l.pos] == '>' {
				l.emit(token.ARROW, "->", startLine, startCol)
				l.advance()
			} else {
				l.emit(token.MINUS, "-", startLine, startCol)
			}
		case ch == '*':
			l.advance()
			if l.pos < len(l.input) && l.input[l.pos] == '*' {
				l.emit(token.POWER, "**", startLine, startCol)
				l.advance()
			} else {
				l.emit(token.STAR, "*", startLine, startCol)
			}
		case ch == '/':
			l.emit(token.SLASH, "/", startLine, startCol)
			l.advance()
		case ch == '=':
			l.advance()
			if l.pos < len(l.input) && l.input[l.pos] == '=' {
				l.emit(token.EQEQ, "==", startLine, startCol)
				l.advance()
			} else {
				l.emit(token.EQUALS, "=", startLine, startCol)
			}
		case ch == '!':
			l.advance()
			if l.pos < len(l.input) && l.input[l.pos] == '=' {
				l.emit(token.NEQ, "!=", startLine, startCol)
				l.advance()
			} else {
				l.emit(token.BANG, "!", startLine, startCol)
			}
		case ch == '<':
			l.advance()
			if l.pos < len(l.input) && l.input[l.pos] == '=' {
				l.emit(token.LE, "<=", startLine, startCol)
				l.advance()
			} else if l.pos < len(l.input) && l.input[l.pos] == '<' {
				l.emit(token.LSHIFT, "<<", startLine, startCol)
				l.advance()
			} else {
				l.emit(token.LT, "<", startLine, startCol)
			}
		case ch == '>':
			l.advance()
			if l.pos < len(l.input) && l.input[l.pos] == '=' {
				l.emit(token.GE, ">=", startLine, startCol)
				l.advance()
			} else if l.pos < len(l.input) && l.input[l.pos] == '>' {
				l.emit(token.RSHIFT, ">>", startLine, startCol)
				l.advance()
			} else {
				l.emit(token.GT, ">", startLine, startCol)
			}
		case ch == '&':
			l.advance()
			if l.pos < len(l.input) && l.input[l.pos] == '&' {
				l.emit(token.AMPAMP, "&&", startLine, startCol)
				l.advance()
			} else {
				l.emit(token.AMP, "&", startLine, startCol)
			}
		case ch == '|':
			l.advance()
			if l.pos < len(l.input) && l.input[l.pos] == '|' {
				l.emit(token.PIPEPIPE, "||", startLine, startCol)
				l.advance()
			} else {
				l.emit(token.PIPE, "|", startLine, startCol)
			}
		case ch == '"':
			l.readString(startLine, startCol)
		case isDigit(ch):
			l.readNumber(startLine, startCol)
		case isIdentStart(ch) || ch >= 0x80:
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

func (l *lexer) skipBlockComment() {
	l.advance() // skip /
	l.advance() // skip *
	for l.pos < len(l.input)-1 {
		if l.input[l.pos] == '*' && l.input[l.pos+1] == '/' {
			l.advance() // skip *
			l.advance() // skip /
			return
		}
		l.advance()
	}
	// Unterminated block comment — consume rest.
	l.pos = len(l.input)
}

func (l *lexer) readString(line, col int) {
	l.advance() // skip opening quote
	start := l.pos
	for l.pos < len(l.input) && l.input[l.pos] != '"' {
		l.advance()
	}
	lit := l.input[start:l.pos]
	if l.pos < len(l.input) {
		l.advance() // skip closing quote
	}
	l.emit(token.STRING, lit, line, col)
}

func (l *lexer) readNumber(line, col int) {
	start := l.pos
	isFloat := false
	for l.pos < len(l.input) && (isDigit(l.input[l.pos]) || l.input[l.pos] == '.') {
		if l.input[l.pos] == '.' {
			isFloat = true
		}
		l.advance()
	}
	// Handle scientific notation: 1e-3, 1.5E+2
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
	// Handle UTF-8 identifiers (π, τ, ℇ).
	for l.pos < len(l.input) {
		r, size := utf8.DecodeRuneInString(l.input[l.pos:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		if l.pos == start {
			if !isIdentStart(byte(r)) && !unicode.IsLetter(r) {
				break
			}
		} else {
			if !isIdentContinue(byte(r)) && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				break
			}
		}
		for range size {
			l.advance()
		}
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
	return isIdentStart(ch) || isDigit(ch)
}
