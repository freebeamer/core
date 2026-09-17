package calibration

import (
	"errors"
	"math"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func floatPtr(value float64) *float64 { return &value }

func interpolatedTableConversion(defaultValue *float64) types.Conversion {
	return types.Conversion{
		Language:     "freehorse-lookup-table-v1",
		Interpolated: true,
		Points: []types.ConversionPoint{
			{Input: 0, Output: 100},
			{Input: 10, Output: 110},
			{Input: 20, Output: 130},
		},
		Default: defaultValue,
	}
}

func steppedTableConversion(defaultValue *float64) types.Conversion {
	return types.Conversion{
		Language: "freehorse-lookup-table-v1",
		Points: []types.ConversionPoint{
			{Input: 0, Output: 100},
			{Input: 10, Output: 110},
			{Input: 20, Output: 130},
		},
		Default: defaultValue,
	}
}

func TestLookupTableInterpolatedEval(t *testing.T) {
	table, err := newLookupTable(interpolatedTableConversion(nil))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		raw  float64
		want float64
	}{
		{0, 100}, {10, 110}, {20, 130}, {5, 105}, {15, 120},
	}
	for _, test := range tests {
		got, err := table.Eval(test.raw)
		if err != nil {
			t.Fatalf("Eval(%v) error = %v", test.raw, err)
		}
		if got != test.want {
			t.Errorf("Eval(%v) = %v, want %v", test.raw, got, test.want)
		}
	}
}

func TestLookupTableInterpolatedInvert(t *testing.T) {
	table, err := newLookupTable(interpolatedTableConversion(nil))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		output float64
		want   float64
	}{
		{100, 0}, {110, 10}, {130, 20}, {105, 5}, {120, 15},
	}
	for _, test := range tests {
		got, err := table.Invert(test.output)
		if err != nil {
			t.Fatalf("Invert(%v) error = %v", test.output, err)
		}
		if got != test.want {
			t.Errorf("Invert(%v) = %v, want %v", test.output, got, test.want)
		}
	}
}

func TestLookupTableInterpolatedOutOfRangeUsesDefaultOrErrors(t *testing.T) {
	withoutDefault, err := newLookupTable(interpolatedTableConversion(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := withoutDefault.Eval(-1); !errors.Is(err, ErrValueOutsideLookupTable) {
		t.Fatalf("error = %v, want ErrValueOutsideLookupTable", err)
	}
	if _, err := withoutDefault.Eval(21); !errors.Is(err, ErrValueOutsideLookupTable) {
		t.Fatalf("error = %v, want ErrValueOutsideLookupTable", err)
	}

	withDefault, err := newLookupTable(interpolatedTableConversion(floatPtr(999)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := withDefault.Eval(-1)
	if err != nil || got != 999 {
		t.Fatalf("Eval(-1) = %v, %v, want 999, nil", got, err)
	}
}

func TestLookupTableSteppedEvalExactMatchOnly(t *testing.T) {
	table, err := newLookupTable(steppedTableConversion(nil))
	if err != nil {
		t.Fatal(err)
	}
	got, err := table.Eval(10)
	if err != nil || got != 110 {
		t.Fatalf("Eval(10) = %v, %v, want 110, nil", got, err)
	}
	if _, err := table.Eval(5); !errors.Is(err, ErrValueNotInLookupTable) {
		t.Fatalf("error = %v, want ErrValueNotInLookupTable", err)
	}

	withDefault, err := newLookupTable(steppedTableConversion(floatPtr(-1)))
	if err != nil {
		t.Fatal(err)
	}
	got, err = withDefault.Eval(5)
	if err != nil || got != -1 {
		t.Fatalf("Eval(5) with default = %v, %v, want -1, nil", got, err)
	}
}

func TestLookupTableSteppedInvertExactMatchOnly(t *testing.T) {
	table, err := newLookupTable(steppedTableConversion(nil))
	if err != nil {
		t.Fatal(err)
	}
	got, err := table.Invert(110)
	if err != nil || got != 10 {
		t.Fatalf("Invert(110) = %v, %v, want 10, nil", got, err)
	}
	if _, err := table.Invert(105); !errors.Is(err, ErrValueNotInLookupTable) {
		t.Fatalf("error = %v, want ErrValueNotInLookupTable", err)
	}
}

func TestLookupTableRejectsNonMonotonicInvert(t *testing.T) {
	table, err := newLookupTable(types.Conversion{
		Language:     "freehorse-lookup-table-v1",
		Interpolated: true,
		Points: []types.ConversionPoint{
			{Input: 0, Output: 100},
			{Input: 10, Output: 90},  // decreasing here...
			{Input: 20, Output: 130}, // ...then increasing: not monotonic.
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Eval still works: forward lookup never needed monotonicity.
	if got, err := table.Eval(5); err != nil || got != 95 {
		t.Fatalf("Eval(5) = %v, %v, want 95, nil", got, err)
	}
	if _, err := table.Invert(95); !errors.Is(err, ErrLookupTableNotInvertible) {
		t.Fatalf("error = %v, want ErrLookupTableNotInvertible", err)
	}
}

func TestLookupTableHandlesDecreasingOutput(t *testing.T) {
	table, err := newLookupTable(types.Conversion{
		Language:     "freehorse-lookup-table-v1",
		Interpolated: true,
		Points: []types.ConversionPoint{
			{Input: 0, Output: 100},
			{Input: 10, Output: 50},
			{Input: 20, Output: 0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := table.Eval(5)
	if err != nil || got != 75 {
		t.Fatalf("Eval(5) = %v, %v, want 75, nil", got, err)
	}
	inverted, err := table.Invert(75)
	if err != nil || inverted != 5 {
		t.Fatalf("Invert(75) = %v, %v, want 5, nil", inverted, err)
	}
}

func TestLookupTableUnsortedPointsAreSortedByInput(t *testing.T) {
	table, err := newLookupTable(types.Conversion{
		Language:     "freehorse-lookup-table-v1",
		Interpolated: true,
		Points: []types.ConversionPoint{
			{Input: 20, Output: 130},
			{Input: 0, Output: 100},
			{Input: 10, Output: 110},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := table.Eval(5)
	if err != nil || got != 105 {
		t.Fatalf("Eval(5) = %v, %v, want 105, nil", got, err)
	}
}

func TestLookupTableRejectsEmptyPoints(t *testing.T) {
	if _, err := newLookupTable(types.Conversion{Language: "freehorse-lookup-table-v1"}); !errors.Is(err, ErrEmptyLookupTable) {
		t.Fatalf("error = %v, want ErrEmptyLookupTable", err)
	}
}

func TestLookupTableRejectsNonFiniteInput(t *testing.T) {
	table, err := newLookupTable(interpolatedTableConversion(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := table.Eval(math.NaN()); err == nil {
		t.Fatal("expected error for NaN input")
	}
}
