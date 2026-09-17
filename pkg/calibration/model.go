// Package calibration decodes and edits map definitions against binary images.
package calibration

import (
	"errors"
	"fmt"

	"github.com/freebeamer/core/pkg/binfile"
	"github.com/freebeamer/core/pkg/types"
)

var (
	ErrUnsupportedDataType  = errors.New("unsupported mapdef data type")
	ErrUnsupportedByteOrder = errors.New("unsupported mapdef byte order")
	ErrUnsupportedStride    = errors.New("non-zero mapdef strides are unsupported")
	ErrMissingAddress       = errors.New("mapdef layout has no address")
	ErrInvalidDimensions    = errors.New("invalid mapdef dimensions")
	ErrAddressOverflow      = errors.New("mapdef address calculation overflow")
	ErrInvalidBitPosition   = errors.New("mapdef bit position exceeds storage width")
)

type resolvedDataType struct {
	bits        int
	signed      bool
	endian      binfile.Endian
	columnMajor bool
}

type Decoder struct {
	definition *types.MapDefinition
	image      *binfile.Image
}

func New(definition *types.MapDefinition, image *binfile.Image) (*Decoder, error) {
	if definition == nil {
		return nil, errors.New("calibration: nil map definition")
	}
	if image == nil {
		return nil, errors.New("calibration: nil binary image")
	}
	return &Decoder{definition: definition, image: image}, nil
}

type DefinitionError struct {
	Parameter string
	Axis      string
	Err       error
}

func (e DefinitionError) Error() string {
	return fmt.Sprintf("map %q axis %s: %v", e.Parameter, e.Axis, e.Err)
}

func (e DefinitionError) Unwrap() error {
	return e.Err
}
