package xdf_test

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/calibration"
	"github.com/freebeamer/core/pkg/mapdef"
	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/xdf"
)

// Synthetic fixtures for XDF IEEE-754 cells, authored for this project. See
// docs/xdf-compatibility.md's IEEE-754 evidence section for the sources.

func TestFloatFlagLayouts(t *testing.T) {
	address := uint64(0x40)
	tests := []struct {
		name        string
		bits        int
		flags       uint64
		dataType    string
		byteOrder   string
		order       string
		diagnostics []string
	}{
		{"big-endian", 32, 0x10000, "float32", "big", "row-major", nil},
		{"signed-bit-ignored", 32, 0x10001, "float32", "big", "row-major", []string{"xdf.float-signed-ignored"}},
		{"little-endian", 32, 0x10002, "float32", "little", "row-major", nil},
		{"little-column-major", 32, 0x10006, "float32", "little", "column-major", nil},
		{"64-bit-unsupported", 64, 0x10000, "unknown", "big", "row-major", []string{"xdf.unsupported-data-type"}},
		{"16-bit-unsupported", 16, 0x10000, "unknown", "big", "row-major", []string{"xdf.unsupported-data-type"}},
		{"unknown-bit-still-rejected", 32, 0x10008, "unknown", "big", "row-major", []string{"xdf.unsupported-data-type"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			embedded := types.XDFEmbeddedData{Address: address, ElementSizeBits: test.bits, TypeFlags: test.flags, RowCount: 2, ColumnCount: 2, Raw: map[string]string{"mmedaddress": "0x40", "mmedtypeflags": "explicit"}}
			document, err := (xdf.Converter{}).Convert(definitionWithZ(embedded), "float.xdf")
			if err != nil {
				t.Fatal(err)
			}
			layout := document.Parameters[0].Layout
			if layout.DataType != test.dataType || layout.ByteOrder != test.byteOrder || layout.Order != test.order {
				t.Fatalf("layout = %#v", layout)
			}
			var codes []string
			for _, diagnostic := range document.ImportDiagnostics {
				codes = append(codes, diagnostic.Code)
			}
			if !reflect.DeepEqual(codes, test.diagnostics) {
				t.Fatalf("diagnostics = %v, want %v", codes, test.diagnostics)
			}
			if err := mapdef.Validate(document); err != nil {
				t.Fatal(err)
			}
		})
	}
}

const floatXDF = `<?xml version="1.0" encoding="utf-8"?>
<XDFFORMAT version="1.70">
  <XDFHEADER>
    <title>Synthetic float definition</title>
    <DEFAULTS datasizeinbits="8" sigdigits="2" outputtype="1" signed="0" lsbfirst="0" float="0" />
  </XDFHEADER>
  <XDFTABLE uniqueid="0x1">
    <title>Float curve</title>
    <XDFAXIS id="x">
      <EMBEDDEDDATA mmedtypeflags="0x10002" mmedaddress="0x00" mmedelementsizebits="32" mmedcolcount="3" mmedmajorstridebits="0" mmedminorstridebits="0" />
      <indexcount>3</indexcount>
      <MATH equation="X" />
    </XDFAXIS>
    <XDFAXIS id="z">
      <EMBEDDEDDATA mmedtypeflags="0x10002" mmedaddress="0x0C" mmedelementsizebits="32" mmedrowcount="1" mmedcolcount="3" mmedmajorstridebits="0" mmedminorstridebits="0" />
      <MATH equation="X*2" />
    </XDFAXIS>
  </XDFTABLE>
  <XDFCONSTANT uniqueid="0x2">
    <title>Float scalar</title>
    <EMBEDDEDDATA mmedtypeflags="0x10000" mmedaddress="0x18" mmedelementsizebits="32" />
    <MATH equation="X" />
  </XDFCONSTANT>
  <XDFFLAG uniqueid="0x3">
    <title>Float flag</title>
    <EMBEDDEDDATA mmedtypeflags="0x10000" mmedaddress="0x1C" mmedelementsizebits="32" />
    <mask>0x1</mask>
  </XDFFLAG>
</XDFFORMAT>`

func floatImage(t *testing.T) (*binfile.Image, []byte) {
	t.Helper()
	data := make([]byte, 0x20)
	for i, point := range []float32{800, 2400.5, 6000} {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(point))
	}
	for i, value := range []float32{-0.75, 0, 1.125} {
		binary.LittleEndian.PutUint32(data[0x0C+i*4:], math.Float32bits(value))
	}
	binary.BigEndian.PutUint32(data[0x18:], math.Float32bits(14.7))
	path := filepath.Join(t.TempDir(), "float.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return image, data
}

func TestFloatXDFDecodesAndEdits(t *testing.T) {
	source, err := xdf.Parse(strings.NewReader(floatXDF))
	if err != nil {
		t.Fatal(err)
	}
	document, err := (xdf.Converter{}).Convert(source, "float.xdf")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]types.MapParameter{}
	for _, parameter := range document.Parameters {
		byName[parameter.Name] = parameter
	}
	curve, scalar, flag := byName["Float curve"], byName["Float scalar"], byName["Float flag"]
	if flag.Layout.DataType != "unknown" {
		t.Fatalf("float-backed flag layout = %#v, want unknown (no evidence for flags over floats)", flag.Layout)
	}

	image, data := floatImage(t)
	decoder, err := calibration.New(document, image)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(curve)
	if err != nil {
		t.Fatal(err)
	}
	// Independent oracle: the same bytes read directly with math.Float32frombits.
	wantX := make([]float64, 3)
	wantZ := make([]float64, 3)
	for i := range wantX {
		wantX[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:])))
		wantZ[i] = 2 * float64(math.Float32frombits(binary.LittleEndian.Uint32(data[0x0C+i*4:])))
	}
	if !reflect.DeepEqual(decoded.X, wantX) || !reflect.DeepEqual(decoded.Z, [][]float64{wantZ}) {
		t.Fatalf("decoded x %v z %v, want x %v z %v", decoded.X, decoded.Z, wantX, wantZ)
	}

	decodedScalar, err := decoder.DecodeParameter(scalar)
	if err != nil {
		t.Fatal(err)
	}
	if decodedScalar.Z[0][0] != float64(float32(14.7)) {
		t.Fatalf("scalar = %v, want big-endian float32 14.7", decodedScalar.Z[0][0])
	}

	result, err := decoder.SetIndex(curve, 2, 3.3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Raw != float64(float32(1.65)) || len(result.Bytes) != 4 {
		t.Fatalf("edit result = %#v, want raw float32(1.65) in 4 bytes", result)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(image.Bytes()[0x14:])); got != float32(1.65) {
		t.Fatalf("stored %v after edit", got)
	}
	if !reflect.DeepEqual(image.Bytes()[0x0C:0x14], data[0x0C:0x14]) {
		t.Fatal("edit touched neighbouring cells")
	}
}

func TestFloatDefaultsApplyWhenTypeFlagsAbsent(t *testing.T) {
	source := &types.XDFDefinition{
		Header: types.XDFHeader{Title: "Defaults", Defaults: types.XDFDefaults{DataSizeBits: 32, LSBFirst: true, Float: true}},
		Tables: []types.XDFTable{{ID: "1", Title: "Map", Z: types.XDFAxis{ID: "z", Formula: "X", Embedded: types.XDFEmbeddedData{Address: 0, RowCount: 1, ColumnCount: 1, Raw: map[string]string{"mmedaddress": "0x0"}}}}},
	}
	document, err := (xdf.Converter{}).Convert(source, "defaults.xdf")
	if err != nil {
		t.Fatal(err)
	}
	if layout := document.Parameters[0].Layout; layout.DataType != "float32" || layout.ByteOrder != "little" {
		t.Fatalf("layout = %#v, want DEFAULTS float=1 lsbfirst=1 to give little-endian float32", layout)
	}
}
