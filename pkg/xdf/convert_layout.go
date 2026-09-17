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

func convertLayout(embedded types.XDFEmbeddedData, defaults types.XDFDefaults, dimensions []int, path string) (types.DataLayout, []types.ImportDiagnostic) {
	flags := effectiveTypeFlags(embedded, defaults)
	bits := embedded.ElementSizeBits
	if bits == 0 {
		bits = defaults.DataSizeBits
	}

	diagnostics := make([]types.ImportDiagnostic, 0)
	dataType, supported := integerDataType(bits, flags)
	if !supported {
		diagnostics = append(diagnostics, unsupportedDataTypeDiagnostic(path, bits, flags))
	}
	if embedded.MajorStrideBits != 0 || embedded.MinorStrideBits != 0 {
		diagnostics = append(diagnostics, unsupportedStrideDiagnostic(path))
	}

	return types.DataLayout{
		Address:    optionalAddress(embedded),
		DataType:   dataType,
		ByteOrder:  byteOrder(bits, flags),
		Dimensions: dimensions,
		Order:      storageOrder(dimensions, flags),
		StrideBits: []int{embedded.MajorStrideBits, embedded.MinorStrideBits},
	}, diagnostics
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

func integerDataType(bits int, flags uint64) (string, bool) {
	if bits != 8 && bits != 16 && bits != 32 || flags&typeFloat != 0 || flags&^knownTypeFlags != 0 {
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

func unsupportedStrideDiagnostic(path string) types.ImportDiagnostic {
	return types.ImportDiagnostic{
		Severity: "warning",
		Code:     "xdf.unsupported-stride",
		Path:     path,
		Message:  "Non-zero XDF strides are preserved but not decoded by FreeBeamer V0.",
	}
}
