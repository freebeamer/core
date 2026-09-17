package mapdef_test

import (
	"bytes"
	"encoding/json"
	"github.com/freebeamer/core/pkg/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/freebeamer/core/pkg/mapdef"
)

func TestDocumentOwnsSafeFileIO(t *testing.T) {
	document := validDocument()
	path := filepath.Join(t.TempDir(), "definition.mapdef")
	if err := mapdef.Save(path, document); err != nil {
		t.Fatal(err)
	}
	loaded, err := mapdef.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != document.ID || loaded.FormatVersion != mapdef.CurrentVersion {
		t.Fatalf("loaded = %#v", loaded)
	}
	if err := mapdef.Save(path, document); err == nil {
		t.Fatal("Save overwrote an existing definition")
	}
}

func TestSaveRemovesPartialFileWhenValidationFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.mapdef")
	if err := mapdef.Save(path, &types.MapDefinition{}); err == nil {
		t.Fatal("invalid document was saved")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("partial output remains: %v", err)
	}
}

func TestDecodeRejectsTrailingDocument(t *testing.T) {
	var encoded bytes.Buffer
	if err := mapdef.Encode(&encoded, validDocument()); err != nil {
		t.Fatal(err)
	}
	encoded.WriteString(`{"format":"freehorse.mapdef"}`)
	if _, err := mapdef.Decode(&encoded); err == nil {
		t.Fatal("trailing JSON document accepted")
	}
}

func TestPublishedSchemaAndExampleMatchModel(t *testing.T) {
	schemaData, err := os.ReadFile(filepath.Join("..", "..", "schemas", "mapdef-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("schema dialect = %v", schema["$schema"])
	}
	example, err := os.Open(filepath.Join("..", "..", "examples", "synthetic.mapdef"))
	if err != nil {
		t.Fatal(err)
	}
	defer example.Close()
	if _, err := mapdef.Decode(example); err != nil {
		t.Fatal(err)
	}
}

func TestValidationRejectsUnknownVersionAndInvalidFormula(t *testing.T) {
	document := &types.MapDefinition{Format: mapdef.Format, FormatVersion: "2.0.0", ID: "test", Name: "Test"}
	if err := mapdef.Validate(document); err == nil {
		t.Fatal("unknown major version accepted")
	}
	document.FormatVersion = mapdef.CurrentVersion
	document.Parameters = []types.MapParameter{{
		ID: "p", Kind: "scalar", Name: "P",
		Layout:     types.DataLayout{DataType: "uint8", ByteOrder: "not-applicable", Dimensions: []int{1}, Order: "not-applicable"},
		Conversion: types.Conversion{Language: "freehorse-expression-v1", Expression: "X+"},
	}}
	if err := mapdef.Validate(document); err == nil {
		t.Fatal("invalid formula accepted")
	}
}

func validDocument() *types.MapDefinition {
	return &types.MapDefinition{
		Format: mapdef.Format, FormatVersion: mapdef.CurrentVersion, ID: "test", Name: "Test",
		Parameters: []types.MapParameter{{
			ID: "p", Kind: "scalar", Name: "P",
			Layout:     types.DataLayout{DataType: "uint8", ByteOrder: "not-applicable", Dimensions: []int{1}, Order: "not-applicable"},
			Conversion: types.IdentityConversion(),
		}},
	}
}
