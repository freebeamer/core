package mapdef

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/freebeamer/core/pkg/types"
)

// ErrUnknownFormatVersion is returned when a document's formatVersion has no
// known migration step toward CurrentVersion.
var ErrUnknownFormatVersion = errors.New("mapdef: unknown format version")

// migrationStep transforms document, already confirmed to be at the version
// this step handles, one adjacent version forward, and returns a diagnostic
// describing what it did.
type migrationStep func(document *types.MapDefinition) types.ImportDiagnostic

// migrations maps a formatVersion to the step that advances a document from
// it to the next adjacent version. Each step is deterministic: it only
// reshapes data already present in the document (for example, promoting
// data an older version preserved but could not yet type) and never reaches
// out to the original XDF/BIN or guesses values.
var migrations = map[string]migrationStep{
	"1.0.0": migrateV1_0_0ToV1_1_0,
	"1.1.0": migrateV1_1_0ToV1_2_0,
	"1.2.0": migrateV1_2_0ToV1_3_0,
}

// Migrate upgrades a copy of document to CurrentVersion, applying adjacent
// migration steps in sequence, and validates the result before returning it.
// document itself is never modified. A document already at CurrentVersion is
// returned as an unmodified, validated copy with no migration diagnostic
// appended.
func Migrate(document *types.MapDefinition) (*types.MapDefinition, error) {
	if document == nil {
		return nil, errors.New("mapdef: nil document")
	}
	current, err := cloneDocument(document)
	if err != nil {
		return nil, fmt.Errorf("mapdef: migrate: %w", err)
	}
	for current.FormatVersion != CurrentVersion {
		step, ok := migrations[current.FormatVersion]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownFormatVersion, current.FormatVersion)
		}
		diagnostic := step(current)
		current.ImportDiagnostics = append(current.ImportDiagnostics, diagnostic)
	}
	if err := Validate(current); err != nil {
		return nil, fmt.Errorf("mapdef: migrated document is invalid: %w", err)
	}
	return current, nil
}

// cloneDocument deep-copies document via a JSON round trip. This also
// normalizes every SourceMetadata value to the same shapes Decode would
// produce (plain map[string]interface{}/[]interface{}/float64/string),
// which the migration steps below rely on.
func cloneDocument(document *types.MapDefinition) (*types.MapDefinition, error) {
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	var clone types.MapDefinition
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &clone, nil
}

// migrateV1_0_0ToV1_1_0 promotes XDF axis labels and display bounds that a
// 1.0.0 document (produced by an older xdf.Converter) could only preserve in
// sourceMetadata into mapdef 1.1's typed axis.staticValues and
// display.min/max fields. See docs/xdf-compatibility.md's axis label and
// bounds evidence sections for what these represent.
func migrateV1_0_0ToV1_1_0(document *types.MapDefinition) types.ImportDiagnostic {
	document.FormatVersion = "1.1.0"
	promoted := 0
	for i := range document.Parameters {
		parameter := &document.Parameters[i]
		promoted += promoteBounds(&parameter.Display, parameter.SourceMetadata)
		for name, axis := range parameter.Axes {
			promoted += promoteBounds(&axis.Display, axis.SourceMetadata)
			promoted += promoteStaticValues(&axis, axis.SourceMetadata)
			parameter.Axes[name] = axis
		}
	}
	return types.ImportDiagnostic{
		Severity: "info",
		Code:     "mapdef.migrated-1.0.0-to-1.1.0",
		Message: fmt.Sprintf(
			"Migrated from mapdef 1.0.0 to 1.1.0; promoted %d previously-preserved value(s) (XDF axis labels/bounds) into typed fields.",
			promoted,
		),
	}
}

// migrateV1_1_0ToV1_2_0 is a version bump only: mapdef 1.2 adds
// axis.fixedPoints (A2L FIX_AXIS's formula/list-derived breakpoints, an axis
// with no on-disk storage at all) and gives axis.layout.strideBits real
// decode semantics (A2L RES_AXIS's interleaved rescale pairs). Both are new,
// additive capabilities with no prior lossy representation in a 1.1.0
// document to promote, so this step only advances the version number.
func migrateV1_1_0ToV1_2_0(document *types.MapDefinition) types.ImportDiagnostic {
	document.FormatVersion = "1.2.0"
	return types.ImportDiagnostic{
		Severity: "info",
		Code:     "mapdef.migrated-1.1.0-to-1.2.0",
		Message:  "Migrated from mapdef 1.1.0 to 1.2.0; no data changes (1.2 only adds new optional axis fields, axis.fixedPoints and layout.strideBits semantics).",
	}
}

// migrateV1_2_0ToV1_3_0 is a version bump only: mapdef 1.3 adds a second
// conversion shape, "freehorse-lookup-table-v1" (conversion.points/
// interpolated/default), for a numeric lookup table with no arithmetic-
// expression representation (A2L TAB_INTP/TAB_NOINTP). This is a new,
// additive capability — a 1.2.0 document only ever used
// "freehorse-expression-v1" conversions, so there is nothing to promote.
func migrateV1_2_0ToV1_3_0(document *types.MapDefinition) types.ImportDiagnostic {
	document.FormatVersion = "1.3.0"
	return types.ImportDiagnostic{
		Severity: "info",
		Code:     "mapdef.migrated-1.2.0-to-1.3.0",
		Message:  "Migrated from mapdef 1.2.0 to 1.3.0; no data changes (1.3 only adds the new freehorse-lookup-table-v1 conversion shape).",
	}
}

func promoteBounds(display *types.Display, sourceMetadata map[string]interface{}) int {
	bounds, ok := sourceMetadata["xdf.bounds"].(map[string]interface{})
	if !ok {
		return 0
	}
	promoted := 0
	if min, ok := bounds["min"].(float64); ok {
		display.Min = &min
		promoted++
	}
	if max, ok := bounds["max"].(float64); ok {
		display.Max = &max
		promoted++
	}
	return promoted
}

func promoteStaticValues(axis *types.MapAxis, sourceMetadata map[string]interface{}) int {
	raw, ok := sourceMetadata["xdf.labels"].([]interface{})
	if !ok {
		return 0
	}
	values := make([]types.StaticValue, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		index, indexOK := entry["index"].(float64)
		value, valueOK := entry["value"].(string)
		if !indexOK || !valueOK {
			continue
		}
		values = append(values, types.StaticValue{Index: int(index), Value: value})
	}
	if len(values) == 0 {
		return 0
	}
	axis.StaticValues = values
	return len(values)
}
