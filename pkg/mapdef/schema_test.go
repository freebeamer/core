package mapdef_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/freebeamer/core/pkg/mapdef"
)

// TestEmbeddedSchemaMatchesPublishedSchema guards against the embedded copy
// under pkg/mapdef/schema/ drifting from the published schemas/ copy that
// README.md and the ADR link to. go:embed cannot reach outside this package
// directory, so the two files must be kept identical by hand.
func TestEmbeddedSchemaMatchesPublishedSchema(t *testing.T) {
	published, err := os.ReadFile(filepath.Join("..", "..", "schemas", "mapdef-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := os.ReadFile(filepath.Join("schema", "mapdef-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(published) != string(embedded) {
		t.Fatal("pkg/mapdef/schema/mapdef-v1.schema.json has drifted from schemas/mapdef-v1.schema.json")
	}
}

func TestValidateSchemaAcceptsSyntheticExample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "synthetic.mapdef"))
	if err != nil {
		t.Fatal(err)
	}
	if err := mapdef.ValidateSchema(data); err != nil {
		t.Fatalf("published example failed schema validation: %v", err)
	}
}

func TestValidateSchemaRejectsMalformedJSON(t *testing.T) {
	if err := mapdef.ValidateSchema([]byte(`{not json`)); err == nil {
		t.Fatal("malformed JSON was accepted")
	}
}

func TestValidateSchemaReportsPointerAndMessagePerFailure(t *testing.T) {
	cases := []struct {
		name        string
		document    string
		wantPointer string
		wantSubstr  string
	}{
		{
			name:        "missing required top-level field",
			document:    `{"formatVersion":"1.0.0","id":"test","name":"Test","compatibility":{},"provenance":{},"memory":{"baseOffset":0},"parameters":[]}`,
			wantPointer: "",
			wantSubstr:  "format",
		},
		{
			name:        "unsupported format version",
			document:    `{"format":"freehorse.mapdef","formatVersion":"9.9.9","id":"test","name":"Test","compatibility":{},"provenance":{},"memory":{"baseOffset":0},"parameters":[]}`,
			wantPointer: "/formatVersion",
			wantSubstr:  "1.0.0",
		},
		{
			name:        "id violates the required pattern",
			document:    `{"format":"freehorse.mapdef","formatVersion":"1.0.0","id":"BAD ID","name":"Test","compatibility":{},"provenance":{},"memory":{"baseOffset":0},"parameters":[]}`,
			wantPointer: "/id",
			wantSubstr:  "pattern",
		},
		{
			name:        "unknown top-level property",
			document:    `{"format":"freehorse.mapdef","formatVersion":"1.0.0","id":"test","name":"Test","compatibility":{},"provenance":{},"memory":{"baseOffset":0},"parameters":[],"bogus":1}`,
			wantPointer: "",
			wantSubstr:  "bogus",
		},
		{
			name: "unsupported parameter kind",
			document: `{"format":"freehorse.mapdef","formatVersion":"1.0.0","id":"test","name":"Test","compatibility":{},"provenance":{},"memory":{"baseOffset":0},` +
				`"parameters":[{"id":"p","kind":"weird","name":"P","layout":{"dataType":"uint8","byteOrder":"not-applicable","dimensions":[1],"order":"not-applicable"},"conversion":{"language":"freehorse-expression-v1","expression":"X"}}]}`,
			wantPointer: "/parameters/0/kind",
			wantSubstr:  "scalar",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := mapdef.ValidateSchema([]byte(testCase.document))
			var schemaErrs mapdef.SchemaErrors
			if !errors.As(err, &schemaErrs) {
				t.Fatalf("error = %v (%T), want mapdef.SchemaErrors", err, err)
			}
			for _, violation := range schemaErrs {
				if violation.Pointer == testCase.wantPointer && strings.Contains(violation.Message, testCase.wantSubstr) {
					return
				}
			}
			t.Fatalf("no violation at %q containing %q, got %#v", testCase.wantPointer, testCase.wantSubstr, schemaErrs)
		})
	}
}

func TestValidateRejectsNegativeDecimalPlacesBelowTheXDFSentinel(t *testing.T) {
	document := validDocument()
	document.Parameters[0].Display.DecimalPlaces = -2
	if err := mapdef.Validate(document); err == nil {
		t.Fatal("decimalPlaces below the -1 XDF sentinel was accepted")
	}
}
