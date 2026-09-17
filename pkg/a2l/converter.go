package a2l

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/mapdef"
	"github.com/freebeamer/core/pkg/types"
)

// MapdefConverter mirrors xdf.MapdefConverter's contract: an independent
// per-format converter into the canonical mapdef model (ADR 0001).
type MapdefConverter interface {
	Convert(source *types.A2LDefinition, sourceName string) (*types.MapDefinition, error)
}

type Converter struct{}

var _ MapdefConverter = Converter{}

// Convert converts a parsed A2L document to a mapdef document. Only scalar
// (Type VALUE) CHARACTERISTICs are converted in this slice; see
// docs/a2l-v0-plan.md for the full grammar subset and docs/a2l-compatibility.md
// for what every other CHARACTERISTIC/COMPU_METHOD/RECORD_LAYOUT shape does.
func (Converter) Convert(source *types.A2LDefinition, sourceName string) (*types.MapDefinition, error) {
	if source == nil {
		return nil, fmt.Errorf("a2l: cannot convert a nil definition")
	}
	document := newMapDefinition(source, sourceName)
	compuMethods := indexCompuMethods(source.Project.Module.CompuMethods)
	compuVtabs := indexCompuVtabs(source.Project.Module.CompuVtabs)
	compuVtabRanges := indexCompuVtabRanges(source.Project.Module.CompuVtabRanges)
	compuTabs := indexCompuTabs(source.Project.Module.CompuTabs)
	recordLayouts := indexRecordLayouts(source.Project.Module.RecordLayouts)
	axisPtsByName := indexAxisPts(source.Project.Module.AxisPts)
	characteristicsByName := indexCharacteristics(source.Project.Module.Characteristics)
	categories, categoryMembership := convertGroups(source.Project.Module.Groups)
	document.Categories = categories
	usedParameterIDs := make(map[string]int)

	moduleByteOrder := source.Project.Module.ByteOrder
	for index, characteristic := range source.Project.Module.Characteristics {
		parameter, err := convertCharacteristic(characteristic, index, compuMethods, compuVtabs, compuVtabRanges, compuTabs, characteristicsByName, recordLayouts, axisPtsByName, moduleByteOrder)
		if err != nil {
			return nil, fmt.Errorf("a2l: characteristic %q: %w", characteristic.Name, err)
		}
		parameter.ID = uniqueIdentifier(parameter.ID, usedParameterIDs)
		parameter.Categories = categoryMembership[characteristic.Name]
		document.Parameters = append(document.Parameters, parameter)
	}

	if err := mapdef.Validate(document); err != nil {
		return nil, fmt.Errorf("a2l: converted mapdef is invalid: %w", err)
	}
	return document, nil
}

func newMapDefinition(source *types.A2LDefinition, sourceName string) *types.MapDefinition {
	digest := sha256.Sum256(source.RawText)
	moduleName := source.Project.Module.Name
	return &types.MapDefinition{
		Format:        mapdef.Format,
		FormatVersion: mapdef.CurrentVersion,
		ID:            identifier(moduleName, "imported-a2l"),
		Name:          valueOrFallback(moduleName, "Imported A2L definition"),
		Description:   source.Project.Module.Description,
		Provenance: types.MapProvenance{
			Source:       sourceName,
			SourceFormat: "a2l",
			ConvertedBy:  "freebeamer",
			SourceSHA256: hex.EncodeToString(digest[:]),
		},
		Extensions: map[string]interface{}{
			"a2l.rawDocument": string(source.RawText),
		},
	}
}

func indexCompuTabs(compuTabs []types.A2LCompuTab) map[string]types.A2LCompuTab {
	result := make(map[string]types.A2LCompuTab, len(compuTabs))
	for _, entry := range compuTabs {
		result[entry.Name] = entry
	}
	return result
}

func indexCompuMethods(methods []types.A2LCompuMethod) map[string]types.A2LCompuMethod {
	result := make(map[string]types.A2LCompuMethod, len(methods))
	for _, method := range methods {
		result[method.Name] = method
	}
	return result
}

func indexRecordLayouts(layouts []types.A2LRecordLayout) map[string]types.A2LRecordLayout {
	result := make(map[string]types.A2LRecordLayout, len(layouts))
	for _, layout := range layouts {
		result[layout.Name] = layout
	}
	return result
}

func indexAxisPts(axisPts []types.A2LAxisPts) map[string]types.A2LAxisPts {
	result := make(map[string]types.A2LAxisPts, len(axisPts))
	for _, entry := range axisPts {
		result[entry.Name] = entry
	}
	return result
}

func indexCompuVtabs(compuVtabs []types.A2LCompuVtab) map[string]types.A2LCompuVtab {
	result := make(map[string]types.A2LCompuVtab, len(compuVtabs))
	for _, entry := range compuVtabs {
		result[entry.Name] = entry
	}
	return result
}

func indexCompuVtabRanges(compuVtabRanges []types.A2LCompuVtabRange) map[string]types.A2LCompuVtabRange {
	result := make(map[string]types.A2LCompuVtabRange, len(compuVtabRanges))
	for _, entry := range compuVtabRanges {
		result[entry.Name] = entry
	}
	return result
}

func indexCharacteristics(characteristics []types.A2LCharacteristic) map[string]types.A2LCharacteristic {
	result := make(map[string]types.A2LCharacteristic, len(characteristics))
	for _, entry := range characteristics {
		result[entry.Name] = entry
	}
	return result
}

// convertGroups converts every GROUP to a mapdef category (analogous to
// XDF's <CATEGORY>), and returns a lookup from a CHARACTERISTIC's own Name
// to the category IDs of every GROUP whose REF_CHARACTERISTIC lists it
// directly. SUB_GROUP nesting is not expanded transitively into that
// membership — mapdef's categories have no parent/child relationship to
// represent it correctly — see docs/a2l-v0-plan.md's GROUP evidence.
func convertGroups(groups []types.A2LGroup) ([]types.MapCategory, map[string][]string) {
	if len(groups) == 0 {
		return nil, nil
	}
	categories := make([]types.MapCategory, 0, len(groups))
	membership := make(map[string][]string)
	usedCategoryIDs := make(map[string]int)
	for index, group := range groups {
		id := uniqueIdentifier(identifier(group.Name, fmt.Sprintf("group-%d", index)), usedCategoryIDs)
		categories = append(categories, types.MapCategory{
			ID:   id,
			Name: valueOrFallback(group.Description, group.Name),
		})
		for _, characteristicName := range group.RefCharacteristics {
			membership[characteristicName] = append(membership[characteristicName], id)
		}
	}
	return categories, membership
}

func convertCharacteristic(characteristic types.A2LCharacteristic, index int, compuMethods map[string]types.A2LCompuMethod, compuVtabs map[string]types.A2LCompuVtab, compuVtabRanges map[string]types.A2LCompuVtabRange, compuTabs map[string]types.A2LCompuTab, characteristicsByName map[string]types.A2LCharacteristic, recordLayouts map[string]types.A2LRecordLayout, axisPtsByName map[string]types.A2LAxisPts, moduleByteOrder string) (types.MapParameter, error) {
	kind, err := characteristicKind(characteristic)
	if err != nil {
		return types.MapParameter{}, err
	}
	recordLayout, ok := recordLayouts[characteristic.Deposit]
	if !ok {
		return types.MapParameter{}, fmt.Errorf("%w: %q", ErrRecordLayoutNotFound, characteristic.Deposit)
	}
	axisSpecs := make([]axisSpec, len(characteristic.AxisDescrs))
	for i, axisDescr := range characteristic.AxisDescrs {
		spec, err := resolveAxisSpec(axisDescr, recordLayouts, axisPtsByName, characteristicsByName, compuMethods, compuVtabs, compuVtabRanges, compuTabs, characteristic, moduleByteOrder)
		if err != nil {
			return types.MapParameter{}, fmt.Errorf("axis %d: %w", i, err)
		}
		axisSpecs[i] = spec
	}
	plan, err := planRecordLayout(characteristic, recordLayout, moduleByteOrder, axisSpecs)
	if err != nil {
		return types.MapParameter{}, err
	}
	resolved, err := convertConversionReference(characteristic.Conversion, characteristic.Format, compuMethods, compuVtabs, compuVtabRanges, compuTabs)
	if err != nil {
		return types.MapParameter{}, err
	}
	if len(resolved.StaticValues) > 0 {
		return types.MapParameter{}, fmt.Errorf("%w: %q uses TAB_VERB as its own Conversion; mapdef has no staticValues field on a parameter's own Z data yet (only on an axis), so this needs a follow-on slice", ErrUnsupportedCompuMethod, characteristic.Name)
	}

	minimum, maximum := characteristic.LowerLimit, characteristic.UpperLimit
	sourceMetadata := map[string]interface{}{
		"a2l.deposit":    characteristic.Deposit,
		"a2l.conversion": characteristic.Conversion,
		"a2l.maxDiff":    characteristic.MaxDiff,
	}
	if characteristic.DisplayIdentifier != "" {
		sourceMetadata["a2l.displayIdentifier"] = characteristic.DisplayIdentifier
	}
	if characteristic.ExtendedLimits != nil {
		sourceMetadata["a2l.extendedLimits"] = map[string]interface{}{
			"low": characteristic.ExtendedLimits.Low, "high": characteristic.ExtendedLimits.High,
		}
	}

	parameter := types.MapParameter{
		ID:             identifier(characteristic.Name, fmt.Sprintf("characteristic-%d", index)),
		Kind:           kind,
		Name:           valueOrFallback(characteristic.Name, fmt.Sprintf("Characteristic %d", index)),
		Description:    characteristic.Description,
		Layout:         plan.Z,
		Unit:           resolved.Unit,
		Conversion:     resolved.Conversion,
		Display:        types.Display{DecimalPlaces: resolved.DecimalPlaces, Min: &minimum, Max: &maximum},
		SourceMetadata: sourceMetadata,
	}
	if len(plan.Axes) > 0 {
		axes := make(map[string]types.MapAxis, len(plan.Axes))
		axisNames := [2]string{"x", "y"}
		for i, axisLayout := range plan.Axes {
			axisDescr := characteristic.AxisDescrs[i]
			spec := axisSpecs[i]
			var axisResolved resolvedConversion
			if spec.ResolvedConversion != nil {
				// CURVE_AXIS: the referencing AXIS_DESCR's own Conversion is
				// always just a NO_COMPU_METHOD placeholder (see
				// docs/a2l-v0-plan.md's CURVE_AXIS evidence); resolveAxisSpec
				// already resolved the referenced CHARACTERISTIC's own axis
				// Conversion instead.
				axisResolved = *spec.ResolvedConversion
			} else {
				axisResolved, err = convertConversionReference(axisDescr.Conversion, "", compuMethods, compuVtabs, compuVtabRanges, compuTabs)
				if err != nil {
					return types.MapParameter{}, fmt.Errorf("axis %s: %w", axisNames[i], err)
				}
			}
			axisMin, axisMax := spec.LowerLimit, spec.UpperLimit
			axisSourceMetadata := map[string]interface{}{
				"a2l.attribute":      axisDescr.Attribute,
				"a2l.inputQuantity":  axisDescr.InputQuantity,
				"a2l.conversionName": axisDescr.Conversion,
			}
			if spec.AxisPtsName != "" {
				axisSourceMetadata["a2l.axisPts"] = spec.AxisPtsName
			}
			if axisDescr.CurveAxisRef != "" {
				axisSourceMetadata["a2l.curveAxisRef"] = axisDescr.CurveAxisRef
			}
			axis := types.MapAxis{
				Count:          spec.MaxAxisPoints,
				Unit:           axisResolved.Unit,
				Conversion:     axisResolved.Conversion,
				StaticValues:   axisResolved.StaticValues,
				Display:        types.Display{Min: &axisMin, Max: &axisMax},
				SourceMetadata: axisSourceMetadata,
			}
			if len(spec.FixedPoints) > 0 {
				axis.FixedPoints = spec.FixedPoints
			} else {
				axisLayout := axisLayout
				axis.Layout = &axisLayout
			}
			axes[axisNames[i]] = axis
		}
		parameter.Axes = axes
	}
	return parameter, nil
}

// axisSpec is one CHARACTERISTIC axis, resolved to what planRecordLayout
// needs regardless of whether it's embedded (STD_AXIS), a reference to a
// standalone AXIS_PTS (COM_AXIS/RES_AXIS), computed with no on-disk storage
// at all (FIX_AXIS), or borrowed from another CHARACTERISTIC's own axis
// (CURVE_AXIS).
type axisSpec struct {
	MaxAxisPoints int
	LowerLimit    float64
	UpperLimit    float64
	// Embedded is true for STD_AXIS: the axis's breakpoints live in the
	// enclosing CHARACTERISTIC's own RECORD_LAYOUT (an AXIS_PTS_X/Y entry
	// planRecordLayout must find and size). It is false for every other
	// supported Attribute, whose breakpoints (if any) are already fully
	// resolved below.
	Embedded    bool
	Layout      *types.DataLayout
	AxisPtsName string // non-empty for COM_AXIS/RES_AXIS, for sourceMetadata provenance
	// FixedPoints gives this axis's raw breakpoints directly (FIX_AXIS: no
	// on-disk storage at all, computed from FixAxisParDist/FixAxisParList).
	// Layout is nil whenever this is set.
	FixedPoints []int64
	// ResolvedConversion, when non-nil, overrides the normal
	// axisDescr.Conversion-based resolution the caller would otherwise do
	// (CURVE_AXIS: the referencing AXIS_DESCR's own Conversion is always
	// the NO_COMPU_METHOD placeholder per real evidence; the referenced
	// CHARACTERISTIC's own axis Conversion is authoritative instead — see
	// docs/a2l-v0-plan.md's CURVE_AXIS evidence).
	ResolvedConversion *resolvedConversion
}

// resolveAxisSpec resolves one AXIS_DESCR to an axisSpec. STD_AXIS defers
// its layout to planRecordLayout (it needs sizes for every axis first, to
// size FNC_VALUES correctly, so it can't be resolved independently).
// COM_AXIS/RES_AXIS are fully resolved here via their referenced AXIS_PTS;
// FIX_AXIS needs no on-disk resolution at all; CURVE_AXIS recursively
// resolves the referenced CHARACTERISTIC's own single axis (guaranteed to
// be STD_AXIS/COM_AXIS/RES_AXIS below, so this recursion is one level deep,
// never chained).
func resolveAxisSpec(axisDescr types.A2LAxisDescr, recordLayouts map[string]types.A2LRecordLayout, axisPtsByName map[string]types.A2LAxisPts, characteristicsByName map[string]types.A2LCharacteristic, compuMethods map[string]types.A2LCompuMethod, compuVtabs map[string]types.A2LCompuVtab, compuVtabRanges map[string]types.A2LCompuVtabRange, compuTabs map[string]types.A2LCompuTab, characteristic types.A2LCharacteristic, moduleByteOrder string) (axisSpec, error) {
	switch axisDescr.Attribute {
	case "STD_AXIS":
		return axisSpec{MaxAxisPoints: axisDescr.MaxAxisPoints, LowerLimit: axisDescr.LowerLimit, UpperLimit: axisDescr.UpperLimit, Embedded: true}, nil
	case "COM_AXIS":
		axisPts, ok := axisPtsByName[axisDescr.AxisPtsRef]
		if !ok {
			return axisSpec{}, fmt.Errorf("%w: %q", ErrAxisPtsNotFound, axisDescr.AxisPtsRef)
		}
		axisRecordLayout, ok := recordLayouts[axisPts.Deposit]
		if !ok {
			return axisSpec{}, fmt.Errorf("AXIS_PTS %q: %w: %q", axisPts.Name, ErrRecordLayoutNotFound, axisPts.Deposit)
		}
		layout, err := planAxisPtsLayout(axisPts, axisRecordLayout, characteristic, moduleByteOrder)
		if err != nil {
			return axisSpec{}, fmt.Errorf("AXIS_PTS %q: %w", axisPts.Name, err)
		}
		return axisSpec{
			MaxAxisPoints: axisPts.MaxAxisPoints, LowerLimit: axisPts.LowerLimit, UpperLimit: axisPts.UpperLimit,
			Layout: &layout, AxisPtsName: axisPts.Name,
		}, nil
	case "RES_AXIS":
		axisPts, ok := axisPtsByName[axisDescr.AxisPtsRef]
		if !ok {
			return axisSpec{}, fmt.Errorf("%w: %q", ErrAxisPtsNotFound, axisDescr.AxisPtsRef)
		}
		axisRecordLayout, ok := recordLayouts[axisPts.Deposit]
		if !ok {
			return axisSpec{}, fmt.Errorf("AXIS_PTS %q: %w: %q", axisPts.Name, ErrRecordLayoutNotFound, axisPts.Deposit)
		}
		layout, err := planAxisPtsRescaleLayout(axisPts, axisRecordLayout, characteristic, moduleByteOrder)
		if err != nil {
			return axisSpec{}, fmt.Errorf("AXIS_PTS %q: %w", axisPts.Name, err)
		}
		return axisSpec{
			MaxAxisPoints: axisPts.MaxAxisPoints, LowerLimit: axisPts.LowerLimit, UpperLimit: axisPts.UpperLimit,
			Layout: &layout, AxisPtsName: axisPts.Name,
		}, nil
	case "FIX_AXIS":
		points, err := fixAxisRawPoints(axisDescr)
		if err != nil {
			return axisSpec{}, err
		}
		if len(points) != axisDescr.MaxAxisPoints {
			return axisSpec{}, fmt.Errorf("%w: FIX_AXIS has %d computed point(s) but MaxAxisPoints is %d", ErrUnsupportedAxisDescr, len(points), axisDescr.MaxAxisPoints)
		}
		return axisSpec{MaxAxisPoints: axisDescr.MaxAxisPoints, LowerLimit: axisDescr.LowerLimit, UpperLimit: axisDescr.UpperLimit, FixedPoints: points}, nil
	case "CURVE_AXIS":
		referenced, ok := characteristicsByName[axisDescr.CurveAxisRef]
		if !ok {
			return axisSpec{}, fmt.Errorf("%w: %q", ErrCurveAxisNotFound, axisDescr.CurveAxisRef)
		}
		if referenced.Type != "CURVE" || len(referenced.AxisDescrs) != 1 {
			return axisSpec{}, fmt.Errorf("%w: CURVE_AXIS_REF %q must be a CURVE with exactly one AXIS_DESCR", ErrUnsupportedAxisDescr, axisDescr.CurveAxisRef)
		}
		referencedAxisDescr := referenced.AxisDescrs[0]
		if referencedAxisDescr.Attribute != "STD_AXIS" && referencedAxisDescr.Attribute != "COM_AXIS" {
			return axisSpec{}, fmt.Errorf("%w: CURVE_AXIS_REF %q's own axis has Attribute %q (only STD_AXIS/COM_AXIS referenced axes are supported per real evidence; chained CURVE_AXIS/RES_AXIS is not)", ErrUnsupportedAxisDescr, axisDescr.CurveAxisRef, referencedAxisDescr.Attribute)
		}
		referencedSpec, err := resolveAxisSpec(referencedAxisDescr, recordLayouts, axisPtsByName, characteristicsByName, compuMethods, compuVtabs, compuVtabRanges, compuTabs, referenced, moduleByteOrder)
		if err != nil {
			return axisSpec{}, fmt.Errorf("CURVE_AXIS_REF %q: %w", axisDescr.CurveAxisRef, err)
		}
		if referencedSpec.Embedded {
			referencedRecordLayout, ok := recordLayouts[referenced.Deposit]
			if !ok {
				return axisSpec{}, fmt.Errorf("CURVE_AXIS_REF %q: %w: %q", axisDescr.CurveAxisRef, ErrRecordLayoutNotFound, referenced.Deposit)
			}
			plan, err := planRecordLayout(referenced, referencedRecordLayout, moduleByteOrder, []axisSpec{referencedSpec})
			if err != nil {
				return axisSpec{}, fmt.Errorf("CURVE_AXIS_REF %q: %w", axisDescr.CurveAxisRef, err)
			}
			layout := plan.Axes[0]
			referencedSpec.Layout = &layout
			referencedSpec.Embedded = false
		}
		axisResolved, err := convertConversionReference(referencedAxisDescr.Conversion, "", compuMethods, compuVtabs, compuVtabRanges, compuTabs)
		if err != nil {
			return axisSpec{}, fmt.Errorf("CURVE_AXIS_REF %q: %w", axisDescr.CurveAxisRef, err)
		}
		return axisSpec{
			MaxAxisPoints: referencedSpec.MaxAxisPoints, LowerLimit: referencedSpec.LowerLimit, UpperLimit: referencedSpec.UpperLimit,
			Layout: referencedSpec.Layout, AxisPtsName: referencedSpec.AxisPtsName, ResolvedConversion: &axisResolved,
		}, nil
	default:
		return axisSpec{}, fmt.Errorf("%w: attribute %q (only STD_AXIS/COM_AXIS/RES_AXIS/FIX_AXIS/CURVE_AXIS are supported in this slice)", ErrUnsupportedAxisDescr, axisDescr.Attribute)
	}
}

// fixAxisRawPoints computes a FIX_AXIS's raw breakpoints from whichever of
// FixAxisParDist/FixAxisParList the parser recorded (see
// docs/a2l-v0-plan.md's FIX_AXIS evidence). Neither present, or both
// present, is rejected rather than guessed.
func fixAxisRawPoints(axisDescr types.A2LAxisDescr) ([]int64, error) {
	switch {
	case axisDescr.FixAxisParDist != nil && axisDescr.FixAxisParList != nil:
		return nil, fmt.Errorf("%w: FIX_AXIS has both FIX_AXIS_PAR_DIST and FIX_AXIS_PAR_LIST", ErrUnsupportedAxisDescr)
	case axisDescr.FixAxisParDist != nil:
		dist := axisDescr.FixAxisParDist
		points := make([]int64, dist.NumberPts)
		for i := range points {
			points[i] = int64(dist.Offset) + int64(i)*int64(dist.Distance)
		}
		return points, nil
	case axisDescr.FixAxisParList != nil:
		points := make([]int64, len(axisDescr.FixAxisParList))
		for i, value := range axisDescr.FixAxisParList {
			points[i] = int64(value)
		}
		return points, nil
	default:
		return nil, fmt.Errorf("%w: FIX_AXIS has neither FIX_AXIS_PAR_DIST nor FIX_AXIS_PAR_LIST", ErrUnsupportedAxisDescr)
	}
}

// planAxisPtsLayout resolves a standalone AXIS_PTS's own RECORD_LAYOUT to a
// concrete DataLayout, addressed relative to the AXIS_PTS's own Address (not
// the referencing CHARACTERISTIC's). Only a single AXIS_PTS_X entry
// (optionally preceded by NO_AXIS_PTS_X) is supported: an AXIS_PTS's
// RECORD_LAYOUT is structurally axis-agnostic, always naming its one entry
// AXIS_PTS_X regardless of whether the CHARACTERISTIC that references it
// treats it as an X or Y axis. AXIS_PTS has no BYTE_ORDER keyword of its
// own in available evidence, so a multi-byte AXIS_PTS falls back to the
// referencing CHARACTERISTIC's effective byte order (its own override, else
// MOD_COMMON's module-wide default) — the same default every other
// multi-byte value in the file would use absent a more specific one.
func planAxisPtsLayout(axisPts types.A2LAxisPts, recordLayout types.A2LRecordLayout, referencingCharacteristic types.A2LCharacteristic, moduleByteOrder string) (types.DataLayout, error) {
	entriesByRole := make(map[string]types.A2LRecordLayoutEntry, len(recordLayout.Entries))
	for _, entry := range recordLayout.Entries {
		if entry.Role != "AXIS_PTS_X" && entry.Role != "NO_AXIS_PTS_X" {
			return types.DataLayout{}, fmt.Errorf("%w: %q has a %s entry (an AXIS_PTS's RECORD_LAYOUT only supports AXIS_PTS_X and NO_AXIS_PTS_X)", ErrUnsupportedRecordLayout, recordLayout.Name, entry.Role)
		}
		if _, duplicate := entriesByRole[entry.Role]; duplicate {
			return types.DataLayout{}, fmt.Errorf("%w: %q has more than one %s entry", ErrUnsupportedRecordLayout, recordLayout.Name, entry.Role)
		}
		entriesByRole[entry.Role] = entry
	}
	axisEntry, ok := entriesByRole["AXIS_PTS_X"]
	if !ok {
		return types.DataLayout{}, fmt.Errorf("%w: %q has no AXIS_PTS_X entry", ErrUnsupportedRecordLayout, recordLayout.Name)
	}
	if axisEntry.IndexMode != "INDEX_INCR" {
		return types.DataLayout{}, fmt.Errorf("%w: %q AXIS_PTS_X has IndexMode %q (only INDEX_INCR is supported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, axisEntry.IndexMode)
	}
	if axisEntry.AddrType != "DIRECT" {
		return types.DataLayout{}, fmt.Errorf("%w: %q AXIS_PTS_X has AddrType %q (only DIRECT is supported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, axisEntry.AddrType)
	}
	dataType, ok := convertDataType(axisEntry.DataType)
	if !ok {
		return types.DataLayout{}, fmt.Errorf("%w: %q AXIS_PTS_X has data type %q (unsupported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, axisEntry.DataType)
	}
	byteOrder, err := effectiveByteOrder(referencingCharacteristic, moduleByteOrder, dataTypeBits(dataType))
	if err != nil {
		return types.DataLayout{}, err
	}

	var offset uint64
	if noAxisPts, ok := entriesByRole["NO_AXIS_PTS_X"]; ok && noAxisPts.Position < axisEntry.Position {
		countType, ok := convertDataType(noAxisPts.DataType)
		if !ok {
			return types.DataLayout{}, fmt.Errorf("%w: %q NO_AXIS_PTS_X has data type %q (unsupported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, noAxisPts.DataType)
		}
		offset = uint64(dataTypeBits(countType) / 8)
	}

	address := axisPts.Address + offset
	return types.DataLayout{
		Address:    &address,
		DataType:   dataType,
		ByteOrder:  byteOrder,
		Dimensions: []int{axisPts.MaxAxisPoints},
		Order:      "not-applicable",
	}, nil
}

// planAxisPtsRescaleLayout resolves a RES_AXIS's standalone AXIS_PTS, whose
// RECORD_LAYOUT stores rescale (position, value) pairs via AXIS_RESCALE_X
// (optionally preceded by NO_RESCALE_X and/or a RESERVED padding entry —
// both structural per real evidence, see docs/a2l-v0-plan.md's RES_AXIS
// evidence). Only the evidenced case where the layout's own
// MaxNumberOfRescalePairs equals the AXIS_PTS's MaxAxisPoints is supported:
// this slice reads each pair's second (value) element directly as that
// axis point's raw value via a mapdef stride, skipping the interleaved
// first (position) element — it does not implement any interpolation
// between fewer pairs and more axis points, since real evidence doesn't
// establish that this is even how RES_AXIS behaves in the general case.
func planAxisPtsRescaleLayout(axisPts types.A2LAxisPts, recordLayout types.A2LRecordLayout, referencingCharacteristic types.A2LCharacteristic, moduleByteOrder string) (types.DataLayout, error) {
	entriesByRole := make(map[string]types.A2LRecordLayoutEntry, len(recordLayout.Entries))
	for _, entry := range recordLayout.Entries {
		switch entry.Role {
		case "AXIS_RESCALE_X", "NO_RESCALE_X", "RESERVED":
		default:
			return types.DataLayout{}, fmt.Errorf("%w: %q has a %s entry (a RES_AXIS's RECORD_LAYOUT only supports AXIS_RESCALE_X, NO_RESCALE_X, RESERVED)", ErrUnsupportedRecordLayout, recordLayout.Name, entry.Role)
		}
		if _, duplicate := entriesByRole[entry.Role]; duplicate {
			return types.DataLayout{}, fmt.Errorf("%w: %q has more than one %s entry", ErrUnsupportedRecordLayout, recordLayout.Name, entry.Role)
		}
		entriesByRole[entry.Role] = entry
	}
	rescaleEntry, ok := entriesByRole["AXIS_RESCALE_X"]
	if !ok {
		return types.DataLayout{}, fmt.Errorf("%w: %q has no AXIS_RESCALE_X entry", ErrUnsupportedRecordLayout, recordLayout.Name)
	}
	if rescaleEntry.IndexMode != "INDEX_INCR" {
		return types.DataLayout{}, fmt.Errorf("%w: %q AXIS_RESCALE_X has IndexMode %q (only INDEX_INCR is supported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, rescaleEntry.IndexMode)
	}
	if rescaleEntry.AddrType != "DIRECT" {
		return types.DataLayout{}, fmt.Errorf("%w: %q AXIS_RESCALE_X has AddrType %q (only DIRECT is supported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, rescaleEntry.AddrType)
	}
	if rescaleEntry.RescalePairs != axisPts.MaxAxisPoints {
		return types.DataLayout{}, fmt.Errorf("%w: %q has %d rescale pair(s) but AXIS_PTS %q has MaxAxisPoints %d", ErrRescalePairCountMismatch, recordLayout.Name, rescaleEntry.RescalePairs, axisPts.Name, axisPts.MaxAxisPoints)
	}
	dataType, ok := convertDataType(rescaleEntry.DataType)
	if !ok {
		return types.DataLayout{}, fmt.Errorf("%w: %q AXIS_RESCALE_X has data type %q (unsupported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, rescaleEntry.DataType)
	}
	byteOrder, err := effectiveByteOrder(referencingCharacteristic, moduleByteOrder, dataTypeBits(dataType))
	if err != nil {
		return types.DataLayout{}, err
	}

	sortedEntries := append([]types.A2LRecordLayoutEntry(nil), recordLayout.Entries...)
	sort.Slice(sortedEntries, func(i, j int) bool { return sortedEntries[i].Position < sortedEntries[j].Position })
	var cumulative uint64
	for _, entry := range sortedEntries {
		if entry.Role == "AXIS_RESCALE_X" {
			break
		}
		size, ok := rescalePaddingEntrySize(entry)
		if !ok {
			return types.DataLayout{}, fmt.Errorf("%w: %q %s has data type %q (unsupported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, entry.Role, entry.DataType)
		}
		cumulative += size
	}

	elementSize := uint64(dataTypeBits(dataType) / 8)
	// Skip past this pair's own leading (position) element to land on its
	// (value) element; StrideBits then skips every other pair's leading
	// element for every subsequent point.
	address := axisPts.Address + cumulative + elementSize
	return types.DataLayout{
		Address:    &address,
		DataType:   dataType,
		ByteOrder:  byteOrder,
		Dimensions: []int{axisPts.MaxAxisPoints},
		Order:      "not-applicable",
		StrideBits: []int{int(elementSize) * 8 * 2},
	}, nil
}

// rescalePaddingEntrySize returns the byte size of a RECORD_LAYOUT entry
// preceding a RES_AXIS's AXIS_RESCALE_X: NO_RESCALE_X's DataType is a
// regular mapdef-supported type, while RESERVED (real evidence: "to adapt
// the start of the rescale pairs to an even address") is only evidenced as
// the literal DataType BYTE, one byte, and rejected otherwise.
func rescalePaddingEntrySize(entry types.A2LRecordLayoutEntry) (uint64, bool) {
	if entry.Role == "RESERVED" {
		if entry.DataType == "BYTE" {
			return 1, true
		}
		return 0, false
	}
	dataType, ok := convertDataType(entry.DataType)
	if !ok {
		return 0, false
	}
	return uint64(dataTypeBits(dataType) / 8), true
}

// characteristicKind maps a CHARACTERISTIC's Type and AXIS_DESCR count to a
// mapdef Kind, rejecting any combination this slice does not support.
func characteristicKind(characteristic types.A2LCharacteristic) (string, error) {
	switch characteristic.Type {
	case "VALUE":
		if len(characteristic.AxisDescrs) != 0 {
			return "", fmt.Errorf("%w: %q is Type VALUE but has %d AXIS_DESCR block(s)", ErrUnsupportedCharacteristicType, characteristic.Name, len(characteristic.AxisDescrs))
		}
		return "scalar", nil
	case "CURVE":
		if len(characteristic.AxisDescrs) != 1 {
			return "", fmt.Errorf("%w: %q is Type CURVE but has %d AXIS_DESCR block(s), want 1", ErrUnsupportedCharacteristicType, characteristic.Name, len(characteristic.AxisDescrs))
		}
		return "curve", nil
	case "MAP":
		if len(characteristic.AxisDescrs) != 2 {
			return "", fmt.Errorf("%w: %q is Type MAP but has %d AXIS_DESCR block(s), want 2", ErrUnsupportedCharacteristicType, characteristic.Name, len(characteristic.AxisDescrs))
		}
		return "map", nil
	case "VAL_BLK":
		if len(characteristic.AxisDescrs) != 0 {
			return "", fmt.Errorf("%w: %q is Type VAL_BLK but has %d AXIS_DESCR block(s) (VAL_BLK has none; its shape comes from MATRIX_DIM)", ErrUnsupportedCharacteristicType, characteristic.Name, len(characteristic.AxisDescrs))
		}
		if characteristic.MatrixDim == nil {
			return "", fmt.Errorf("%w: %q is Type VAL_BLK but has no MATRIX_DIM", ErrUnsupportedCharacteristicType, characteristic.Name)
		}
		if characteristic.MatrixDim.Z != 1 {
			return "", fmt.Errorf("%w: %q has MATRIX_DIM z=%d (only a 1- or 2-dimensional VAL_BLK, z=1, is supported in this slice)", ErrUnsupportedCharacteristicType, characteristic.Name, characteristic.MatrixDim.Z)
		}
		return "map", nil
	default:
		return "", fmt.Errorf("%w: %q (only VALUE/CURVE/MAP/VAL_BLK are supported in this slice)", ErrUnsupportedCharacteristicType, characteristic.Type)
	}
}

// recordLayoutPlan is the computed byte layout for one CHARACTERISTIC: the
// FNC_VALUES (Z) layout, and its axis layouts (0 for a scalar, 1 for a
// curve, 2 — X then Y — for a map).
type recordLayoutPlan struct {
	Z    types.DataLayout
	Axes []types.DataLayout
}

// planRecordLayout resolves a CHARACTERISTIC's RECORD_LAYOUT into concrete
// addresses. RECORD_LAYOUT positions are an ordinal sequence, not byte
// offsets (see types.A2LRecordLayoutEntry): the actual byte offset of each
// entry is the cumulative size of every lower-positioned entry, so this
// walks every entry (including NO_AXIS_PTS_X/Y, whose stored runtime point
// count this slice does not decode — every axis is always treated as
// exactly its resolved MaxAxisPoints long) in position order, summing sizes
// to find where FNC_VALUES and any embedded (STD_AXIS) AXIS_PTS_X/Y
// actually live. A non-embedded (COM_AXIS) axis's Layout is already fully
// resolved in axisSpecs and contributes nothing to this RECORD_LAYOUT's own
// offsets — its storage lives in its own AXIS_PTS instead.
//
// Only AXIS_PTS_X/Y IndexMode INDEX_INCR is supported (INDEX_DECR would need
// the axis array read in reverse, which this slice does not implement);
// only AddrType DIRECT is supported (no pointer indirection). See
// docs/a2l-v0-plan.md.
func planRecordLayout(characteristic types.A2LCharacteristic, recordLayout types.A2LRecordLayout, moduleByteOrder string, axisSpecs []axisSpec) (recordLayoutPlan, error) {
	axisRoles := [2]string{"AXIS_PTS_X", "AXIS_PTS_Y"}
	requiredRoles := []string{"FNC_VALUES"}
	axisPointCounts := map[string]int{} // every axis, embedded or not — FNC_VALUES needs all of them
	embeddedRoles := map[string]bool{}
	for i, spec := range axisSpecs {
		axisPointCounts[axisRoles[i]] = spec.MaxAxisPoints
		if spec.Embedded {
			requiredRoles = append(requiredRoles, axisRoles[i])
			embeddedRoles[axisRoles[i]] = true
		}
	}

	entriesByRole := make(map[string]types.A2LRecordLayoutEntry, len(recordLayout.Entries))
	for _, entry := range recordLayout.Entries {
		if _, duplicate := entriesByRole[entry.Role]; duplicate {
			return recordLayoutPlan{}, fmt.Errorf("%w: %q has more than one %s entry", ErrUnsupportedRecordLayout, recordLayout.Name, entry.Role)
		}
		entriesByRole[entry.Role] = entry
	}
	for _, role := range requiredRoles {
		if _, ok := entriesByRole[role]; !ok {
			return recordLayoutPlan{}, fmt.Errorf("%w: %q has no %s entry (required for this characteristic's %d axis/axes)", ErrUnsupportedRecordLayout, recordLayout.Name, role, len(characteristic.AxisDescrs))
		}
	}
	for _, role := range axisRoles {
		if _, present := entriesByRole[role]; present && !embeddedRoles[role] {
			return recordLayoutPlan{}, fmt.Errorf("%w: %q has a %s entry, but that axis is not STD_AXIS (embedded here) — either remove the entry or make the axis STD_AXIS", ErrUnsupportedRecordLayout, recordLayout.Name, role)
		}
	}

	zCount := 1
	for _, count := range axisPointCounts {
		zCount *= count
	}
	elementCounts := map[string]int{"FNC_VALUES": zCount}
	for role, count := range axisPointCounts {
		if embeddedRoles[role] {
			elementCounts[role] = count
		}
	}

	sortedEntries := append([]types.A2LRecordLayoutEntry(nil), recordLayout.Entries...)
	sort.Slice(sortedEntries, func(i, j int) bool { return sortedEntries[i].Position < sortedEntries[j].Position })

	offsets := make(map[string]uint64, len(sortedEntries))
	var cumulative uint64
	for _, entry := range sortedEntries {
		offsets[entry.Role] = cumulative
		dataType, ok := convertDataType(entry.DataType)
		if !ok {
			return recordLayoutPlan{}, fmt.Errorf("%w: %q entry %s has data type %q (unsupported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, entry.Role, entry.DataType)
		}
		count := elementCounts[entry.Role]
		if count == 0 {
			count = 1 // NO_AXIS_PTS_X/Y: a single count byte, not an array.
		}
		cumulative += uint64(dataTypeBits(dataType)/8) * uint64(count)
	}

	buildLayout := func(role string, dimensions []int, order string) (types.DataLayout, error) {
		entry := entriesByRole[role]
		dataType, _ := convertDataType(entry.DataType) // already validated above
		if entry.AddrType != "DIRECT" {
			return types.DataLayout{}, fmt.Errorf("%w: %q entry %s has AddrType %q (only DIRECT is supported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, role, entry.AddrType)
		}
		byteOrder, err := effectiveByteOrder(characteristic, moduleByteOrder, dataTypeBits(dataType))
		if err != nil {
			return types.DataLayout{}, err
		}
		address := characteristic.Address + offsets[role]
		return types.DataLayout{
			Address:    &address,
			DataType:   dataType,
			ByteOrder:  byteOrder,
			Dimensions: dimensions,
			Order:      order,
		}, nil
	}

	// resolveAxis returns a COM_AXIS/RES_AXIS/CURVE_AXIS's already-fully-
	// resolved Layout as is, a zero-value placeholder for a FIX_AXIS (which
	// has no Layout at all — the caller uses spec.FixedPoints instead and
	// never looks at this return value), or builds a STD_AXIS's Layout from
	// this RECORD_LAYOUT's own entry.
	resolveAxis := func(role string, spec axisSpec) (types.DataLayout, error) {
		if len(spec.FixedPoints) > 0 {
			return types.DataLayout{}, nil
		}
		if !spec.Embedded {
			return *spec.Layout, nil
		}
		if entriesByRole[role].IndexMode != "INDEX_INCR" {
			return types.DataLayout{}, fmt.Errorf("%w: %q %s has IndexMode %q (only INDEX_INCR is supported in this slice)", ErrUnsupportedRecordLayout, recordLayout.Name, role, entriesByRole[role].IndexMode)
		}
		return buildLayout(role, []int{spec.MaxAxisPoints}, "not-applicable")
	}

	plan := recordLayoutPlan{}
	switch len(axisSpecs) {
	case 0:
		// A scalar (VALUE) has no shape of its own: a single element. A
		// VAL_BLK has no AXIS_DESCR either, but its shape comes from its own
		// MATRIX_DIM instead — see docs/a2l-v0-plan.md's VAL_BLK evidence.
		dimensions, order := []int{1}, "not-applicable"
		if characteristic.MatrixDim != nil {
			dimensions = []int{characteristic.MatrixDim.Y, characteristic.MatrixDim.X}
			var err error
			order, err = convertFNCValuesOrder(recordLayout, entriesByRole["FNC_VALUES"])
			if err != nil {
				return recordLayoutPlan{}, err
			}
		}
		zLayout, err := buildLayout("FNC_VALUES", dimensions, order)
		if err != nil {
			return recordLayoutPlan{}, err
		}
		plan.Z = zLayout
	case 1:
		order, err := convertFNCValuesOrder(recordLayout, entriesByRole["FNC_VALUES"])
		if err != nil {
			return recordLayoutPlan{}, err
		}
		zLayout, err := buildLayout("FNC_VALUES", []int{axisPointCounts["AXIS_PTS_X"]}, order)
		if err != nil {
			return recordLayoutPlan{}, err
		}
		xLayout, err := resolveAxis("AXIS_PTS_X", axisSpecs[0])
		if err != nil {
			return recordLayoutPlan{}, err
		}
		plan.Z = zLayout
		plan.Axes = []types.DataLayout{xLayout}
	case 2:
		order, err := convertFNCValuesOrder(recordLayout, entriesByRole["FNC_VALUES"])
		if err != nil {
			return recordLayoutPlan{}, err
		}
		// dimensions: [rows, columns] = [Y count, X count] — Y varies down
		// rows, X varies across columns, the conventional Cartesian mapping.
		zLayout, err := buildLayout("FNC_VALUES", []int{axisPointCounts["AXIS_PTS_Y"], axisPointCounts["AXIS_PTS_X"]}, order)
		if err != nil {
			return recordLayoutPlan{}, err
		}
		xLayout, err := resolveAxis("AXIS_PTS_X", axisSpecs[0])
		if err != nil {
			return recordLayoutPlan{}, err
		}
		yLayout, err := resolveAxis("AXIS_PTS_Y", axisSpecs[1])
		if err != nil {
			return recordLayoutPlan{}, err
		}
		plan.Z = zLayout
		plan.Axes = []types.DataLayout{xLayout, yLayout}
	}
	return plan, nil
}

func convertFNCValuesOrder(recordLayout types.A2LRecordLayout, fncValues types.A2LRecordLayoutEntry) (string, error) {
	switch fncValues.IndexMode {
	case "ROW_DIR":
		return "row-major", nil
	case "COLUMN_DIR":
		return "column-major", nil
	default:
		return "", fmt.Errorf("%w: %q FNC_VALUES has IndexMode %q (only ROW_DIR/COLUMN_DIR are supported)", ErrUnsupportedRecordLayout, recordLayout.Name, fncValues.IndexMode)
	}
}

// effectiveByteOrder resolves the byte order to use for one entry: a
// single-byte type never needs one ("not-applicable"); a multi-byte type
// needs the characteristic's own BYTE_ORDER, else MOD_COMMON's module-wide
// default. Neither present, or a mixed word/byte order this project's
// layout model can't represent (MSB_FIRST_MSW_LAST/MSB_LAST_MSW_FIRST), is
// rejected rather than guessed.
func effectiveByteOrder(characteristic types.A2LCharacteristic, moduleByteOrder string, bits int) (string, error) {
	if bits <= 8 {
		return "not-applicable", nil
	}
	declared := characteristic.ByteOrder
	if declared == "" {
		declared = moduleByteOrder
	}
	if declared == "" {
		return "", fmt.Errorf("%w: %q has no BYTE_ORDER (on the CHARACTERISTIC or MOD_COMMON) for a multi-byte type", ErrUnsupportedRecordLayout, characteristic.Name)
	}
	mapped, ok := convertByteOrder(declared)
	if !ok {
		return "", fmt.Errorf("%w: BYTE_ORDER %q is not supported (only LITTLE_ENDIAN/MSB_LAST or BIG_ENDIAN/MSB_FIRST)", ErrUnsupportedRecordLayout, declared)
	}
	return mapped, nil
}

func convertDataType(a2lType string) (string, bool) {
	switch a2lType {
	case "UBYTE":
		return "uint8", true
	case "SBYTE":
		return "int8", true
	case "UWORD":
		return "uint16", true
	case "SWORD":
		return "int16", true
	case "ULONG":
		return "uint32", true
	case "SLONG":
		return "int32", true
	default:
		return "", false
	}
}

func dataTypeBits(mapdefType string) int {
	switch mapdefType {
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

// convertByteOrder maps a BYTE_ORDER token to mapdef's byteOrder enum.
// MSB_FIRST_MSW_LAST/MSB_LAST_MSW_FIRST (mixed word/byte order) have no
// representation in mapdef's layout model and are deliberately not mapped
// here rather than approximated.
func convertByteOrder(value string) (string, bool) {
	switch value {
	case "LITTLE_ENDIAN", "MSB_LAST":
		return "little", true
	case "BIG_ENDIAN", "MSB_FIRST":
		return "big", true
	default:
		return "", false
	}
}

// resolvedConversion is what convertConversionReference produces: the
// mapdef conversion, its unit and decimal-place count, and — for TAB_VERB —
// the discrete value/text pairs a consumer may render instead of the raw
// number. StaticValues is nil for every other ConversionType.
type resolvedConversion struct {
	Conversion    types.Conversion
	Unit          string
	DecimalPlaces int
	StaticValues  []types.StaticValue
}

// convertConversionReference resolves a Conversion reference (a COMPU_METHOD
// name, or the NO_COMPU_METHOD sentinel) to a mapdef conversion, unit,
// decimal-place count, and (for TAB_VERB) static value labels.
// formatOverride is a CHARACTERISTIC-level FORMAT override, if any; pass ""
// for an AXIS_DESCR, which has no such override. COMPU_METHOD types
// IDENTICAL, LINEAR, TAB_VERB, RAT_FUNC, FORM, TAB_INTP, and TAB_NOINTP
// are supported — see docs/a2l-v0-plan.md's RAT_FUNC / FORM / COMPU_TAB
// evidence for the latter three.
func convertConversionReference(reference, formatOverride string, compuMethods map[string]types.A2LCompuMethod, compuVtabs map[string]types.A2LCompuVtab, compuVtabRanges map[string]types.A2LCompuVtabRange, compuTabs map[string]types.A2LCompuTab) (resolvedConversion, error) {
	format := formatOverride
	if reference == "NO_COMPU_METHOD" {
		return resolvedConversion{Conversion: types.IdentityConversion(), DecimalPlaces: decimalPlacesFromFormat(format)}, nil
	}
	compuMethod, ok := compuMethods[reference]
	if !ok {
		return resolvedConversion{}, fmt.Errorf("%w: %q", ErrCompuMethodNotFound, reference)
	}
	if format == "" {
		format = compuMethod.Format
	}
	decimalPlaces := decimalPlacesFromFormat(format)
	switch compuMethod.ConversionType {
	case "IDENTICAL":
		return resolvedConversion{Conversion: types.IdentityConversion(), Unit: compuMethod.Unit, DecimalPlaces: decimalPlaces}, nil
	case "LINEAR":
		if !compuMethod.HasCoeffsLinear {
			return resolvedConversion{}, fmt.Errorf("%w: %q is LINEAR with no COEFFS_LINEAR", ErrUnsupportedCompuMethod, compuMethod.Name)
		}
		expression := types.Conversion{
			Language:   "freehorse-expression-v1",
			Expression: formatLinearExpression(compuMethod.CoeffsLinearA, compuMethod.CoeffsLinearB),
		}
		return resolvedConversion{Conversion: expression, Unit: compuMethod.Unit, DecimalPlaces: decimalPlaces}, nil
	case "TAB_VERB":
		compuVtab, ok := compuVtabs[compuMethod.CompuTabRef]
		if !ok {
			if _, isRange := compuVtabRanges[compuMethod.CompuTabRef]; isRange {
				return resolvedConversion{}, fmt.Errorf("%w: %q", ErrCompuVtabRangeUnsupported, compuMethod.CompuTabRef)
			}
			return resolvedConversion{}, fmt.Errorf("%w: %q", ErrCompuVtabNotFound, compuMethod.CompuTabRef)
		}
		staticValues := make([]types.StaticValue, len(compuVtab.Entries))
		for i, entry := range compuVtab.Entries {
			staticValues[i] = types.StaticValue{Index: entry.Value, Value: entry.Text}
		}
		return resolvedConversion{
			Conversion: types.IdentityConversion(), Unit: compuMethod.Unit, DecimalPlaces: decimalPlaces, StaticValues: staticValues,
		}, nil
	case "RAT_FUNC":
		if !compuMethod.HasCoeffs {
			return resolvedConversion{}, fmt.Errorf("%w: %q is RAT_FUNC with no COEFFS", ErrUnsupportedCompuMethod, compuMethod.Name)
		}
		ratFuncExpression, err := formatRatFuncExpression(compuMethod.CoeffsA, compuMethod.CoeffsB, compuMethod.CoeffsC, compuMethod.CoeffsD, compuMethod.CoeffsE, compuMethod.CoeffsF)
		if err != nil {
			return resolvedConversion{}, fmt.Errorf("%w: %q: %v", ErrUnsupportedCompuMethod, compuMethod.Name, err)
		}
		return resolvedConversion{
			Conversion:    types.Conversion{Language: "freehorse-expression-v1", Expression: ratFuncExpression},
			Unit:          compuMethod.Unit,
			DecimalPlaces: decimalPlaces,
		}, nil
	case "FORM":
		if compuMethod.Formula == "" {
			return resolvedConversion{}, fmt.Errorf("%w: %q is FORM with no FORMULA", ErrUnsupportedCompuMethod, compuMethod.Name)
		}
		formExpression, err := translateFormFormula(compuMethod.Formula)
		if err != nil {
			return resolvedConversion{}, fmt.Errorf("%w: %q: %v", ErrUnsupportedCompuMethod, compuMethod.Name, err)
		}
		if _, err := expression.Parse(formExpression); err != nil {
			return resolvedConversion{}, fmt.Errorf("%w: %q FORMULA %q does not fit freehorse-expression-v1's supported grammar: %w", ErrUnsupportedCompuMethod, compuMethod.Name, compuMethod.Formula, err)
		}
		return resolvedConversion{
			Conversion:    types.Conversion{Language: "freehorse-expression-v1", Expression: formExpression},
			Unit:          compuMethod.Unit,
			DecimalPlaces: decimalPlaces,
		}, nil
	case "TAB_INTP", "TAB_NOINTP":
		compuTab, ok := compuTabs[compuMethod.CompuTabRef]
		if !ok {
			return resolvedConversion{}, fmt.Errorf("%w: %q", ErrCompuTabNotFound, compuMethod.CompuTabRef)
		}
		points := make([]types.ConversionPoint, len(compuTab.Entries))
		for i, entry := range compuTab.Entries {
			points[i] = types.ConversionPoint{Input: entry.InVal, Output: entry.OutVal}
		}
		lookupConversion := types.Conversion{Language: "freehorse-lookup-table-v1", Points: points, Interpolated: compuTab.Interpolated}
		if compuTab.HasDefaultValue {
			defaultValue := compuTab.DefaultValue
			lookupConversion.Default = &defaultValue
		}
		return resolvedConversion{Conversion: lookupConversion, Unit: compuMethod.Unit, DecimalPlaces: decimalPlaces}, nil
	default:
		return resolvedConversion{}, fmt.Errorf("%w: %q has ConversionType %q (unsupported in this slice)", ErrUnsupportedCompuMethod, compuMethod.Name, compuMethod.ConversionType)
	}
}

// formatLinearExpression renders phys = a*int + b as a freehorse-expression-v1
// string.
func formatLinearExpression(a, b float64) string {
	coefficient := strconv.FormatFloat(a, 'f', -1, 64)
	if b == 0 {
		if a == 1 {
			return "X"
		}
		return fmt.Sprintf("X*%s", coefficient)
	}
	if b < 0 {
		return fmt.Sprintf("X*%s-%s", coefficient, strconv.FormatFloat(-b, 'f', -1, 64))
	}
	return fmt.Sprintf("X*%s+%s", coefficient, strconv.FormatFloat(b, 'f', -1, 64))
}

// formatRatFuncExpression renders A2L RAT_FUNC's phys =
// (a*int^2 + b*int + c) / (d*int^2 + e*int + f) as a
// freehorse-expression-v1 string. Real evidence only ever has a purely
// scaling denominator (d=e=0, f!=0), but this handles the general case:
// when the denominator is a nonzero constant, the numerator's own
// coefficients are divided through into a bare polynomial (no
// unnecessary division); otherwise both sides are rendered and divided
// explicitly. A denominator that is always zero (d=e=f=0) is rejected.
func formatRatFuncExpression(a, b, c, d, e, f float64) (string, error) {
	if d == 0 && e == 0 {
		if f == 0 {
			return "", fmt.Errorf("COEFFS d, e, and f are all zero (denominator is always zero)")
		}
		return formatQuadratic(a/f, b/f, c/f), nil
	}
	numerator := formatQuadratic(a, b, c)
	denominator := formatQuadratic(d, e, f)
	return fmt.Sprintf("(%s)/(%s)", numerator, denominator), nil
}

// formatQuadratic renders a*X*X + b*X + c as a freehorse-expression-v1
// string, omitting zero terms and simplifying a coefficient of exactly 1
// (mirroring formatLinearExpression's own style). X*X stands in for an
// exponent: freehorse-expression-v1's grammar has no "^" operator, but
// repeated multiplication is exactly equivalent and already correctly
// detected as non-affine by expression.Expression.Affine.
func formatQuadratic(a, b, c float64) string {
	var terms []string
	if a != 0 {
		terms = append(terms, formatTerm(a, "X*X", len(terms) == 0))
	}
	if b != 0 {
		terms = append(terms, formatTerm(b, "X", len(terms) == 0))
	}
	if c != 0 || len(terms) == 0 {
		terms = append(terms, formatTerm(c, "", len(terms) == 0))
	}
	return strings.Join(terms, "")
}

// formatTerm renders one coefficient*variable term with its own leading
// sign (e.g. "+2*X", "-X*X", "+5"); first omits a leading "+", since it's
// the first term in the overall expression, not a binary operator.
func formatTerm(coefficient float64, variable string, first bool) string {
	sign, magnitude := "+", coefficient
	if coefficient < 0 {
		sign, magnitude = "-", -coefficient
	}
	prefix := sign
	if first && sign == "+" {
		prefix = ""
	}
	if variable == "" {
		return prefix + strconv.FormatFloat(magnitude, 'f', -1, 64)
	}
	if magnitude == 1 {
		return prefix + variable
	}
	return prefix + strconv.FormatFloat(magnitude, 'f', -1, 64) + "*" + variable
}

var formVariablePattern = regexp.MustCompile(`X\d+`)

// translateFormFormula translates a FORM COMPU_METHOD's FORMULA text from
// ASAM's own variable-naming convention (X1 for the first, and only,
// input a CHARACTERISTIC/AXIS_DESCR Conversion ever has) to
// freehorse-expression-v1's "X". A formula referencing any other
// variable (X2, X3, ...) is rejected rather than guessed at: this slice
// doesn't parse MEASUREMENT/FUNCTION, so a multi-input formula has no
// other input to resolve. The result is not otherwise validated here —
// the caller feeds it through expression.Parse, which rejects anything
// using syntax (functions, exponents, ...) outside
// freehorse-expression-v1's own small grammar.
func translateFormFormula(formula string) (string, error) {
	for _, match := range formVariablePattern.FindAllString(formula, -1) {
		if match != "X1" {
			return "", fmt.Errorf("FORMULA %q references %q, not just X1 (a multi-input formula has no other input to resolve)", formula, match)
		}
	}
	return formVariablePattern.ReplaceAllString(formula, "X"), nil
}

var formatSpecPattern = regexp.MustCompile(`^%\d*\.(\d+)$`)

// decimalPlacesFromFormat extracts the fractional digit count from an A2L
// "%W.D"-style FORMAT string (e.g. "%6.1" -> 1). An empty or unrecognized
// format yields 0, matching mapdef's Display.DecimalPlaces zero value.
func decimalPlacesFromFormat(format string) int {
	matches := formatSpecPattern.FindStringSubmatch(format)
	if matches == nil {
		return 0
	}
	places, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return places
}
