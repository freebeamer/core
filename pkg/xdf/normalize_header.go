package xdf

import (
	"github.com/freebeamer/core/pkg/types"
	"github.com/freebeamer/core/pkg/utils"
)

func normalizeBaseOffset(raw types.XDFXMLElement) (types.XDFBaseOffset, error) {
	values := utils.Attrs(raw.Attrs)
	offset, err := utils.SignedInteger(values, "offset")
	if err != nil {
		return types.XDFBaseOffset{}, err
	}
	subtract, err := utils.BoolField(values, "subtract")
	return types.XDFBaseOffset{Offset: offset, Subtract: subtract, Raw: values}, err
}

func normalizeDefaults(raw types.XDFXMLElement) (types.XDFDefaults, error) {
	values := utils.Attrs(raw.Attrs)
	result := types.XDFDefaults{Raw: values}
	var err error
	if result.DataSizeBits, err = utils.IntField(values, "datasizeinbits"); err != nil {
		return result, err
	}
	if result.SignificantDigits, err = utils.IntField(values, "sigdigits"); err != nil {
		return result, err
	}
	if result.OutputType, err = utils.IntField(values, "outputtype"); err != nil {
		return result, err
	}
	if result.Signed, err = utils.BoolField(values, "signed"); err != nil {
		return result, err
	}
	if result.LSBFirst, err = utils.BoolField(values, "lsbfirst"); err != nil {
		return result, err
	}
	if result.Float, err = utils.BoolField(values, "float"); err != nil {
		return result, err
	}
	return result, nil
}
