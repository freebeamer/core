package types

type IdentificationConfidence uint8

const (
	IdentificationNone IdentificationConfidence = iota
	IdentificationLow
	IdentificationMedium
	IdentificationHigh
)

func (confidence IdentificationConfidence) String() string {
	switch confidence {
	case IdentificationLow:
		return "low"
	case IdentificationMedium:
		return "medium"
	case IdentificationHigh:
		return "high"
	default:
		return "none"
	}
}

type ECUIdentity struct {
	Manufacturer string
	ECUVendor    string
	ECUFamily    string
	Engine       string
	Software     []string
	Confidence   IdentificationConfidence
}
