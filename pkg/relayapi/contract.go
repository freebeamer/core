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

// PairingLinkRequest is POST /v1/pairing-links's request body
// (admin-only): the tuner names the session before generating a
// disposable link for a device to consume.
type PairingLinkRequest struct {
	DisplayName string `json:"display_name"`
}

// PairingLinkResponse is POST /v1/pairing-links's response: PairingToken
// is shown/encoded once and never retrievable again — the relay only
// ever stores its hash. DeepLink is the ready-to-encode
// freebeamer://connect URI carrying it.
type PairingLinkResponse struct {
	PairingToken string    `json:"pairing_token"`
	DeepLink     string    `json:"deep_link"`
	DisplayName  string    `json:"display_name"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// PairRequest is POST /v1/pair's request body: a device redeeming a
// pairing link with the Ed25519 public key half of a keypair it
// generated locally and will hold onto as its durable identity.
type PairRequest struct {
	PairingToken string `json:"pairing_token"`
	// PublicKey is the raw 32-byte Ed25519 public key, base64-encoded.
	PublicKey string `json:"public_key"`
}

// PairResponse is POST /v1/pair's response: the client identity the
// relay created for this device. The pairing link that produced it is
// consumed and cannot be redeemed again.
type PairResponse struct {
	ClientID    string `json:"client_id"`
	DisplayName string `json:"display_name"`
}

// ChallengeRequest is POST /v1/auth/challenge's request body.
type ChallengeRequest struct {
	ClientID string `json:"client_id"`
}

// ChallengeResponse is POST /v1/auth/challenge's response: a nonce the
// device must sign with its private key to prove possession of it.
type ChallengeResponse struct {
	// Nonce is 32 random bytes, base64-encoded.
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionRequest is POST /v1/auth/session's request body: the
// signature over the most recently issued challenge nonce for
// ClientID, proving possession of the paired private key.
type SessionRequest struct {
	ClientID string `json:"client_id"`
	// Signature is the raw 64-byte Ed25519 signature, base64-encoded.
	Signature string `json:"signature"`
}

// SessionResponse is POST /v1/auth/session's response: a short-lived
// bearer token scoped to device-facing endpoints only (POST
// /v1/telemetry, GET /v1/capabilities) — never valid on admin routes
// (/v1/live, /v1/clients, /v1/catalog, /v1/pairing-links), which stay
// gated by the separate, longer-lived admin token.
type SessionResponse struct {
	SessionToken string    `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}
