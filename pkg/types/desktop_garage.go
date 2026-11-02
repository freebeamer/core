package types

// GarageSettings intentionally excludes the saved API credential.
type GarageSettings struct {
	URL        string `json:"url"`
	Configured bool   `json:"configured"`
}
