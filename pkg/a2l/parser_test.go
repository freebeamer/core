package a2l

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func TestParseSyntheticFixture(t *testing.T) {
	definition, err := Load(filepath.Join("testdata", "basic.a2l"))
	if err != nil {
		t.Fatal(err)
	}
	if definition.Project.Name != "SyntheticExample" {
		t.Errorf("project name = %q", definition.Project.Name)
	}
	module := definition.Project.Module
	if module.Name != "Example" {
		t.Errorf("module name = %q", module.Name)
	}
	if len(module.CompuMethods) != 7 || len(module.CompuVtabs) != 1 || len(module.CompuVtabRanges) != 1 || len(module.CompuTabs) != 2 || len(module.RecordLayouts) != 9 || len(module.Characteristics) != 16 || len(module.AxisPts) != 2 || len(module.Groups) != 2 {
		t.Fatalf("counts = compuMethods %d compuVtabs %d compuVtabRanges %d compuTabs %d recordLayouts %d characteristics %d axisPts %d groups %d",
			len(module.CompuMethods), len(module.CompuVtabs), len(module.CompuVtabRanges), len(module.CompuTabs), len(module.RecordLayouts), len(module.Characteristics), len(module.AxisPts), len(module.Groups))
	}
	if module.ByteOrder != "MSB_LAST" {
		t.Errorf("module byte order = %q", module.ByteOrder)
	}

	linear := module.CompuMethods[1]
	if linear.Name != "CM.LINEAR.BOOST" || linear.ConversionType != "LINEAR" || linear.Unit != "kPa" {
		t.Errorf("linear compu method = %#v", linear)
	}
	if !linear.HasCoeffsLinear || linear.CoeffsLinearA != 0.1 || linear.CoeffsLinearB != 0 {
		t.Errorf("linear coeffs = %#v", linear)
	}

	ratFunc := module.CompuMethods[3]
	if ratFunc.Name != "CM.RAT_FUNC.SCALE" || ratFunc.ConversionType != "RAT_FUNC" || ratFunc.Unit != "counts" {
		t.Errorf("rat func compu method = %#v", ratFunc)
	}
	wantCoeffs := [6]float64{0, 5, 0, 0, 0, 1}
	gotCoeffs := [6]float64{ratFunc.CoeffsA, ratFunc.CoeffsB, ratFunc.CoeffsC, ratFunc.CoeffsD, ratFunc.CoeffsE, ratFunc.CoeffsF}
	if !ratFunc.HasCoeffs || gotCoeffs != wantCoeffs {
		t.Errorf("rat func coeffs = %#v, want %#v (hasCoeffs=%v)", gotCoeffs, wantCoeffs, ratFunc.HasCoeffs)
	}

	form := module.CompuMethods[4]
	if form.Name != "CM.FORM.PLUS4" || form.ConversionType != "FORM" || form.Formula != "X1+4" || form.FormulaInv != "X1-4" {
		t.Errorf("form compu method = %#v", form)
	}

	tabIntp := module.CompuMethods[5]
	if tabIntp.Name != "CM.TAB_INTP.RPM" || tabIntp.ConversionType != "TAB_INTP" || tabIntp.CompuTabRef != "CM.TAB_INTP.RPM.REF" {
		t.Errorf("tab intp compu method = %#v", tabIntp)
	}
	tabNointp := module.CompuMethods[6]
	if tabNointp.Name != "CM.TAB_NOINTP.GEAR" || tabNointp.ConversionType != "TAB_NOINTP" || tabNointp.CompuTabRef != "CM.TAB_NOINTP.GEAR.REF" {
		t.Errorf("tab nointp compu method = %#v", tabNointp)
	}

	compuTabIntp := module.CompuTabs[0]
	wantIntpEntries := []types.A2LCompuTabEntry{{InVal: 0, OutVal: 100}, {InVal: 10, OutVal: 110}, {InVal: 20, OutVal: 130}}
	if compuTabIntp.Name != "CM.TAB_INTP.RPM.REF" || !compuTabIntp.Interpolated || !reflect.DeepEqual(compuTabIntp.Entries, wantIntpEntries) {
		t.Errorf("compu tab (interpolated) = %#v", compuTabIntp)
	}
	if !compuTabIntp.HasDefaultValue || compuTabIntp.DefaultValue != 999 {
		t.Errorf("compu tab (interpolated) default = %v hasDefault=%v", compuTabIntp.DefaultValue, compuTabIntp.HasDefaultValue)
	}
	compuTabNointp := module.CompuTabs[1]
	wantNointpEntries := []types.A2LCompuTabEntry{{InVal: 0, OutVal: 100}, {InVal: 1, OutVal: 110}, {InVal: 2, OutVal: 130}}
	if compuTabNointp.Name != "CM.TAB_NOINTP.GEAR.REF" || compuTabNointp.Interpolated || !reflect.DeepEqual(compuTabNointp.Entries, wantNointpEntries) {
		t.Errorf("compu tab (stepped) = %#v", compuTabNointp)
	}
	if compuTabNointp.HasDefaultValue {
		t.Errorf("compu tab (stepped) unexpectedly has a default value: %v", compuTabNointp.DefaultValue)
	}

	sbyteLayout := module.RecordLayouts[1]
	wantEntry := types.A2LRecordLayoutEntry{Role: "FNC_VALUES", Position: 1, DataType: "SBYTE", IndexMode: "ROW_DIR", AddrType: "DIRECT"}
	if sbyteLayout.Name != "RL.SBYTE" || len(sbyteLayout.Entries) != 1 || sbyteLayout.Entries[0] != wantEntry {
		t.Errorf("record layout = %#v", sbyteLayout)
	}

	boost := module.Characteristics[1]
	if boost.Name != "BoostTarget" || boost.Type != "VALUE" || boost.Address != 0x101 || boost.Deposit != "RL.SBYTE" {
		t.Errorf("characteristic = %#v", boost)
	}
	if boost.Conversion != "CM.LINEAR.BOOST" || boost.LowerLimit != -128 || boost.UpperLimit != 127 {
		t.Errorf("characteristic conversion/limits = %#v", boost)
	}
	if boost.Format != "%5.2" || boost.ExtendedLimits == nil || boost.ExtendedLimits.Low != -128 || boost.ExtendedLimits.High != 127 {
		t.Errorf("characteristic optional fields = %#v", boost)
	}

	idle := module.Characteristics[0]
	if idle.DisplayIdentifier != "DI.IdleTarget" {
		t.Errorf("idle characteristic = %#v", idle)
	}

	pressure := module.Characteristics[3]
	if pressure.Name != "PressureTarget" || pressure.Deposit != "RL.SWORD" || pressure.ByteOrder != "MSB_FIRST" {
		t.Errorf("pressure characteristic = %#v", pressure)
	}

	curve := module.Characteristics[4]
	if curve.Name != "ShiftDelay" || curve.Type != "CURVE" || len(curve.AxisDescrs) != 1 {
		t.Fatalf("curve characteristic = %#v", curve)
	}
	if curve.AxisDescrs[0].Attribute != "STD_AXIS" || curve.AxisDescrs[0].MaxAxisPoints != 3 || curve.AxisDescrs[0].Conversion != "CM.IDENTICAL" {
		t.Errorf("curve axis descr = %#v", curve.AxisDescrs[0])
	}
	curveLayout := module.RecordLayouts[4]
	if curveLayout.Name != "RL.CURVE.UBYTE" || len(curveLayout.Entries) != 3 {
		t.Errorf("curve record layout = %#v", curveLayout)
	}

	mapCharacteristic := module.Characteristics[5]
	if mapCharacteristic.Name != "BoostMap" || mapCharacteristic.Type != "MAP" || len(mapCharacteristic.AxisDescrs) != 2 {
		t.Fatalf("map characteristic = %#v", mapCharacteristic)
	}
	if mapCharacteristic.AxisDescrs[0].MaxAxisPoints != 2 || mapCharacteristic.AxisDescrs[1].MaxAxisPoints != 3 {
		t.Errorf("map axis descrs = %#v", mapCharacteristic.AxisDescrs)
	}

	comAxisCurve := module.Characteristics[6]
	if comAxisCurve.Name != "ShiftDelayShared" || len(comAxisCurve.AxisDescrs) != 1 {
		t.Fatalf("com axis curve = %#v", comAxisCurve)
	}
	if comAxisCurve.AxisDescrs[0].Attribute != "COM_AXIS" || comAxisCurve.AxisDescrs[0].AxisPtsRef != "AXIS_PTS.RPM_SHARED" {
		t.Errorf("com axis descr = %#v", comAxisCurve.AxisDescrs[0])
	}

	axisPts := module.AxisPts[0]
	if axisPts.Name != "AXIS_PTS.RPM_SHARED" || axisPts.Address != 0x400 || axisPts.Deposit != "RL.AXIS_PTS.UBYTE" || axisPts.MaxAxisPoints != 3 {
		t.Errorf("axis pts = %#v", axisPts)
	}
	if axisPts.DisplayIdentifier != "DI.AXIS_PTS.RPM_SHARED" {
		t.Errorf("axis pts display identifier = %q", axisPts.DisplayIdentifier)
	}

	tabVerb := module.CompuMethods[2]
	if tabVerb.Name != "CM.TAB_VERB.GEAR" || tabVerb.ConversionType != "TAB_VERB" || tabVerb.CompuTabRef != "CM.TAB_VERB.GEAR.REF" {
		t.Errorf("tab verb compu method = %#v", tabVerb)
	}

	compuVtab := module.CompuVtabs[0]
	wantEntries := []types.A2LCompuVtabEntry{{Value: 0, Text: "Park"}, {Value: 1, Text: "Reverse"}, {Value: 2, Text: "Neutral"}, {Value: 3, Text: "Drive"}}
	if compuVtab.Name != "CM.TAB_VERB.GEAR.REF" || len(compuVtab.Entries) != 4 {
		t.Fatalf("compu vtab = %#v", compuVtab)
	}
	for i, want := range wantEntries {
		if compuVtab.Entries[i] != want {
			t.Errorf("compu vtab entry %d = %#v, want %#v", i, compuVtab.Entries[i], want)
		}
	}
	if !compuVtab.HasDefaultValue || compuVtab.DefaultValue != "unknown gear" {
		t.Errorf("compu vtab default value = %q hasDefault=%v", compuVtab.DefaultValue, compuVtab.HasDefaultValue)
	}

	gearCurve := module.Characteristics[7]
	if gearCurve.Name != "GearShiftPoints" || len(gearCurve.AxisDescrs) != 1 || gearCurve.AxisDescrs[0].Conversion != "CM.TAB_VERB.GEAR" {
		t.Errorf("gear curve = %#v", gearCurve)
	}

	fixAxisCurve := module.Characteristics[8]
	if fixAxisCurve.Name != "FixAxisCurve" || len(fixAxisCurve.AxisDescrs) != 1 {
		t.Fatalf("fix axis curve = %#v", fixAxisCurve)
	}
	fixAxisDescr := fixAxisCurve.AxisDescrs[0]
	wantFixAxisParDist := &types.A2LFixAxisParDist{Offset: 10, Distance: 5, NumberPts: 3}
	if fixAxisDescr.Attribute != "FIX_AXIS" || fixAxisDescr.FixAxisParDist == nil || *fixAxisDescr.FixAxisParDist != *wantFixAxisParDist {
		t.Errorf("fix axis descr = %#v", fixAxisDescr)
	}
	if fixAxisDescr.FixAxisParList != nil {
		t.Errorf("fix axis par list = %#v, want nil", fixAxisDescr.FixAxisParList)
	}

	rescaleAxisCurve := module.Characteristics[9]
	if rescaleAxisCurve.Name != "RescaleAxisCurve" || len(rescaleAxisCurve.AxisDescrs) != 1 {
		t.Fatalf("rescale axis curve = %#v", rescaleAxisCurve)
	}
	rescaleAxisDescr := rescaleAxisCurve.AxisDescrs[0]
	if rescaleAxisDescr.Attribute != "RES_AXIS" || rescaleAxisDescr.AxisPtsRef != "AXIS_PTS.PRESSURE_RESCALE" {
		t.Errorf("rescale axis descr = %#v", rescaleAxisDescr)
	}

	sharedCurveAxisCurve := module.Characteristics[10]
	if sharedCurveAxisCurve.Name != "SharedCurveAxisCurve" || len(sharedCurveAxisCurve.AxisDescrs) != 1 {
		t.Fatalf("shared curve axis curve = %#v", sharedCurveAxisCurve)
	}
	curveAxisDescr := sharedCurveAxisCurve.AxisDescrs[0]
	if curveAxisDescr.Attribute != "CURVE_AXIS" || curveAxisDescr.CurveAxisRef != "ShiftDelay" || curveAxisDescr.Conversion != "NO_COMPU_METHOD" {
		t.Errorf("curve axis descr = %#v", curveAxisDescr)
	}

	valBlock := module.Characteristics[11]
	wantMatrixDim := &types.A2LMatrixDim{X: 3, Y: 2, Z: 1}
	if valBlock.Name != "ValBlock" || valBlock.Type != "VAL_BLK" || len(valBlock.AxisDescrs) != 0 || valBlock.MatrixDim == nil || *valBlock.MatrixDim != *wantMatrixDim {
		t.Errorf("val block = %#v", valBlock)
	}

	ratFuncTarget := module.Characteristics[12]
	if ratFuncTarget.Name != "RatFuncTarget" || ratFuncTarget.Conversion != "CM.RAT_FUNC.SCALE" {
		t.Errorf("rat func target = %#v", ratFuncTarget)
	}
	formTarget := module.Characteristics[13]
	if formTarget.Name != "FormTarget" || formTarget.Conversion != "CM.FORM.PLUS4" {
		t.Errorf("form target = %#v", formTarget)
	}
	tabIntpTarget := module.Characteristics[14]
	if tabIntpTarget.Name != "TabIntpTarget" || tabIntpTarget.Conversion != "CM.TAB_INTP.RPM" {
		t.Errorf("tab intp target = %#v", tabIntpTarget)
	}
	tabNointpTarget := module.Characteristics[15]
	if tabNointpTarget.Name != "TabNointpTarget" || tabNointpTarget.Conversion != "CM.TAB_NOINTP.GEAR" {
		t.Errorf("tab nointp target = %#v", tabNointpTarget)
	}

	groupAll := module.Groups[0]
	if groupAll.Name != "Group_All" || !groupAll.Root || !reflect.DeepEqual(groupAll.SubGroups, []string{"Group_Scalars"}) {
		t.Errorf("group all = %#v", groupAll)
	}
	groupScalars := module.Groups[1]
	if groupScalars.Name != "Group_Scalars" || groupScalars.Root {
		t.Errorf("group scalars = %#v", groupScalars)
	}
	if !reflect.DeepEqual(groupScalars.RefCharacteristics, []string{"IdleTarget", "BoostTarget"}) {
		t.Errorf("group scalars ref characteristics = %#v", groupScalars.RefCharacteristics)
	}
	if !reflect.DeepEqual(groupScalars.RefMeasurements, []string{"M.SOME.MEASUREMENT"}) {
		t.Errorf("group scalars ref measurements = %#v", groupScalars.RefMeasurements)
	}
	if !reflect.DeepEqual(groupScalars.FunctionRefs, []string{"SomeFunction"}) {
		t.Errorf("group scalars function refs = %#v", groupScalars.FunctionRefs)
	}

	rescaleAxisPts := module.AxisPts[1]
	if rescaleAxisPts.Name != "AXIS_PTS.PRESSURE_RESCALE" || rescaleAxisPts.Address != 0x800 || rescaleAxisPts.Deposit != "RL.AXIS_PTS.RESCALE" || rescaleAxisPts.MaxAxisPoints != 3 {
		t.Errorf("rescale axis pts = %#v", rescaleAxisPts)
	}
	rescaleRecordLayout := module.RecordLayouts[8]
	wantRescaleEntries := []types.A2LRecordLayoutEntry{
		{Role: "NO_RESCALE_X", Position: 1, DataType: "UBYTE"},
		{Role: "RESERVED", Position: 2, DataType: "BYTE"},
		{Role: "AXIS_RESCALE_X", Position: 3, DataType: "UBYTE", RescalePairs: 3, IndexMode: "INDEX_INCR", AddrType: "DIRECT"},
	}
	if rescaleRecordLayout.Name != "RL.AXIS_PTS.RESCALE" || len(rescaleRecordLayout.Entries) != 3 {
		t.Fatalf("rescale record layout = %#v", rescaleRecordLayout)
	}
	for i, want := range wantRescaleEntries {
		if rescaleRecordLayout.Entries[i] != want {
			t.Errorf("rescale record layout entry %d = %#v, want %#v", i, rescaleRecordLayout.Entries[i], want)
		}
	}

	compuVtabRange := module.CompuVtabRanges[0]
	wantRangeEntries := []types.A2LCompuVtabRangeEntry{
		{LowerValue: 0, UpperValue: 49, Text: "Low"},
		{LowerValue: 50, UpperValue: 99, Text: "Normal"},
		{LowerValue: 100, UpperValue: 255, Text: "High"},
	}
	if compuVtabRange.Name != "CM.TAB_VERB.PRESSURE_BAND.REF" || len(compuVtabRange.Entries) != 3 {
		t.Fatalf("compu vtab range = %#v", compuVtabRange)
	}
	for i, want := range wantRangeEntries {
		if compuVtabRange.Entries[i] != want {
			t.Errorf("compu vtab range entry %d = %#v, want %#v", i, compuVtabRange.Entries[i], want)
		}
	}
	if !compuVtabRange.HasDefaultValue || compuVtabRange.DefaultValue != "out of range" {
		t.Errorf("compu vtab range default value = %q hasDefault=%v", compuVtabRange.DefaultValue, compuVtabRange.HasDefaultValue)
	}
}

func TestParseSkipsUnknownBlocks(t *testing.T) {
	source := `ASAP2_VERSION 1 61
/begin PROJECT P ""
  /begin MODULE M ""
    /begin A2ML
      block "IF_DATA" taggedunion if_data {
        "XCP" struct { char[100]; };
      };
    /end A2ML
    /begin IF_DATA XCP
      /begin PROTOCOL_LAYER
        0x0100
      /end PROTOCOL_LAYER
    /end IF_DATA
    /begin COMPU_METHOD CM.IDENTICAL
      "identity"
      IDENTICAL "%3.0" "counts"
    /end COMPU_METHOD
  /end MODULE
/end PROJECT
`
	definition, err := Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Project.Module.CompuMethods) != 1 {
		t.Fatalf("compu methods = %#v", definition.Project.Module.CompuMethods)
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing end project", `/begin PROJECT P ""` + "\n  /begin MODULE M \"\"\n  /end MODULE\n"},
		{"unterminated string", `/begin PROJECT P "` + "\n"},
		{"no module", `/begin PROJECT P ""` + "\n/end PROJECT\n"},
		{"record layout missing FNC_VALUES", `/begin PROJECT P ""` + "\n  /begin MODULE M \"\"\n" +
			"    /begin RECORD_LAYOUT RL\n      IDENTIFICATION 1 UBYTE\n    /end RECORD_LAYOUT\n" +
			"  /end MODULE\n/end PROJECT\n"},
		{"group with unsupported nested block", `/begin PROJECT P ""` + "\n  /begin MODULE M \"\"\n" +
			"    /begin GROUP G \"\"\n      /begin ANNOTATION\n      /end ANNOTATION\n    /end GROUP\n" +
			"  /end MODULE\n/end PROJECT\n"},
		{"group with unsupported bare keyword", `/begin PROJECT P ""` + "\n  /begin MODULE M \"\"\n" +
			"    /begin GROUP G \"\"\n      LEAF\n    /end GROUP\n" +
			"  /end MODULE\n/end PROJECT\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(test.body)); err == nil {
				t.Fatal("malformed input was accepted")
			}
		})
	}
}
