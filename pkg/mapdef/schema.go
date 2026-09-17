package mapdef

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// schemaJSON is a byte-identical copy of the published schema at
// schemas/mapdef-v1.schema.json, kept here because go:embed cannot reach
// outside this package directory. TestPublishedSchemaMatchesEmbeddedCopy
// guards the two files against drift.
//
//go:embed schema/mapdef-v1.schema.json
var schemaJSON []byte

const schemaID = "https://freehorse.dev/schemas/mapdef-v1.schema.json"

var compiledSchema = mustCompileSchema()

func mustCompileSchema() *jsonschema.Schema {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		panic(fmt.Sprintf("mapdef: embedded schema is invalid JSON: %v", err))
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaID, document); err != nil {
		panic(fmt.Sprintf("mapdef: embedded schema failed to register: %v", err))
	}
	schema, err := compiler.Compile(schemaID)
	if err != nil {
		panic(fmt.Sprintf("mapdef: embedded schema failed to compile: %v", err))
	}
	return schema
}

// SchemaError is one Draft 2020-12 validation failure, located by the JSON
// Pointer of the offending value within the document that was validated.
type SchemaError struct {
	Pointer string
	Message string
}

func (e SchemaError) Error() string {
	if e.Pointer == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Pointer, e.Message)
}

// SchemaErrors is every failure found by one schema validation call.
type SchemaErrors []SchemaError

func (errs SchemaErrors) Error() string {
	lines := make([]string, len(errs))
	for i, err := range errs {
		lines[i] = err.Error()
	}
	return "mapdef: schema validation failed:\n" + strings.Join(lines, "\n")
}

// ValidateSchema checks raw mapdef JSON against the published Draft 2020-12
// schema (schemas/mapdef-v1.schema.json) and reports every failure found,
// each located by the JSON Pointer of the offending value.
func ValidateSchema(data []byte) error {
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("mapdef: invalid JSON: %w", err)
	}
	if err := compiledSchema.Validate(instance); err != nil {
		validationErr, ok := err.(*jsonschema.ValidationError)
		if !ok {
			return fmt.Errorf("mapdef: schema validation failed: %w", err)
		}
		return flattenSchemaError(validationErr)
	}
	return nil
}

var schemaErrorPrinter = message.NewPrinter(language.English)

// flattenSchemaError walks the library's cause tree down to its leaves.
// The library's own BasicOutput collapses a $ref application (used
// throughout this schema via $defs) to a generic "validation failed"
// summary instead of the underlying keyword failure, so this walks the raw
// ValidationError tree directly to keep the specific leaf message.
func flattenSchemaError(root *jsonschema.ValidationError) SchemaErrors {
	var errs SchemaErrors
	var walk func(node *jsonschema.ValidationError)
	walk = func(node *jsonschema.ValidationError) {
		if len(node.Causes) == 0 {
			errs = append(errs, SchemaError{
				Pointer: jsonPointer(node.InstanceLocation),
				Message: node.ErrorKind.LocalizedString(schemaErrorPrinter),
			})
			return
		}
		for _, cause := range node.Causes {
			walk(cause)
		}
	}
	walk(root)
	return errs
}

var jsonPointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

// jsonPointer renders raw instance-location tokens as an RFC 6901 JSON
// Pointer. The empty pointer ("") denotes the whole document.
func jsonPointer(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	var b strings.Builder
	for _, tok := range tokens {
		b.WriteByte('/')
		b.WriteString(jsonPointerEscaper.Replace(tok))
	}
	return b.String()
}
