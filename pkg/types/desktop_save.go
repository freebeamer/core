package types

type SaveOutcome struct {
	Path            string `json:"path"`
	SHA256          string `json:"sha256"`
	Provider        string `json:"provider"`
	RegionsVerified int    `json:"regionsVerified"`
	WasAlreadyValid bool   `json:"wasAlreadyValid"`
}
