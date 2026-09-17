package types

type ChecksumDetectionConfidence uint8

const (
	ChecksumDetectionNone ChecksumDetectionConfidence = iota
	ChecksumDetectionLow
	ChecksumDetectionMedium
	ChecksumDetectionHigh
)

type ChecksumDetection struct {
	Confidence ChecksumDetectionConfidence
	Reason     string
}

type ChecksumResult struct {
	Name          string
	RegionStart   uint64
	RegionEnd     uint64
	StorageOffset uint64
	Stored        uint64
	Calculated    uint64
	Valid         bool
	Corrected     bool
}
