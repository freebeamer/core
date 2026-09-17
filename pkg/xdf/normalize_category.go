package xdf

import (
	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/utils"
)

func normalizeCategory(raw types.XDFXMLElement) (types.XDFCategory, error) {
	values := utils.Attrs(raw.Attrs)
	index, err := utils.IntField(values, "index")
	return types.XDFCategory{
		Index: index,
		Name:  values["name"],
		Raw:   values,
	}, err
}

func normalizeCategoryMemberships(raw []types.XDFXMLElement) ([]int, error) {
	result := make([]int, 0, len(raw))
	for _, item := range raw {
		values := utils.Attrs(item.Attrs)
		value, err := utils.IntField(values, "category")
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}
