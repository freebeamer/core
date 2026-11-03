package types

// RawTable holds each element's stored value before Conversion is applied:
// the integer for an integer data type, or the IEEE-754 value for float32/
// float64 (mapdef 1.4+). float64 represents every supported integer width
// (up to 32 bits) exactly.
type RawTable struct {
	X []float64
	Y []float64
	Z [][]float64
}

type DecodedTable struct {
	Definition MapParameter
	X          []float64
	Y          []float64
	Z          [][]float64
	Raw        RawTable
	Addresses  AddressTable
}

type AddressTable struct {
	X []uint64
	Y []uint64
	Z [][]uint64
}

type CalibrationEditResult struct {
	Requested float64
	Raw       float64
	Actual    float64
	Offset    uint64
	Bytes     []byte
	Previous  []byte
}
