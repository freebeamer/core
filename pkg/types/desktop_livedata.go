package types

import "time"

// LiveSample is one interpreted telemetry reading, ready for direct display
// or charting. Unlike pkg/relayapi.LiveEnvelope (the wire shape from the
// relay), GapBefore is already computed and ordering/dedup already applied —
// the Go desktop layer does that interpretation once, so Angular never has
// to re-derive it from raw sequence numbers. See
// docs/desktop-v1-plan.md's "Angular receives presentation DTOs, never Go
// domain objects."
//
// A legacy (pre-v2) sample — no session, no sequence — is represented with
// SessionID "" and Sequence 0; GapBefore is always 0 for it, since gap
// detection requires a sequence number this sample doesn't have.
type LiveSample struct {
	DeviceID  string             `json:"deviceId"`
	SessionID string             `json:"sessionId"`
	Sequence  uint64             `json:"sequence"`
	Timestamp time.Time          `json:"timestamp"` // acquisition time, never relay receipt time
	Values    map[string]float64 `json:"values"`
	Backfill  bool               `json:"backfill"`
	GapBefore uint64             `json:"gapBefore"` // count of missing sequence numbers immediately before this sample, within its own session; 0 if none or unknown
}

// LiveSource is one (device, session) pair discovered on the current live
// connection. The set of known sources only ever grows during a connection
// — the relay has no "session ended" signal, so this reflects sources seen
// so far, not sources currently active.
type LiveSource struct {
	BindingRevision string `json:"bindingRevision"` // changes with session metadata or observed channel set
	DeviceID        string `json:"deviceId"`
	SessionID       string `json:"sessionId"`
}

// LiveBatch is one coalesced group of samples, emitted on a fixed timer
// rather than per-sample — see internal/desktop/livedata.go's batching
// policy. Generation lets a consumer discard a batch that arrived after a
// scope change (client/device/session selection) superseded it.
type LiveBatch struct {
	Generation uint64       `json:"generation"`
	Samples    []LiveSample `json:"samples"` // already deduped, ordered oldest-first
	Sources    []LiveSource `json:"sources"` // full current discovered-source list; never filtered by selection
}

// LiveConnectionStatus reports the live relay connection's current state.
// Never carries the admin token or any other credential.
type LiveConnectionStatus struct {
	Generation uint64 `json:"generation"` // display scope, shared with LiveBatch
	State      string `json:"state"`      // "disconnected" | "connecting" | "connected" | "reconnecting" | "backingOff" | "error"
	// Message is drawn from a fixed set of known categories (e.g. "invalid
	// admin token"), never a raw error string — see livedata.go's
	// credential-handling discipline.
	Message string `json:"message,omitempty"`
	// Retryable is false for a terminal failure (e.g. a rejected admin
	// token) that reconnect/backoff will not resolve on its own.
	Retryable bool `json:"retryable"`
	// BackoffUntil is set only in the "backingOff" state.
	BackoffUntil *time.Time `json:"backoffUntil,omitempty"`
	// CatchingUp is true from Connect until the connect-time backfill
	// burst finishes (or immediately false if it was empty).
	CatchingUp bool `json:"catchingUp"`
}

// LiveClientSummary is one relay client, for the desktop app's client
// picker — the camelCase presentation form of pkg/relayapi.ClientSummary.
type LiveClientSummary struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

// LiveChannel is one telemetry channel's display metadata — the camelCase
// presentation form of pkg/telemetry.Channel.
type LiveChannel struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Unit       string `json:"unit"`
	SourceUnit string `json:"sourceUnit"`
	Validation string `json:"validation"`
}

// LiveCatalog is the channel catalog the connected relay advertises — the
// camelCase presentation form of pkg/telemetry.Catalog.
type LiveCatalog struct {
	ID         string        `json:"id"`
	Version    int           `json:"version"`
	Provenance string        `json:"provenance"`
	Channels   []LiveChannel `json:"channels"`
}

// LivePairingLink is a freshly minted, single-use device pairing link —
// the camelCase presentation form of pkg/relayapi.PairingLinkResponse.
// Unlike LiveConnectionStatus, this DOES carry a credential
// (PairingToken) deliberately: it's meant to be shown to the operator
// once, to hand to a device, and discarded — never persisted or logged
// by the desktop app.
type LivePairingLink struct {
	PairingToken string    `json:"pairingToken"`
	DeepLink     string    `json:"deepLink"`
	DisplayName  string    `json:"displayName"`
	ExpiresAt    time.Time `json:"expiresAt"`
}
