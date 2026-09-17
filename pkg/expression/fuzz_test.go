package expression

import "testing"

func FuzzParseAndEvaluate(f *testing.F) {
	for _, seed := range []struct {
		equation string
		x        float64
	}{
		{"X", 1},
		{"X*0.1+1", 65535},
		{"(X*2 + 6) / 4 - 1", -20},
		{"1/(X-X)", 2},
		{"X+", 0},
		{"", 0},
	} {
		f.Add(seed.equation, seed.x)
	}

	f.Fuzz(func(t *testing.T, equation string, x float64) {
		expression, err := Parse(equation)
		if err != nil {
			return
		}
		if expression == nil {
			t.Fatal("Parse returned a nil expression without an error")
		}
		value, evalErr := expression.Eval(x)
		_, _, _ = expression.Affine()
		if evalErr == nil {
			_, _ = expression.Invert(value)
		}
	})
}
