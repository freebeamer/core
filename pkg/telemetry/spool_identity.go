package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DestinationID returns a short, stable fingerprint of a relay endpoint +
// API key pair. It identifies which destination a spool file was written
// for, without persisting the raw endpoint/key anywhere on disk — see
// ResolveSpoolPath. The scheme mirrors internal/relay's own existing
// practice of storing only a SHA-256 hash of a client's API key, never the
// key itself.
func DestinationID(endpoint, apiKey string) string {
	sum := sha256.Sum256([]byte(endpoint + "\x00" + apiKey))
	return hex.EncodeToString(sum[:])[:16]
}

type spoolIdentity struct {
	DestinationID string `json:"destination_id"`
}

func identitySidecarPath(bufferPath string) string { return bufferPath + ".identity.json" }

func readSpoolIdentity(path string) (spoolIdentity, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return spoolIdentity{}, false, nil
	}
	if err != nil {
		return spoolIdentity{}, false, err
	}
	var id spoolIdentity
	if err := json.Unmarshal(data, &id); err != nil {
		return spoolIdentity{}, false, fmt.Errorf("telemetry: spool identity sidecar %q: %w", path, err)
	}
	return id, true, nil
}

func writeSpoolIdentity(path string, id spoolIdentity) error {
	data, err := json.Marshal(id)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// destinationSpoolPath returns a sibling of bufferPath scoped to
// destinationID, e.g. "mhd-buffer.jsonl" -> "mhd-buffer.<id>.jsonl".
func destinationSpoolPath(bufferPath, destinationID string) string {
	dir := filepath.Dir(bufferPath)
	base := filepath.Base(bufferPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", stem, destinationID, ext))
}

// ResolveSpoolPath ensures bufferPath is safe to use as a Spool.Path for
// destinationID (a fingerprint from DestinationID), so that a device
// reprovisioned to a new relay/client destination can never have its old
// buffered samples flushed under the new destination's credentials — see
// docs/live-data-and-vehicle-mapping-plan.md's Phase 3 "isolate spool data
// by relay/client destination" requirement.
//
//   - No sidecar exists and bufferPath is empty or absent, or the sidecar's
//     destination matches: bufferPath is used as-is (writing/refreshing the
//     sidecar to record the current destination).
//   - No sidecar exists but bufferPath already has content (a pre-Phase-3
//     file, or one from an even older run — its destination is not
//     recoverable): that file is quarantined once, by renaming it to
//     bufferPath+".legacy-unknown-destination" (never deleted), and a fresh
//     file is started at bufferPath bound to the current destination.
//   - A sidecar exists and names a different destination (a real
//     reprovisioning): the mismatched file and its sidecar are left
//     untouched, and a destination-specific sibling path is used instead.
//
// The returned notice is a human-readable, one-time explanation of
// whichever of the last two cases happened, or "" if neither did — callers
// should print it once, not on every subsequent run.
func ResolveSpoolPath(bufferPath, destinationID string) (path string, notice string, err error) {
	sidecarPath := identitySidecarPath(bufferPath)
	existing, hasSidecar, err := readSpoolIdentity(sidecarPath)
	if err != nil {
		return "", "", err
	}

	if hasSidecar {
		if existing.DestinationID == destinationID {
			return bufferPath, "", nil
		}
		altPath := destinationSpoolPath(bufferPath, destinationID)
		if err := writeSpoolIdentity(identitySidecarPath(altPath), spoolIdentity{DestinationID: destinationID}); err != nil {
			return "", "", err
		}
		notice = fmt.Sprintf("buffer file %q belongs to a different relay/client destination; using %q instead (neither file is deleted)", bufferPath, altPath)
		return altPath, notice, nil
	}

	info, statErr := os.Stat(bufferPath)
	switch {
	case statErr == nil && info.Size() > 0:
		quarantine := bufferPath + ".legacy-unknown-destination"
		if err := os.Rename(bufferPath, quarantine); err != nil {
			return "", "", err
		}
		notice = fmt.Sprintf("existing buffer file %q has unknown destination provenance (predates destination tracking); preserved as %q, starting a new buffer", bufferPath, quarantine)
	case statErr != nil && !errors.Is(statErr, os.ErrNotExist):
		return "", "", statErr
	}

	if err := writeSpoolIdentity(sidecarPath, spoolIdentity{DestinationID: destinationID}); err != nil {
		return "", "", err
	}
	return bufferPath, notice, nil
}
