package calibration

import (
	"strings"

	"github.com/freebeamer/core/pkg/types"
)

func FindParameter(definition *types.MapDefinition, name string) (*types.MapParameter, error) {
	var insensitive *types.MapParameter
	for index := range definition.Parameters {
		parameter := &definition.Parameters[index]
		if parameter.Name == name {
			return parameter, nil
		}
		if strings.EqualFold(parameter.Name, name) {
			if insensitive != nil {
				return nil, &AmbiguousTableError{Name: name}
			}
			insensitive = parameter
		}
	}
	if insensitive != nil {
		return insensitive, nil
	}
	return nil, &TableNotFoundError{Name: name}
}

type TableNotFoundError struct{ Name string }

func (e *TableNotFoundError) Error() string { return "map not found: " + e.Name }

type AmbiguousTableError struct{ Name string }

func (e *AmbiguousTableError) Error() string { return "map name is ambiguous: " + e.Name }

func (d *Decoder) Validate() []error {
	var failures []error
	for _, parameter := range d.definition.Parameters {
		if parameter.Kind == "flag" {
			continue
		}
		if _, err := d.DecodeParameter(parameter); err != nil {
			failures = append(failures, err)
		}
	}
	return failures
}
