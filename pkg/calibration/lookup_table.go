package calibration

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/types"
)

var (
	ErrEmptyLookupTable         = errors.New("lookup table conversion has no points")
	ErrValueOutsideLookupTable  = errors.New("value is outside the lookup table's range")
	ErrValueNotInLookupTable    = errors.New("value has no exact match in the lookup table")
	ErrLookupTableNotInvertible = errors.New("lookup table output is not monotonic")
)

// conversionEvaluator abstracts over freehorse-expression-v1
// (*expression.Expression, used as is) and freehorse-lookup-table-v1
// (*lookupTable) conversions so decode/encode code doesn't need to branch
// on which one it's using.
type conversionEvaluator interface {
	Eval(raw float64) (float64, error)
	Invert(output float64) (float64, error)
}

// lookupTable evaluates a "freehorse-lookup-table-v1" conversion (mapdef
// 1.3+): A2L TAB_INTP/TAB_NOINTP converts to this, since a piecewise
// numeric lookup has no representation as a freehorse-expression-v1
// arithmetic expression. points is always sorted by Input ascending.
type lookupTable struct {
	points       []types.ConversionPoint
	interpolated bool
	defaultValue float64
	hasDefault   bool
}

func newLookupTable(conversion types.Conversion) (*lookupTable, error) {
	if len(conversion.Points) == 0 {
		return nil, ErrEmptyLookupTable
	}
	points := append([]types.ConversionPoint(nil), conversion.Points...)
	sort.Slice(points, func(i, j int) bool { return points[i].Input < points[j].Input })
	table := &lookupTable{points: points, interpolated: conversion.Interpolated}
	if conversion.Default != nil {
		table.defaultValue = *conversion.Default
		table.hasDefault = true
	}
	return table, nil
}

// Eval maps a raw (stored) value to its physical value: an exact match
// against Input for a non-interpolated table (A2L TAB_NOINTP), or linear
// interpolation between the two bracketing points for an interpolated one
// (A2L TAB_INTP). A raw value with no exact match (non-interpolated) or
// outside the table's covered Input range (interpolated) falls back to
// Default when set (A2L DEFAULT_VALUE_NUMERIC), else is an error.
func (t *lookupTable) Eval(raw float64) (float64, error) {
	if !finiteValue(raw) {
		return 0, expression.ErrNonFinite
	}
	if !t.interpolated {
		for _, point := range t.points {
			if point.Input == raw {
				return point.Output, nil
			}
		}
		if t.hasDefault {
			return t.defaultValue, nil
		}
		return 0, fmt.Errorf("%w: %g", ErrValueNotInLookupTable, raw)
	}
	first, last := t.points[0], t.points[len(t.points)-1]
	if raw < first.Input || raw > last.Input {
		if t.hasDefault {
			return t.defaultValue, nil
		}
		return 0, fmt.Errorf("%w: %g", ErrValueOutsideLookupTable, raw)
	}
	for i := 0; i < len(t.points)-1; i++ {
		low, high := t.points[i], t.points[i+1]
		if raw < low.Input || raw > high.Input {
			continue
		}
		if high.Input == low.Input {
			return low.Output, nil
		}
		fraction := (raw - low.Input) / (high.Input - low.Input)
		return low.Output + fraction*(high.Output-low.Output), nil
	}
	return last.Output, nil
}

// Invert maps a physical value back to its raw (stored) value: the exact
// inverse of Eval, valid only when the table's Output values are
// monotonic in Input order (otherwise which raw value a given physical
// value should invert to is ambiguous, so this reports
// ErrLookupTableNotInvertible rather than guessing). Default is never
// consulted here: it is a decode-side fallback for values Eval can't
// otherwise resolve, not something meaningful to invert from.
func (t *lookupTable) Invert(output float64) (float64, error) {
	if !finiteValue(output) {
		return 0, expression.ErrNonFinite
	}
	increasing, decreasing := true, true
	for i := 1; i < len(t.points); i++ {
		if t.points[i].Output <= t.points[i-1].Output {
			increasing = false
		}
		if t.points[i].Output >= t.points[i-1].Output {
			decreasing = false
		}
	}
	if !increasing && !decreasing {
		return 0, ErrLookupTableNotInvertible
	}
	within := func(lowOutput, highOutput float64) bool {
		if increasing {
			return output >= lowOutput && output <= highOutput
		}
		return output <= lowOutput && output >= highOutput
	}
	if !t.interpolated {
		for _, point := range t.points {
			if point.Output == output {
				return point.Input, nil
			}
		}
		return 0, fmt.Errorf("%w: %g", ErrValueNotInLookupTable, output)
	}
	first, last := t.points[0], t.points[len(t.points)-1]
	if !within(first.Output, last.Output) {
		return 0, fmt.Errorf("%w: %g", ErrValueOutsideLookupTable, output)
	}
	for i := 0; i < len(t.points)-1; i++ {
		low, high := t.points[i], t.points[i+1]
		if !within(low.Output, high.Output) {
			continue
		}
		if high.Output == low.Output {
			return low.Input, nil
		}
		fraction := (output - low.Output) / (high.Output - low.Output)
		return low.Input + fraction*(high.Input-low.Input), nil
	}
	return last.Input, nil
}

func finiteValue(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
