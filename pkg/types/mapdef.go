package types

import "sort"

type MapDefinition struct {
	Format               string                 `json:"format"`
	FormatVersion        string                 `json:"formatVersion"`
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description,omitempty"`
	Compatibility        MapCompatibility       `json:"compatibility"`
	Provenance           MapProvenance          `json:"provenance"`
	Memory               MapMemory              `json:"memory"`
	Categories           []MapCategory          `json:"categories,omitempty"`
	Parameters           []MapParameter         `json:"parameters"`
	ChecksumRequirements []ChecksumRequirement  `json:"checksumRequirements,omitempty"`
	ImportDiagnostics    []ImportDiagnostic     `json:"importDiagnostics,omitempty"`
	Extensions           map[string]interface{} `json:"extensions,omitempty"`
}

type MapCompatibility struct {
	Manufacturers []string `json:"manufacturers,omitempty"`
	ECUFamilies   []string `json:"ecuFamilies,omitempty"`
	SoftwareIDs   []string `json:"softwareIds,omitempty"`
	ImageSizes    []uint64 `json:"imageSizes,omitempty"`
}

type MapProvenance struct {
	Author        string `json:"author,omitempty"`
	License       string `json:"license,omitempty"`
	Source        string `json:"source,omitempty"`
	SourceFormat  string `json:"sourceFormat,omitempty"`
	SourceVersion string `json:"sourceVersion,omitempty"`
	SourceSHA256  string `json:"sourceSha256,omitempty"`
	ConvertedBy   string `json:"convertedBy,omitempty"`
}

type MapMemory struct {
	BaseOffset int64           `json:"baseOffset"`
	Segments   []MemorySegment `json:"segments,omitempty"`
}

type MemorySegment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Start       uint64 `json:"start"`
	Size        uint64 `json:"size"`
	Type        string `json:"type,omitempty"`
}

type MapCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MapParameter struct {
	ID             string                 `json:"id"`
	Kind           string                 `json:"kind"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	Categories     []string               `json:"categories,omitempty"`
	Layout         DataLayout             `json:"layout"`
	Axes           map[string]MapAxis     `json:"axes,omitempty"`
	Unit           string                 `json:"unit,omitempty"`
	Conversion     Conversion             `json:"conversion"`
	Display        Display                `json:"display,omitempty"`
	SourceMetadata map[string]interface{} `json:"sourceMetadata,omitempty"`
}

type DataLayout struct {
	Address     *uint64 `json:"address,omitempty"`
	DataType    string  `json:"dataType"`
	ByteOrder   string  `json:"byteOrder"`
	Dimensions  []int   `json:"dimensions"`
	Order       string  `json:"order"`
	StrideBits  []int   `json:"strideBits,omitempty"`
	BitPosition *int    `json:"bitPosition,omitempty"`
}

type MapAxis struct {
	Count      int         `json:"count"`
	Layout     *DataLayout `json:"layout,omitempty"`
	Unit       string      `json:"unit,omitempty"`
	Conversion Conversion  `json:"conversion"`
	Display    Display     `json:"display,omitempty"`
	// StaticValues gives this axis's breakpoints as an explicit index/value
	// list rather than computed addresses (mapdef 1.1+; an XDF <LABEL> axis
	// converts to this).
	StaticValues []StaticValue `json:"staticValues,omitempty"`
	// FixedPoints gives this axis's raw (pre-conversion) breakpoints
	// directly, for an axis with no on-disk storage at all (mapdef 1.2+;
	// A2L's FIX_AXIS, computed from a formula or literal list, converts to
	// this). Mutually exclusive with Layout — exactly one of the two is
	// set for an axis to decode.
	FixedPoints    []int64                `json:"fixedPoints,omitempty"`
	SourceMetadata map[string]interface{} `json:"sourceMetadata,omitempty"`
}

// StaticValue is one explicit index/value pair of an axis's static
// breakpoints (mapdef 1.1+).
type StaticValue struct {
	Index int    `json:"index"`
	Value string `json:"value"`
}

// Conversion is either a "freehorse-expression-v1" arithmetic expression
// (Expression set, Points/Interpolated unused) or a "freebeamer-lookup-
// table-v1" numeric lookup table (mapdef 1.3+; Points/Interpolated set,
// Expression unused) — A2L's TAB_INTP/TAB_NOINTP convert to the latter,
// since a piecewise lookup has no representation as an arithmetic
// expression. Exactly one of the two shapes applies, selected by Language.
type Conversion struct {
	Language   string `json:"language"`
	Expression string `json:"expression,omitempty"`
	// Points gives the raw-to-physical mapping as (input, output) pairs,
	// used only when Language is "freehorse-lookup-table-v1". Order is
	// not significant; consumers sort by Input before evaluating.
	Points []ConversionPoint `json:"points,omitempty"`
	// Interpolated selects linear interpolation between adjacent Points
	// (true, A2L TAB_INTP) versus an exact-match-only lookup (false, A2L
	// TAB_NOINTP). Used only when Language is "freehorse-lookup-table-v1".
	Interpolated bool `json:"interpolated,omitempty"`
	// Default, when non-nil, is the physical output to use when a raw
	// value falls outside Points' range, or (Interpolated: false) has no
	// exact match (A2L DEFAULT_VALUE_NUMERIC). Used only when Language is
	// "freehorse-lookup-table-v1".
	Default *float64 `json:"default,omitempty"`
}

// ConversionPoint is one (input, output) pair of a
// "freehorse-lookup-table-v1" Conversion (mapdef 1.3+).
type ConversionPoint struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

type Display struct {
	DecimalPlaces int `json:"decimalPlaces,omitempty"`
	OutputType    int `json:"outputType,omitempty"`
	// Min and Max are optional informational display bounds (mapdef 1.1+).
	// They are not enforced as edit-time range checks.
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

type ChecksumRequirement struct {
	Provider string `json:"provider"`
	Required bool   `json:"required"`
}

type ImportDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

func IdentityConversion() Conversion {
	return Conversion{Language: "freehorse-expression-v1", Expression: "X"}
}

func (definition *MapDefinition) MemoryFits(imageSize uint64) bool {
	if len(definition.Memory.Segments) == 0 {
		return false
	}
	for _, segment := range definition.Memory.Segments {
		if segment.Start > imageSize || segment.Size > imageSize-segment.Start {
			return false
		}
	}
	return true
}

func (definition *MapDefinition) CategoryNames(categoryIDs []string) []string {
	namesByID := make(map[string]string, len(definition.Categories))
	for _, category := range definition.Categories {
		namesByID[category.ID] = category.Name
	}
	names := make([]string, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		name := namesByID[categoryID]
		if name == "" {
			name = categoryID
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
