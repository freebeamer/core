package mapdef

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/freebeamer/core/pkg/types"
)

func Encode(writer io.Writer, document *types.MapDefinition) error {
	if err := Validate(document); err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

// Decode reads one mapdef document and validates it against the published
// Draft 2020-12 schema before decoding into the Go model. Schema validation
// runs first so a malformed file is reported with the JSON Pointer of every
// offending value rather than a raw Go unmarshal error.
func Decode(reader io.Reader) (*types.MapDefinition, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("decode mapdef: %w", err)
	}
	if err := ValidateSchema(data); err != nil {
		return nil, fmt.Errorf("decode mapdef: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document types.MapDefinition
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode mapdef: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode mapdef: multiple JSON documents")
		}
		return nil, fmt.Errorf("decode mapdef trailing data: %w", err)
	}
	if err := Validate(&document); err != nil {
		return nil, err
	}
	return &document, nil
}

// Save validates and writes a new mapdef file. It refuses to overwrite an
// existing path so callers cannot destroy a user's only definition by default.
func Save(path string, document *types.MapDefinition) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create mapdef %q: %w", path, err)
	}
	if err := Encode(file, document); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write mapdef %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close mapdef %q: %w", path, err)
	}
	return nil
}

// Load reads and validates a mapdef file.
func Load(path string) (*types.MapDefinition, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open mapdef %q: %w", path, err)
	}
	defer file.Close()
	document, err := Decode(file)
	if err != nil {
		return nil, fmt.Errorf("load mapdef %q: %w", path, err)
	}
	return document, nil
}
