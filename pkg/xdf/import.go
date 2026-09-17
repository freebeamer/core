package xdf

import "github.com/freebeamer/core/pkg/types"

func ImportMapDefinition(path string) (*types.MapDefinition, error) {
	source, err := Load(path)
	if err != nil {
		return nil, err
	}
	return (Converter{}).Convert(source, path)
}
