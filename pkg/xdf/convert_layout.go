package xdf

import (
	"fmt"
	"strings"

	"github.com/freebeamer/core/pkg/types"
)

const (
	typeSigned      = uint64(0x01)
	typeLSBFirst    = uint64(0x02)
	typeColumnMajor = uint64(0x04)
	typeFloat       = uint64(0x10000)
	knownTypeFlags  = typeSigned | typeLSBFirst | typeColumnMajor | typeFloat
)

// convertLayout converts one EMBEDDEDDATA. Its third result reports a
// non-zero stride whose layout is not established; callers whose decode path
// would otherwise honour a stride (axes) must not let it decode.
func convertLayout(embedded types.XDFEmbeddedData, defaults types.XDFDefaults, dimensions []int, path string) (types.DataLayout, []types.ImportDiagnostic, bool) {
	flags := effectiveTypeFlags(embedded, defaults)
	bits := embedded.ElementSizeBits
	if bits == 0 {
		bits = defaults.DataSizeBits
	}

	diagnostics := make([]types.ImportDiagnostic, 0)
	dataType, supported := xdfDataType(bits, flags)
	if !supported {
		diagnostics = append(diagnostics, unsupportedDataTypeDiagnostic(path, bits, flags))
	} else if flags&typeFloat != 0 && flags&typeSigned != 0 {
		diagnostics = append(diagnostics, floatSignedIgnoredDiagnostic(path))
	}
	strideBits := []int{embedded.MajorStrideBits, embedded.MinorStrideBits}
	unsupportedStride := false
	switch {
	case embedded.MajorStrideBits == 0 && embedded.MinorStrideBits == 0:
	case contiguousStride(embedded, bits, dimensions):
		strideBits = []int{0, 0}
		diagnostics = append(diagnostics, contiguousStrideDiagnostic(path, embedded.MajorStrideBits))
	default:
		unsupportedStride = true
		diagnostics = append(diagnostics, unsupportedStrideDiagnostic(path, embedded))
	}

	return types.DataLayout{
		Address:    optionalAddress(embedded),
		DataType:   dataType,
		ByteOrder:  byteOrder(bits, flags),
		Dimensions: dimensions,
		Order:      storageOrder(dimensions, flags),
		StrideBits: strideBits,
	}, diagnostics, unsupportedStride
}

// contiguousStride reports the one stride shape every available source reads
// the same way: a one-dimensional run (an axis, or a table with a single row
// or column) whose major stride equals the element width and whose minor
// stride is zero. That is plain packed storage; a2l2xdf writes it for every
// contiguous axis. Readers disagree about a major stride on a genuinely 2D
// table (row pitch or cell step) and about any non-zero minor stride (packed
// or padding), so those stay unsupported. See xdf-compatibility.md's stride
// evidence section.
func contiguousStride(embedded types.XDFEmbeddedData, bits int, dimensions []int) bool {
	if embedded.MinorStrideBits != 0 || bits <= 0 || embedded.MajorStrideBits != bits {
		return false
	}
	longDimensions := 0
	for _, dimension := range dimensions {
		if dimension > 1 {
			longDimensions++
		}
	}
	return longDimensions <= 1
}

func effectiveTypeFlags(embedded types.XDFEmbeddedData, defaults types.XDFDefaults) uint64 {
	flags := embedded.TypeFlags
	if _, explicit := embedded.Raw["mmedtypeflags"]; explicit {
		return flags
	}
	if defaults.Signed {
		flags |= typeSigned
	}
	if defaults.LSBFirst {
		flags |= typeLSBFirst
	}
	if defaults.Float {
		flags |= typeFloat
	}
	return flags
}

// xdfDataType maps an element size and mmedtypeflags to a mapdef data type.
// The 0x10000 float flag with a 32-bit cell is IEEE-754 binary32. 64-bit and
// narrower float cells stay unsupported: TunerPro documents only 8/16/32-bit
// cells, and no real XDF with a 64-bit float was found. The signed bit has no
// effect on a float. See xdf-compatibility.md's IEEE-754 evidence section.
func xdfDataType(bits int, flags uint64) (string, bool) {
	if flags&^knownTypeFlags != 0 {
		return "unknown", false
	}
	if flags&typeFloat != 0 {
		if bits != 32 {
			return "unknown", false
		}
		return "float32", true
	}
	if bits != 8 && bits != 16 && bits != 32 {
		return "unknown", false
	}
	prefix := "uint"
	if flags&typeSigned != 0 {
		prefix = "int"
	}
	return fmt.Sprintf("%s%d", prefix, bits), true
}

func byteOrder(bits int, flags uint64) string {
	if bits == 8 {
		return "not-applicable"
	}
	if flags&typeLSBFirst != 0 {
		return "little"
	}
	return "big"
}

func storageOrder(dimensions []int, flags uint64) string {
	if len(dimensions) == 1 {
		return "not-applicable"
	}
	if flags&typeColumnMajor != 0 {
		return "column-major"
	}
	return "row-major"
}

func optionalAddress(embedded types.XDFEmbeddedData) *uint64 {
	if _, present := embedded.Raw["mmedaddress"]; !present {
		return nil
	}
	address := embedded.Address
	return &address
}

func convertFormula(formula string) types.Conversion {
	if strings.TrimSpace(formula) == "" {
		return types.IdentityConversion()
	}
	return types.Conversion{Language: "freehorse-expression-v1", Expression: formula}
}

func effectiveBaseOffset(offset types.XDFBaseOffset) int64 {
	if offset.Subtract {
		return -offset.Offset
	}
	return offset.Offset
}

func unsupportedDataTypeDiagnostic(path string, bits int, flags uint64) types.ImportDiagnostic {
	return types.ImportDiagnostic{
		Severity: "warning",
		Code:     "xdf.unsupported-data-type",
		Path:     path,
		Message:  fmt.Sprintf("XDF data representation bits=%d flags=%#x is preserved but unsupported.", bits, flags),
	}
}

func floatSignedIgnoredDiagnostic(path string) types.ImportDiagnostic {
	return types.ImportDiagnostic{
		Severity: "info",
		Code:     "xdf.float-signed-ignored",
		Path:     path,
		Message:  "XDF signed flag is set on an IEEE-754 cell; it has no effect on a float and is ignored.",
	}
}

func contiguousStrideDiagnostic(path string, majorBits int) types.ImportDiagnostic {
	return types.ImportDiagnostic{
		Severity: "info",
		Code:     "xdf.contiguous-stride",
		Path:     path,
		Message:  fmt.Sprintf("XDF major stride of %d bits equals the element width on a one-dimensional run; decoded as contiguous storage.", majorBits),
	}
}

func unsupportedStrideDiagnostic(path string, embedded types.XDFEmbeddedData) types.ImportDiagnostic {
	return types.ImportDiagnostic{
		Severity: "warning",
		Code:     "xdf.unsupported-stride",
		Path:     path,
		Message:  fmt.Sprintf("XDF stride (major %d bits, minor %d bits) is preserved but its layout is not established; this element is not decoded.", embedded.MajorStrideBits, embedded.MinorStrideBits),
	}
}
