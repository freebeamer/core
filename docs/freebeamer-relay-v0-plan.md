# FreeBeamer relay — V0 plan

Date: 2026-09-15

Status: `cmd/freebeamer-relay`, `internal/relay`, and `pkg/relayapi` are
implemented and tested (unit tests plus a real end-to-end smoke test — see
"Testing" below), not yet deployed anywhere. The Flutter mobile app that
will be the relay's real-world client is a separate, not-yet-built piece —
see "Non-goals" below.

## Why this exists

`docs/mhd-live-monitor-v0-plan.md` built `pkg/doip`/`pkg/mhd` (the DoIP/UDS
transport and MHD live-monitor decode) and `pkg/telemetry` (a client-side
`Uploader` that posts JSON samples to a configurable endpoint, buffering
locally when it's unreachable). Both were evidenced and tested against mock
servers, but nothing existed to actually receive an upload — `freebeamer mhd
monitor` was always a stand-in for the real client (a mobile app on a
client's own phone), not the final answer.

This surfaced a real product requirement during scoping: FreeBeamer's live
data path is client-facing — a client drives their own car with their own
phone connected to their own MHD/ENET adapter, and the project owner needs
to see that data on their desktop app for remote tuning. This relay is the
server sitting between those two: client devices POST samples to it, the
desktop app watches a client's feed over a WebSocket.

Four architecture decisions were made deliberately for this v0 slice (by
the project owner) and are reflected directly in the implementation below,
not re-litigated here: **self-hosted deployment** (their own server, not a
managed cloud platform), **minimal auth** (a per-client API key plus one
static admin token, no user accounts), **SQLite over Postgres** (a short
rolling buffer for live-feed backfill, not durable analytics storage), and
**WebSocket over SSE/polling** for desktop delivery.

## What's implemented

```
cmd/freebeamer-relay/
    main.go, command_serve.go, command_client.go
internal/relay/
    server.go, auth.go, hub.go, respond.go
    telemetry_handler.go, live_handler.go, clients_handler.go, healthz.go
    store.go (interface), store_sqlite.go
    migrations/0001_init.sql (go:embed)
pkg/relayapi/
    contract.go — LiveEnvelope, ClientSummary, ErrorResponse
```

### API (v1)

| Method/Path | Auth | Purpose |
|---|---|---|
| `POST /v1/telemetry` | client API key (`Authorization: Bearer <key>`) | Ingest one `telemetry.Sample` (the same JSON shape `pkg/telemetry.Uploader` already posts — not redefined here); persists it and fans it out to any live WS subscriber for that client. `202 Accepted`. |
| `GET /v1/live?client_id=ID` | admin token | WebSocket. Sends a short backfill burst (`relayapi.LiveEnvelope{Backfill: true}`, most-recent `backfillLimit` samples, oldest first) of recently stored samples, then live pushes (`Backfill: false`) as new samples arrive. |
| `GET /v1/clients` | admin token | Lists active (non-revoked) clients as `[]relayapi.ClientSummary`, for the desktop's feed picker. |
| `GET /healthz` | none | Liveness/readiness for a reverse proxy or container orchestrator. |

`telemetry.Sample.DeviceID` (which physical device uploaded) is tracked
separately from `client_id` (the API-key owner, i.e. the customer) — one
client can have more than one device (a phone today, a dedicated gateway
later) without an API contract change.

### Storage

SQLite via `modernc.org/sqlite` (pure Go, no cgo — keeps the relay binary
and its Docker image dependency-free of a system SQLite/cgo toolchain).
Two tables: `clients` (id, display name, API key hash, created/revoked
timestamps) and `samples` (client id, device id, both the client-supplied
timestamp and the server-received timestamp, and the field values as
JSON). A background sweep (`Server.pruneLoop`, `--prune-after`/
`--prune-interval` on `serve`, defaulting to 30 minutes / 5 minutes) deletes
samples past the retention window — this is a rolling buffer for
reconnect/backfill, not a telemetry archive.

`database/sql`'s connection pool doesn't safely serialize concurrent SQLite
writers by default. `store_sqlite.go` uses WAL mode plus
`db.SetMaxOpenConns(1)`, so every read and write serializes through one
connection — the simplest correct fix at this data volume (documented in
`SQLiteStore`'s own doc comment, along with the tradeoff: it caps read
concurrency too).

**A real bug this caught**: an early version stored `time.Now()` values
directly, which carry a monotonic clock reading in their internal
representation; different timestamps taken microseconds apart could
serialize inconsistently and compare out of chronological order in SQLite,
causing `PruneSamplesOlderThan` to delete a sample it should have kept (one
of the tests below caught this directly — `pruned = 2, want 1`). Fixed via
a `normalizeTime` helper (`t.Round(0).UTC()`, stripping the monotonic
component) applied to every timestamp this store binds as a query
parameter.

### Auth

The original v0 model was a single static, permanent API key per client,
minted by an admin and baked straight into the device's provisioning
link — simple, but a leaked link was a leaked permanent credential, and
there was no way for a device to prove *itself* rather than just
something it had been handed. Superseded by device pairing +
challenge/session auth:

- **Pairing links**: `POST /v1/pairing-links` (admin token; also
  `freebeamer-relay client pairing-link create --name "<name>"` on the
  CLI) mints a short-lived (15 min default), single-use token. It carries
  no long-lived secret — it only bootstraps trust.
- **Pairing**: the device generates its own Ed25519 keypair locally (the
  private key never leaves it) and redeems the link with `POST /v1/pair
  {pairing_token, public_key}`. The relay creates a `Client` row holding
  the public key and immediately disposes of the pairing link — a second
  redemption of the same token, whether a retry or a different device
  that saw the same link, always gets `410 Gone`.
- **Challenge/session ("second factor")**: `POST /v1/auth/challenge
  {client_id}` returns a random nonce (2 min TTL); the device signs it
  with its private key and calls `POST /v1/auth/session {client_id,
  signature}`, which verifies the signature against the stored public key
  and issues a short-lived (24h default) opaque session token, stored
  server-side by hash so it can be revoked instantly. `POST
  /v1/auth/logout` does exactly that, immediately, without unpairing the
  device — it can request a new session the same way next time.
- **Scope, strictly**: the session token authorizes device-facing routes
  only (`POST /v1/telemetry`, `GET /v1/capabilities`) — it is never valid
  on the admin surface (`/v1/live`, `/v1/clients`, `/v1/catalog`,
  `/v1/pairing-links`), and the admin token is never valid on the device
  routes. Each tier is checked with its own middleware
  (`requireDeviceSession` vs. `requireAdminAuth` in `relay/auth.go`) so
  neither can be satisfied by the other's credential by accident.
- **Admin token**: unchanged — one static token from
  `$FREEBEAMER_RELAY_ADMIN_TOKEN`; `freebeamer-relay serve` refuses to
  start without one (fails closed rather than silently booting admin
  endpoints open).

Revoking a client (`freebeamer-relay client revoke <id>`) now also
revokes every device session it currently holds, so kicking a device (a
lost phone, a shop ending a session) takes effect immediately rather than
only blocking its next re-authentication.

**A real gap this caught**: the first end-to-end smoke test (see below)
came back reporting every sample as `buffered`, not `live`, even with the
relay reachable — because `pkg/telemetry.Uploader` had no way to attach a
bearer token at all; that requirement didn't exist when `pkg/telemetry` was
built, before this relay's auth model was decided. Fixed by adding
`Uploader.APIKey` (sent as `Authorization: Bearer <APIKey>` in `post`) and
threading it through as `--api-key`/`$FREEBEAMER_RELAY_API_KEY` on the CLI —
confirmed fixed by rerunning the same smoke test, which then reported
`live` for every sample.

### Desktop delivery: WebSocket

Chosen over Server-Sent Events (Go's desktop-side client gets none of a
browser's automatic reconnect either way, and `net/http` has no SSE decoder
— no real simplicity win) and polling (wrong latency/overhead trade for
near-real-time viewing). `gorilla/websocket` was already an indirect
dependency via Wails; this promotes it to direct. Switching which client's
feed the desktop watches is close-and-reopen with a new `client_id` query
parameter for v0 — a subscribe/control channel on one persistent connection
is a reasonable later refinement, not needed yet.

### Deployment shape (named, not built here)

Single Docker image for `cmd/freebeamer-relay` (multi-stage build; the
pure-Go SQLite driver keeps it cgo-free), SQLite file on a mounted volume
(no separate DB container), a reverse proxy (Caddy or nginx) on the
project owner's own self-managed host terminating TLS (`https://` +
`wss://`) in front of the relay's plain-HTTP listener. Admin token and DB
path via environment/container config, not baked into the image.

## Non-goals for this slice

- **The Flutter mobile app** that will be the relay's real client in
  practice — a separate, not-yet-built piece of work. This relay was
  smoke-tested against `freebeamer mhd monitor` (the existing Go CLI)
  standing in for it.
- **Desktop app integration** (`internal/desktop`, `app.go`, the Angular
  frontend) — wiring the desktop app to actually connect to `GET
  /v1/live` and render it. Not built in this slice; the smoke test below
  used a standalone Go WebSocket client instead.
- Historical analytics or dashboards, multi-admin accounts/RBAC, rate
  limiting beyond a basic request-body-size cap, a WS
  subscribe/unsubscribe control channel, any write/live-tuning relay, a
  web dashboard.
- Actual deployment (Docker image, reverse proxy config, TLS) — the shape
  is named above, not built.

## Testing

- Unit/integration tests (`internal/relay/*_test.go`, all against a real
  temp-file SQLite store, no mocking of the store layer): client
  lifecycle (create/list/revoke, revoked keys rejected), sample
  ordering/limit/pruning, the `Hub`'s pub/sub semantics (including that a
  slow/absent subscriber never blocks `Publish`), and full HTTP-level
  request/response behavior for every endpoint including a real WebSocket
  handshake and backfill-then-live message sequence
  (`TestLiveFeedBackfillThenLive`).
- `pkg/telemetry`: new tests confirming the `Authorization` header is
  sent when `APIKey` is set and omitted when it isn't.
- **Real end-to-end smoke test**, not just unit tests: built the actual
  `freebeamer-relay` and `freebeamer` binaries, ran `freebeamer-relay serve`
  for real, issued a real client API key via `freebeamer-relay client add`,
  ran `freebeamer mhd monitor --upload-endpoint ... --api-key ...` against
  the existing fake mock DoIP/MHD adapter script (see
  `docs/mhd-live-monitor-v0-plan.md`), and watched a standalone Go
  WebSocket client receive the correct backfill-then-live sequence with
  correct per-device attribution. This is what caught both real issues
  above (the timestamp bug and the missing API-key plumbing) — neither
  was visible from unit tests alone.
- `go build ./... && go vet ./... && gofmt -l cmd pkg internal && go test
  ./... -count=1` all pass as of this writing.

## Next steps

- Flutter mobile app (see the project's broader v0 plan for its own
  scoping) — the relay's real client.
- Desktop app integration: a `GET /v1/live` WebSocket client in
  `internal/desktop`, new `App` methods, Wails `EventsEmit` push to the
  Angular frontend.
- Actual deployment on the project owner's own server.
