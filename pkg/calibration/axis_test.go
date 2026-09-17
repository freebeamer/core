package calibration

import (
	"errors"
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

// baseParameter returns a minimal scalar MapParameter (its own Z data is
// irrelevant to these tests, which only exercise decodeAxis via
// DecodeParameter's "x" axis handling) with the given axis attached.
func baseParameterWithAxis(axis types.MapAxis) types.MapParameter {
	address := uint64(0x100)
	return types.MapParameter{
		ID:   "p",
		Kind: "curve",
		Name: "P",
		Layout: types.DataLayout{
			Address: &address, DataType: "uint8", ByteOrder: "not-applicable",
			Dimensions: []int{1}, Order: "not-applicable",
		},
		Conversion: types.IdentityConversion(),
		Axes:       map[string]types.MapAxis{"x": axis},
	}
}

func TestDecodeAxisWithFixedPoints(t *testing.T) {
	axis := types.MapAxis{
		Count:       3,
		Conversion:  types.IdentityConversion(),
		FixedPoints: []int64{-1, 4, 6},
	}
	decoder, err := New(&types.MapDefinition{}, imageFor(t, make([]byte, 0x200)))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(baseParameterWithAxis(axis))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Raw.X, []int64{-1, 4, 6}) || !reflect.DeepEqual(decoded.X, []float64{-1, 4, 6}) {
		t.Fatalf("fixed axis decoded Raw.X=%v X=%v", decoded.Raw.X, decoded.X)
	}
	if decoded.Addresses.X != nil {
		t.Fatalf("fixed axis addresses = %v, want nil (no on-disk storage)", decoded.Addresses.X)
	}
}

func TestDecodeAxisWithFixedPointsAppliesConversion(t *testing.T) {
	axis := types.MapAxis{
		Count:       2,
		Conversion:  types.Conversion{Language: "freehorse-expression-v1", Expression: "X*0.5"},
		FixedPoints: []int64{10, 20},
	}
	decoder, err := New(&types.MapDefinition{}, imageFor(t, make([]byte, 0x200)))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(baseParameterWithAxis(axis))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.X, []float64{5, 10}) {
		t.Fatalf("converted fixed axis X = %v, want [5 10]", decoded.X)
	}
}

func TestDecodeAxisWithRescaleStride(t *testing.T) {
	// [pos0 val0 pos1 val1 pos2 val2] = [9 10 9 20 9 30]; the axis's own
	// Address (1) skips the first pair's leading position element, and a
	// StrideBits of 16 (2x the natural 8-bit width) skips every other
	// (position) element thereafter.
	data := make([]byte, 0x200)
	copy(data, []byte{9, 10, 9, 20, 9, 30})
	address := uint64(1)
	axis := types.MapAxis{
		Count:      3,
		Conversion: types.IdentityConversion(),
		Layout: &types.DataLayout{
			Address: &address, DataType: "uint8", ByteOrder: "not-applicable",
			Dimensions: []int{3}, Order: "not-applicable", StrideBits: []int{16},
		},
	}
	decoder, err := New(&types.MapDefinition{}, imageFor(t, data))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(baseParameterWithAxis(axis))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Raw.X, []int64{10, 20, 30}) {
		t.Fatalf("rescale axis decoded Raw.X = %v, want [10 20 30]", decoded.Raw.X)
	}
	if !reflect.DeepEqual(decoded.Addresses.X, []uint64{1, 3, 5}) {
		t.Fatalf("rescale axis addresses = %v, want [1 3 5]", decoded.Addresses.X)
	}
}

func TestDecodeAxisRejectsUnsupportedStride(t *testing.T) {
	tests := []struct {
		name       string
		strideBits []int
	}{
		{"not a whole byte", []int{12}},
		{"narrower than natural width", []int{4}},
		{"more than one non-zero entry", []int{16, 16}},
	}
	address := uint64(0)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			axis := types.MapAxis{
				Count:      1,
				Conversion: types.IdentityConversion(),
				Layout: &types.DataLayout{
					Address: &address, DataType: "uint8", ByteOrder: "not-applicable",
					Dimensions: []int{1}, Order: "not-applicable", StrideBits: test.strideBits,
				},
			}
			decoder, err := New(&types.MapDefinition{}, imageFor(t, make([]byte, 0x10)))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decoder.DecodeParameter(baseParameterWithAxis(axis)); !errors.Is(err, ErrUnsupportedStride) {
				t.Fatalf("error = %v, want ErrUnsupportedStride", err)
			}
		})
	}
}

// XDF's converter always sets a 2-entry [major, minor] StrideBits pair on
// every layout, [0, 0] when no real striding applies. This must keep
// decoding at the natural width, not be mistaken for an ambiguous
// multi-dimension stride.
func TestDecodeAxisToleratesAllZeroMultiEntryStride(t *testing.T) {
	address := uint64(0)
	axis := types.MapAxis{
		Count:      2,
		Conversion: types.IdentityConversion(),
		Layout: &types.DataLayout{
			Address: &address, DataType: "uint8", ByteOrder: "not-applicable",
			Dimensions: []int{2}, Order: "not-applicable", StrideBits: []int{0, 0},
		},
	}
	data := make([]byte, 0x200)
	copy(data, []byte{7, 8})
	decoder, err := New(&types.MapDefinition{}, imageFor(t, data))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(baseParameterWithAxis(axis))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Raw.X, []int64{7, 8}) {
		t.Fatalf("decoded Raw.X = %v, want [7 8]", decoded.Raw.X)
	}
}

func TestDecodeAxisWithNoLayoutAndNoFixedPointsIsEmpty(t *testing.T) {
	axis := types.MapAxis{Count: 3, Conversion: types.IdentityConversion()}
	decoder, err := New(&types.MapDefinition{}, imageFor(t, make([]byte, 0x200)))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(baseParameterWithAxis(axis))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.X != nil || decoded.Raw.X != nil {
		t.Fatalf("X = %v Raw.X = %v, want nil for an axis with neither Layout nor FixedPoints", decoded.X, decoded.Raw.X)
	}
}
