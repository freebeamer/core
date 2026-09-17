package xdf

import (
	"fmt"
	"math/bits"

	"github.com/freebeamer/core/pkg/types"
)

func convertTable(table types.XDFTable, index int, defaults types.XDFDefaults, categories map[int]string) (types.MapParameter, []types.ImportDiagnostic) {
	rows := positiveOrOne(table.Z.Embedded.RowCount)
	columns := positiveOrOne(table.Z.Embedded.ColumnCount)
	path := fmt.Sprintf("tables[%d].z", index)
	layout, diagnostics := convertLayout(table.Z.Embedded, defaults, []int{rows, columns}, path)

	sourceMetadata := map[string]interface{}{"xdf.attributes": table.Raw}
	if table.Z.Min != nil || table.Z.Max != nil {
		sourceMetadata["xdf.bounds"] = boundsMap(table.Z.Min, table.Z.Max)
	}
	parameter := types.MapParameter{
		ID:          identifier(table.ID, fmt.Sprintf("table-%d", index)),
		Kind:        classifyTable(table, rows, columns),
		Name:        valueOrFallback(table.Title, fmt.Sprintf("Table %d", index)),
		Description: table.Description,
		Categories:  resolveCategories(table.Categories, categories),
		Layout:      layout,
		Unit:        table.Z.Units,
		Conversion:  convertFormula(table.Z.Formula),
		Display: types.Display{
			DecimalPlaces: table.Z.DecimalPlaces,
			OutputType:    table.Z.OutputType,
			Min:           table.Z.Min,
			Max:           table.Z.Max,
		},
		SourceMetadata: sourceMetadata,
	}

	parameter.Axes, diagnostics = convertAxes(table, index, defaults, diagnostics)
	return parameter, diagnostics
}

func classifyTable(table types.XDFTable, rows, columns int) string {
	if rows == 1 && columns == 1 && table.X.ID == "" && table.Y.ID == "" {
		return "scalar"
	}
	if rows == 1 || columns == 1 {
		return "curve"
	}
	return "map"
}

func convertAxes(table types.XDFTable, tableIndex int, defaults types.XDFDefaults, diagnostics []types.ImportDiagnostic) (map[string]types.MapAxis, []types.ImportDiagnostic) {
	result := make(map[string]types.MapAxis)
	for name, source := range map[string]types.XDFAxis{"x": table.X, "y": table.Y} {
		if source.ID == "" {
			continue
		}
		axis, axisDiagnostics := convertAxis(source, name, tableIndex, defaults)
		result[name] = axis
		diagnostics = append(diagnostics, axisDiagnostics...)
	}
	if len(result) == 0 {
		return nil, diagnostics
	}
	return result, diagnostics
}

func convertAxis(source types.XDFAxis, name string, tableIndex int, defaults types.XDFDefaults) (types.MapAxis, []types.ImportDiagnostic) {
	count := source.IndexCount
	if count < 1 {
		count = positiveOrOne(source.Embedded.ColumnCount)
	}
	path := fmt.Sprintf("tables[%d].%s", tableIndex, name)
	sourceMetadata := map[string]interface{}{
		"xdf.attributes":         source.Raw,
		"xdf.embeddedAttributes": source.Embedded.Raw,
	}
	if len(source.Labels) > 0 {
		entries := make([]map[string]interface{}, len(source.Labels))
		for i, label := range source.Labels {
			entries[i] = map[string]interface{}{"index": label.Index, "value": label.Value}
		}
		sourceMetadata["xdf.labels"] = entries
	}
	if source.Min != nil || source.Max != nil {
		sourceMetadata["xdf.bounds"] = boundsMap(source.Min, source.Max)
	}
	axis := types.MapAxis{
		Count:        count,
		Unit:         source.Units,
		Conversion:   convertFormula(source.Formula),
		StaticValues: convertStaticValues(source.Labels),
		Display: types.Display{
			DecimalPlaces: source.DecimalPlaces,
			OutputType:    source.OutputType,
			Min:           source.Min,
			Max:           source.Max,
		},
		SourceMetadata: sourceMetadata,
	}
	if _, addressed := source.Embedded.Raw["mmedaddress"]; !addressed {
		return axis, nil
	}
	layout, layoutDiagnostics := convertLayout(source.Embedded, defaults, []int{count}, path)
	axis.Layout = &layout
	return axis, layoutDiagnostics
}

// convertStaticValues converts an axis's static <LABEL index="" value=""> XML
// entries to mapdef 1.1's typed axis.staticValues (see
// docs/xdf-compatibility.md's axis label evidence section for sourcing).
func convertStaticValues(labels []types.XDFLabel) []types.StaticValue {
	if len(labels) == 0 {
		return nil
	}
	values := make([]types.StaticValue, len(labels))
	for i, label := range labels {
		values[i] = types.StaticValue{Index: label.Index, Value: label.Value}
	}
	return values
}

// boundsMap renders optional <min>/<max> bounds for sourceMetadata.xdf.bounds
// preservation, alongside their typed display.min/max home.
func boundsMap(min, max *float64) map[string]interface{} {
	bounds := make(map[string]interface{}, 2)
	if min != nil {
		bounds["min"] = *min
	}
	if max != nil {
		bounds["max"] = *max
	}
	return bounds
}

func convertFlag(flag types.XDFFlag, index int, defaults types.XDFDefaults, categories map[int]string) (types.MapParameter, []types.ImportDiagnostic) {
	path := fmt.Sprintf("flags[%d]", index)
	layout, diagnostics := convertLayout(flag.Embedded, defaults, []int{1}, path)
	bitPosition, maskDiagnostics := flagBitPosition(flag.Mask, layout.DataType, path)
	layout.BitPosition = bitPosition
	diagnostics = append(diagnostics, maskDiagnostics...)
	return types.MapParameter{
		ID:          identifier(flag.ID, fmt.Sprintf("flag-%d", index)),
		Kind:        "flag",
		Name:        valueOrFallback(flag.Title, fmt.Sprintf("Flag %d", index)),
		Description: flag.Description,
		Categories:  resolveCategories(flag.Categories, categories),
		Layout:      layout,
		Conversion:  convertFormula(flag.Formula),
		SourceMetadata: map[string]interface{}{
			"xdf.attributes":         flag.Raw,
			"xdf.embeddedAttributes": flag.Embedded.Raw,
		},
	}, diagnostics
}

// flagBitPosition resolves an XDFFLAG's optional <mask> to the single bit it
// addresses within its storage word. Two independent open-source XDF
// implementations agree <mask> holds a bitmask (see normalizeMask); a mask
// that is absent, zero, spans more than one bit, or exceeds the addressed
// word's width falls back to treating the whole word as the raw value,
// matching the pre-existing V0 behavior, with a diagnostic explaining why.
func flagBitPosition(mask *uint64, dataType, path string) (*int, []types.ImportDiagnostic) {
	if mask == nil {
		return nil, []types.ImportDiagnostic{{
			Severity: "warning",
			Code:     "xdf.flag-semantics",
			Path:     path,
			Message:  "XDF flag has no <mask>; the full addressed word is treated as the raw value rather than a single bit.",
		}}
	}
	width := dataTypeBits(dataType)
	if bits.OnesCount64(*mask) != 1 || (width > 0 && bits.Len64(*mask) > width) {
		return nil, []types.ImportDiagnostic{{
			Severity: "warning",
			Code:     "xdf.flag-semantics",
			Path:     path,
			Message:  fmt.Sprintf("XDF flag mask %#x is not a single in-range bit; the full addressed word is treated as the raw value.", *mask),
		}}
	}
	position := bits.TrailingZeros64(*mask)
	return &position, []types.ImportDiagnostic{{
		Severity: "info",
		Code:     "xdf.flag-bit",
		Path:     path,
		Message:  fmt.Sprintf("XDF flag mask %#x decodes bit %d of the addressed byte.", *mask, position),
	}}
}

func dataTypeBits(dataType string) int {
	switch dataType {
	case "uint8", "int8":
		return 8
	case "uint16", "int16":
		return 16
	case "uint32", "int32":
		return 32
	default:
		return 0
	}
}

func convertConstant(constant types.XDFConstant, index int, defaults types.XDFDefaults, categories map[int]string) (types.MapParameter, []types.ImportDiagnostic) {
	path := fmt.Sprintf("constants[%d]", index)
	layout, diagnostics := convertLayout(constant.Embedded, defaults, []int{1}, path)
	sourceMetadata := map[string]interface{}{
		"xdf.attributes":         constant.Raw,
		"xdf.embeddedAttributes": constant.Embedded.Raw,
	}
	if constant.Min != nil || constant.Max != nil {
		sourceMetadata["xdf.bounds"] = boundsMap(constant.Min, constant.Max)
	}
	return types.MapParameter{
		ID:          identifier(constant.ID, fmt.Sprintf("constant-%d", index)),
		Kind:        "scalar",
		Name:        valueOrFallback(constant.Title, fmt.Sprintf("Constant %d", index)),
		Description: constant.Description,
		Categories:  resolveCategories(constant.Categories, categories),
		Layout:      layout,
		Unit:        constant.Units,
		Conversion:  convertFormula(constant.Formula),
		Display: types.Display{
			DecimalPlaces: constant.DecimalPlaces,
			OutputType:    constant.OutputType,
			Min:           constant.Min,
			Max:           constant.Max,
		},
		SourceMetadata: sourceMetadata,
	}, diagnostics
}

func positiveOrOne(value int) int {
	if value > 0 {
		return value
	}
	return 1
}
