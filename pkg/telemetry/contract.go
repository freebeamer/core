package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

const ContractVersion = 2
const MG1CatalogID = "mhd.mg1.monitor"
const MG1CatalogVersion = 1
const MaxSequence uint64 = 9007199254740991 // exactly representable by JavaScript

// Session metadata is immutable within a client/session ID. Vehicle is an
// operator assertion, never firmware-match or hardware-validation evidence.
type Session struct {
	ID             string        `json:"id"`
	CatalogID      string        `json:"catalog_id"`
	CatalogVersion int           `json:"catalog_version"`
	Vehicle        *VehicleClaim `json:"vehicle,omitempty"`
}

type VehicleClaim struct {
	Label     string `json:"label"`
	ECUFamily string `json:"ecu_family,omitempty"`
	Software  string `json:"software,omitempty"`
}

func NewSession() (Session, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Session{}, err
	}
	return Session{ID: hex.EncodeToString(id[:]), CatalogID: MG1CatalogID, CatalogVersion: MG1CatalogVersion}, nil
}

// Validate accepts legacy samples without inventing identity. V2 samples must
// carry the complete identity needed for deduplication and channel interpretation.
func (s Sample) Validate() error {
	if s.Version == 0 || s.Version == 1 {
		if s.Session != nil || s.Sequence != nil {
			return fmt.Errorf("legacy samples cannot carry v2 identity")
		}
		return nil
	}
	if s.Version != ContractVersion {
		return fmt.Errorf("unsupported telemetry version %d", s.Version)
	}
	if strings.TrimSpace(s.DeviceID) == "" || len(s.DeviceID) > 256 || s.Timestamp.IsZero() {
		return fmt.Errorf("v2 requires device_id and timestamp")
	}
	if s.Session == nil || strings.TrimSpace(s.Session.ID) == "" || len(s.Session.ID) > 128 {
		return fmt.Errorf("v2 requires a session ID of at most 128 bytes")
	}
	if s.Session.CatalogID != MG1CatalogID || s.Session.CatalogVersion != MG1CatalogVersion {
		return fmt.Errorf("unsupported channel catalog")
	}
	if s.Sequence == nil || *s.Sequence > MaxSequence {
		return fmt.Errorf("v2 requires sequence in 0..%d", MaxSequence)
	}
	if s.Session.Vehicle != nil {
		v := s.Session.Vehicle
		if strings.TrimSpace(v.Label) == "" || len(v.Label) > 256 || len(v.ECUFamily) > 128 || len(v.Software) > 128 {
			return fmt.Errorf("invalid vehicle claim")
		}
	}
	if len(s.Values) == 0 {
		return fmt.Errorf("v2 requires channel values")
	}
	for id, value := range s.Values {
		if !KnownChannel(id) || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("invalid channel value %q", id)
		}
	}
	return nil
}
