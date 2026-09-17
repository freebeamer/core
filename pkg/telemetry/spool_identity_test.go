package telemetry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDestinationIDStableAndDistinct(t *testing.T) {
	a1 := DestinationID("https://relay.example/v1/telemetry", "key-a")
	a2 := DestinationID("https://relay.example/v1/telemetry", "key-a")
	if a1 != a2 {
		t.Fatalf("DestinationID not stable: %q vs %q", a1, a2)
	}
	b := DestinationID("https://relay.example/v1/telemetry", "key-b")
	if a1 == b {
		t.Fatal("DestinationID did not change with a different API key")
	}
	c := DestinationID("https://other.example/v1/telemetry", "key-a")
	if a1 == c {
		t.Fatal("DestinationID did not change with a different endpoint")
	}
}

func TestResolveSpoolPathFirstRun(t *testing.T) {
	bufferPath := filepath.Join(t.TempDir(), "mhd-buffer.jsonl")
	id := DestinationID("https://relay.example/v1/telemetry", "key-a")

	path, notice, err := ResolveSpoolPath(bufferPath, id)
	if err != nil {
		t.Fatal(err)
	}
	if path != bufferPath {
		t.Fatalf("path = %q, want %q", path, bufferPath)
	}
	if notice != "" {
		t.Fatalf("notice = %q, want empty on a first run", notice)
	}
	if _, err := os.Stat(identitySidecarPath(bufferPath)); err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
}

func TestResolveSpoolPathMatchingDestinationReuses(t *testing.T) {
	bufferPath := filepath.Join(t.TempDir(), "mhd-buffer.jsonl")
	id := DestinationID("https://relay.example/v1/telemetry", "key-a")

	if _, _, err := ResolveSpoolPath(bufferPath, id); err != nil {
		t.Fatal(err)
	}
	spool := &Spool{Path: bufferPath}
	if err := spool.Append(NewSample("a", fixedTime(0), nil)); err != nil {
		t.Fatal(err)
	}

	path, notice, err := ResolveSpoolPath(bufferPath, id)
	if err != nil {
		t.Fatal(err)
	}
	if path != bufferPath || notice != "" {
		t.Fatalf("path=%q notice=%q, want %q and empty", path, notice, bufferPath)
	}
	pending, err := (&Spool{Path: path}).Pending()
	if err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("Pending = %d, want 1 (existing content preserved for a matching destination)", pending)
	}
}

func TestResolveSpoolPathQuarantinesUnknownProvenanceFile(t *testing.T) {
	bufferPath := filepath.Join(t.TempDir(), "mhd-buffer.jsonl")
	// Simulate a pre-Phase-3 buffer file with no identity sidecar.
	if err := os.WriteFile(bufferPath, []byte(`{"device_id":"old"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id := DestinationID("https://relay.example/v1/telemetry", "key-a")

	path, notice, err := ResolveSpoolPath(bufferPath, id)
	if err != nil {
		t.Fatal(err)
	}
	if path != bufferPath {
		t.Fatalf("path = %q, want %q (fresh file at the same path)", path, bufferPath)
	}
	if notice == "" {
		t.Fatal("notice = empty, want a quarantine explanation")
	}
	quarantine := bufferPath + ".legacy-unknown-destination"
	data, err := os.ReadFile(quarantine)
	if err != nil {
		t.Fatalf("quarantined file missing: %v", err)
	}
	if string(data) != `{"device_id":"old"}`+"\n" {
		t.Fatalf("quarantined content changed: %q", data)
	}
	if info, err := os.Stat(bufferPath); err == nil && info.Size() > 0 {
		t.Fatal("bufferPath should be empty/fresh after quarantining the old content")
	}
}

func TestResolveSpoolPathRedirectsOnMismatch(t *testing.T) {
	bufferPath := filepath.Join(t.TempDir(), "mhd-buffer.jsonl")
	oldID := DestinationID("https://relay.example/v1/telemetry", "old-key")
	newID := DestinationID("https://relay.example/v1/telemetry", "new-key")

	if _, _, err := ResolveSpoolPath(bufferPath, oldID); err != nil {
		t.Fatal(err)
	}
	if err := (&Spool{Path: bufferPath}).Append(NewSample("old-client", fixedTime(0), nil)); err != nil {
		t.Fatal(err)
	}

	path, notice, err := ResolveSpoolPath(bufferPath, newID)
	if err != nil {
		t.Fatal(err)
	}
	if path == bufferPath {
		t.Fatal("path should differ from bufferPath on a destination mismatch")
	}
	if notice == "" {
		t.Fatal("notice = empty, want a redirection explanation")
	}

	oldPending, err := (&Spool{Path: bufferPath}).Pending()
	if err != nil {
		t.Fatal(err)
	}
	if oldPending != 1 {
		t.Fatalf("original buffer was modified: pending = %d, want 1", oldPending)
	}

	newPending, err := (&Spool{Path: path}).Pending()
	if err != nil {
		t.Fatal(err)
	}
	if newPending != 0 {
		t.Fatalf("new destination's spool should start empty, pending = %d", newPending)
	}

	// Resolving again for the new destination must be stable (idempotent):
	// same alt path every time, never a second, different sibling path.
	// bufferPath's own sidecar permanently names the old destination (it is
	// never touched), so the mismatch notice is expected again too — the
	// caller decides whether to print it on every run.
	again, notice2, err := ResolveSpoolPath(bufferPath, newID)
	if err != nil {
		t.Fatal(err)
	}
	if again != path {
		t.Fatalf("second resolve for the same new destination = %q, want %q", again, path)
	}
	if notice2 == "" {
		t.Fatal("second resolve notice = empty, want the redirection explanation again")
	}
}
