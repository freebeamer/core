// Package telemetry buffers and uploads pkg/mhd live-monitor samples to
// a FreeBeamer relay API. It exists for one specific, real constraint:
// the device reading live data (a client's phone/tablet on the MHD/ENET
// adapter's own WiFi) frequently can't also be on a WiFi network with
// internet at the same time — most hardware has a single WiFi radio,
// so joining the adapter's AP drops any other WiFi-based internet
// route. Devices with their own independent connection (cellular data,
// wired Ethernet) don't hit this at all; this package doesn't try to
// detect which situation it's in up front — it just attempts live
// delivery and falls back to a local buffer file on any failure, which
// handles both cases (and everything in between, like spotty cellular)
// without platform-specific connectivity APIs.
//
// The client/backend HTTP contract here is provisional: FreeBeamer owns
// both ends, so this defines the initial
// shape rather than conforming to a pre-existing one — see Uploader's
// post method for exactly what's sent.
package telemetry

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/freebeamer/core/pkg/mhd"
)

// Sample is one MHD monitor reading, tagged with when it was taken and
// which device read it, ready to upload or buffer.
type Sample struct {
	Version    int                `json:"version,omitempty"`
	Session    *Session           `json:"session,omitempty"`
	Sequence   *uint64            `json:"sequence,omitempty"`
	ReceivedAt *time.Time         `json:"received_at,omitempty"`
	DeviceID   string             `json:"device_id"`
	Timestamp  time.Time          `json:"timestamp"`
	Values     map[string]float64 `json:"values"`
}

// NewSample builds a Sample from a mhd.Monitor.Read result.
func NewSample(deviceID string, timestamp time.Time, values []mhd.Value) Sample {
	fields := make(map[string]float64, len(values))
	for _, v := range values {
		fields[v.Field.Name] = v.Value
	}
	return Sample{DeviceID: deviceID, Timestamp: timestamp, Values: fields}
}

// Uploader delivers Samples to Endpoint (an HTTP URL) when reachable,
// and otherwise appends them as JSON Lines to BufferPath.
type Uploader struct {
	// Endpoint is the relay API URL to POST samples to. If empty,
	// Send always buffers — useful for a device known to have no
	// direct connectivity of its own.
	Endpoint string
	// APIKey, if set, is sent as "Authorization: Bearer <APIKey>" on
	// every delivery attempt — the client API key a FreeBeamer relay
	// (cmd/freebeamer-relay) requires for POST /v1/telemetry.
	APIKey string
	// BufferPath is where undelivered samples accumulate, one JSON
	// object per line, oldest first.
	BufferPath string
	// HTTPClient is used for delivery; http.DefaultClient if nil.
	HTTPClient *http.Client
	// Timeout bounds each delivery attempt; 5s if zero.
	Timeout time.Duration
}

// Send delivers sample immediately if Endpoint is set, reachable, and
// no backlog remains ahead of it; otherwise sample is appended to
// BufferPath. delivered reports which happened. err is non-nil only
// when even buffering failed (e.g. BufferPath isn't writable) — a live
// delivery failure on its own is the expected, handled case, not an
// error.
//
// Before attempting live delivery, Send always tries FlushBuffer
// first, so a backlog drains in order as soon as connectivity returns
// rather than letting a newer sample overtake older buffered ones.
func (u *Uploader) Send(sample Sample) (delivered bool, err error) {
	if u.Endpoint != "" {
		// Best-effort: bufferEmpty below is what actually decides
		// whether the backlog is clear, not this call's own error.
		_ = u.FlushBuffer()
		if empty, ferr := u.bufferEmpty(); ferr == nil && empty {
			if perr := u.post(sample); perr == nil {
				return true, nil
			}
		}
	}
	return false, u.buffer(sample)
}

// FlushBuffer attempts to upload every buffered sample in order,
// stopping at the first delivery failure so ordering and at-least-once
// delivery are preserved. Everything from that point on stays in
// BufferPath for the next attempt. An unreadable record stops flushing and remains buffered for inspection.
func (u *Uploader) FlushBuffer() error {
	lines, err := u.readBufferLines()
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return nil
	}

	uploaded := 0
	for uploaded < len(lines) {
		var sample Sample
		if err := json.Unmarshal([]byte(lines[uploaded]), &sample); err != nil {
			break // Preserve unreadable records for inspection; never discard identity.
		}
		if err := u.post(sample); err != nil {
			break
		}
		uploaded++
	}

	if err := u.rewriteBuffer(lines[uploaded:]); err != nil {
		return err
	}
	if uploaded < len(lines) {
		return fmt.Errorf("telemetry: %d sample(s) still buffered", len(lines)-uploaded)
	}
	return nil
}

func (u *Uploader) post(sample Sample) error {
	if err := sample.Validate(); err != nil {
		return err
	}
	if sample.Version == ContractVersion {
		version, err := u.Negotiate()
		if err != nil {
			return err
		}
		if version != ContractVersion {
			return fmt.Errorf("telemetry: relay cannot preserve v2 sample identity")
		}
	}
	return u.postValidated(sample)
}

// postValidated does the actual HTTP delivery for an already-validated
// sample, without post's per-call negotiation check. It exists so a caller
// that negotiates and validates once per batch (pkg/telemetry.UploaderService,
// see uploader_service.go) doesn't pay post's per-sample capability-check
// round trip — post itself keeps negotiating on every call, unchanged, since
// Uploader.Send/FlushBuffer's existing contract and tests depend on that.
func (u *Uploader) postValidated(sample Sample) error {
	client := u.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	timeout := u.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	data, err := json.Marshal(sample)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.Endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if u.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+u.APIKey)
	}

	deliveryClient := *client
	deliveryClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := deliveryClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telemetry: upload failed: status %d", resp.StatusCode)
	}
	return nil
}

func (u *Uploader) buffer(sample Sample) error {
	data, err := json.Marshal(sample)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(u.BufferPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (u *Uploader) bufferEmpty() (bool, error) {
	info, err := os.Stat(u.BufferPath)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return info.Size() == 0, nil
}

func (u *Uploader) readBufferLines() ([]string, error) {
	f, err := os.Open(u.BufferPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func (u *Uploader) rewriteBuffer(lines []string) error {
	if len(lines) == 0 {
		err := os.Remove(u.BufferPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var buf bytes.Buffer
	for _, line := range lines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	tmp := u.BufferPath + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, u.BufferPath)
}
