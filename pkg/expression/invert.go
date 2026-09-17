package expression

import (
	"fmt"
	"math"
)

// Affine returns scale and offset such that the expression equals scale*X+offset.
func (e *Expression) Affine() (scale, offset float64, err error) {
	if e == nil || e.root == nil {
		return 0, 0, fmt.Errorf("%w: nil expression", ErrSyntax)
	}
	scale, offset, err = e.root.affine()
	if err == nil && (!finite(scale) || !finite(offset)) {
		err = ErrNonFinite
	}
	return
}

// Invert returns X for the requested expression output.
func (e *Expression) Invert(output float64) (float64, error) {
	if !finite(output) {
		return 0, ErrNonFinite
	}
	scale, offset, err := e.Affine()
	if err != nil {
		return 0, err
	}
	if scale == 0 {
		return 0, ErrNotInvertible
	}
	value := (output - offset) / scale
	if !finite(value) {
		return 0, ErrNonFinite
	}
	return value, nil
}

func (n *node) affine() (float64, float64, error) {
	switch n.kind {
	case literalNode:
		return 0, n.value, nil
	case variableNode:
		return 1, 0, nil
	case negateNode:
		a, b, err := n.left.affine()
		return -a, -b, err
	}
	leftA, leftB, err := n.left.affine()
	if err != nil {
		return 0, 0, err
	}
	rightA, rightB, err := n.right.affine()
	if err != nil {
		return 0, 0, err
	}
	switch n.kind {
	case addNode:
		return leftA + rightA, leftB + rightB, nil
	case subtractNode:
		return leftA - rightA, leftB - rightB, nil
	case multiplyNode:
		if leftA != 0 && rightA != 0 {
			return 0, 0, ErrNonAffine
		}
		return leftA*rightB + rightA*leftB, leftB * rightB, nil
	case divideNode:
		if rightA != 0 {
			return 0, 0, ErrNonAffine
		}
		if rightB == 0 {
			return 0, 0, ErrDivisionByZero
		}
		return leftA / rightB, leftB / rightB, nil
	default:
		return 0, 0, fmt.Errorf("%w: unknown AST node", ErrSyntax)
	}
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
