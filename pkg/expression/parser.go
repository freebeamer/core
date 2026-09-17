// Package expression parses and evaluates the deliberately small arithmetic
// expression subset used by FreeBeamer V0 XDF definitions.
package expression

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"unicode"
)

var (
	ErrSyntax         = errors.New("invalid expression")
	ErrDivisionByZero = errors.New("division by zero")
	ErrNonFinite      = errors.New("expression result is not finite")
	ErrNonAffine      = errors.New("expression is not affine in X")
	ErrNotInvertible  = errors.New("expression is not invertible")
)

type Expression struct {
	source string
	root   *node
}

type nodeKind uint8

const (
	literalNode nodeKind = iota
	variableNode
	addNode
	subtractNode
	multiplyNode
	divideNode
	negateNode
)

type node struct {
	kind  nodeKind
	value float64
	left  *node
	right *node
}

type parser struct {
	input string
	pos   int
}

func Parse(input string) (*Expression, error) {
	p := parser{input: input}
	root, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if root == nil || p.pos != len(p.input) {
		return nil, p.syntax("unexpected input")
	}
	return &Expression{source: input, root: root}, nil
}

func (p *parser) parseAdditive() (*node, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if p.pos == len(p.input) || (p.input[p.pos] != '+' && p.input[p.pos] != '-') {
			return left, nil
		}
		operator := p.input[p.pos]
		p.pos++
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		kind := addNode
		if operator == '-' {
			kind = subtractNode
		}
		left = &node{kind: kind, left: left, right: right}
	}
}

func (p *parser) parseMultiplicative() (*node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if p.pos == len(p.input) || (p.input[p.pos] != '*' && p.input[p.pos] != '/') {
			return left, nil
		}
		operator := p.input[p.pos]
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		kind := multiplyNode
		if operator == '/' {
			kind = divideNode
		}
		left = &node{kind: kind, left: left, right: right}
	}
}

func (p *parser) parseUnary() (*node, error) {
	p.skipSpace()
	if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
		operator := p.input[p.pos]
		p.pos++
		child, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		if operator == '+' {
			return child, nil
		}
		return &node{kind: negateNode, left: child}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (*node, error) {
	p.skipSpace()
	if p.pos == len(p.input) {
		return nil, p.syntax("expected value")
	}
	if p.input[p.pos] == '(' {
		p.pos++
		value, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos == len(p.input) || p.input[p.pos] != ')' {
			return nil, p.syntax("expected closing parenthesis")
		}
		p.pos++
		return value, nil
	}
	if p.input[p.pos] == 'X' {
		p.pos++
		return &node{kind: variableNode}, nil
	}
	if isDigit(p.input[p.pos]) || p.input[p.pos] == '.' {
		return p.parseNumber()
	}
	return nil, p.syntax("expected number, X, or parenthesis")
}

func (p *parser) parseNumber() (*node, error) {
	start := p.pos
	digits := 0
	for p.pos < len(p.input) && isDigit(p.input[p.pos]) {
		p.pos++
		digits++
	}
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		p.pos++
		for p.pos < len(p.input) && isDigit(p.input[p.pos]) {
			p.pos++
			digits++
		}
	}
	if digits == 0 {
		return nil, p.syntax("invalid numeric literal")
	}
	if p.pos < len(p.input) && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			p.pos++
		}
		exponentStart := p.pos
		for p.pos < len(p.input) && isDigit(p.input[p.pos]) {
			p.pos++
		}
		if exponentStart == p.pos {
			return nil, p.syntax("invalid numeric exponent")
		}
	}
	value, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return nil, p.syntax("invalid numeric literal")
	}
	return &node{kind: literalNode, value: value}, nil
}

func (p *parser) skipSpace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}
func (p *parser) syntax(message string) error {
	return fmt.Errorf("%w at byte %d: %s", ErrSyntax, p.pos, message)
}
func isDigit(value byte) bool { return value >= '0' && value <= '9' }
