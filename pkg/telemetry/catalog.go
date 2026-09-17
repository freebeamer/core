package telemetry

import "github.com/freebeamer/core/pkg/mhd"

// Channel separates the source's original unit from its display label. Meaning
// and scaling remain definition-derived until independently vehicle-validated.
type Channel struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Unit       string `json:"unit"`
	SourceUnit string `json:"source_unit"`
	Validation string `json:"validation"`
}

type Catalog struct {
	ID         string    `json:"id"`
	Version    int       `json:"version"`
	Provenance string    `json:"provenance"`
	Channels   []Channel `json:"channels"`
}

func MonitorCatalog() Catalog {
	c := Catalog{ID: MG1CatalogID, Version: MG1CatalogVersion, Provenance: "MHD MG1.adx; see docs/mhd-live-monitor-v0-plan.md"}
	for _, f := range mhd.Fields {
		unit := f.Units
		if f.Name == "RPM" {
			unit = "rpm"
		}
		c.Channels = append(c.Channels, Channel{ID: f.Name, Label: f.Title, Unit: unit, SourceUnit: f.Units, Validation: "definition-derived"})
	}
	return c
}

func KnownChannel(id string) bool {
	for _, f := range mhd.Fields {
		if f.Name == id {
			return true
		}
	}
	return false
}
