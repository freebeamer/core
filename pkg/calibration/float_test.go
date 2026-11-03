package calibration

import (
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/types"
)

func floatParameter(dataType, byteOrder string, dimensions []int, expr string) types.MapParameter {
	address := uint64(0)
	return types.MapParameter{
		ID:         "float",
		Kind:       "curve",
		Name:       "float",
		Layout:     types.DataLayout{Address: &address, DataType: dataType, ByteOrder: byteOrder, Dimensions: dimensions, Order: "row-major"},
		Conversion: types.Conversion{Language: "freehorse-expression-v1", Expression: expr},
	}
}

func floatDecoder(t *testing.T, parameter types.MapParameter, data []byte) *Decoder {
	t.Helper()
	decoder, err := New(&types.MapDefinition{Parameters: []types.MapParameter{parameter}}, imageFor(t, data))
	if err != nil {
		t.Fatal(err)
	}
	return decoder
}

func TestDecodeFloat32BothByteOrders(t *testing.T) {
	values := []float32{1.5, -0.25}
	for _, order := range []string{"big", "little"} {
		data := make([]byte, 8)
		for i, value := range values {
			if order == "big" {
				binary.BigEndian.PutUint32(data[i*4:], math.Float32bits(value))
			} else {
				binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(value))
			}
		}
		parameter := floatParameter("float32", order, []int{1, 2}, "X*2")
		got, err := floatDecoder(t, parameter, data).DecodeParameter(parameter)
		if err != nil {
			t.Fatalf("%s: %v", order, err)
		}
		if !reflect.DeepEqual(got.Raw.Z, [][]float64{{1.5, -0.25}}) || !reflect.DeepEqual(got.Z, [][]float64{{3, -0.5}}) {
			t.Fatalf("%s: raw/physical = %v/%v", order, got.Raw.Z, got.Z)
		}
		if !reflect.DeepEqual(got.Addresses.Z, [][]uint64{{0, 4}}) {
			t.Fatalf("%s: addresses = %v, want 4-byte elements", order, got.Addresses.Z)
		}
	}
}

func TestDecodeFloat64UsesEightByteElements(t *testing.T) {
	data := make([]byte, 16)
	binary.BigEndian.PutUint64(data[0:], math.Float64bits(1e300))
	binary.BigEndian.PutUint64(data[8:], math.Float64bits(-3.25))
	parameter := floatParameter("float64", "big", []int{1, 2}, "X")
	got, err := floatDecoder(t, parameter, data).DecodeParameter(parameter)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Z, [][]float64{{1e300, -3.25}}) || !reflect.DeepEqual(got.Addresses.Z, [][]uint64{{0, 8}}) {
		t.Fatalf("physical/addresses = %v/%v", got.Z, got.Addresses.Z)
	}
}

func TestDecodeFloatAxis(t *testing.T) {
	data := make([]byte, 9)
	binary.LittleEndian.PutUint32(data[0:], math.Float32bits(800))
	binary.LittleEndian.PutUint32(data[4:], math.Float32bits(1200.5))
	data[8] = 7
	axisAddress, zAddress := uint64(0), uint64(8)
	parameter := types.MapParameter{
		ID: "curve", Kind: "curve", Name: "curve",
		Layout:     types.DataLayout{Address: &zAddress, DataType: "uint8", ByteOrder: "not-applicable", Dimensions: []int{1, 1}, Order: "row-major"},
		Conversion: types.Conversion{Language: "freehorse-expression-v1", Expression: "X"},
		Axes: map[string]types.MapAxis{"x": {
			Count:      2,
			Layout:     &types.DataLayout{Address: &axisAddress, DataType: "float32", ByteOrder: "little", Dimensions: []int{2}, Order: "not-applicable"},
			Conversion: types.Conversion{Language: "freehorse-expression-v1", Expression: "X"},
		}},
	}
	got, err := floatDecoder(t, parameter, data).DecodeParameter(parameter)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.X, []float64{800, 1200.5}) || !reflect.DeepEqual(got.Addresses.X, []uint64{0, 4}) {
		t.Fatalf("axis = %v at %v", got.X, got.Addresses.X)
	}
}

func TestDecodeStoredNaNIsRejected(t *testing.T) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, math.Float32bits(float32(math.NaN())))
	parameter := floatParameter("float32", "big", []int{1, 1}, "X")
	if _, err := floatDecoder(t, parameter, data).DecodeParameter(parameter); !errors.Is(err, expression.ErrNonFinite) {
		t.Fatalf("error = %v, want ErrNonFinite", err)
	}
}

func TestFloatRejectsBitPosition(t *testing.T) {
	parameter := floatParameter("float32", "big", []int{1, 1}, "X")
	bit := 0
	parameter.Layout.BitPosition = &bit
	decoder := floatDecoder(t, parameter, make([]byte, 4))
	if _, err := decoder.DecodeParameter(parameter); !errors.Is(err, ErrFloatBitPosition) {
		t.Fatalf("decode error = %v, want ErrFloatBitPosition", err)
	}
	if _, err := decoder.SetIndex(parameter, 0, 1); !errors.Is(err, ErrFloatBitPosition) {
		t.Fatalf("edit error = %v, want ErrFloatBitPosition", err)
	}
}

func TestSetFloat32RoundsToStoredPrecision(t *testing.T) {
	parameter := floatParameter("float32", "little", []int{1, 1}, "X")
	decoder := floatDecoder(t, parameter, make([]byte, 4))
	result, err := decoder.SetIndex(parameter, 0, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	want := float64(float32(0.1))
	if result.Raw != want || result.Actual != want {
		t.Fatalf("raw/actual = %v/%v, want %v (nearest float32, not the requested 0.1)", result.Raw, result.Actual, want)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(result.Bytes)); got != float32(0.1) {
		t.Fatalf("stored bytes decode to %v", got)
	}
	if _, err := decoder.SetIndex(parameter, 0, 1e39); !errors.Is(err, ErrRawOutOfRange) {
		t.Fatalf("overflow error = %v, want ErrRawOutOfRange", err)
	}
}

func TestSetFloat64RoundTripsExactly(t *testing.T) {
	parameter := floatParameter("float64", "big", []int{1, 2}, "X/4")
	decoder := floatDecoder(t, parameter, make([]byte, 16))
	result, err := decoder.SetIndex(parameter, 1, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Raw != 0.4 || result.Actual != 0.1 || result.Offset != 8 || len(result.Bytes) != 8 {
		t.Fatalf("result = %#v", result)
	}
	decoded, err := decoder.DecodeParameter(parameter)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Z[0][0] != 0 || decoded.Z[0][1] != 0.1 {
		t.Fatalf("decoded = %v; the neighbouring cell must be untouched", decoded.Z)
	}
}

func TestSetIntegerStillRoundsAndRangeChecks(t *testing.T) {
	parameter := floatParameter("uint8", "not-applicable", []int{1, 1}, "X")
	decoder := floatDecoder(t, parameter, []byte{0})
	result, err := decoder.SetIndex(parameter, 0, 41.6)
	if err != nil {
		t.Fatal(err)
	}
	if result.Raw != 42 {
		t.Fatalf("raw = %v, want 42", result.Raw)
	}
	if _, err := decoder.SetIndex(parameter, 0, 256); !errors.Is(err, ErrRawOutOfRange) {
		t.Fatalf("error = %v, want ErrRawOutOfRange", err)
	}
}
