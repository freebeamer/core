package mapdef

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/types"
)

const (
	Format         = "freehorse.mapdef"
	CurrentVersion = "1.3.0"
)

// Validate checks document structure against the published Draft 2020-12
// mapdef schema, then applies the semantic rules the schema cannot express:
// unique parameter IDs and parseable conversion expressions.
func Validate(document *types.MapDefinition) error {
	if document == nil {
		return errors.New("mapdef: nil document")
	}
	data, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("mapdef: encode for validation: %w", err)
	}
	if err := ValidateSchema(data); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for index, parameter := range document.Parameters {
		path := fmt.Sprintf("parameters[%d]", index)
		if seen[parameter.ID] {
			return fmt.Errorf("mapdef: duplicate parameter id %q", parameter.ID)
		}
		seen[parameter.ID] = true
		if err := validateConversion(parameter.Conversion); err != nil {
			return fmt.Errorf("mapdef: %s: %w", path, err)
		}
		for name, axis := range parameter.Axes {
			if err := validateConversion(axis.Conversion); err != nil {
				return fmt.Errorf("mapdef: %s axis %s: %w", path, name, err)
			}
		}
	}
	return nil
}

func validateConversion(conversion types.Conversion) error {
	switch conversion.Language {
	case "freehorse-expression-v1":
		if _, err := expression.Parse(conversion.Expression); err != nil {
			return fmt.Errorf("invalid conversion %q: %w", conversion.Expression, err)
		}
		return nil
	case "freehorse-lookup-table-v1":
		if len(conversion.Points) == 0 {
			return errors.New("lookup table conversion has no points")
		}
		return nil
	default:
		return fmt.Errorf("unsupported conversion language %q", conversion.Language)
	}
}
