package calibration

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/xdf"
)

const (
	testTypeSigned      = uint64(0x01)
	testTypeLSBFirst    = uint64(0x02)
	testTypeColumnMajor = uint64(0x04)
	testTypeFloat       = uint64(0x10000)
)

func imageFor(t *testing.T, data []byte) *binfile.Image {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return image
}

func embedded(address uint64, bits int, flags uint64) types.XDFEmbeddedData {
	return types.XDFEmbeddedData{Address: address, ElementSizeBits: bits, TypeFlags: flags, Raw: map[string]string{"mmedaddress": "present", "mmedtypeflags": "present"}}
}

func definition(table types.XDFTable) *types.MapDefinition {
	source := &types.XDFDefinition{
		Header: types.XDFHeader{Defaults: types.XDFDefaults{DataSizeBits: 8}},
		Tables: []types.XDFTable{table},
	}
	document, err := (xdf.Converter{}).Convert(source, "test.xdf")
	if err != nil {
		panic(err)
	}
	return document
}

func parameter(table types.XDFTable) types.MapParameter {
	return definition(table).Parameters[0]
}

func TestResolveDataTypeFlagCombinations(t *testing.T) {
	tests := []struct {
		flags  uint64
		signed bool
		endian binfile.Endian
		column bool
	}{
		{0x00, false, binfile.BigEndian, false}, {0x01, true, binfile.BigEndian, false},
		{0x02, false, binfile.LittleEndian, false}, {0x03, true, binfile.LittleEndian, false},
		{0x04, false, binfile.BigEndian, true}, {0x06, false, binfile.LittleEndian, true},
	}
	for _, test := range tests {
		layout := parameter(types.XDFTable{Z: types.XDFAxis{Embedded: embedded(0, 16, test.flags)}}).Layout
		got, err := resolveDataType(layout)
		if err != nil {
			t.Fatalf("flags %#x: %v", test.flags, err)
		}
		if got.signed != test.signed || got.endian != test.endian || got.columnMajor != test.column {
			t.Errorf("flags %#x = %#v", test.flags, got)
		}
	}
}

func TestResolveDataTypeDefaultsAndRejections(t *testing.T) {
	for _, test := range []struct {
		embedded types.XDFEmbeddedData
		target   error
	}{
		{embedded(0, 64, 0), ErrUnsupportedDataType},
		{embedded(0, 32, testTypeFloat), ErrUnsupportedDataType},
		{embedded(0, 16, 0x80), ErrUnsupportedDataType},
	} {
		layout := parameter(types.XDFTable{Z: types.XDFAxis{Embedded: test.embedded}}).Layout
		if _, err := resolveDataType(layout); !errors.Is(err, test.target) {
			t.Errorf("resolveDataType(%#v) = %v", layout, err)
		}
	}
}

func TestDecodeUnsignedBigEndian8And16(t *testing.T) {
	table := types.XDFTable{Title: "big", Z: types.XDFAxis{Formula: "X*0.5", Embedded: embedded(0, 16, 0)}}
	table.Z.Embedded.RowCount = 1
	table.Z.Embedded.ColumnCount = 2
	decoder, _ := New(definition(table), imageFor(t, []byte{0x00, 0x02, 0x01, 0x00}))
	got, err := decoder.DecodeParameter(parameter(table))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Raw.Z, [][]int64{{2, 256}}) || !reflect.DeepEqual(got.Z, [][]float64{{1, 128}}) {
		t.Fatalf("decoded = %#v", got)
	}

	table.Z.Embedded = embedded(0, 8, 0)
	table.Z.Embedded.RowCount = 1
	table.Z.Embedded.ColumnCount = 4
	table.Z.Formula = "X"
	decoder, _ = New(definition(table), imageFor(t, []byte{1, 2, 3, 4}))
	got, err = decoder.DecodeParameter(parameter(table))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Raw.Z, [][]int64{{1, 2, 3, 4}}) {
		t.Fatalf("8-bit = %#v", got.Raw.Z)
	}
}

func TestDecodeSignedLittleEndian32(t *testing.T) {
	table := types.XDFTable{Title: "signed", Z: types.XDFAxis{Formula: "X+1", Embedded: embedded(0, 32, testTypeSigned|testTypeLSBFirst)}}
	table.Z.Embedded.RowCount = 1
	table.Z.Embedded.ColumnCount = 2
	decoder, _ := New(definition(table), imageFor(t, []byte{0xfe, 0xff, 0xff, 0xff, 0x04, 0x03, 0x02, 0x01}))
	got, err := decoder.DecodeParameter(parameter(table))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Raw.Z, [][]int64{{-2, 0x01020304}}) || !reflect.DeepEqual(got.Z, [][]float64{{-1, 0x01020305}}) {
		t.Fatalf("decoded = %#v", got)
	}
}

func TestDecodeColumnMajorIntoRows(t *testing.T) {
	table := types.XDFTable{Title: "column", Z: types.XDFAxis{Formula: "X", Embedded: embedded(0, 8, testTypeColumnMajor)}}
	table.Z.Embedded.RowCount = 2
	table.Z.Embedded.ColumnCount = 3
	decoder, _ := New(definition(table), imageFor(t, []byte{1, 4, 2, 5, 3, 6}))
	got, err := decoder.DecodeParameter(parameter(table))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Raw.Z, [][]int64{{1, 2, 3}, {4, 5, 6}}) {
		t.Fatalf("column-major = %#v", got.Raw.Z)
	}
}

func TestDecodeAxesAndBaseOffset(t *testing.T) {
	x := types.XDFAxis{ID: "x", IndexCount: 2, Formula: "X*10", Embedded: embedded(2, 8, 0)}
	y := types.XDFAxis{ID: "y", IndexCount: 2, Formula: "X-1", Embedded: embedded(4, 8, 0)}
	z := types.XDFAxis{ID: "z", Formula: "X", Embedded: embedded(6, 8, 0)}
	z.Embedded.RowCount = 1
	z.Embedded.ColumnCount = 2
	table := types.XDFTable{Title: "axes", X: x, Y: y, Z: z}
	def := definition(table)
	def.Memory.BaseOffset = 1
	decoder, _ := New(def, imageFor(t, []byte{0, 0, 0, 2, 3, 5, 6, 7, 8}))
	got, err := decoder.DecodeParameter(parameter(table))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.X, []float64{20, 30}) || !reflect.DeepEqual(got.Y, []float64{4, 5}) || !reflect.DeepEqual(got.Z, [][]float64{{7, 8}}) {
		t.Fatalf("axes = %#v", got)
	}
	if !reflect.DeepEqual(got.Addresses.X, []uint64{3, 4}) || !reflect.DeepEqual(got.Addresses.Y, []uint64{5, 6}) || !reflect.DeepEqual(got.Addresses.Z, [][]uint64{{7, 8}}) {
		t.Fatalf("addresses = %#v", got.Addresses)
	}
}

func TestDecodeReportsInvalidDefinitionsWithoutPanicking(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*types.XDFAxis)
		target error
	}{
		{"stride", func(a *types.XDFAxis) { a.Embedded.MajorStrideBits = 8 }, ErrUnsupportedStride},
		{"bounds", func(a *types.XDFAxis) { a.Embedded.Address = 100 }, binfile.ErrOutOfBounds},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			axis := types.XDFAxis{Formula: "X", Embedded: embedded(0, 8, 0)}
			axis.Embedded.RowCount = 1
			axis.Embedded.ColumnCount = 1
			test.mutate(&axis)
			table := types.XDFTable{Title: test.name, Z: axis}
			decoder, _ := New(definition(table), imageFor(t, []byte{1}))
			_, err := decoder.DecodeParameter(parameter(table))
			if err == nil {
				t.Fatal("expected error")
			}
			if test.target != nil && !errors.Is(err, test.target) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestValidateAllTablesAndFindTable(t *testing.T) {
	valid := types.XDFTable{Title: "Boost Target", Z: types.XDFAxis{Formula: "X", Embedded: embedded(0, 8, 0)}}
	valid.Z.Embedded.RowCount = 1
	valid.Z.Embedded.ColumnCount = 1
	invalid := valid
	invalid.Title = "Bad"
	invalid.Z.Embedded.Address = 9
	def := definition(valid)
	def.Parameters = append(def.Parameters, parameter(invalid))
	decoder, _ := New(def, imageFor(t, []byte{1}))
	failures := decoder.Validate()
	if len(failures) != 1 {
		t.Fatalf("failures=%v", failures)
	}
	if found, err := FindParameter(def, "boost target"); err != nil || found.Name != "Boost Target" {
		t.Fatalf("FindParameter=%#v,%v", found, err)
	}
	if _, err := FindParameter(def, "missing"); err == nil {
		t.Fatal("missing table found")
	}
}
