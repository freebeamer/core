package types

type RawTable struct {
	X []int64
	Y []int64
	Z [][]int64
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
	Raw       int64
	Actual    float64
	Offset    uint64
	Bytes     []byte
	Previous  []byte
}
