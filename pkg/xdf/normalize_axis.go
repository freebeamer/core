package xdf

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/utils"
)

func normalizeAxis(raw types.XDFXMLAxis) (types.XDFAxis, error) {
	values := utils.Attrs(raw.Attrs)
	embedded, err := normalizeEmbedded(raw.Embedded)
	if err != nil {
		return types.XDFAxis{}, err
	}
	result := types.XDFAxis{
		ID:       values["id"],
		Units:    strings.TrimSpace(raw.Units),
		Embedded: embedded,
		Formula:  utils.Attrs(raw.Math.Attrs)["equation"],
		Raw:      values,
	}
	if result.IndexCount, err = optionalDecimal(raw.IndexCount, "indexcount"); err != nil {
		return result, err
	}
	if result.DecimalPlaces, err = optionalDecimal(raw.DecimalPlaces, "decimalpl"); err != nil {
		return result, err
	}
	if result.OutputType, err = optionalDecimal(raw.OutputType, "outputtype"); err != nil {
		return result, err
	}
	if result.Labels, err = normalizeLabels(raw.Labels); err != nil {
		return result, err
	}
	if result.Min, err = optionalFloat(raw.Min, "min"); err != nil {
		return result, err
	}
	if result.Max, err = optionalFloat(raw.Max, "max"); err != nil {
		return result, err
	}
	return result, nil
}

func normalizeLabels(raw []types.XDFXMLElement) ([]types.XDFLabel, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	labels := make([]types.XDFLabel, 0, len(raw))
	for _, item := range raw {
		values := utils.Attrs(item.Attrs)
		index, err := utils.IntField(values, "index")
		if err != nil {
			return nil, fmt.Errorf("invalid LABEL: %w", err)
		}
		labels = append(labels, types.XDFLabel{Index: index, Value: values["value"], Raw: values})
	}
	return labels, nil
}

func optionalDecimal(value, field string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", field, value, err)
	}
	return parsed, nil
}

// optionalFloat parses an optional <min>/<max> element's decimal text (e.g.
// "25.000000"). A blank element returns a nil bound rather than zero, since
// zero is a meaningful bound.
func optionalFloat(value, field string) (*float64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", field, trimmed, err)
	}
	return &parsed, nil
}
