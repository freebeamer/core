package calibration

import (
	"bytes"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func editableTable(bits int, flags uint64, rows, columns int, formulaText string) types.XDFTable {
	axis := types.XDFAxis{Formula: formulaText, Embedded: embedded(2, bits, flags)}
	axis.Embedded.RowCount = rows
	axis.Embedded.ColumnCount = columns
	return types.XDFTable{Title: "Editable", Z: axis}
}

func TestSetCellChangesOnlyExpectedBytesAndRoundTrips(t *testing.T) {
	table := editableTable(16, testTypeLSBFirst, 2, 3, "X*0.1")
	image := imageFor(t, make([]byte, 16))
	original := image.Original()
	decoder, _ := New(definition(table), image)
	result, err := decoder.SetCell(parameter(table), 1, 1, 12.34)
	if err != nil {
		t.Fatal(err)
	}
	if result.Raw != 123 || result.Actual != 12.3 || result.Offset != 10 || !bytes.Equal(result.Bytes, []byte{123, 0}) {
		t.Fatalf("result=%#v", result)
	}
	if !bytes.Equal(result.Previous, []byte{0, 0}) {
		t.Fatalf("Previous=%#v, want the pre-edit zero bytes", result.Previous)
	}
	changes := image.Diff()
	want := []types.BinaryChange{{Offset: 10, Original: 0, Current: 123}}
	if !reflect.DeepEqual(changes, want) {
		t.Fatalf("changes=%#v", changes)
	}
	if !bytes.Equal(image.Original(), original) {
		t.Fatal("original mutated")
	}
	decoded, err := decoder.DecodeParameter(parameter(table))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Z[1][1] != 12.3 || decoded.Z[1][0] != 0 || decoded.Z[1][2] != 0 {
		t.Fatalf("decoded=%#v", decoded.Z)
	}

	second, err := decoder.SetCell(parameter(table), 1, 1, 45.6)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(second.Previous, result.Bytes) {
		t.Fatalf("second edit's Previous=%#v, want the first edit's Bytes=%#v", second.Previous, result.Bytes)
	}
}

func TestSetColumnMajorUsesPhysicalColumnOrder(t *testing.T) {
	table := editableTable(8, testTypeColumnMajor, 2, 3, "X")
	image := imageFor(t, make([]byte, 8))
	decoder, _ := New(definition(table), image)
	result, err := decoder.SetCell(parameter(table), 1, 2, 77)
	if err != nil {
		t.Fatal(err)
	}
	if result.Offset != 7 {
		t.Fatalf("offset=%d", result.Offset)
	}
	if got := image.Bytes()[7]; got != 77 {
		t.Fatalf("byte=%d", got)
	}
}

func TestSetIndexAndEndianSignedness(t *testing.T) {
	for _, test := range []struct {
		name  string
		flags uint64
		want  []byte
	}{{"big-signed", testTypeSigned, []byte{0xff, 0xfe}}, {"little-signed", testTypeSigned | testTypeLSBFirst, []byte{0xfe, 0xff}}} {
		t.Run(test.name, func(t *testing.T) {
			table := editableTable(16, test.flags, 1, 2, "X")
			image := imageFor(t, make([]byte, 8))
			decoder, _ := New(definition(table), image)
			result, err := decoder.SetIndex(parameter(table), 1, -2)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(result.Bytes, test.want) {
				t.Fatalf("bytes=%x", result.Bytes)
			}
		})
	}
}

func TestSetRejectsOverflowInvalidIndexAndNonFinite(t *testing.T) {
	table := editableTable(8, 0, 1, 1, "X")
	for _, test := range []struct {
		value  float64
		target error
	}{{-1, ErrRawOutOfRange}, {256, ErrRawOutOfRange}, {math.Inf(1), nil}} {
		image := imageFor(t, []byte{0, 0, 0})
		decoder, _ := New(definition(table), image)
		_, err := decoder.SetIndex(parameter(table), 0, test.value)
		if err == nil {
			t.Fatalf("value %v succeeded", test.value)
		}
		if test.target != nil && !errors.Is(err, test.target) {
			t.Fatalf("value %v error=%v", test.value, err)
		}
		if len(image.Diff()) != 0 {
			t.Fatalf("value %v mutated image", test.value)
		}
	}
	image := imageFor(t, []byte{0, 0, 0})
	decoder, _ := New(definition(table), image)
	if _, err := decoder.SetIndex(parameter(table), 1, 1); !errors.Is(err, ErrCellOutOfRange) {
		t.Fatalf("index error=%v", err)
	}
}

func TestSetDoesNotSilentlyClampSignedRanges(t *testing.T) {
	for _, bits := range []int{8, 16, 32} {
		table := editableTable(bits, testTypeSigned, 1, 1, "X")
		image := imageFor(t, make([]byte, 8))
		decoder, _ := New(definition(table), image)
		maximum := math.Pow(2, float64(bits-1))
		if _, err := decoder.SetCell(parameter(table), 0, 0, maximum); !errors.Is(err, ErrRawOutOfRange) {
			t.Errorf("%d-bit overflow error=%v", bits, err)
		}
	}
}
