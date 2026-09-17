package expression

import (
	"errors"
	"math"
	"testing"
)

func TestObservedFixtureExpressions(t *testing.T) {
	tests := []struct {
		equation string
		x        float64
		want     float64
	}{
		{"X", 12, 12}, {"X*1", 12, 12}, {"X*0.1", 12, 1.2},
		{"X*0.01", 12, 0.12}, {"X/10", 12, 1.2},
		{"X/4096*14.7", 4096, 14.7}, {"X*0.03125/3.6", 115.2, 1},
		{"X*3*0.145038", 2, 0.870228}, {"X*100/65536", 65536, 100},
	}
	for _, test := range tests {
		t.Run(test.equation, func(t *testing.T) {
			expression, err := Parse(test.equation)
			if err != nil {
				t.Fatal(err)
			}
			got, err := expression.Eval(test.x)
			if err != nil {
				t.Fatal(err)
			}
			if !close(got, test.want) {
				t.Errorf("Eval(%v) = %.12g, want %.12g", test.x, got, test.want)
			}
			raw, err := expression.Invert(got)
			if err != nil {
				t.Fatal(err)
			}
			if !close(raw, test.x) {
				t.Errorf("Invert(%v) = %.12g, want %.12g", got, raw, test.x)
			}
		})
	}
}

func TestPrecedenceParenthesesUnaryAndExponent(t *testing.T) {
	tests := map[string]float64{
		"1+2*3":        7,
		"(1+2)*3":      9,
		"--X + -(2)":   3,
		".5X":          0, // invalid, checked separately below
		"1e2 + X / +2": 102.5,
	}
	for equation, want := range tests {
		expression, err := Parse(equation)
		if equation == ".5X" {
			if !errors.Is(err, ErrSyntax) {
				t.Errorf("Parse(%q) error = %v", equation, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): %v", equation, err)
			continue
		}
		got, err := expression.Eval(5)
		if err != nil || got != want {
			t.Errorf("%q = %v, %v; want %v", equation, got, err, want)
		}
	}
}

func TestAffineCoefficients(t *testing.T) {
	expression, err := Parse("(X*2 + 6) / 4 - 1")
	if err != nil {
		t.Fatal(err)
	}
	scale, offset, err := expression.Affine()
	if err != nil {
		t.Fatal(err)
	}
	if scale != 0.5 || offset != 0.5 {
		t.Fatalf("Affine = %v*X + %v", scale, offset)
	}
	if got, err := expression.Invert(10.5); err != nil || got != 20 {
		t.Fatalf("Invert = %v, %v", got, err)
	}
}

func TestNonAffineAndNonInvertible(t *testing.T) {
	for _, equation := range []string{"X*X", "X/(X+1)"} {
		expression, err := Parse(equation)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := expression.Affine(); !errors.Is(err, ErrNonAffine) {
			t.Errorf("Affine(%q) error = %v", equation, err)
		}
	}
	constant, err := Parse("42")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := constant.Invert(42); !errors.Is(err, ErrNotInvertible) {
		t.Errorf("constant inversion error = %v", err)
	}
}

func TestEvaluationErrors(t *testing.T) {
	division, _ := Parse("1/(X-X)")
	if _, err := division.Eval(2); !errors.Is(err, ErrDivisionByZero) {
		t.Errorf("division error = %v", err)
	}
	if _, _, err := division.Affine(); !errors.Is(err, ErrDivisionByZero) {
		t.Errorf("affine division error = %v", err)
	}
	expression, _ := Parse("X*1e308")
	if _, err := expression.Eval(2); !errors.Is(err, ErrNonFinite) {
		t.Errorf("overflow error = %v", err)
	}
	if _, err := expression.Eval(math.Inf(1)); !errors.Is(err, ErrNonFinite) {
		t.Errorf("non-finite input error = %v", err)
	}
}

func TestSyntaxErrors(t *testing.T) {
	for _, equation := range []string{"", " ", "x", "X+", "()", "(X", "X)", "1..2", "1e", "2 X", "X%2", "NaN"} {
		if _, err := Parse(equation); !errors.Is(err, ErrSyntax) {
			t.Errorf("Parse(%q) error = %v, want ErrSyntax", equation, err)
		}
	}
}

func TestQuantizedRoundTrip(t *testing.T) {
	expression, err := Parse("X*0.1+1")
	if err != nil {
		t.Fatal(err)
	}
	for raw := 0.0; raw <= 65535; raw += 257 {
		display, err := expression.Eval(raw)
		if err != nil {
			t.Fatal(err)
		}
		inverted, err := expression.Invert(display)
		if err != nil {
			t.Fatal(err)
		}
		quantized := math.Round(inverted)
		if math.Abs(quantized-raw) > 0.5 {
			t.Fatalf("raw %.0f round-tripped to %.0f", raw, quantized)
		}
	}
}

func close(left, right float64) bool {
	return math.Abs(left-right) <= 1e-9*math.Max(1, math.Abs(right))
}
