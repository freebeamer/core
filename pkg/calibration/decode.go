package calibration

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/types"
)

func resolveDataType(layout types.DataLayout) (resolvedDataType, error) {
	result := resolvedDataType{}
	switch layout.DataType {
	case "uint8":
		result.bits = 8
	case "int8":
		result.bits, result.signed = 8, true
	case "uint16":
		result.bits = 16
	case "int16":
		result.bits, result.signed = 16, true
	case "uint32":
		result.bits = 32
	case "int32":
		result.bits, result.signed = 32, true
	default:
		return resolvedDataType{}, fmt.Errorf("%w: %q", ErrUnsupportedDataType, layout.DataType)
	}

	switch layout.ByteOrder {
	case "little":
		result.endian = binfile.LittleEndian
	case "big", "not-applicable":
		result.endian = binfile.BigEndian
	default:
		return resolvedDataType{}, fmt.Errorf("%w: %q", ErrUnsupportedByteOrder, layout.ByteOrder)
	}

	switch layout.Order {
	case "column-major":
		result.columnMajor = true
	case "row-major", "not-applicable":
	default:
		return resolvedDataType{}, fmt.Errorf("unsupported mapdef storage order %q", layout.Order)
	}
	return result, nil
}

func (d *Decoder) DecodeParameter(parameter types.MapParameter) (*types.DecodedTable, error) {
	result := &types.DecodedTable{Definition: parameter}

	if axis, present := parameter.Axes["x"]; present {
		var err error
		result.Raw.X, result.X, result.Addresses.X, err = d.decodeAxis(axis)
		if err != nil {
			return nil, definitionError(parameter, "x", err)
		}
	}
	if axis, present := parameter.Axes["y"]; present {
		var err error
		result.Raw.Y, result.Y, result.Addresses.Y, err = d.decodeAxis(axis)
		if err != nil {
			return nil, definitionError(parameter, "y", err)
		}
	}

	rows, columns, err := dimensions(parameter.Layout)
	if err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	result.Raw.Z = make([][]int64, rows)
	result.Z = make([][]float64, rows)
	result.Addresses.Z = make([][]uint64, rows)

	dataType, err := resolveDataType(parameter.Layout)
	if err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	if err := supportedStride(parameter.Layout); err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	conversion, err := parseConversion(parameter.Conversion)
	if err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	base, err := d.fileOffset(parameter.Layout)
	if err != nil {
		return nil, definitionError(parameter, "z", err)
	}

	for row := 0; row < rows; row++ {
		result.Raw.Z[row] = make([]int64, columns)
		result.Z[row] = make([]float64, columns)
		result.Addresses.Z[row] = make([]uint64, columns)
		for column := 0; column < columns; column++ {
			index := row*columns + column
			if dataType.columnMajor {
				index = column*rows + row
			}
			offset, offsetErr := elementOffset(base, index, dataType.bits/8)
			if offsetErr != nil {
				return nil, definitionError(parameter, "z", offsetErr)
			}
			raw, readErr := d.readRaw(offset, dataType, parameter.Layout.BitPosition)
			if readErr != nil {
				return nil, definitionError(parameter, "z", readErr)
			}
			value, evaluationErr := conversion.Eval(float64(raw))
			if evaluationErr != nil {
				return nil, definitionError(parameter, "z", evaluationErr)
			}
			result.Raw.Z[row][column] = raw
			result.Z[row][column] = value
			result.Addresses.Z[row][column] = offset
		}
	}
	return result, nil
}

func (d *Decoder) decodeAxis(axis types.MapAxis) ([]int64, []float64, []uint64, error) {
	if axis.Layout == nil {
		return d.decodeFixedAxis(axis)
	}
	dataType, err := resolveDataType(*axis.Layout)
	if err != nil {
		return nil, nil, nil, err
	}
	elementSize, err := axisElementSize(*axis.Layout, dataType.bits)
	if err != nil {
		return nil, nil, nil, err
	}
	conversion, err := parseConversion(axis.Conversion)
	if err != nil {
		return nil, nil, nil, err
	}
	base, err := d.fileOffset(*axis.Layout)
	if err != nil {
		return nil, nil, nil, err
	}

	rawValues := make([]int64, axis.Count)
	values := make([]float64, axis.Count)
	addresses := make([]uint64, axis.Count)
	for index := 0; index < axis.Count; index++ {
		offset, offsetErr := elementOffset(base, index, elementSize)
		if offsetErr != nil {
			return nil, nil, nil, offsetErr
		}
		raw, readErr := d.readRaw(offset, dataType, axis.Layout.BitPosition)
		if readErr != nil {
			return nil, nil, nil, readErr
		}
		value, evaluationErr := conversion.Eval(float64(raw))
		if evaluationErr != nil {
			return nil, nil, nil, evaluationErr
		}
		rawValues[index] = raw
		values[index] = value
		addresses[index] = offset
	}
	return rawValues, values, addresses, nil
}

// decodeFixedAxis decodes an axis with no on-disk storage at all (A2L
// FIX_AXIS): its raw breakpoints are given directly on the mapdef axis,
// precomputed ahead of time from FIX_AXIS_PAR_DIST/FIX_AXIS_PAR_LIST, so
// this only applies the axis's own Conversion to each one — no file read,
// no addresses.
func (d *Decoder) decodeFixedAxis(axis types.MapAxis) ([]int64, []float64, []uint64, error) {
	if len(axis.FixedPoints) == 0 {
		return nil, nil, nil, nil
	}
	conversion, err := parseConversion(axis.Conversion)
	if err != nil {
		return nil, nil, nil, err
	}
	values := make([]float64, len(axis.FixedPoints))
	for i, raw := range axis.FixedPoints {
		value, evalErr := conversion.Eval(float64(raw))
		if evalErr != nil {
			return nil, nil, nil, evalErr
		}
		values[i] = value
	}
	return append([]int64(nil), axis.FixedPoints...), values, nil, nil
}

// axisElementSize resolves an axis's own per-element byte size: normally
// the DataType's natural width, or — for a rescale axis, whose storage
// interleaves an unused "position" element between each "value" element
// this project reads (see docs/a2l-v0-plan.md's RES_AXIS evidence) — an
// explicit StrideBits override, which must be a whole-byte multiple of the
// natural width. StrideBits may carry more than one entry (XDF's
// major/minor pair convention always populates two, typically [0, 0] for
// an axis with no real striding); this only demands that at most one entry
// be non-zero, since combining two non-zero per-dimension strides into one
// flat 1-D axis iteration isn't evidenced. A parameter's own Z layout still
// goes through the stricter supportedStride below, which continues to
// reject any non-zero stride: no evidence yet needs stride support there.
func axisElementSize(layout types.DataLayout, naturalBits int) (int, error) {
	natural := naturalBits / 8
	nonZeroCount, stride := 0, 0
	for _, candidate := range layout.StrideBits {
		if candidate != 0 {
			nonZeroCount++
			stride = candidate
		}
	}
	if nonZeroCount == 0 {
		return natural, nil
	}
	if nonZeroCount > 1 {
		return 0, fmt.Errorf("%w: %v (an axis only supports a single non-zero stride dimension)", ErrUnsupportedStride, layout.StrideBits)
	}
	if stride < naturalBits || stride%8 != 0 {
		return 0, fmt.Errorf("%w: %v", ErrUnsupportedStride, layout.StrideBits)
	}
	return stride / 8, nil
}

func (d *Decoder) readUnsigned(offset uint64, dataType resolvedDataType) (uint64, error) {
	data, err := d.image.Read(offset, dataType.bits/8)
	if err != nil {
		return 0, err
	}
	switch dataType.bits {
	case 8:
		return uint64(data[0]), nil
	case 16:
		if dataType.endian == binfile.LittleEndian {
			return uint64(binary.LittleEndian.Uint16(data)), nil
		}
		return uint64(binary.BigEndian.Uint16(data)), nil
	case 32:
		if dataType.endian == binfile.LittleEndian {
			return uint64(binary.LittleEndian.Uint32(data)), nil
		}
		return uint64(binary.BigEndian.Uint32(data)), nil
	default:
		return 0, ErrUnsupportedDataType
	}
}

func (d *Decoder) readInteger(offset uint64, dataType resolvedDataType) (int64, error) {
	unsigned, err := d.readUnsigned(offset, dataType)
	if err != nil {
		return 0, err
	}
	if !dataType.signed {
		return int64(unsigned), nil
	}
	switch dataType.bits {
	case 8:
		return int64(int8(unsigned)), nil
	case 16:
		return int64(int16(unsigned)), nil
	case 32:
		return int64(int32(unsigned)), nil
	default:
		return 0, ErrUnsupportedDataType
	}
}

// readRaw reads the calibration's raw value: the full word for an ordinary
// parameter, or a single extracted bit (0 or 1) when bitPosition addresses
// one bit within that word, as XDF flag/switch masks do. Sign never applies
// to a bit read; only the underlying bit pattern matters.
func (d *Decoder) readRaw(offset uint64, dataType resolvedDataType, bitPosition *int) (int64, error) {
	if bitPosition == nil {
		return d.readInteger(offset, dataType)
	}
	if *bitPosition < 0 || *bitPosition >= dataType.bits {
		return 0, fmt.Errorf("%w: bit %d in %d-bit storage", ErrInvalidBitPosition, *bitPosition, dataType.bits)
	}
	unsigned, err := d.readUnsigned(offset, dataType)
	if err != nil {
		return 0, err
	}
	return int64((unsigned >> uint(*bitPosition)) & 1), nil
}

func (d *Decoder) fileOffset(layout types.DataLayout) (uint64, error) {
	if layout.Address == nil {
		return 0, ErrMissingAddress
	}
	address := *layout.Address
	base := d.definition.Memory.BaseOffset
	if base >= 0 {
		if address > math.MaxUint64-uint64(base) {
			return 0, ErrAddressOverflow
		}
		return address + uint64(base), nil
	}
	delta := uint64(-(base + 1)) + 1
	if address < delta {
		return 0, ErrAddressOverflow
	}
	return address - delta, nil
}

func supportedStride(layout types.DataLayout) error {
	for _, stride := range layout.StrideBits {
		if stride != 0 {
			return fmt.Errorf("%w: %v", ErrUnsupportedStride, layout.StrideBits)
		}
	}
	return nil
}

func elementOffset(base uint64, index, size int) (uint64, error) {
	if index < 0 || size <= 0 || uint64(index) > (math.MaxUint64-base)/uint64(size) {
		return 0, ErrAddressOverflow
	}
	return base + uint64(index)*uint64(size), nil
}

func dimensions(layout types.DataLayout) (int, int, error) {
	if len(layout.Dimensions) == 0 || len(layout.Dimensions) > 2 {
		return 0, 0, ErrInvalidDimensions
	}
	rows := 1
	columns := layout.Dimensions[0]
	if len(layout.Dimensions) == 2 {
		rows, columns = layout.Dimensions[0], layout.Dimensions[1]
	}
	if rows < 1 || columns < 1 || rows > math.MaxInt/columns {
		return 0, 0, ErrInvalidDimensions
	}
	return rows, columns, nil
}

func parseConversion(conversion types.Conversion) (conversionEvaluator, error) {
	switch conversion.Language {
	case "freehorse-expression-v1":
		return expression.Parse(conversion.Expression)
	case "freehorse-lookup-table-v1":
		return newLookupTable(conversion)
	default:
		return nil, fmt.Errorf("unsupported conversion language %q", conversion.Language)
	}
}

func definitionError(parameter types.MapParameter, axis string, err error) DefinitionError {
	return DefinitionError{Parameter: parameter.Name, Axis: axis, Err: err}
}
