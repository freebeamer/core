package calibration

import (
	"errors"
	"math"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func FuzzAddressAndDimensions(f *testing.F) {
	for _, seed := range []struct {
		base          uint64
		index, size   int
		rows, columns int
	}{
		{0, 0, 1, 1, 1},
		{0x18171c, 35, 2, 6, 6},
		{math.MaxUint64, 1, 1, math.MaxInt, 2},
		{1, -1, 0, -1, 0},
	} {
		f.Add(seed.base, seed.index, seed.size, seed.rows, seed.columns)
	}

	f.Fuzz(func(t *testing.T, base uint64, index, size, rows, columns int) {
		offset, err := elementOffset(base, index, size)
		if err == nil {
			if index < 0 || size <= 0 || offset < base ||
				(offset-base)/uint64(size) != uint64(index) {
				t.Fatalf("invalid offset: base=%d index=%d size=%d offset=%d", base, index, size, offset)
			}
		} else if !errors.Is(err, ErrAddressOverflow) {
			t.Fatalf("unexpected offset error: %v", err)
		}

		layout := types.DataLayout{Dimensions: []int{rows, columns}}
		gotRows, gotColumns, err := dimensions(layout)
		if err == nil {
			if gotRows <= 0 || gotColumns <= 0 || gotRows > math.MaxInt/gotColumns {
				t.Fatalf("invalid dimensions: %d x %d", gotRows, gotColumns)
			}
		} else if !errors.Is(err, ErrInvalidDimensions) {
			t.Fatalf("unexpected dimension error: %v", err)
		}
	})
}
