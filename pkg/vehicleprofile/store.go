package vehicleprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/freebeamer/core/pkg/types"
)

const MaxDocumentBytes = 1 << 20
const Format = "freehorse-vehicle-profiles"

type Document struct {
	Format   string                 `json:"format"`
	Version  int                    `json:"version"`
	Profiles []types.VehicleProfile `json:"profiles"`
}

func Read(path string) ([]types.VehicleProfile, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []types.VehicleProfile{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxDocumentBytes {
		return nil, fmt.Errorf("profile file exceeds 1 MiB")
	}
	var doc Document
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("invalid profile file: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("profile file contains trailing data")
	}
	if doc.Format != Format || doc.Version != 1 {
		return nil, fmt.Errorf("unsupported profile format/version")
	}
	if err := validateProfiles(doc.Profiles); err != nil {
		return nil, err
	}
	return doc.Profiles, nil
}
func validateProfiles(profiles []types.VehicleProfile) error {
	if len(profiles) > 128 {
		return fmt.Errorf("at most 128 profiles may be saved")
	}
	ids := map[string]bool{}
	for _, p := range profiles {
		if err := Validate(p); err != nil {
			return err
		}
		if ids[p.ID] {
			return fmt.Errorf("duplicate profile ID")
		}
		ids[p.ID] = true
	}
	return nil
}

// Write atomically replaces only the profile document, never an ECU file.
// Callers serialize read-modify-write operations.
func Write(path string, profiles []types.VehicleProfile) error {
	if err := validateProfiles(profiles); err != nil {
		return err
	}
	data, err := json.MarshalIndent(Document{Format: Format, Version: 1, Profiles: profiles}, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > MaxDocumentBytes {
		return fmt.Errorf("profile file exceeds 1 MiB")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".profiles-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
