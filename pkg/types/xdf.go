package types

import (
	"fmt"
	"sort"
)

type XDFDefinition struct {
	Version    string
	Header     XDFHeader
	Categories []XDFCategory
	Tables     []XDFTable
	Flags      []XDFFlag
	Constants  []XDFConstant
	RawXML     []byte
}

type XDFHeader struct {
	Title       string
	Description string
	Author      string
	BaseOffset  XDFBaseOffset
	Defaults    XDFDefaults
	Regions     []XDFRegion
}

type XDFBaseOffset struct {
	Offset   int64
	Subtract bool
	Raw      map[string]string
}

type XDFDefaults struct {
	DataSizeBits      int
	SignificantDigits int
	OutputType        int
	Signed            bool
	LSBFirst          bool
	Float             bool
	Raw               map[string]string
}

type XDFRegion struct {
	Name        string
	Description string
	Start       uint64
	Size        uint64
	Type        string
	Flags       string
	Raw         map[string]string
}

func (region XDFRegion) End() uint64 {
	if region.Size == 0 {
		return region.Start
	}
	return region.Start + region.Size - 1
}

func (definition *XDFDefinition) RegionsFit(imageSize uint64) bool {
	if len(definition.Header.Regions) == 0 {
		return false
	}
	for _, region := range definition.Header.Regions {
		if region.Start > imageSize || region.Size > imageSize-region.Start {
			return false
		}
	}
	return true
}

type XDFCategory struct {
	Index int
	Name  string
	Raw   map[string]string
}

func (definition *XDFDefinition) CategoryNames(memberships []int) []string {
	lookup := make(map[int]string, len(definition.Categories))
	for _, category := range definition.Categories {
		lookup[category.Index] = category.Name
	}
	names := make([]string, 0, len(memberships))
	for _, membership := range memberships {
		name := lookup[membership]
		if name == "" {
			name = fmt.Sprintf("0x%X", membership)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type XDFTable struct {
	ID          string
	Title       string
	Description string
	Categories  []int
	X           XDFAxis
	Y           XDFAxis
	Z           XDFAxis
	Raw         map[string]string
}

type XDFFlag struct {
	ID          string
	Title       string
	Description string
	Categories  []int
	Embedded    XDFEmbeddedData
	Formula     string
	// Mask is the flag's <mask> bitmask, applied to the addressed byte.
	// It is nil when the XDF element declares no mask.
	Mask *uint64
	Raw  map[string]string
}

// XDFConstant is a scalar value declared outside any table (XDFCONSTANT).
type XDFConstant struct {
	ID            string
	Title         string
	Description   string
	Categories    []int
	Embedded      XDFEmbeddedData
	Units         string
	DecimalPlaces int
	OutputType    int
	// Min and Max are the optional <min>/<max> display bounds. Nil means
	// the element declared no bound.
	Min     *float64
	Max     *float64
	Formula string
	Raw     map[string]string
}

type XDFAxis struct {
	ID            string
	Units         string
	IndexCount    int
	DecimalPlaces int
	OutputType    int
	Embedded      XDFEmbeddedData
	Formula       string
	// Labels are the axis's static <LABEL index="" value=""> text values, if
	// any. FreeBeamer preserves them in the converted parameter's source
	// metadata; it does not yet expose them as typed calibration data (see
	// docs/xdf-compatibility.md's axis label evidence section).
	Labels []XDFLabel
	// Min and Max are the optional <min>/<max> display bounds. Nil means
	// the element declared no bound.
	Min *float64
	Max *float64
	Raw map[string]string
}

// XDFLabel is one <LABEL index="" value=""> entry of a static axis.
type XDFLabel struct {
	Index int
	Value string
	Raw   map[string]string
}

type XDFEmbeddedData struct {
	TypeFlags       uint64
	Address         uint64
	ElementSizeBits int
	RowCount        int
	ColumnCount     int
	MajorStrideBits int
	MinorStrideBits int
	Raw             map[string]string
}
