package qasmparse

import (
	"unicode"
)

// lexerState holds the lexer position for save/restore.
type lexerState struct {
	pos  int
	line int
	col  int
}

// lexer tokenizes OpenPulse QASM source text.
type lexer struct {
	src  []rune
	pos  int
	line int
	col  int
}

func newLexer(src string) *lexer {
	return &lexer{src: []rune(src), line: 1, col: 1}
}

func (l *lexer) save() lexerState {
	return lexerState{pos: l.pos, line: l.line, col: l.col}
}

func (l *lexer) restore(s lexerState) {
	l.pos = s.pos
	l.line = s.line
	l.col = s.col
}

func (l *lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) advance() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	ch := l.src[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *lexer) skipWhitespace() {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t' || l.src[l.pos] == '\n' || l.src[l.pos] == '\r') {
		l.advance()
	}
}

func (l *lexer) skipLineComment() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
}

func (l *lexer) skipBlockComment() {
	// Already consumed "/*"
	for l.pos < len(l.src) {
		if l.src[l.pos] == '*' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/' {
			l.advance() // *
			l.advance() // /
			return
		}
		l.advance()
	}
}

func (l *lexer) next() token {
	for {
		l.skipWhitespace()
		if l.pos >= len(l.src) {
			return token{typ: tokEOF, line: l.line, col: l.col}
		}

		// Check for comments.
		if l.src[l.pos] == '/' && l.pos+1 < len(l.src) {
			if l.src[l.pos+1] == '/' {
				l.advance()
				l.advance()
				l.skipLineComment()
				continue
			}
			if l.src[l.pos+1] == '*' {
				l.advance()
				l.advance()
				l.skipBlockComment()
				continue
			}
		}
		break
	}

	startLine, startCol := l.line, l.col
	ch := l.peek()

	// Single-character tokens.
	switch ch {
	case '(':
		l.advance()
		return token{typ: tokLPAREN, literal: "(", line: startLine, col: startCol}
	case ')':
		l.advance()
		return token{typ: tokRPAREN, literal: ")", line: startLine, col: startCol}
	case '[':
		l.advance()
		return token{typ: tokLBRACKET, literal: "[", line: startLine, col: startCol}
	case ']':
		l.advance()
		return token{typ: tokRBRACKET, literal: "]", line: startLine, col: startCol}
	case '{':
		l.advance()
		return token{typ: tokLBRACE, literal: "{", line: startLine, col: startCol}
	case '}':
		l.advance()
		return token{typ: tokRBRACE, literal: "}", line: startLine, col: startCol}
	case ';':
		l.advance()
		return token{typ: tokSEMICOLON, literal: ";", line: startLine, col: startCol}
	case ',':
		l.advance()
		return token{typ: tokCOMMA, literal: ",", line: startLine, col: startCol}
	case '=':
		l.advance()
		return token{typ: tokEQUAL, literal: "=", line: startLine, col: startCol}
	case '+':
		l.advance()
		return token{typ: tokPLUS, literal: "+", line: startLine, col: startCol}
	case '-':
		l.advance()
		return token{typ: tokMINUS, literal: "-", line: startLine, col: startCol}
	case '*':
		l.advance()
		return token{typ: tokSTAR, literal: "*", line: startLine, col: startCol}
	case '/':
		l.advance()
		return token{typ: tokSLASH, literal: "/", line: startLine, col: startCol}
	}

	// Number: starts with digit or '.'
	if ch >= '0' && ch <= '9' || ch == '.' {
		return l.readNumber(startLine, startCol)
	}

	// Identifier or keyword.
	if isIdentStart(ch) {
		return l.readIdent(startLine, startCol)
	}

	l.advance()
	return token{typ: tokILLEGAL, literal: string(ch), line: startLine, col: startCol}
}

func (l *lexer) readNumber(line, col int) token {
	start := l.pos
	// Integer or decimal part.
	for l.pos < len(l.src) && (l.src[l.pos] >= '0' && l.src[l.pos] <= '9') {
		l.advance()
	}
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		l.advance()
		for l.pos < len(l.src) && (l.src[l.pos] >= '0' && l.src[l.pos] <= '9') {
			l.advance()
		}
	}
	// Scientific notation.
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.advance()
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.advance()
		}
		for l.pos < len(l.src) && (l.src[l.pos] >= '0' && l.src[l.pos] <= '9') {
			l.advance()
		}
	}

	lit := string(l.src[start:l.pos])

	// Check for duration suffix 's' (not followed by identifier character).
	if l.pos < len(l.src) && l.src[l.pos] == 's' {
		if l.pos+1 >= len(l.src) || !isIdentContinue(l.src[l.pos+1]) {
			l.advance() // consume 's'
			return token{typ: tokDURATION, literal: lit, line: line, col: col}
		}
	}

	return token{typ: tokNUMBER, literal: lit, line: line, col: col}
}

func (l *lexer) readIdent(line, col int) token {
	start := l.pos
	l.advance() // first char already validated
	for l.pos < len(l.src) && isIdentContinue(l.src[l.pos]) {
		l.advance()
	}
	lit := string(l.src[start:l.pos])
	if typ, ok := keywords[lit]; ok {
		return token{typ: typ, literal: lit, line: line, col: col}
	}
	return token{typ: tokIDENT, literal: lit, line: line, col: col}
}

func isIdentStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isIdentContinue(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}
