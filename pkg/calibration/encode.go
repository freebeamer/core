package calibration

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/expression"
	"github.com/freebeamer/core/pkg/types"
)

var (
	ErrCellOutOfRange = errors.New("calibration cell index out of range")
	ErrRawOutOfRange  = errors.New("raw value does not fit mapdef data type")
)

func (d *Decoder) SetCell(parameter types.MapParameter, row, column int, requested float64) (*types.CalibrationEditResult, error) {
	rows, columns, err := dimensions(parameter.Layout)
	if err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	if row < 0 || row >= rows || column < 0 || column >= columns {
		return nil, fmt.Errorf("%w: row %d column %d for %dx%d map", ErrCellOutOfRange, row, column, rows, columns)
	}
	return d.setCell(parameter, row, column, requested)
}

func (d *Decoder) SetIndex(parameter types.MapParameter, index int, requested float64) (*types.CalibrationEditResult, error) {
	rows, columns, err := dimensions(parameter.Layout)
	if err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	if index < 0 || index >= rows*columns {
		return nil, fmt.Errorf("%w: index %d for %d-cell map", ErrCellOutOfRange, index, rows*columns)
	}
	return d.setCell(parameter, index/columns, index%columns, requested)
}

func (d *Decoder) setCell(parameter types.MapParameter, row, column int, requested float64) (*types.CalibrationEditResult, error) {
	if math.IsNaN(requested) || math.IsInf(requested, 0) {
		return nil, expression.ErrNonFinite
	}
	if err := supportedStride(parameter.Layout); err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	dataType, err := resolveDataType(parameter.Layout)
	if err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	conversion, err := parseConversion(parameter.Conversion)
	if err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	rawFloat, err := conversion.Invert(requested)
	if err != nil {
		return nil, definitionError(parameter, "z", err)
	}
	if math.IsNaN(rawFloat) || math.IsInf(rawFloat, 0) || rawFloat > math.MaxInt64 || rawFloat < math.MinInt64 {
		return nil, ErrRawOutOfRange
	}
	raw := int64(math.Round(rawFloat))
	if err := checkRange(raw, dataType, parameter.Layout.BitPosition); err != nil {
		return nil, fmt.Errorf("value %g: %w", requested, err)
	}
	rows, columns, _ := dimensions(parameter.Layout)
	index := row*columns + column
	if dataType.columnMajor {
		index = column*rows + row
	}
	base, err := d.fileOffset(parameter.Layout)
	if err != nil {
		return nil, err
	}
	offset, err := elementOffset(base, index, dataType.bits/8)
	if err != nil {
		return nil, err
	}
	encoded, previous, err := d.writeRaw(offset, dataType, parameter.Layout.BitPosition, raw)
	if err != nil {
		return nil, err
	}
	decoded, err := d.DecodeParameter(parameter)
	if err != nil {
		_ = d.image.Write(offset, previous)
		return nil, fmt.Errorf("verify edited map: %w", err)
	}
	actual := decoded.Z[row][column]
	return &types.CalibrationEditResult{
		Requested: requested,
		Raw:       raw,
		Actual:    actual,
		Offset:    offset,
		Bytes:     append([]byte(nil), encoded...),
		Previous:  previous,
	}, nil
}

func checkRange(value int64, dataType resolvedDataType, bitPosition *int) error {
	if bitPosition != nil {
		if value != 0 && value != 1 {
			return fmt.Errorf("%w: %d cannot fit a single bit (0 or 1)", ErrRawOutOfRange, value)
		}
		return nil
	}
	if dataType.signed {
		switch dataType.bits {
		case 8:
			if value < math.MinInt8 || value > math.MaxInt8 {
				return fmt.Errorf("%w: %d cannot fit signed 8-bit", ErrRawOutOfRange, value)
			}
		case 16:
			if value < math.MinInt16 || value > math.MaxInt16 {
				return fmt.Errorf("%w: %d cannot fit signed 16-bit", ErrRawOutOfRange, value)
			}
		case 32:
			if value < math.MinInt32 || value > math.MaxInt32 {
				return fmt.Errorf("%w: %d cannot fit signed 32-bit", ErrRawOutOfRange, value)
			}
		}
		return nil
	}
	if value < 0 {
		return fmt.Errorf("%w: %d cannot fit unsigned %d-bit", ErrRawOutOfRange, value, dataType.bits)
	}
	var maximum int64
	switch dataType.bits {
	case 8:
		maximum = math.MaxUint8
	case 16:
		maximum = math.MaxUint16
	case 32:
		maximum = math.MaxUint32
	}
	if value > maximum {
		return fmt.Errorf("%w: %d cannot fit unsigned %d-bit", ErrRawOutOfRange, value, dataType.bits)
	}
	return nil
}

// writeRaw commits a raw value at offset: a full-word write for an ordinary
// parameter, or a read-modify-write of one bit when bitPosition addresses a
// single bit within that word, so sibling bits belonging to other flags
// packed into the same byte are preserved untouched. It returns the bytes
// written and the bytes they replaced, for diffing and rollback.
func (d *Decoder) writeRaw(offset uint64, dataType resolvedDataType, bitPosition *int, raw int64) (encoded, previous []byte, err error) {
	if bitPosition == nil {
		encoded = encodeInteger(raw, dataType)
		previous, err = d.image.Read(offset, len(encoded))
		if err != nil {
			return nil, nil, err
		}
		if err = d.image.Write(offset, encoded); err != nil {
			return nil, nil, err
		}
		return encoded, previous, nil
	}
	if *bitPosition < 0 || *bitPosition >= dataType.bits {
		return nil, nil, fmt.Errorf("%w: bit %d in %d-bit storage", ErrInvalidBitPosition, *bitPosition, dataType.bits)
	}
	unsigned, err := d.readUnsigned(offset, dataType)
	if err != nil {
		return nil, nil, err
	}
	previous = encodeInteger(int64(unsigned), dataType)
	mask := uint64(1) << uint(*bitPosition)
	updated := unsigned &^ mask
	if raw != 0 {
		updated |= mask
	}
	encoded = encodeInteger(int64(updated), dataType)
	if err = d.image.Write(offset, encoded); err != nil {
		return nil, nil, err
	}
	return encoded, previous, nil
}

func encodeInteger(value int64, dataType resolvedDataType) []byte {
	result := make([]byte, dataType.bits/8)
	switch dataType.bits {
	case 8:
		result[0] = byte(value)
	case 16:
		if dataType.endian == binfile.LittleEndian {
			binary.LittleEndian.PutUint16(result, uint16(value))
		} else {
			binary.BigEndian.PutUint16(result, uint16(value))
		}
	case 32:
		if dataType.endian == binfile.LittleEndian {
			binary.LittleEndian.PutUint32(result, uint32(value))
		} else {
			binary.BigEndian.PutUint32(result, uint32(value))
		}
	}
	return result
}
