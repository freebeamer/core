package expression_test

import (
	"errors"
	"github.com/freebeamer/core/pkg/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/xdf"
)

func TestEveryPrivateBMWMathExpression(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "private", "00000FEF81A001.xdf")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		t.Skip("private BMW XDF fixture not installed")
	} else if err != nil {
		t.Fatal(err)
	}
	definition, err := xdf.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, table := range definition.Tables {
		for _, axis := range []types.XDFAxis{table.X, table.Y, table.Z} {
			if axis.Formula == "" {
				continue
			}
			count++
			parsed, parseErr := expression.Parse(axis.Formula)
			if parseErr != nil {
				t.Errorf("map %q axis %q equation %q: %v", table.Title, axis.ID, axis.Formula, parseErr)
				continue
			}
			if _, _, affineErr := parsed.Affine(); affineErr != nil {
				t.Errorf("map %q axis %q equation %q cannot be inverted: %v", table.Title, axis.ID, axis.Formula, affineErr)
			}
		}
	}
	for _, flag := range definition.Flags {
		if flag.Formula == "" {
			continue
		}
		count++
		parsed, parseErr := expression.Parse(flag.Formula)
		if parseErr != nil {
			t.Errorf("flag %q equation %q: %v", flag.Title, flag.Formula, parseErr)
			continue
		}
		if _, _, affineErr := parsed.Affine(); affineErr != nil {
			t.Errorf("flag %q equation %q cannot be inverted: %v", flag.Title, flag.Formula, affineErr)
		}
	}
	if count == 0 {
		t.Fatal("private XDF contains no parsed MATH expressions")
	}
}

func TestEveryPublicFixtureMathExpression(t *testing.T) {
	definition, err := xdf.Load(filepath.Join("..", "xdf", "testdata", "basic.xdf"))
	if err != nil {
		t.Fatal(err)
	}
	formulas := []string{definition.Tables[0].X.Formula, definition.Tables[0].Z.Formula, definition.Flags[0].Formula}
	for _, formula := range formulas {
		parsed, parseErr := expression.Parse(formula)
		if parseErr != nil {
			t.Errorf("equation %q: %v", formula, parseErr)
			continue
		}
		if _, _, affineErr := parsed.Affine(); affineErr != nil {
			t.Errorf("equation %q cannot be inverted: %v", formula, affineErr)
		}
	}
}
