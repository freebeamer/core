// Package relayapi defines the v1 DTOs the FreeBeamer relay service
// (cmd/freebeamer-relay, internal/relay) exposes to its desktop-facing
// consumers — the ones that don't already exist elsewhere. The
// client-facing ingest shape is deliberately NOT redefined here:
// POST /v1/telemetry's request body is pkg/telemetry.Sample as-is, the
// same JSON shape pkg/telemetry.Uploader already posts. This package
// only holds what's new: the live feed envelope and the admin client
// listing.
//
// See docs/mhd-live-monitor-v0-plan.md's "Telemetry relay" section and
// docs/freebeamer-relay-v0-plan.md for the full design this implements.
package relayapi

import "time"
import "github.com/freebeamer/core/pkg/telemetry"

// LiveEnvelope is one message sent over GET /v1/live's WebSocket
// connection: either a live sample as it arrives, or part of the
// short backfill burst sent right after connecting.
type LiveEnvelope struct {
	Version    int                `json:"version,omitempty"`
	Session    *telemetry.Session `json:"session,omitempty"`
	Sequence   *uint64            `json:"sequence,omitempty"`
	ReceivedAt *time.Time         `json:"received_at,omitempty"`
	DeviceID   string             `json:"device_id"`
	Timestamp  time.Time          `json:"timestamp"`
	Values     map[string]float64 `json:"values"`
	// Backfill is true for samples sent as part of the connect-time
	// backfill burst, false for samples pushed live afterward — lets
	// the desktop app distinguish "catching up" from "happening now"
	// without guessing from timestamps.
	Backfill bool `json:"backfill"`
}

// ClientSummary is one entry in GET /v1/clients's response: enough for
// the desktop app's client picker, nothing more.
type ClientSummary struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// ErrorResponse is the JSON body every non-2xx response from the relay
// API uses.
type ErrorResponse struct {
	Error string `json:"error"`
}
