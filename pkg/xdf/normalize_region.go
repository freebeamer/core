package xdf

import (
	"fmt"

	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/utils"
)

func normalizeRegion(raw types.XDFXMLElement) (types.XDFRegion, error) {
	values := utils.Attrs(raw.Attrs)
	start, err := utils.Integer(values, "startaddress", 64)
	if err != nil {
		return types.XDFRegion{}, err
	}
	size, err := utils.Integer(values, "size", 64)
	if err != nil {
		return types.XDFRegion{}, err
	}
	if size > 0 && start > ^uint64(0)-(size-1) {
		return types.XDFRegion{}, fmt.Errorf("region address range overflows uint64")
	}
	return types.XDFRegion{
		Name:        values["name"],
		Description: values["desc"],
		Start:       start,
		Size:        size,
		Type:        values["type"],
		Flags:       values["regionflags"],
		Raw:         values,
	}, nil
}
