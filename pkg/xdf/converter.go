package xdf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/freebeamer/core/pkg/mapdef"
	"github.com/freebeamer/core/pkg/types"
)

type MapdefConverter interface {
	Convert(source *types.XDFDefinition, sourceName string) (*types.MapDefinition, error)
}

type Converter struct{}

var _ MapdefConverter = Converter{}

func (Converter) Convert(source *types.XDFDefinition, sourceName string) (*types.MapDefinition, error) {
	if source == nil {
		return nil, fmt.Errorf("xdf: cannot convert a nil definition")
	}

	document := newMapDefinition(source, sourceName)
	categoryIDs := convertCategories(source, document)
	usedParameterIDs := make(map[string]int)

	for index, table := range source.Tables {
		parameter, diagnostics := convertTable(table, index, source.Header.Defaults, categoryIDs)
		parameter.ID = uniqueIdentifier(parameter.ID, usedParameterIDs)
		document.Parameters = append(document.Parameters, parameter)
		document.ImportDiagnostics = append(document.ImportDiagnostics, diagnostics...)
	}

	for index, flag := range source.Flags {
		parameter, diagnostics := convertFlag(flag, index, source.Header.Defaults, categoryIDs)
		parameter.ID = uniqueIdentifier(parameter.ID, usedParameterIDs)
		document.Parameters = append(document.Parameters, parameter)
		document.ImportDiagnostics = append(document.ImportDiagnostics, diagnostics...)
	}

	for index, constant := range source.Constants {
		parameter, diagnostics := convertConstant(constant, index, source.Header.Defaults, categoryIDs)
		parameter.ID = uniqueIdentifier(parameter.ID, usedParameterIDs)
		document.Parameters = append(document.Parameters, parameter)
		document.ImportDiagnostics = append(document.ImportDiagnostics, diagnostics...)
	}

	if len(source.RawXML) > 0 {
		document.ImportDiagnostics = append(document.ImportDiagnostics, sourcePreservedDiagnostic())
	}
	if err := mapdef.Validate(document); err != nil {
		return nil, fmt.Errorf("xdf: converted mapdef is invalid: %w", err)
	}
	return document, nil
}

func newMapDefinition(source *types.XDFDefinition, sourceName string) *types.MapDefinition {
	digest := sha256.Sum256(source.RawXML)
	document := &types.MapDefinition{
		Format:        mapdef.Format,
		FormatVersion: mapdef.CurrentVersion,
		ID:            identifier(source.Header.Title, "imported-xdf"),
		Name:          valueOrFallback(strings.TrimSpace(source.Header.Title), "Imported XDF definition"),
		Description:   strings.TrimSpace(source.Header.Description),
		Provenance: types.MapProvenance{
			Author:        source.Header.Author,
			Source:        sourceName,
			SourceFormat:  "xdf",
			SourceVersion: source.Version,
			SourceSHA256:  hex.EncodeToString(digest[:]),
			ConvertedBy:   "freebeamer",
		},
		Memory: types.MapMemory{BaseOffset: effectiveBaseOffset(source.Header.BaseOffset)},
		Extensions: map[string]interface{}{
			"xdf.rawDocument": string(source.RawXML),
		},
	}

	for index, region := range source.Header.Regions {
		segmentID := identifier(region.Name, fmt.Sprintf("region-%d", index))
		document.Memory.Segments = append(document.Memory.Segments, types.MemorySegment{
			ID:          segmentID,
			Name:        valueOrFallback(region.Name, segmentID),
			Description: region.Description,
			Start:       region.Start,
			Size:        region.Size,
			Type:        region.Type,
		})
		if region.Start == 0 && region.Size > 0 {
			document.Compatibility.ImageSizes = append(document.Compatibility.ImageSizes, region.Size)
		}
	}
	return document
}

func sourcePreservedDiagnostic() types.ImportDiagnostic {
	return types.ImportDiagnostic{
		Severity: "info",
		Code:     "xdf.source-preserved",
		Message:  "The original XDF document is preserved in extensions.xdf.rawDocument; only documented compatibility-matrix fields were interpreted.",
	}
}
