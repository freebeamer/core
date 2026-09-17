package a2l

import (
	"bytes"
	"fmt"
)

// A2L is a custom keyword-block text format, not XML, so it needs its own
// small hand-written lexer rather than encoding/xml. See docs/a2l-v0-plan.md
// for the grammar subset this package understands.

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokBegin
	tokEnd
	tokIdent
	tokString
	tokNumber
	tokPunct
)

type token struct {
	kind tokenKind
	text string
	line int
}

func (t token) describe() string {
	switch t.kind {
	case tokEOF:
		return "end of file"
	case tokBegin:
		return "'/begin'"
	case tokEnd:
		return "'/end'"
	case tokString:
		return fmt.Sprintf("string %q", t.text)
	default:
		return fmt.Sprintf("%q", t.text)
	}
}

type lexer struct {
	data []byte
	pos  int
	line int
}

func newLexer(data []byte) *lexer {
	return &lexer{data: data, line: 1}
}

// tokenize reads every token up to and including a trailing EOF token.
// Characters outside the grammar this package parses (for example inside an
// A2ML or IF_DATA block, which use their own brace-based syntax) become
// single-character tokPunct tokens rather than lexer errors, so an unknown
// block can still be skipped structurally by counting /begin and /end.
func tokenize(data []byte) ([]token, error) {
	lex := newLexer(data)
	var tokens []token
	for {
		tok, err := lex.next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		if tok.kind == tokEOF {
			return tokens, nil
		}
	}
}

func (l *lexer) next() (token, error) {
	l.skipWhitespaceAndComments()
	if l.pos >= len(l.data) {
		return token{kind: tokEOF, line: l.line}, nil
	}
	line := l.line
	c := l.data[l.pos]
	switch {
	case c == '"':
		return l.readString(line)
	case c == '/':
		return l.readSlash(line)
	case c == '-' || isDigit(c):
		return l.readNumber(line)
	case isIdentStart(c):
		return l.readIdent(line)
	default:
		l.pos++
		return token{kind: tokPunct, text: string(c), line: line}, nil
	}
}

func (l *lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.data) {
		c := l.data[l.pos]
		switch {
		case c == '\n':
			l.line++
			l.pos++
		case c == ' ' || c == '\t' || c == '\r':
			l.pos++
		case c == '/' && l.pos+1 < len(l.data) && l.data[l.pos+1] == '*':
			l.pos += 2
			for l.pos < len(l.data) && !(l.data[l.pos] == '*' && l.pos+1 < len(l.data) && l.data[l.pos+1] == '/') {
				if l.data[l.pos] == '\n' {
					l.line++
				}
				l.pos++
			}
			if l.pos+1 < len(l.data) {
				l.pos += 2
			} else {
				l.pos = len(l.data)
			}
		default:
			return
		}
	}
}

func (l *lexer) readString(line int) (token, error) {
	l.pos++
	start := l.pos
	for l.pos < len(l.data) && l.data[l.pos] != '"' {
		if l.data[l.pos] == '\n' {
			l.line++
		}
		l.pos++
	}
	if l.pos >= len(l.data) {
		return token{}, fmt.Errorf("a2l: unterminated string starting at line %d", line)
	}
	text := string(l.data[start:l.pos])
	l.pos++
	return token{kind: tokString, text: text, line: line}, nil
}

func (l *lexer) readSlash(line int) (token, error) {
	rest := l.data[l.pos:]
	if bytes.HasPrefix(rest, []byte("/begin")) && !isIdentChar(byteAt(rest, len("/begin"))) {
		l.pos += len("/begin")
		return token{kind: tokBegin, line: line}, nil
	}
	if bytes.HasPrefix(rest, []byte("/end")) && !isIdentChar(byteAt(rest, len("/end"))) {
		l.pos += len("/end")
		return token{kind: tokEnd, line: line}, nil
	}
	l.pos++
	return token{kind: tokPunct, text: "/", line: line}, nil
}

func byteAt(data []byte, index int) byte {
	if index >= len(data) {
		return 0
	}
	return data[index]
}

func (l *lexer) readNumber(line int) (token, error) {
	start := l.pos
	if l.data[l.pos] == '-' {
		l.pos++
	}
	for l.pos < len(l.data) && isNumberChar(l.data[l.pos]) {
		l.pos++
	}
	return token{kind: tokNumber, text: string(l.data[start:l.pos]), line: line}, nil
}

func (l *lexer) readIdent(line int) (token, error) {
	start := l.pos
	for l.pos < len(l.data) && isIdentChar(l.data[l.pos]) {
		l.pos++
	}
	return token{kind: tokIdent, text: string(l.data[start:l.pos]), line: line}, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isNumberChar(c byte) bool {
	return isDigit(c) || c == '.' || c == 'x' || c == 'X' ||
		(c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == 'e' || c == 'E' || c == '+' || c == '-'
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '.'
}
