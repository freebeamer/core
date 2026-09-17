package types

type HealthStatus struct {
	Ready      bool   `json:"ready"`
	APIVersion string `json:"apiVersion"`
	Engine     string `json:"engine"`
}

type WorkspaceSummary struct {
	Generation     uint64               `json:"generation"`
	XDFPath        string               `json:"xdfPath"`
	BinPath        string               `json:"binPath"`
	ROM            RomSummary           `json:"rom"`
	ECU            ECUSummary           `json:"ecu"`
	XDF            XDFSummary           `json:"xdf"`
	Compatibility  DesktopCompatibility `json:"compatibility"`
	Checksum       ChecksumSummary      `json:"checksum"`
	EditingAllowed bool                 `json:"editingAllowed"`
	RefusalReason  string               `json:"refusalReason,omitempty"`
	ChangedBytes   int                  `json:"changedBytes"`
	EditCount      int                  `json:"editCount"`
	CanUndo        bool                 `json:"canUndo"`
	CanRedo        bool                 `json:"canRedo"`
}

type RomSummary struct {
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

type ECUSummary struct {
	Manufacturer string `json:"manufacturer"`
	Vendor       string `json:"vendor"`
	Family       string `json:"family"`
	Engine       string `json:"engine"`
	Confidence   string `json:"confidence"`
}

type XDFSummary struct {
	Version    string `json:"version"`
	Categories int    `json:"categories"`
	Tables     int    `json:"tables"`
	Flags      int    `json:"flags"`
}

type DesktopCompatibility struct {
	RegionFitsROM       bool `json:"regionFitsRom"`
	AddressedMaps       int  `json:"addressedMaps"`
	TotalMaps           int  `json:"totalMaps"`
	MapValidationErrors int  `json:"mapValidationErrors"`
}

type ChecksumSummary struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Regions  int    `json:"regions"`
	Message  string `json:"message,omitempty"`
}

type RecentEntry struct {
	XDFPath string `json:"xdfPath"`
	BinPath string `json:"binPath"`
}
