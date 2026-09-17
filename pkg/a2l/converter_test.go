package a2l

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/calibration"
	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/mapdef"
	"github.com/freebeamer/core/pkg/types"
)

func TestConvertSyntheticFixture(t *testing.T) {
	source, err := Load(filepath.Join("testdata", "basic.a2l"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := (Converter{}).Convert(source, "basic.a2l")
	if err != nil {
		t.Fatal(err)
	}
	if document.Format != mapdef.Format || document.FormatVersion != mapdef.CurrentVersion {
		t.Fatalf("document format = %#v", document)
	}
	if len(document.Parameters) != 16 {
		t.Fatalf("parameters = %#v", document.Parameters)
	}
	wantCategories := []types.MapCategory{
		{ID: "group-all", Name: "contains every sub-group"},
		{ID: "group-scalars", Name: "Contains the scalar characteristics"},
	}
	if !reflect.DeepEqual(document.Categories, wantCategories) {
		t.Fatalf("categories = %#v, want %#v", document.Categories, wantCategories)
	}

	idle := document.Parameters[0]
	if idle.Kind != "scalar" || idle.Name != "IdleTarget" || idle.Layout.DataType != "uint8" {
		t.Fatalf("idle parameter = %#v", idle)
	}
	if idle.Layout.Address == nil || *idle.Layout.Address != 0x100 {
		t.Fatalf("idle address = %#v", idle.Layout.Address)
	}
	if idle.Conversion.Expression != "X" || idle.Unit != "counts" {
		t.Fatalf("idle conversion = %#v", idle)
	}
	if idle.Display.Min == nil || *idle.Display.Min != 0 || idle.Display.Max == nil || *idle.Display.Max != 255 {
		t.Fatalf("idle bounds = %#v", idle.Display)
	}
	if !reflect.DeepEqual(idle.Categories, []string{"group-scalars"}) {
		t.Fatalf("idle categories = %#v, want [group-scalars] (Group_Scalars' own REF_CHARACTERISTIC, not Group_All's SUB_GROUP nesting)", idle.Categories)
	}

	boost := document.Parameters[1]
	if boost.Layout.DataType != "int8" || boost.Layout.Address == nil || *boost.Layout.Address != 0x101 {
		t.Fatalf("boost layout = %#v", boost.Layout)
	}
	if boost.Conversion.Expression != "X*0.1" || boost.Unit != "kPa" {
		t.Fatalf("boost conversion = %#v", boost.Conversion)
	}
	if boost.Display.DecimalPlaces != 2 {
		t.Fatalf("boost decimal places = %d, want 2 (from characteristic-level FORMAT override)", boost.Display.DecimalPlaces)
	}
	if boost.Display.Min == nil || *boost.Display.Min != -128 || boost.Display.Max == nil || *boost.Display.Max != 127 {
		t.Fatalf("boost bounds = %#v", boost.Display)
	}
	extendedLimits, ok := boost.SourceMetadata["a2l.extendedLimits"].(map[string]interface{})
	if !ok || extendedLimits["low"] != -128.0 || extendedLimits["high"] != 127.0 {
		t.Fatalf("boost extended limits metadata = %#v", boost.SourceMetadata["a2l.extendedLimits"])
	}
	if !reflect.DeepEqual(boost.Categories, []string{"group-scalars"}) {
		t.Fatalf("boost categories = %#v, want [group-scalars]", boost.Categories)
	}

	rpm := document.Parameters[2]
	if rpm.Layout.DataType != "uint16" || rpm.Layout.ByteOrder != "little" {
		t.Fatalf("rpm layout = %#v, want uint16/little (from the MOD_COMMON BYTE_ORDER MSB_LAST default)", rpm.Layout)
	}
	if rpm.Categories != nil {
		t.Fatalf("rpm categories = %#v, want nil (no GROUP references it)", rpm.Categories)
	}

	pressure := document.Parameters[3]
	if pressure.Layout.DataType != "int16" || pressure.Layout.ByteOrder != "big" {
		t.Fatalf("pressure layout = %#v, want int16/big (from its own BYTE_ORDER MSB_FIRST override)", pressure.Layout)
	}

	curve := document.Parameters[4]
	if curve.Kind != "curve" || curve.Layout.Address == nil || *curve.Layout.Address != 0x204 {
		t.Fatalf("curve layout = %#v, want address 0x204 (0x200 + 1 count byte + 3 axis bytes)", curve.Layout)
	}
	if !reflect.DeepEqual(curve.Layout.Dimensions, []int{3}) || curve.Layout.Order != "row-major" {
		t.Fatalf("curve dimensions/order = %#v/%q", curve.Layout.Dimensions, curve.Layout.Order)
	}
	curveX, ok := curve.Axes["x"]
	if !ok || curveX.Count != 3 || curveX.Layout == nil || curveX.Layout.Address == nil || *curveX.Layout.Address != 0x201 {
		t.Fatalf("curve x axis = %#v", curveX)
	}

	boostMap := document.Parameters[5]
	if boostMap.Kind != "map" || boostMap.Layout.Address == nil || *boostMap.Layout.Address != 0x307 {
		t.Fatalf("map layout = %#v, want address 0x307 (0x300 + 2 count bytes + 2 x-axis + 3 y-axis bytes)", boostMap.Layout)
	}
	if !reflect.DeepEqual(boostMap.Layout.Dimensions, []int{3, 2}) {
		t.Fatalf("map dimensions = %#v, want [3, 2] (rows=y count, columns=x count)", boostMap.Layout.Dimensions)
	}
	mapX, mapY := boostMap.Axes["x"], boostMap.Axes["y"]
	if mapX.Count != 2 || mapX.Layout == nil || *mapX.Layout.Address != 0x302 {
		t.Fatalf("map x axis = %#v", mapX)
	}
	if mapY.Count != 3 || mapY.Layout == nil || *mapY.Layout.Address != 0x304 {
		t.Fatalf("map y axis = %#v", mapY)
	}

	comAxisCurve := document.Parameters[6]
	if comAxisCurve.Kind != "curve" || comAxisCurve.Layout.Address == nil || *comAxisCurve.Layout.Address != 0x500 {
		t.Fatalf("com axis curve layout = %#v, want address 0x500 (no embedded axis to offset past)", comAxisCurve.Layout)
	}
	comAxisX, ok := comAxisCurve.Axes["x"]
	if !ok || comAxisX.Count != 3 || comAxisX.Layout == nil || comAxisX.Layout.Address == nil || *comAxisX.Layout.Address != 0x400 {
		t.Fatalf("com axis x axis = %#v, want address 0x400 (AXIS_PTS.RPM_SHARED's own address)", comAxisX)
	}
	if comAxisX.SourceMetadata["a2l.axisPts"] != "AXIS_PTS.RPM_SHARED" {
		t.Fatalf("com axis metadata = %#v", comAxisX.SourceMetadata)
	}

	gearCurve := document.Parameters[7]
	gearAxis, ok := gearCurve.Axes["x"]
	if !ok || gearAxis.Conversion.Expression != "X" {
		t.Fatalf("gear axis = %#v", gearAxis)
	}
	wantStaticValues := []types.StaticValue{{Index: 0, Value: "Park"}, {Index: 1, Value: "Reverse"}, {Index: 2, Value: "Neutral"}, {Index: 3, Value: "Drive"}}
	if !reflect.DeepEqual(gearAxis.StaticValues, wantStaticValues) {
		t.Fatalf("gear axis static values = %#v", gearAxis.StaticValues)
	}

	fixAxisCurve := document.Parameters[8]
	fixAxis, ok := fixAxisCurve.Axes["x"]
	if !ok || fixAxis.Layout != nil || !reflect.DeepEqual(fixAxis.FixedPoints, []int64{10, 15, 20}) {
		t.Fatalf("fix axis = %#v", fixAxis)
	}
	if fixAxisCurve.Layout.Address == nil || *fixAxisCurve.Layout.Address != 0x700 {
		t.Fatalf("fix axis curve layout = %#v, want address 0x700 (no embedded axis storage to offset past)", fixAxisCurve.Layout)
	}

	rescaleAxisCurve := document.Parameters[9]
	rescaleAxis, ok := rescaleAxisCurve.Axes["x"]
	if !ok || rescaleAxis.Layout == nil || rescaleAxis.Layout.Address == nil || *rescaleAxis.Layout.Address != 0x803 {
		t.Fatalf("rescale axis = %#v, want address 0x803 (skips AXIS_PTS.PRESSURE_RESCALE's NO_RESCALE_X+RESERVED and the first pair's own position element)", rescaleAxis)
	}
	if !reflect.DeepEqual(rescaleAxis.Layout.StrideBits, []int{16}) {
		t.Fatalf("rescale axis stride = %v, want [16] (skip every other pair's position element)", rescaleAxis.Layout.StrideBits)
	}
	if rescaleAxisCurve.Layout.Address == nil || *rescaleAxisCurve.Layout.Address != 0x710 {
		t.Fatalf("rescale axis curve layout = %#v, want address 0x710", rescaleAxisCurve.Layout)
	}

	sharedCurveAxisCurve := document.Parameters[10]
	sharedCurveAxis, ok := sharedCurveAxisCurve.Axes["x"]
	if !ok || sharedCurveAxis.Count != 3 || sharedCurveAxis.Layout == nil || sharedCurveAxis.Layout.Address == nil || *sharedCurveAxis.Layout.Address != 0x201 {
		t.Fatalf("shared curve axis = %#v, want count 3 address 0x201 (ShiftDelay's own embedded X axis)", sharedCurveAxis)
	}
	if sharedCurveAxis.Conversion.Expression != "X" {
		t.Fatalf("shared curve axis conversion = %#v, want the referenced curve's own CM.IDENTICAL, not the NO_COMPU_METHOD placeholder", sharedCurveAxis.Conversion)
	}
	if sharedCurveAxisCurve.Layout.Address == nil || *sharedCurveAxisCurve.Layout.Address != 0x720 {
		t.Fatalf("shared curve axis curve layout = %#v, want address 0x720", sharedCurveAxisCurve.Layout)
	}

	valBlock := document.Parameters[11]
	if valBlock.Kind != "map" || len(valBlock.Axes) != 0 {
		t.Fatalf("val block = %#v, want Kind map with no axes", valBlock)
	}
	if !reflect.DeepEqual(valBlock.Layout.Dimensions, []int{2, 3}) {
		t.Fatalf("val block dimensions = %#v, want [2 3] (MATRIX_DIM 3 2 1 -> [y, x])", valBlock.Layout.Dimensions)
	}
	if valBlock.Layout.Address == nil || *valBlock.Layout.Address != 0x730 {
		t.Fatalf("val block layout = %#v, want address 0x730 (no other RECORD_LAYOUT entries to offset past)", valBlock.Layout)
	}

	ratFuncTarget := document.Parameters[12]
	if ratFuncTarget.Conversion.Language != "freehorse-expression-v1" || ratFuncTarget.Conversion.Expression != "5*X" {
		t.Fatalf("rat func conversion = %#v, want freehorse-expression-v1 \"5*X\"", ratFuncTarget.Conversion)
	}

	formTarget := document.Parameters[13]
	if formTarget.Conversion.Language != "freehorse-expression-v1" || formTarget.Conversion.Expression != "X+4" {
		t.Fatalf("form conversion = %#v, want freehorse-expression-v1 \"X+4\" (X1+4 with X1 translated to X)", formTarget.Conversion)
	}

	tabIntpTarget := document.Parameters[14]
	wantIntpPoints := []types.ConversionPoint{{Input: 0, Output: 100}, {Input: 10, Output: 110}, {Input: 20, Output: 130}}
	if tabIntpTarget.Conversion.Language != "freehorse-lookup-table-v1" || !tabIntpTarget.Conversion.Interpolated || !reflect.DeepEqual(tabIntpTarget.Conversion.Points, wantIntpPoints) {
		t.Fatalf("tab intp conversion = %#v", tabIntpTarget.Conversion)
	}
	if tabIntpTarget.Conversion.Default == nil || *tabIntpTarget.Conversion.Default != 999 {
		t.Fatalf("tab intp default = %#v, want 999", tabIntpTarget.Conversion.Default)
	}

	tabNointpTarget := document.Parameters[15]
	wantNointpPoints := []types.ConversionPoint{{Input: 0, Output: 100}, {Input: 1, Output: 110}, {Input: 2, Output: 130}}
	if tabNointpTarget.Conversion.Language != "freehorse-lookup-table-v1" || tabNointpTarget.Conversion.Interpolated || !reflect.DeepEqual(tabNointpTarget.Conversion.Points, wantNointpPoints) {
		t.Fatalf("tab nointp conversion = %#v", tabNointpTarget.Conversion)
	}
	if tabNointpTarget.Conversion.Default != nil {
		t.Fatalf("tab nointp default = %#v, want nil", tabNointpTarget.Conversion.Default)
	}

	if document.Extensions["a2l.rawDocument"] != string(source.RawText) {
		t.Fatal("raw A2L source was not preserved")
	}

	var encoded strings.Builder
	if err := mapdef.Encode(&encoded, document); err != nil {
		t.Fatal(err)
	}
}

func TestConvertDecodesAndEditsThroughCalibration(t *testing.T) {
	source, err := Load(filepath.Join("testdata", "basic.a2l"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := (Converter{}).Convert(source, "basic.a2l")
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 0x900)
	data[0x100] = 42                      // IdleTarget: raw uint8, identity conversion.
	data[0x101] = byte(50)                // BoostTarget: raw int8, X*0.1 -> 5.0 kPa.
	data[0x102], data[0x103] = 0xD2, 0x04 // RpmTarget: raw uint16 little-endian 1234.
	pressureValue := int16(-300)          // PressureTarget: raw int16 big-endian -300.
	pressureRaw := uint16(pressureValue)
	data[0x104], data[0x105] = byte(pressureRaw>>8), byte(pressureRaw)
	// ShiftDelay (curve): 0x200 count byte (unused), 0x201-0x203 axis [10,20,30], 0x204-0x206 Z [1,2,3].
	data[0x201], data[0x202], data[0x203] = 10, 20, 30
	data[0x204], data[0x205], data[0x206] = 1, 2, 3
	// BoostMap (map): 0x300-0x301 count bytes (unused), 0x302-0x303 x-axis
	// [5,15], 0x304-0x306 y-axis [100,200,300], 0x307-0x30C Z row-major 3x2.
	data[0x302], data[0x303] = 5, 15
	data[0x304], data[0x305], data[0x306] = 50, 100, 150
	data[0x307], data[0x308] = 1, 2
	data[0x309], data[0x30A] = 3, 4
	data[0x30B], data[0x30C] = 5, 6
	// AXIS_PTS.RPM_SHARED: 0x400-0x402 axis [1,2,3]. ShiftDelayShared's own
	// Z (no embedded axis of its own, 3 elements sized by the shared axis)
	// at 0x500-0x502.
	data[0x400], data[0x401], data[0x402] = 1, 2, 3
	data[0x500], data[0x501], data[0x502] = 7, 8, 9
	// GearShiftPoints (curve): 0x600 count byte (unused), 0x601-0x604
	// gear-index axis [0,1,2,3] (labeled via TAB_VERB, not decoded
	// differently), 0x605-0x608 Z shift RPM per gear [10,20,30,40].
	data[0x601], data[0x602], data[0x603], data[0x604] = 0, 1, 2, 3
	data[0x605], data[0x606], data[0x607], data[0x608] = 10, 20, 30, 40
	// FixAxisCurve (curve): no axis storage at all (FIX_AXIS_PAR_DIST
	// computes [10,15,20]); Z at 0x700-0x702.
	data[0x700], data[0x701], data[0x702] = 11, 22, 33
	// RescaleAxisCurve (curve): Z at 0x710-0x712.
	data[0x710], data[0x711], data[0x712] = 44, 55, 66
	// AXIS_PTS.PRESSURE_RESCALE: NO_RESCALE_X count byte (unused) at 0x800,
	// RESERVED padding (unused) at 0x801, then 3 (position, value) pairs at
	// 0x802-0x807 — position bytes are sentinels that must never be read.
	data[0x800], data[0x801] = 0xEE, 0xEE
	data[0x802], data[0x803] = 0xEE, 50
	data[0x804], data[0x805] = 0xEE, 100
	data[0x806], data[0x807] = 0xEE, 150
	// SharedCurveAxisCurve (curve): borrows ShiftDelay's own embedded X axis
	// (0x201-0x203, already set above); its own Z at 0x720-0x722.
	data[0x720], data[0x721], data[0x722] = 77, 88, 99
	// ValBlock (VAL_BLK, MATRIX_DIM 3 2 1): 2x3 row-major block at
	// 0x730-0x735, no axes at all.
	data[0x730], data[0x731], data[0x732] = 1, 2, 3
	data[0x733], data[0x734], data[0x735] = 4, 5, 6
	// RatFuncTarget: raw int8 10, phys = 5*10 = 50.
	data[0x740] = 10
	// FormTarget: raw int8 3, phys = 3+4 = 7.
	data[0x741] = 3
	// TabIntpTarget: raw uint8 5, interpolated between (0,100) and (10,110) -> 105.
	data[0x742] = 5
	// TabNointpTarget: raw uint8 1, exact match -> 110.
	data[0x743] = 1
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

	idle, err := decoder.DecodeParameter(document.Parameters[0])
	if err != nil {
		t.Fatal(err)
	}
	if idle.Z[0][0] != 42 {
		t.Fatalf("idle decoded = %v, want 42", idle.Z[0][0])
	}

	boost, err := decoder.DecodeParameter(document.Parameters[1])
	if err != nil {
		t.Fatal(err)
	}
	if boost.Z[0][0] != 5.0 {
		t.Fatalf("boost decoded = %v, want 5.0", boost.Z[0][0])
	}

	result, err := decoder.SetIndex(document.Parameters[1], 0, 10.0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Raw != 100 {
		t.Fatalf("boost raw after edit = %d, want 100", result.Raw)
	}
	if image.Bytes()[0x101] != 100 {
		t.Fatalf("boost byte after edit = %d, want 100", image.Bytes()[0x101])
	}

	rpm, err := decoder.DecodeParameter(document.Parameters[2])
	if err != nil {
		t.Fatal(err)
	}
	if rpm.Z[0][0] != 1234 {
		t.Fatalf("rpm decoded = %v, want 1234", rpm.Z[0][0])
	}
	if _, err := decoder.SetIndex(document.Parameters[2], 0, 4321); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x102] != 0xE1 || image.Bytes()[0x103] != 0x10 {
		t.Fatalf("rpm bytes after edit = %x %x, want little-endian 4321 (0x10E1)", image.Bytes()[0x102], image.Bytes()[0x103])
	}

	pressure, err := decoder.DecodeParameter(document.Parameters[3])
	if err != nil {
		t.Fatal(err)
	}
	if pressure.Z[0][0] != -300 {
		t.Fatalf("pressure decoded = %v, want -300", pressure.Z[0][0])
	}
	if _, err := decoder.SetIndex(document.Parameters[3], 0, -1); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x104] != 0xFF || image.Bytes()[0x105] != 0xFF {
		t.Fatalf("pressure bytes after edit = %x %x, want big-endian -1 (0xFFFF)", image.Bytes()[0x104], image.Bytes()[0x105])
	}

	curve, err := decoder.DecodeParameter(document.Parameters[4])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(curve.X, []float64{10, 20, 30}) || !reflect.DeepEqual(curve.Z, [][]float64{{1, 2, 3}}) {
		t.Fatalf("curve decoded X=%v Z=%v", curve.X, curve.Z)
	}
	if _, err := decoder.SetCell(document.Parameters[4], 0, 1, 99); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x205] != 99 {
		t.Fatalf("curve byte after edit = %d, want 99 (offset 0x205, the middle Z cell)", image.Bytes()[0x205])
	}

	boostMap, err := decoder.DecodeParameter(document.Parameters[5])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(boostMap.X, []float64{5, 15}) || !reflect.DeepEqual(boostMap.Y, []float64{50, 100, 150}) {
		t.Fatalf("map decoded axes X=%v Y=%v", boostMap.X, boostMap.Y)
	}
	if !reflect.DeepEqual(boostMap.Z, [][]float64{{1, 2}, {3, 4}, {5, 6}}) {
		t.Fatalf("map decoded Z = %v, want [[1 2] [3 4] [5 6]] (rows=y, columns=x)", boostMap.Z)
	}
	if _, err := decoder.SetCell(document.Parameters[5], 2, 1, 99); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x30C] != 99 {
		t.Fatalf("map byte after edit = %d, want 99 (offset 0x30C, row 2 col 1)", image.Bytes()[0x30C])
	}

	comAxisCurve, err := decoder.DecodeParameter(document.Parameters[6])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(comAxisCurve.X, []float64{1, 2, 3}) || !reflect.DeepEqual(comAxisCurve.Z, [][]float64{{7, 8, 9}}) {
		t.Fatalf("com axis curve decoded X=%v Z=%v, want X=[1 2 3] (shared axis) Z=[[7 8 9]]", comAxisCurve.X, comAxisCurve.Z)
	}
	if _, err := decoder.SetCell(document.Parameters[6], 0, 1, 42); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x501] != 42 {
		t.Fatalf("com axis curve byte after edit = %d, want 42 (offset 0x501)", image.Bytes()[0x501])
	}
	// Editing this curve must never touch the shared axis storage.
	if image.Bytes()[0x400] != 1 || image.Bytes()[0x401] != 2 || image.Bytes()[0x402] != 3 {
		t.Fatalf("shared axis storage was disturbed: %v", image.Bytes()[0x400:0x403])
	}

	gearCurve, err := decoder.DecodeParameter(document.Parameters[7])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gearCurve.X, []float64{0, 1, 2, 3}) || !reflect.DeepEqual(gearCurve.Z, [][]float64{{10, 20, 30, 40}}) {
		t.Fatalf("gear curve decoded X=%v Z=%v, want X=[0 1 2 3] (raw gear index, TAB_VERB only adds display text) Z=[[10 20 30 40]]", gearCurve.X, gearCurve.Z)
	}
	if _, err := decoder.SetCell(document.Parameters[7], 0, 2, 99); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x607] != 99 {
		t.Fatalf("gear curve byte after edit = %d, want 99 (offset 0x607, the third Z cell)", image.Bytes()[0x607])
	}

	fixAxisCurve, err := decoder.DecodeParameter(document.Parameters[8])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fixAxisCurve.X, []float64{10, 15, 20}) || !reflect.DeepEqual(fixAxisCurve.Z, [][]float64{{11, 22, 33}}) {
		t.Fatalf("fix axis curve decoded X=%v Z=%v, want X=[10 15 20] (FIX_AXIS_PAR_DIST, no on-disk storage) Z=[[11 22 33]]", fixAxisCurve.X, fixAxisCurve.Z)
	}
	if _, err := decoder.SetCell(document.Parameters[8], 0, 1, 100); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x701] != 100 {
		t.Fatalf("fix axis curve byte after edit = %d, want 100 (offset 0x701)", image.Bytes()[0x701])
	}

	rescaleAxisCurve, err := decoder.DecodeParameter(document.Parameters[9])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rescaleAxisCurve.X, []float64{50, 100, 150}) || !reflect.DeepEqual(rescaleAxisCurve.Z, [][]float64{{44, 55, 66}}) {
		t.Fatalf("rescale axis curve decoded X=%v Z=%v, want X=[50 100 150] (each pair's value element, position elements skipped) Z=[[44 55 66]]", rescaleAxisCurve.X, rescaleAxisCurve.Z)
	}
	if _, err := decoder.SetCell(document.Parameters[9], 0, 1, 100); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x711] != 100 {
		t.Fatalf("rescale axis curve byte after edit = %d, want 100 (offset 0x711)", image.Bytes()[0x711])
	}
	// Editing this curve must never touch the shared rescale axis storage.
	if image.Bytes()[0x802] != 0xEE || image.Bytes()[0x803] != 50 || image.Bytes()[0x806] != 0xEE || image.Bytes()[0x807] != 150 {
		t.Fatalf("rescale axis storage was disturbed: %v", image.Bytes()[0x802:0x808])
	}

	sharedCurveAxisCurve, err := decoder.DecodeParameter(document.Parameters[10])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sharedCurveAxisCurve.X, []float64{10, 20, 30}) || !reflect.DeepEqual(sharedCurveAxisCurve.Z, [][]float64{{77, 88, 99}}) {
		t.Fatalf("shared curve axis curve decoded X=%v Z=%v, want X=[10 20 30] (ShiftDelay's own axis) Z=[[77 88 99]]", sharedCurveAxisCurve.X, sharedCurveAxisCurve.Z)
	}
	if _, err := decoder.SetCell(document.Parameters[10], 0, 1, 100); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x721] != 100 {
		t.Fatalf("shared curve axis curve byte after edit = %d, want 100 (offset 0x721)", image.Bytes()[0x721])
	}
	// Editing this curve must never touch ShiftDelay's own borrowed axis storage.
	if image.Bytes()[0x201] != 10 || image.Bytes()[0x202] != 20 || image.Bytes()[0x203] != 30 {
		t.Fatalf("borrowed axis storage was disturbed: %v", image.Bytes()[0x201:0x204])
	}

	valBlock, err := decoder.DecodeParameter(document.Parameters[11])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(valBlock.Z, [][]float64{{1, 2, 3}, {4, 5, 6}}) {
		t.Fatalf("val block decoded Z = %v, want [[1 2 3] [4 5 6]] (row-major, no axes)", valBlock.Z)
	}
	if _, err := decoder.SetCell(document.Parameters[11], 1, 2, 99); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x735] != 99 {
		t.Fatalf("val block byte after edit = %d, want 99 (offset 0x735, row 1 col 2)", image.Bytes()[0x735])
	}

	ratFuncTarget, err := decoder.DecodeParameter(document.Parameters[12])
	if err != nil {
		t.Fatal(err)
	}
	if ratFuncTarget.Z[0][0] != 50 {
		t.Fatalf("rat func decoded = %v, want 50 (5*10)", ratFuncTarget.Z[0][0])
	}
	if _, err := decoder.SetIndex(document.Parameters[12], 0, 25); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x740] != 5 {
		t.Fatalf("rat func byte after edit = %d, want 5 (25/5)", image.Bytes()[0x740])
	}

	formTarget, err := decoder.DecodeParameter(document.Parameters[13])
	if err != nil {
		t.Fatal(err)
	}
	if formTarget.Z[0][0] != 7 {
		t.Fatalf("form decoded = %v, want 7 (3+4)", formTarget.Z[0][0])
	}
	if _, err := decoder.SetIndex(document.Parameters[13], 0, 10); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x741] != 6 {
		t.Fatalf("form byte after edit = %d, want 6 (10-4)", image.Bytes()[0x741])
	}

	tabIntpTarget, err := decoder.DecodeParameter(document.Parameters[14])
	if err != nil {
		t.Fatal(err)
	}
	if tabIntpTarget.Z[0][0] != 105 {
		t.Fatalf("tab intp decoded = %v, want 105 (interpolated between (0,100) and (10,110))", tabIntpTarget.Z[0][0])
	}
	if _, err := decoder.SetIndex(document.Parameters[14], 0, 120); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x742] != 15 {
		t.Fatalf("tab intp byte after edit = %d, want 15 (inverted interpolation between (10,110) and (20,130))", image.Bytes()[0x742])
	}

	tabNointpTarget, err := decoder.DecodeParameter(document.Parameters[15])
	if err != nil {
		t.Fatal(err)
	}
	if tabNointpTarget.Z[0][0] != 110 {
		t.Fatalf("tab nointp decoded = %v, want 110 (exact match)", tabNointpTarget.Z[0][0])
	}
	if _, err := decoder.SetIndex(document.Parameters[15], 0, 130); err != nil {
		t.Fatal(err)
	}
	if image.Bytes()[0x743] != 2 {
		t.Fatalf("tab nointp byte after edit = %d, want 2 (exact match)", image.Bytes()[0x743])
	}
}

func TestConvertRejectsUnsupportedCharacteristicType(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "a curve, not a scalar"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCharacteristicType) {
		t.Fatalf("error = %v, want ErrUnsupportedCharacteristicType", err)
	}
}

func TestConvertRejectsUnknownCompuMethod(t *testing.T) {
	source := syntheticSource(t, `
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "missing compu method"
      VALUE
      0x100
      RL
      0
      CM.MISSING
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrCompuMethodNotFound) {
		t.Fatalf("error = %v, want ErrCompuMethodNotFound", err)
	}
}

func TestConvertRejectsMultiByteWithoutDeclaredByteOrder(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UWORD ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "multi-byte with no BYTE_ORDER anywhere"
      VALUE
      0x100
      RL
      0
      CM.IDENTICAL
      0 65535
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedRecordLayout) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordLayout", err)
	}
}

func TestConvertRejectsUnsupportedRecordLayoutDataType(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 FLOAT32_IEEE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "float, not supported in this slice"
      VALUE
      0x100
      RL
      0
      CM.IDENTICAL
      -1e24 1e24
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedRecordLayout) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordLayout", err)
	}
}

func TestConvertRejectsUnsupportedMixedByteOrder(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 ULONG ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "mixed word/byte order has no mapdef representation"
      VALUE
      0x100
      RL
      0
      CM.IDENTICAL
      0 4294967295
      BYTE_ORDER MSB_FIRST_MSW_LAST
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedRecordLayout) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordLayout", err)
	}
}

func TestConvertCharacteristicByteOrderOverridesModuleDefault(t *testing.T) {
	source := syntheticSource(t, `
    /begin MOD_COMMON ""
      BYTE_ORDER MSB_LAST
    /end MOD_COMMON
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UWORD ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "overrides the module default"
      VALUE
      0x100
      RL
      0
      CM.IDENTICAL
      0 65535
      BYTE_ORDER MSB_FIRST
    /end CHARACTERISTIC`)
	document, err := (Converter{}).Convert(source, "test.a2l")
	if err != nil {
		t.Fatal(err)
	}
	if document.Parameters[0].Layout.ByteOrder != "big" {
		t.Fatalf("byte order = %q, want big (characteristic override beats the MSB_LAST module default)", document.Parameters[0].Layout.ByteOrder)
	}
}

func TestConvertHandlesNoCompuMethodSentinel(t *testing.T) {
	source := syntheticSource(t, `
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "no compu method"
      VALUE
      0x100
      RL
      0
      NO_COMPU_METHOD
      0 255
    /end CHARACTERISTIC`)
	document, err := (Converter{}).Convert(source, "test.a2l")
	if err != nil {
		t.Fatal(err)
	}
	if document.Parameters[0].Conversion.Expression != "X" {
		t.Fatalf("conversion = %#v", document.Parameters[0].Conversion)
	}
}

func TestConvertRejectsUnsupportedAxisAttribute(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "computed axis, not supported in this slice"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        FIX_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedAxisDescr) {
		t.Fatalf("error = %v, want ErrUnsupportedAxisDescr", err)
	}
}

func TestConvertRejectsUnknownAxisPtsRef(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "references an AXIS_PTS that doesn't exist"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        COM_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
        AXIS_PTS_REF DOES.NOT.EXIST
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrAxisPtsNotFound) {
		t.Fatalf("error = %v, want ErrAxisPtsNotFound", err)
	}
}

func TestConvertRejectsAxisPointsIndexDecr(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      AXIS_PTS_X 1 UBYTE INDEX_DECR DIRECT
      FNC_VALUES 2 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "descending axis storage, not supported in this slice"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        STD_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedRecordLayout) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordLayout", err)
	}
}

func TestConvertRejectsAxisDescrCountMismatchingType(t *testing.T) {
	tests := []struct {
		name           string
		characteristic string
	}{
		{"CURVE with zero axes", `
    /begin CHARACTERISTIC C
      "curve needs exactly one axis"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
    /end CHARACTERISTIC`},
		{"MAP with only one axis", `
    /begin CHARACTERISTIC C
      "map needs exactly two axes"
      MAP
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        STD_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
      /end AXIS_DESCR
    /end CHARACTERISTIC`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      AXIS_PTS_X 1 UBYTE INDEX_INCR DIRECT
      FNC_VALUES 2 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT`+test.characteristic)
			if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCharacteristicType) {
				t.Fatalf("error = %v, want ErrUnsupportedCharacteristicType", err)
			}
		})
	}
}

func TestConvertRejectsRecordLayoutMissingAxisPtsY(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      AXIS_PTS_X 1 UBYTE INDEX_INCR DIRECT
      FNC_VALUES 2 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "map record layout has no AXIS_PTS_Y"
      MAP
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        STD_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
      /end AXIS_DESCR
      /begin AXIS_DESCR
        STD_AXIS
        M.Y
        CM.IDENTICAL
        2
        0 255
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedRecordLayout) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordLayout", err)
	}
}

func TestConvertRejectsRecordLayoutEntryForAComAxis(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL.SHARED
      AXIS_PTS_X 1 UBYTE INDEX_INCR DIRECT
    /end RECORD_LAYOUT
    /begin AXIS_PTS SHARED.X
      "shared"
      0x200
      M.X
      RL.SHARED
      0
      CM.IDENTICAL
      3
      0 255
    /end AXIS_PTS
    /begin RECORD_LAYOUT RL
      AXIS_PTS_X 1 UBYTE INDEX_INCR DIRECT
      FNC_VALUES 2 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "AXIS_PTS_X entry present but the axis is COM_AXIS"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        COM_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
        AXIS_PTS_REF SHARED.X
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedRecordLayout) {
		t.Fatalf("error = %v, want ErrUnsupportedRecordLayout", err)
	}
}

func TestConvertRejectsTabVerbOnCharacteristicConversion(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.TAB_VERB.GEAR
      "gear labels"
      TAB_VERB "%3.0" ""
      COMPU_TAB_REF CM.TAB_VERB.GEAR.REF
    /end COMPU_METHOD
    /begin COMPU_VTAB CM.TAB_VERB.GEAR.REF
      "gear text"
      TAB_VERB 2
      0 "Park"
      1 "Drive"
    /end COMPU_VTAB
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "TAB_VERB as the characteristic's own conversion, not just an axis"
      VALUE
      0x100
      RL
      0
      CM.TAB_VERB.GEAR
      0 1
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCompuMethod) {
		t.Fatalf("error = %v, want ErrUnsupportedCompuMethod", err)
	}
}

func TestConvertRejectsUnknownCompuTabRef(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.TAB_VERB.GEAR
      "gear labels"
      TAB_VERB "%3.0" ""
      COMPU_TAB_REF DOES.NOT.EXIST
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin RECORD_LAYOUT RL.CURVE
      NO_AXIS_PTS_X 1 UBYTE
      AXIS_PTS_X 2 UBYTE INDEX_INCR DIRECT
      FNC_VALUES 3 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin CHARACTERISTIC C
      "axis conversion references a COMPU_TAB_REF that doesn't exist"
      CURVE
      0x100
      RL.CURVE
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        STD_AXIS
        M.X
        CM.TAB_VERB.GEAR
        3
        0 2
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrCompuVtabNotFound) {
		t.Fatalf("error = %v, want ErrCompuVtabNotFound", err)
	}
}

func TestConvertRejectsCompuVtabRange(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.TAB_VERB.PRESSURE_BAND
      "pressure band labels"
      TAB_VERB "%3.0" ""
      COMPU_TAB_REF CM.TAB_VERB.PRESSURE_BAND.REF
    /end COMPU_METHOD
    /begin COMPU_VTAB_RANGE CM.TAB_VERB.PRESSURE_BAND.REF
      "pressure band text"
      2
      0 127 "Low"
      128 255 "High"
    /end COMPU_VTAB_RANGE
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "axis conversion resolves to a COMPU_VTAB_RANGE, not a COMPU_VTAB"
      VALUE
      0x100
      RL
      0
      CM.TAB_VERB.PRESSURE_BAND
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrCompuVtabRangeUnsupported) {
		t.Fatalf("error = %v, want ErrCompuVtabRangeUnsupported", err)
	}
}

func TestConvertRejectsRescalePairCountMismatch(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL.AXIS_PTS.RESCALE
      AXIS_RESCALE_X 1 UBYTE 5 INDEX_INCR DIRECT
    /end RECORD_LAYOUT
    /begin AXIS_PTS AXIS_PTS.RESCALE
      "rescale"
      0x200
      M.X
      RL.AXIS_PTS.RESCALE
      0
      CM.IDENTICAL
      3
      0 255
    /end AXIS_PTS
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "rescale pair count (5) doesn't match AXIS_PTS MaxAxisPoints (3)"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        RES_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
        AXIS_PTS_REF AXIS_PTS.RESCALE
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrRescalePairCountMismatch) {
		t.Fatalf("error = %v, want ErrRescalePairCountMismatch", err)
	}
}

func TestConvertRejectsFixAxisWithBothParDistAndParList(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "FIX_AXIS with both FIX_AXIS_PAR_DIST and FIX_AXIS_PAR_LIST"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        FIX_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
        FIX_AXIS_PAR_DIST 0 1 3
        /begin FIX_AXIS_PAR_LIST
          0 1 2
        /end FIX_AXIS_PAR_LIST
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedAxisDescr) {
		t.Fatalf("error = %v, want ErrUnsupportedAxisDescr", err)
	}
}

func TestConvertRejectsFixAxisWithNeitherParDistNorParList(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "FIX_AXIS with neither mechanism"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        FIX_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedAxisDescr) {
		t.Fatalf("error = %v, want ErrUnsupportedAxisDescr", err)
	}
}

func TestConvertRejectsFixAxisPointCountMismatch(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "FIX_AXIS_PAR_DIST computes 5 points but MaxAxisPoints says 3"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        FIX_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
        FIX_AXIS_PAR_DIST 0 1 5
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedAxisDescr) {
		t.Fatalf("error = %v, want ErrUnsupportedAxisDescr", err)
	}
}

func TestConvertRejectsCurveAxisRefNotFound(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "references a CHARACTERISTIC that doesn't exist"
      CURVE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        CURVE_AXIS
        M.X
        NO_COMPU_METHOD
        3
        0 255
        CURVE_AXIS_REF DOES.NOT.EXIST
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrCurveAxisNotFound) {
		t.Fatalf("error = %v, want ErrCurveAxisNotFound", err)
	}
}

func TestConvertRejectsCurveAxisRefToNonCurve(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC ScalarTarget
      "a VALUE, not a CURVE"
      VALUE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
    /end CHARACTERISTIC
    /begin CHARACTERISTIC C
      "references a VALUE characteristic, not a CURVE"
      CURVE
      0x101
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        CURVE_AXIS
        M.X
        NO_COMPU_METHOD
        3
        0 255
        CURVE_AXIS_REF ScalarTarget
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedAxisDescr) {
		t.Fatalf("error = %v, want ErrUnsupportedAxisDescr", err)
	}
}

func TestConvertRejectsChainedCurveAxis(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL.CURVE
      NO_AXIS_PTS_X 1 UBYTE
      AXIS_PTS_X 2 UBYTE INDEX_INCR DIRECT
      FNC_VALUES 3 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC Base
      "a real STD_AXIS curve"
      CURVE
      0x100
      RL.CURVE
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        STD_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
      /end AXIS_DESCR
    /end CHARACTERISTIC
    /begin CHARACTERISTIC Middle
      "borrows Base's axis via CURVE_AXIS"
      CURVE
      0x200
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        CURVE_AXIS
        M.X
        NO_COMPU_METHOD
        3
        0 255
        CURVE_AXIS_REF Base
      /end AXIS_DESCR
    /end CHARACTERISTIC
    /begin CHARACTERISTIC Chained
      "tries to borrow Middle's already-borrowed axis"
      CURVE
      0x201
      RL
      0
      CM.IDENTICAL
      0 255
      /begin AXIS_DESCR
        CURVE_AXIS
        M.X
        NO_COMPU_METHOD
        3
        0 255
        CURVE_AXIS_REF Middle
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedAxisDescr) {
		t.Fatalf("error = %v, want ErrUnsupportedAxisDescr", err)
	}
}

func TestConvertValBlk1D(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "a 1-dimensional VAL_BLK (y=1)"
      VAL_BLK
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      MATRIX_DIM 4 1 1
    /end CHARACTERISTIC`)
	document, err := (Converter{}).Convert(source, "test.a2l")
	if err != nil {
		t.Fatal(err)
	}
	if document.Parameters[0].Kind != "map" || !reflect.DeepEqual(document.Parameters[0].Layout.Dimensions, []int{1, 4}) {
		t.Fatalf("1D val block = %#v, want Kind map with dimensions [1 4]", document.Parameters[0])
	}
}

func TestConvertRejectsValBlkWithoutMatrixDim(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "VAL_BLK with no MATRIX_DIM"
      VAL_BLK
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCharacteristicType) {
		t.Fatalf("error = %v, want ErrUnsupportedCharacteristicType", err)
	}
}

func TestConvertRejectsValBlk3D(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "a genuinely 3-dimensional VAL_BLK, not supported in this slice"
      VAL_BLK
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      MATRIX_DIM 2 2 2
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCharacteristicType) {
		t.Fatalf("error = %v, want ErrUnsupportedCharacteristicType", err)
	}
}

func TestConvertRejectsValBlkWithAxisDescr(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      NO_AXIS_PTS_X 1 UBYTE
      AXIS_PTS_X 2 UBYTE INDEX_INCR DIRECT
      FNC_VALUES 3 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "VAL_BLK has no AXIS_DESCR of its own"
      VAL_BLK
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
      MATRIX_DIM 3 1 1
      /begin AXIS_DESCR
        STD_AXIS
        M.X
        CM.IDENTICAL
        3
        0 255
      /end AXIS_DESCR
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCharacteristicType) {
		t.Fatalf("error = %v, want ErrUnsupportedCharacteristicType", err)
	}
}

func TestConvertGroupCategoryIDsAreUnique(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "scalar"
      VALUE
      0x100
      RL
      0
      CM.IDENTICAL
      0 255
    /end CHARACTERISTIC
    /begin GROUP Group.A ""
      /begin REF_CHARACTERISTIC
        C
      /end REF_CHARACTERISTIC
    /end GROUP
    /begin GROUP Group_A ""
      /begin REF_CHARACTERISTIC
        C
      /end REF_CHARACTERISTIC
    /end GROUP`)
	document, err := (Converter{}).Convert(source, "test.a2l")
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Categories) != 2 || document.Categories[0].ID == document.Categories[1].ID {
		t.Fatalf("categories = %#v, want two distinct IDs (Group.A and Group_A both normalize to \"group-a\")", document.Categories)
	}
	if !reflect.DeepEqual(document.Parameters[0].Categories, []string{document.Categories[0].ID, document.Categories[1].ID}) {
		t.Fatalf("parameter categories = %#v, want membership in both groups", document.Parameters[0].Categories)
	}
}

func TestConvertRejectsRatFuncWithoutCoeffs(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.RAT_FUNC
      "no COEFFS"
      RAT_FUNC "%4.0" "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "RAT_FUNC with no COEFFS"
      VALUE
      0x100
      RL
      0
      CM.RAT_FUNC
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCompuMethod) {
		t.Fatalf("error = %v, want ErrUnsupportedCompuMethod", err)
	}
}

func TestConvertRejectsRatFuncZeroDenominator(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.RAT_FUNC
      "denominator always zero"
      RAT_FUNC "%4.0" "counts"
      COEFFS 0 1 0 0 0 0
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "RAT_FUNC with d=e=f=0"
      VALUE
      0x100
      RL
      0
      CM.RAT_FUNC
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCompuMethod) {
		t.Fatalf("error = %v, want ErrUnsupportedCompuMethod", err)
	}
}

func TestConvertNonAffineRatFuncDecodesButCannotBeEdited(t *testing.T) {
	// COEFFS 1 0 0 0 0 1 -> phys = int^2, genuinely non-affine: decode
	// always works (expression.Eval handles any arithmetic), but editing
	// needs an algebraic inverse this project doesn't attempt to derive
	// for a non-affine expression — the same precedent XDF's arbitrary
	// MATH formulas already established.
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.RAT_FUNC.SQUARE
      "phys = int^2"
      RAT_FUNC "%4.0" "counts"
      COEFFS 1 0 0 0 0 1
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "non-affine RAT_FUNC"
      VALUE
      0x100
      RL
      0
      CM.RAT_FUNC.SQUARE
      0 255
    /end CHARACTERISTIC`)
	document, err := (Converter{}).Convert(source, "test.a2l")
	if err != nil {
		t.Fatal(err)
	}
	if document.Parameters[0].Conversion.Expression != "X*X" {
		t.Fatalf("conversion = %#v, want X*X", document.Parameters[0].Conversion)
	}
	data := make([]byte, 0x200)
	data[0x100] = 4
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
	decoded, err := decoder.DecodeParameter(document.Parameters[0])
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Z[0][0] != 16 {
		t.Fatalf("decoded = %v, want 16 (4^2)", decoded.Z[0][0])
	}
	if _, err := decoder.SetIndex(document.Parameters[0], 0, 25); !errors.Is(err, expression.ErrNonAffine) {
		t.Fatalf("error = %v, want expression.ErrNonAffine", err)
	}
}

func TestConvertRejectsFormWithoutFormula(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.FORM
      "no FORMULA"
      FORM
      "%3.0"
      "counts"
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "FORM with no FORMULA"
      VALUE
      0x100
      RL
      0
      CM.FORM
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCompuMethod) {
		t.Fatalf("error = %v, want ErrUnsupportedCompuMethod", err)
	}
}

func TestConvertRejectsFormMultiVariable(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.FORM
      "references a second input this slice has no way to resolve"
      FORM
      "%3.0"
      "counts"
      /begin FORMULA
        "X1+X2"
      /end FORMULA
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "multi-variable FORM"
      VALUE
      0x100
      RL
      0
      CM.FORM
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCompuMethod) {
		t.Fatalf("error = %v, want ErrUnsupportedCompuMethod", err)
	}
}

func TestConvertRejectsFormUnsupportedSyntax(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.FORM
      "uses a function call freehorse-expression-v1 doesn't support"
      FORM
      "%3.0"
      "counts"
      /begin FORMULA
        "SIN(X1)"
      /end FORMULA
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "unsupported FORM syntax"
      VALUE
      0x100
      RL
      0
      CM.FORM
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrUnsupportedCompuMethod) {
		t.Fatalf("error = %v, want ErrUnsupportedCompuMethod", err)
	}
}

func TestConvertRejectsUnknownCompuTabRefForTabIntp(t *testing.T) {
	source := syntheticSource(t, `
    /begin COMPU_METHOD CM.TAB_INTP
      "references a COMPU_TAB that doesn't exist"
      TAB_INTP "%4.0" "counts"
      COMPU_TAB_REF DOES.NOT.EXIST
    /end COMPU_METHOD
    /begin RECORD_LAYOUT RL
      FNC_VALUES 1 UBYTE ROW_DIR DIRECT
    /end RECORD_LAYOUT
    /begin CHARACTERISTIC C
      "unresolvable COMPU_TAB_REF"
      VALUE
      0x100
      RL
      0
      CM.TAB_INTP
      0 255
    /end CHARACTERISTIC`)
	if _, err := (Converter{}).Convert(source, "test.a2l"); !errors.Is(err, ErrCompuTabNotFound) {
		t.Fatalf("error = %v, want ErrCompuTabNotFound", err)
	}
}

func syntheticSource(t *testing.T, moduleBody string) *types.A2LDefinition {
	t.Helper()
	document := "ASAP2_VERSION 1 61\n/begin PROJECT P \"\"\n  /begin MODULE M \"\"\n" + moduleBody + "\n  /end MODULE\n/end PROJECT\n"
	source, err := Parse(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	return source
}
