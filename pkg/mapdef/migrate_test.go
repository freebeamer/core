package mapdef_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/freebeamer/core/pkg/mapdef"
	"github.com/freebeamer/core/pkg/types"
)

const golden1_0_0Document = `{
  "format": "freehorse.mapdef",
  "formatVersion": "1.0.0",
  "id": "golden",
  "name": "Golden",
  "compatibility": {},
  "provenance": {},
  "memory": {"baseOffset": 0},
  "parameters": [
    {
      "id": "gear-table",
      "kind": "curve",
      "name": "Gear table",
      "layout": {"dataType": "uint8", "byteOrder": "not-applicable", "dimensions": [2], "order": "not-applicable"},
      "conversion": {"language": "freehorse-expression-v1", "expression": "X"},
      "sourceMetadata": {"xdf.bounds": {"min": 1, "max": 99}},
      "axes": {
        "x": {
          "count": 2,
          "conversion": {"language": "freehorse-expression-v1", "expression": "X"},
          "sourceMetadata": {
            "xdf.labels": [{"index": 0, "value": "Park"}, {"index": 1, "value": "Drive"}],
            "xdf.bounds": {"min": 0, "max": 25}
          }
        }
      }
    }
  ]
}`

func TestMigrateV1_0_0PromotesPreservedAxisLabelsAndBounds(t *testing.T) {
	original, err := mapdef.Decode(strings.NewReader(golden1_0_0Document))
	if err != nil {
		t.Fatalf("decode 1.0.0 document: %v", err)
	}

	migrated, err := mapdef.Migrate(original)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if migrated.FormatVersion != mapdef.CurrentVersion {
		t.Fatalf("FormatVersion = %q, want %q", migrated.FormatVersion, mapdef.CurrentVersion)
	}
	parameter := migrated.Parameters[0]
	if parameter.Display.Min == nil || *parameter.Display.Min != 1 || parameter.Display.Max == nil || *parameter.Display.Max != 99 {
		t.Fatalf("parameter bounds = %#v", parameter.Display)
	}
	axis := parameter.Axes["x"]
	if axis.Display.Min == nil || *axis.Display.Min != 0 || axis.Display.Max == nil || *axis.Display.Max != 25 {
		t.Fatalf("axis bounds = %#v", axis.Display)
	}
	want := []types.StaticValue{{Index: 0, Value: "Park"}, {Index: 1, Value: "Drive"}}
	if len(axis.StaticValues) != 2 || axis.StaticValues[0] != want[0] || axis.StaticValues[1] != want[1] {
		t.Fatalf("static values = %#v", axis.StaticValues)
	}

	wantCodes := []string{"mapdef.migrated-1.0.0-to-1.1.0", "mapdef.migrated-1.1.0-to-1.2.0", "mapdef.migrated-1.2.0-to-1.3.0"}
	for _, wantCode := range wantCodes {
		found := false
		for _, diagnostic := range migrated.ImportDiagnostics {
			if diagnostic.Code == wantCode {
				found = true
			}
		}
		if !found {
			t.Fatalf("diagnostics = %#v, want a %s entry", migrated.ImportDiagnostics, wantCode)
		}
	}

	if err := mapdef.Validate(migrated); err != nil {
		t.Fatalf("migrated document failed validation: %v", err)
	}

	// original must be untouched.
	if original.FormatVersion != "1.0.0" {
		t.Fatal("Migrate mutated its input document")
	}
	if original.Parameters[0].Axes["x"].StaticValues != nil {
		t.Fatal("Migrate mutated its input document's axis")
	}
}

func TestMigrateIsANoOpAtCurrentVersion(t *testing.T) {
	document := &types.MapDefinition{
		Format: mapdef.Format, FormatVersion: mapdef.CurrentVersion, ID: "test", Name: "Test",
		Parameters: []types.MapParameter{{
			ID: "p", Kind: "scalar", Name: "P",
			Layout:     types.DataLayout{DataType: "uint8", ByteOrder: "not-applicable", Dimensions: []int{1}, Order: "not-applicable"},
			Conversion: types.IdentityConversion(),
		}},
	}
	migrated, err := mapdef.Migrate(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrated.ImportDiagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none for a no-op migration", migrated.ImportDiagnostics)
	}
	if migrated == document {
		t.Fatal("Migrate returned the same pointer instead of a copy")
	}
}

func TestMigrateRejectsUnknownFormatVersion(t *testing.T) {
	document := &types.MapDefinition{Format: mapdef.Format, FormatVersion: "9.9.9", ID: "test", Name: "Test"}
	if _, err := mapdef.Migrate(document); !errors.Is(err, mapdef.ErrUnknownFormatVersion) {
		t.Fatalf("error = %v, want ErrUnknownFormatVersion", err)
	}
}

func TestMigrateRejectsNilDocument(t *testing.T) {
	if _, err := mapdef.Migrate(nil); err == nil {
		t.Fatal("nil document accepted")
	}
}
