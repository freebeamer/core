package xdf

import (
	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/utils"
)

func normalizeEmbedded(raw types.XDFXMLElement) (types.XDFEmbeddedData, error) {
	values := utils.Attrs(raw.Attrs)
	result := types.XDFEmbeddedData{Raw: values}
	var err error
	if result.TypeFlags, err = utils.Integer(values, "mmedtypeflags", 64); err != nil {
		return result, err
	}
	if result.Address, err = utils.Integer(values, "mmedaddress", 64); err != nil {
		return result, err
	}
	if result.ElementSizeBits, err = utils.IntField(values, "mmedelementsizebits"); err != nil {
		return result, err
	}
	if result.RowCount, err = utils.IntField(values, "mmedrowcount"); err != nil {
		return result, err
	}
	if result.ColumnCount, err = utils.IntField(values, "mmedcolcount"); err != nil {
		return result, err
	}
	majorStride, err := utils.SignedInteger(values, "mmedmajorstridebits")
	if err != nil {
		return result, err
	}
	minorStride, err := utils.SignedInteger(values, "mmedminorstridebits")
	if err != nil {
		return result, err
	}
	result.MajorStrideBits = int(majorStride)
	result.MinorStrideBits = int(minorStride)
	return result, nil
}
