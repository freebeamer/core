package expression

import (
	"fmt"
	"math"
)

func (e *Expression) Eval(x float64) (float64, error) {
	if e == nil || e.root == nil {
		return 0, fmt.Errorf("%w: nil expression", ErrSyntax)
	}
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0, ErrNonFinite
	}
	value, err := e.root.eval(x)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, ErrNonFinite
	}
	return value, nil
}

func (n *node) eval(x float64) (float64, error) {
	switch n.kind {
	case literalNode:
		return n.value, nil
	case variableNode:
		return x, nil
	case negateNode:
		value, err := n.left.eval(x)
		return -value, err
	}
	left, err := n.left.eval(x)
	if err != nil {
		return 0, err
	}
	right, err := n.right.eval(x)
	if err != nil {
		return 0, err
	}
	switch n.kind {
	case addNode:
		return left + right, nil
	case subtractNode:
		return left - right, nil
	case multiplyNode:
		return left * right, nil
	case divideNode:
		if right == 0 {
			return 0, ErrDivisionByZero
		}
		return left / right, nil
	default:
		return 0, fmt.Errorf("%w: unknown AST node", ErrSyntax)
	}
}
