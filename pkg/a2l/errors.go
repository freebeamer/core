package a2l

import "errors"

var (
	ErrUnsupportedCharacteristicType = errors.New("a2l: unsupported characteristic type")
	ErrRecordLayoutNotFound          = errors.New("a2l: record layout not found")
	ErrUnsupportedRecordLayout       = errors.New("a2l: unsupported record layout")
	ErrCompuMethodNotFound           = errors.New("a2l: compu method not found")
	ErrUnsupportedCompuMethod        = errors.New("a2l: unsupported compu method")
	ErrUnsupportedAxisDescr          = errors.New("a2l: unsupported axis descriptor")
	ErrAxisPtsNotFound               = errors.New("a2l: axis points not found")
	ErrCompuVtabNotFound             = errors.New("a2l: compu vtab not found")
	ErrCompuVtabRangeUnsupported     = errors.New("a2l: compu vtab range not supported (range-keyed text lookup has no mapdef representation yet)")
	ErrCurveAxisNotFound             = errors.New("a2l: curve axis reference not found")
	ErrRescalePairCountMismatch      = errors.New("a2l: rescale pair count does not match axis point count")
	ErrCompuTabNotFound              = errors.New("a2l: compu tab not found")
)
