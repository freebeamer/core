package xdf

import (
	"strings"

	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/utils"
)

func normalizeConstant(raw types.XDFXMLConstant) (types.XDFConstant, error) {
	values := utils.Attrs(raw.Attrs)
	memberships, err := normalizeCategoryMemberships(raw.Categories)
	if err != nil {
		return types.XDFConstant{}, err
	}
	embedded, err := normalizeEmbedded(raw.Embedded)
	if err != nil {
		return types.XDFConstant{}, err
	}
	result := types.XDFConstant{
		ID:          values["uniqueid"],
		Title:       strings.TrimSpace(raw.Title),
		Description: strings.TrimSpace(raw.Description),
		Categories:  memberships,
		Embedded:    embedded,
		Units:       strings.TrimSpace(raw.Units),
		Formula:     utils.Attrs(raw.Math.Attrs)["equation"],
		Raw:         values,
	}
	if result.DecimalPlaces, err = optionalDecimal(raw.DecimalPlaces, "decimalpl"); err != nil {
		return types.XDFConstant{}, err
	}
	if result.OutputType, err = optionalDecimal(raw.OutputType, "outputtype"); err != nil {
		return types.XDFConstant{}, err
	}
	if result.Min, err = optionalFloat(raw.Min, "min"); err != nil {
		return types.XDFConstant{}, err
	}
	if result.Max, err = optionalFloat(raw.Max, "max"); err != nil {
		return types.XDFConstant{}, err
	}
	return result, nil
}
