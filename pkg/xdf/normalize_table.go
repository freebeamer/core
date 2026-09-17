package xdf

import (
	"fmt"
	"strings"

	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/utils"
)

func normalizeTable(raw types.XDFXMLTable) (types.XDFTable, error) {
	values := utils.Attrs(raw.Attrs)
	memberships, err := normalizeCategoryMemberships(raw.Categories)
	if err != nil {
		return types.XDFTable{}, err
	}
	result := types.XDFTable{
		ID:          values["uniqueid"],
		Title:       strings.TrimSpace(raw.Title),
		Description: strings.TrimSpace(raw.Description),
		Categories:  memberships,
		Raw:         values,
	}
	for _, rawAxis := range raw.Axes {
		axis, err := normalizeAxis(rawAxis)
		if err != nil {
			return result, fmt.Errorf("axis %q: %w", utils.Attrs(rawAxis.Attrs)["id"], err)
		}
		switch strings.ToLower(axis.ID) {
		case "x":
			result.X = axis
		case "y":
			result.Y = axis
		case "z":
			result.Z = axis
		}
	}
	return result, nil
}
