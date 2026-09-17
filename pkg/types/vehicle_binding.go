package types

// BindingWorkspace is immutable for the lifetime of an opened workspace.
type BindingWorkspace struct {
	DefinitionID     string `json:"definitionId"`
	DefinitionSHA256 string `json:"definitionSha256"`
	OriginalSHA256   string `json:"originalSha256"`
	OriginalSize     uint64 `json:"originalSize"`
}
type BindingCatalog struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}
type VehicleProfile struct {
	ID                  string           `json:"id"`
	Label               string           `json:"label"`
	ECUFamily           string           `json:"ecuFamily"`
	Software            string           `json:"software"`
	TelemetryEvidence   string           `json:"telemetryEvidence"`
	CalibrationEvidence string           `json:"calibrationEvidence"`
	Workspace           BindingWorkspace `json:"workspace"`
	Catalogs            []BindingCatalog `json:"catalogs"`
	Bindings            []AxisBinding    `json:"bindings"`
}
type AxisBinding struct {
	ParameterID    string  `json:"parameterId"`
	Axis           string  `json:"axis"`
	ChannelID      string  `json:"channelId"`
	ChannelMeaning string  `json:"channelMeaning"`
	AxisMeaning    string  `json:"axisMeaning"`
	SourceUnit     string  `json:"sourceUnit"`
	TargetUnit     string  `json:"targetUnit"`
	Scale          float64 `json:"scale"`
	Offset         float64 `json:"offset"`
}
type BindingSession struct {
	ScopeSHA256    string   `json:"scopeSha256"`
	DeviceID       string   `json:"deviceId"`
	SessionID      string   `json:"sessionId"`
	CatalogID      string   `json:"catalogId"`
	CatalogVersion int      `json:"catalogVersion"`
	VehicleLabel   string   `json:"vehicleLabel"`
	ECUFamily      string   `json:"ecuFamily"`
	Software       string   `json:"software"`
	Channels       []string `json:"channels"`
}
type BindingCompatibility struct {
	State    string   `json:"state"` // unknown | incompatible | manual | validated
	Reasons  []string `json:"reasons"`
	Verified bool     `json:"verified"`
}
type BindingAxisOption struct {
	ParameterID string `json:"parameterId"`
	Title       string `json:"title"`
	Axis        string `json:"axis"`
	Unit        string `json:"unit"`
	Count       int    `json:"count"`
}
type BindingContext struct {
	SessionFingerprint  string               `json:"sessionFingerprint"`
	WorkspaceGeneration uint64               `json:"workspaceGeneration"`
	LiveGeneration      uint64               `json:"liveGeneration"`
	Workspace           BindingWorkspace     `json:"workspace"`
	Session             BindingSession       `json:"session"`
	Axes                []BindingAxisOption  `json:"axes"`
	Profiles            []VehicleProfile     `json:"profiles"`
	ActiveProfileID     string               `json:"activeProfileId"`
	Compatibility       BindingCompatibility `json:"compatibility"`
}
type BindingRequest struct {
	SessionFingerprint  string         `json:"sessionFingerprint"`
	WorkspaceGeneration uint64         `json:"workspaceGeneration"`
	LiveGeneration      uint64         `json:"liveGeneration"`
	DeviceID            string         `json:"deviceId"`
	SessionID           string         `json:"sessionId"`
	Profile             VehicleProfile `json:"profile"`
}

type BindingConversion struct {
	Scale  float64 `json:"scale"`
	Offset float64 `json:"offset"`
}
