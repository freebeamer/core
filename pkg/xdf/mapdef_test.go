package xdf_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"github.com/freebeamer/core/pkg/types"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/calibration"
	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/mapdef"
	"github.com/freebeamer/core/pkg/xdf"
)

var _ xdf.MapdefConverter = xdf.Converter{}

func TestMapdefConversionContract(t *testing.T) {
	source, err := xdf.Load(filepath.Join("testdata", "basic.xdf"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := (xdf.Converter{}).Convert(source, "basic.xdf")
	if err != nil {
		t.Fatal(err)
	}
	if document.Format != mapdef.Format || document.FormatVersion != mapdef.CurrentVersion || document.Name != source.Header.Title {
		t.Fatalf("identity = %#v", document)
	}
	if len(document.Parameters) != 2 || document.Parameters[0].Kind != "map" || document.Parameters[1].Kind != "flag" {
		t.Fatalf("parameters = %#v", document.Parameters)
	}
	parameter := document.Parameters[0]
	if parameter.Layout.Address == nil || *parameter.Layout.Address != 0x200 || !reflect.DeepEqual(parameter.Layout.Dimensions, []int{2, 2}) || parameter.Layout.DataType != "uint16" || parameter.Layout.ByteOrder != "little" || parameter.Layout.Order != "row-major" {
		t.Fatalf("layout = %#v", parameter.Layout)
	}
	xAxis := parameter.Axes["x"]
	if xAxis.Layout == nil || xAxis.Layout.Address == nil || *xAxis.Layout.Address != 0x100 || xAxis.Layout.DataType != "uint16" || xAxis.Unit != "rpm" || xAxis.Conversion.Expression != "X*10" {
		t.Fatalf("X axis = %#v", xAxis)
	}
	if parameter.Unit != "kPa" || parameter.Conversion.Expression != "X*0.1" {
		t.Fatalf("conversion = %#v", parameter)
	}
	digest := sha256.Sum256(source.RawXML)
	if document.Provenance.SourceSHA256 != hex.EncodeToString(digest[:]) || document.Extensions["xdf.rawDocument"] != string(source.RawXML) {
		t.Fatal("source provenance was not preserved")
	}
	if len(document.ImportDiagnostics) < 2 || document.ImportDiagnostics[0].Code != "xdf.flag-semantics" {
		t.Fatalf("diagnostics = %#v", document.ImportDiagnostics)
	}
	var encoded bytes.Buffer
	if err := mapdef.Encode(&encoded, document); err != nil {
		t.Fatal(err)
	}
	if _, err := mapdef.Decode(bytes.NewReader(encoded.Bytes())); err != nil {
		t.Fatal(err)
	}
}

func TestMapdefConversionPreservesSupportedLayouts(t *testing.T) {
	address := uint64(0x123)
	tests := []struct {
		name      string
		bits      int
		flags     uint64
		dataType  string
		byteOrder string
		order     string
	}{
		{"unsigned-8", 8, 0, "uint8", "not-applicable", "row-major"},
		{"signed-big-16", 16, 0x01, "int16", "big", "row-major"},
		{"unsigned-little-32", 32, 0x02, "uint32", "little", "row-major"},
		{"signed-little-column", 16, 0x01 | 0x02 | 0x04, "int16", "little", "column-major"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			embedded := types.XDFEmbeddedData{Address: address, ElementSizeBits: test.bits, TypeFlags: test.flags, RowCount: 2, ColumnCount: 3, Raw: map[string]string{"mmedaddress": "0x123", "mmedtypeflags": "explicit"}}
			source := definitionWithZ(embedded)
			document, err := (xdf.Converter{}).Convert(source, "layout.xdf")
			if err != nil {
				t.Fatal(err)
			}
			layout := document.Parameters[0].Layout
			if layout.Address == nil || *layout.Address != address || layout.DataType != test.dataType || layout.ByteOrder != test.byteOrder || layout.Order != test.order {
				t.Fatalf("layout = %#v", layout)
			}
		})
	}
}

func TestMapdefConversionDiagnosesUnsupportedLayout(t *testing.T) {
	embedded := types.XDFEmbeddedData{ElementSizeBits: 32, TypeFlags: 0x10000, RowCount: 2, ColumnCount: 2, MajorStrideBits: 32, Raw: map[string]string{"mmedtypeflags": "explicit"}}
	document, err := (xdf.Converter{}).Convert(definitionWithZ(embedded), "unsupported.xdf")
	if err != nil {
		t.Fatal(err)
	}
	if document.Parameters[0].Layout.DataType != "unknown" || len(document.ImportDiagnostics) != 2 || document.ImportDiagnostics[0].Code != "xdf.unsupported-data-type" || document.ImportDiagnostics[1].Code != "xdf.unsupported-stride" {
		t.Fatalf("document = %#v", document)
	}
}

func TestMapdefConversionPreservesDecodedValues(t *testing.T) {
	source, err := xdf.Load(filepath.Join("testdata", "basic.xdf"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := (xdf.Converter{}).Convert(source, "basic.xdf")
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 0x208)
	for index, value := range []uint16{10, 20, 30, 40} {
		binary.LittleEndian.PutUint16(data[0x200+index*2:], value)
	}
	path := filepath.Join(t.TempDir(), "fixture.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := calibration.New(document, image)
	if err != nil {
		t.Fatal(err)
	}
	want, err := decoder.DecodeParameter(document.Parameters[0])
	if err != nil {
		t.Fatal(err)
	}
	raw, physical, err := decodeParameter(document.Parameters[0], data, document.Memory.BaseOffset)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raw, want.Raw.Z) || !reflect.DeepEqual(physical, want.Z) {
		t.Fatalf("mapdef raw/decoded = %v/%v, XDF = %v/%v", raw, physical, want.Raw.Z, want.Z)
	}
}

func TestFlagMaskDecodesASingleBitPosition(t *testing.T) {
	source := flagDefinitionWithMask(t, "0x04")
	document, err := (xdf.Converter{}).Convert(source, "flag.xdf")
	if err != nil {
		t.Fatal(err)
	}
	layout := document.Parameters[0].Layout
	if layout.BitPosition == nil || *layout.BitPosition != 2 {
		t.Fatalf("BitPosition = %v, want 2", layout.BitPosition)
	}
	if !hasDiagnosticCode(document.ImportDiagnostics, "xdf.flag-bit") {
		t.Fatalf("diagnostics = %#v, want xdf.flag-bit", document.ImportDiagnostics)
	}
}

func TestFlagMaskFallsBackToWholeWordWhenMultiBitOrOutOfRange(t *testing.T) {
	for _, test := range []struct {
		name string
		mask string
	}{
		{"multi-bit", "0x06"},
		{"exceeds-8-bit-width", "0x100"},
		{"zero", "0x00"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := flagDefinitionWithMask(t, test.mask)
			document, err := (xdf.Converter{}).Convert(source, "flag.xdf")
			if err != nil {
				t.Fatal(err)
			}
			if document.Parameters[0].Layout.BitPosition != nil {
				t.Fatalf("BitPosition = %v, want nil for mask %s", *document.Parameters[0].Layout.BitPosition, test.mask)
			}
			if !hasDiagnosticCode(document.ImportDiagnostics, "xdf.flag-semantics") {
				t.Fatalf("diagnostics = %#v, want xdf.flag-semantics", document.ImportDiagnostics)
			}
		})
	}
}

func TestFlagWithoutMaskFallsBackToWholeWord(t *testing.T) {
	source, err := xdf.Load(filepath.Join("testdata", "basic.xdf"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := (xdf.Converter{}).Convert(source, "basic.xdf")
	if err != nil {
		t.Fatal(err)
	}
	flag := document.Parameters[1]
	if flag.Kind != "flag" || flag.Layout.BitPosition != nil {
		t.Fatalf("flag = %#v, want no BitPosition", flag)
	}
}

func TestFlagWithMalformedMaskFailsToParse(t *testing.T) {
	xmlDoc := `<XDFFORMAT version="1.70"><XDFHEADER><title>Bad mask</title></XDFHEADER>` +
		`<XDFFLAG uniqueid="0x1"><title>Flag</title>` +
		`<EMBEDDEDDATA mmedaddress="0x0" mmedelementsizebits="8" /><mask>not-a-number</mask></XDFFLAG></XDFFORMAT>`
	if _, err := xdf.Parse(strings.NewReader(xmlDoc)); err == nil {
		t.Fatal("malformed <mask> text was accepted")
	}
}

func TestAxisLabelsConvertToTypedStaticValues(t *testing.T) {
	xmlDoc := `<XDFFORMAT version="1.70"><XDFHEADER><title>Static axis</title></XDFHEADER>` +
		`<XDFTABLE uniqueid="0x1"><title>Gear table</title>` +
		`<XDFAXIS id="x"><units></units>` +
		`<LABEL index="0" value="Park" /><LABEL index="1" value="Drive" />` +
		`<MATH equation="X" /></XDFAXIS>` +
		`<XDFAXIS id="z"><EMBEDDEDDATA mmedaddress="0x0" mmedelementsizebits="8" mmedrowcount="1" mmedcolcount="2" />` +
		`<MATH equation="X" /></XDFAXIS>` +
		`</XDFTABLE></XDFFORMAT>`
	source, err := xdf.Parse(strings.NewReader(xmlDoc))
	if err != nil {
		t.Fatal(err)
	}
	if len(source.Tables[0].X.Labels) != 2 || source.Tables[0].X.Labels[0].Value != "Park" || source.Tables[0].X.Labels[1].Index != 1 {
		t.Fatalf("parsed labels = %#v", source.Tables[0].X.Labels)
	}
	document, err := (xdf.Converter{}).Convert(source, "static-axis.xdf")
	if err != nil {
		t.Fatal(err)
	}
	if document.FormatVersion != mapdef.CurrentVersion {
		t.Fatalf("FormatVersion = %q, want %q", document.FormatVersion, mapdef.CurrentVersion)
	}
	xAxis := document.Parameters[0].Axes["x"]
	want := []types.StaticValue{{Index: 0, Value: "Park"}, {Index: 1, Value: "Drive"}}
	if len(xAxis.StaticValues) != 2 || xAxis.StaticValues[0] != want[0] || xAxis.StaticValues[1] != want[1] {
		t.Fatalf("static values = %#v", xAxis.StaticValues)
	}
	// The raw XDF representation stays available for provenance too.
	labels, ok := xAxis.SourceMetadata["xdf.labels"].([]map[string]interface{})
	if !ok || len(labels) != 2 || labels[0]["value"] != "Park" {
		t.Fatalf("source metadata labels = %#v", xAxis.SourceMetadata["xdf.labels"])
	}
}

func TestAxisAndConstantBoundsConvertToTypedDisplayFields(t *testing.T) {
	xmlDoc := `<XDFFORMAT version="1.70"><XDFHEADER><title>Bounds</title></XDFHEADER>` +
		`<XDFTABLE uniqueid="0x1"><title>Bounded table</title>` +
		`<XDFAXIS id="x"><min>0.000000</min><max>25.000000</max><MATH equation="X" /></XDFAXIS>` +
		`<XDFAXIS id="z"><EMBEDDEDDATA mmedaddress="0x0" mmedelementsizebits="8" mmedrowcount="1" mmedcolcount="2" />` +
		`<min>1.000000</min><max>250.000000</max><MATH equation="X" /></XDFAXIS>` +
		`</XDFTABLE>` +
		`<XDFCONSTANT uniqueid="0x2"><title>Idle target</title>` +
		`<EMBEDDEDDATA mmedaddress="0x10" mmedelementsizebits="8" /><min>10.000000</min><max>90.000000</max>` +
		`<MATH equation="X" /></XDFCONSTANT></XDFFORMAT>`
	source, err := xdf.Parse(strings.NewReader(xmlDoc))
	if err != nil {
		t.Fatal(err)
	}
	if source.Tables[0].X.Min == nil || *source.Tables[0].X.Min != 0 || source.Tables[0].X.Max == nil || *source.Tables[0].X.Max != 25 {
		t.Fatalf("parsed axis bounds = min=%v max=%v", source.Tables[0].X.Min, source.Tables[0].X.Max)
	}
	if source.Constants[0].Min == nil || *source.Constants[0].Min != 10 || source.Constants[0].Max == nil || *source.Constants[0].Max != 90 {
		t.Fatalf("parsed constant bounds = min=%v max=%v", source.Constants[0].Min, source.Constants[0].Max)
	}
	document, err := (xdf.Converter{}).Convert(source, "bounds.xdf")
	if err != nil {
		t.Fatal(err)
	}
	table := document.Parameters[0]
	if table.Display.Min == nil || *table.Display.Min != 1 || table.Display.Max == nil || *table.Display.Max != 250 {
		t.Fatalf("table (Z axis) display bounds = %#v", table.Display)
	}
	xAxis := table.Axes["x"]
	if xAxis.Display.Min == nil || *xAxis.Display.Min != 0 || xAxis.Display.Max == nil || *xAxis.Display.Max != 25 {
		t.Fatalf("axis display bounds = %#v", xAxis.Display)
	}
	constant := document.Parameters[1]
	if constant.Display.Min == nil || *constant.Display.Min != 10 || constant.Display.Max == nil || *constant.Display.Max != 90 {
		t.Fatalf("constant display bounds = %#v", constant.Display)
	}
	// The raw XDF representation stays available for provenance too.
	axisBounds, ok := xAxis.SourceMetadata["xdf.bounds"].(map[string]interface{})
	if !ok || axisBounds["min"] != 0.0 || axisBounds["max"] != 25.0 {
		t.Fatalf("axis bounds metadata = %#v", xAxis.SourceMetadata["xdf.bounds"])
	}
}

func TestConstantConvertsToAScalarParameter(t *testing.T) {
	xmlDoc := `<XDFFORMAT version="1.70"><XDFHEADER><title>Constant test</title>` +
		`<CATEGORY index="0x0" name="Fuel" /></XDFHEADER>` +
		`<XDFCONSTANT uniqueid="0x1"><title>Idle target</title><description>Idle RPM target</description>` +
		`<CATEGORYMEM index="0" category="0x0" />` +
		`<EMBEDDEDDATA mmedtypeflags="0x0" mmedaddress="0x50" mmedelementsizebits="16" />` +
		`<units>rpm</units><decimalpl>1</decimalpl><outputtype>1</outputtype>` +
		`<MATH equation="X*4"><VAR id="X" /></MATH></XDFCONSTANT></XDFFORMAT>`
	source, err := xdf.Parse(strings.NewReader(xmlDoc))
	if err != nil {
		t.Fatal(err)
	}
	if len(source.Constants) != 1 || source.Constants[0].Units != "rpm" || source.Constants[0].DecimalPlaces != 1 {
		t.Fatalf("parsed constant = %#v", source.Constants)
	}
	document, err := (xdf.Converter{}).Convert(source, "constant.xdf")
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Parameters) != 1 {
		t.Fatalf("parameters = %#v", document.Parameters)
	}
	parameter := document.Parameters[0]
	if parameter.Kind != "scalar" || parameter.Name != "Idle target" || parameter.Description != "Idle RPM target" {
		t.Fatalf("parameter = %#v", parameter)
	}
	if parameter.Layout.Address == nil || *parameter.Layout.Address != 0x50 || parameter.Layout.DataType != "uint16" {
		t.Fatalf("layout = %#v", parameter.Layout)
	}
	if parameter.Unit != "rpm" || parameter.Conversion.Expression != "X*4" {
		t.Fatalf("conversion = %#v", parameter)
	}
	if parameter.Display.DecimalPlaces != 1 || parameter.Display.OutputType != 1 {
		t.Fatalf("display = %#v", parameter.Display)
	}
	if len(parameter.Categories) != 1 || parameter.Categories[0] != "fuel" {
		t.Fatalf("categories = %#v", parameter.Categories)
	}
	if names := document.CategoryNames(parameter.Categories); len(names) != 1 || names[0] != "Fuel" {
		t.Fatalf("category names = %#v", names)
	}
}

func TestConstantDecodesAndEditsAsAScalar(t *testing.T) {
	xmlDoc := `<XDFFORMAT version="1.70"><XDFHEADER><title>Constant edit</title></XDFHEADER>` +
		`<XDFCONSTANT uniqueid="0x1"><title>Idle target</title>` +
		`<EMBEDDEDDATA mmedtypeflags="0x0" mmedaddress="0x0" mmedelementsizebits="8" />` +
		`<MATH equation="X" /></XDFCONSTANT></XDFFORMAT>`
	source, err := xdf.Parse(strings.NewReader(xmlDoc))
	if err != nil {
		t.Fatal(err)
	}
	document, err := (xdf.Converter{}).Convert(source, "constant.xdf")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "constant.bin")
	if err := os.WriteFile(path, []byte{7}, 0o600); err != nil {
		t.Fatal(err)
	}
	image, err := binfile.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := calibration.New(document, image)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decoder.DecodeParameter(document.Parameters[0])
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Z[0][0] != 7 {
		t.Fatalf("decoded = %v, want 7", decoded.Z[0][0])
	}
	if _, err := decoder.SetIndex(document.Parameters[0], 0, 42); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0] != 42 {
		t.Fatalf("byte = %d, want 42", image.Bytes()[0])
	}
}

func TestAxisWithMalformedBoundFailsToParse(t *testing.T) {
	xmlDoc := `<XDFFORMAT version="1.70"><XDFHEADER><title>Bad bound</title></XDFHEADER>` +
		`<XDFTABLE uniqueid="0x1"><title>Table</title>` +
		`<XDFAXIS id="x"><min>not-a-number</min><MATH equation="X" /></XDFAXIS>` +
		`<XDFAXIS id="z"><EMBEDDEDDATA mmedaddress="0x0" mmedelementsizebits="8" mmedrowcount="1" mmedcolcount="1" />` +
		`<MATH equation="X" /></XDFAXIS></XDFTABLE></XDFFORMAT>`
	if _, err := xdf.Parse(strings.NewReader(xmlDoc)); err == nil {
		t.Fatal("malformed <min> text was accepted")
	}
}

func TestConstantWithMalformedOutputTypeFailsToParse(t *testing.T) {
	xmlDoc := `<XDFFORMAT version="1.70"><XDFHEADER><title>Bad constant</title></XDFHEADER>` +
		`<XDFCONSTANT uniqueid="0x1"><title>Bad</title>` +
		`<EMBEDDEDDATA mmedaddress="0x0" mmedelementsizebits="8" /><outputtype>not-a-number</outputtype>` +
		`</XDFCONSTANT></XDFFORMAT>`
	if _, err := xdf.Parse(strings.NewReader(xmlDoc)); err == nil {
		t.Fatal("malformed <outputtype> text was accepted")
	}
}

func flagDefinitionWithMask(t *testing.T, mask string) *types.XDFDefinition {
	t.Helper()
	xmlDoc := `<XDFFORMAT version="1.70"><XDFHEADER><title>Flag test</title></XDFHEADER>` +
		`<XDFFLAG uniqueid="0x1"><title>Flag</title>` +
		`<EMBEDDEDDATA mmedaddress="0x0" mmedelementsizebits="8" /><mask>` + mask + `</mask></XDFFLAG></XDFFORMAT>`
	source, err := xdf.Parse(strings.NewReader(xmlDoc))
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func hasDiagnosticCode(diagnostics []types.ImportDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func definitionWithZ(embedded types.XDFEmbeddedData) *types.XDFDefinition {
	return &types.XDFDefinition{Header: types.XDFHeader{Title: "Layout", Defaults: types.XDFDefaults{DataSizeBits: 8}}, Tables: []types.XDFTable{{ID: "1", Title: "Map", Z: types.XDFAxis{ID: "z", Formula: "X", Embedded: embedded}}}}
}

func decodeParameter(parameter types.MapParameter, data []byte, baseOffset int64) ([][]int64, [][]float64, error) {
	rows, columns := parameter.Layout.Dimensions[0], parameter.Layout.Dimensions[1]
	raw, physical := make([][]int64, rows), make([][]float64, rows)
	formula, err := expression.Parse(parameter.Conversion.Expression)
	if err != nil {
		return nil, nil, err
	}
	base := int64(*parameter.Layout.Address) + baseOffset
	for row := 0; row < rows; row++ {
		raw[row], physical[row] = make([]int64, columns), make([]float64, columns)
		for column := 0; column < columns; column++ {
			index := row*columns + column
			if parameter.Layout.Order == "column-major" {
				index = column*rows + row
			}
			offset := int(base) + index*2
			value := int64(binary.LittleEndian.Uint16(data[offset:]))
			raw[row][column] = value
			physical[row][column], err = formula.Eval(float64(value))
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return raw, physical, nil
}
