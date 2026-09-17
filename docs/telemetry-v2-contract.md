# Telemetry v2 contract and rollout

Status: Implemented in the relay, Go CLI, and Android/Flutter acquisition loop.
Deploy the compatible relay first; producers negotiate before sampling.
The `/v1/telemetry` and `/v1/live` routes accept/emit the additive sample version;
route names remain unchanged. Older relay deployments must not receive v2 because
those deployments would silently discard identity fields.

A v2 sample contains `version: 2`, existing `device_id`, `timestamp`, and `values`,
plus `session` and `sequence`. Session contains an opaque random `id`,
`catalog_id: "mhd.mg1.monitor"`, `catalog_version: 1`, and optional `vehicle`
(label, ECU family, software). Vehicle metadata is an operator claim, never a
validated match. Missing vehicle means unknown. Customer identity comes only from
the authenticated API key, not the request body. Device IDs are not vehicle IDs.

Session IDs change on each acquisition connection. Sequences start at zero and
increase for each acquired sample, up to JavaScript's exact integer limit
9007199254740991. Retrying an upload preserves its entire original sample.
The first rollout repeats the small immutable session descriptor on each sample
so every offline record is self-contained; the relay stores one descriptor per
client/session. Catalog definitions are separate from samples.

Timestamp means acquisition time, in RFC3339 UTC. The relay assigns `received_at`
itself; an incoming value cannot override it. Device clock skew must not reorder
samples: use sequence within a session, never timestamps across devices. Late
arrival is permitted. Missing sequence numbers mean gaps, not zero readings.
A new session resets ordering; viewers must not merge sessions implicitly.

V2 validation rejects unknown versions/catalogs, missing identity/sequence,
non-finite or unknown channel values, and identity attached to a legacy sample.
Missing version or version 1 means legacy: preserve existing data without
inventing sessions, sequence, vehicle identity, or validation evidence.

Deduplication is scoped to authenticated client + session ID + sequence. Exact
retries are acknowledged without storage or live fan-out; reuse with different
sample content or session/device metadata is a conflict. Retention bounds this
guarantee: when all samples of a session expire, its descriptor may be removed;
retries beyond the retention window may be accepted anew.

`backfill` only identifies WebSocket connect-time replay. It does not mean a
sample with `backfill: false` was freshly acquired: offline uploads can be old.
Consumers need timestamp/receipt comparison and explicit stale/clock-uncertain
states. Network receipt cannot prove freshness when the acquisition clock is
untrusted. Client-side deduplication by session/sequence is also necessary at the
backfill/live overlap; bounded subscriber queues may drop samples, visible as
sequence gaps. Lossless reconnect/cursor support is deferred to the viewer phase.

The initial catalog preserves the MG1 definition's channel IDs and source units,
uses `rpm` for RPM display, and labels every channel `definition-derived`.
Pressure reference (absolute/gauge) and physical vehicle scaling remain unknown.
This catalog does not assert compatibility with the owner's N55.

The admin-authenticated `GET /v1/catalog` endpoint serves the current channel
catalog. Client-authenticated `GET /v1/capabilities` advertises
`telemetry_versions` and supported catalog ID/version pairs. Both producers use
this decision table at the start of each acquisition connection:

| Result | Acquisition format |
|---|---|
| Version 2 plus the supported MG1 catalog | V2, new random session ID and sequence starting at zero |
| Version 1 offered, or capability route returns 404/405 | Legacy for this connection |
| No relay configured | V2 offline recording |
| Timeout, auth failure, redirect, malformed response, or no compatible contract | V2 recording; buffer until support can be verified |

The format stays fixed within a connection. A subsequent connection renegotiates
and gets a new v2 session ID. Neither producer automatically associates the
owner's N55 identity with arbitrary sessions; vehicle claims remain absent.

Every v2 upload, including buffered replay after restart, rechecks support before
posting. A downgrade never strips identity from stored v2 records: they stay
buffered until a compatible relay is available. Capability and upload redirects
are refused so negotiation is not applied to a different destination. Capabilities
must be served alongside `/telemetry` at the same path prefix. Do not remove v2
support during an in-flight request; a legacy server cannot enforce v2 semantics.

Buffered retries preserve acquisition timestamp, session metadata, sequence and
values. Unreadable/unsupported records stop draining and stay on disk for manual
inspection instead of being silently discarded. A retained v2 record can block
later legacy records when the relay is downgraded; this is deliberate to preserve
identity and ordered delivery. Legacy records are never upgraded retroactively.

Negotiation is currently synchronous and v2 rechecks add one GET per delivery
attempt. Separating acquisition from upload, destination-isolated buffering,
bounded retries, and reducing negotiation overhead without weakening compatibility
checks are phase 3 work. Existing foreground-only mobile behavior remains.

Database migration is transactional and preserves legacy rows without fabricated
identity. Back up the relay DB before deployment; no remote deployment is part of
this code change. Shared capability fixtures and buffer/restart tests cover Go and
Dart; producer tests cover new sessions, sequence reset, legacy and offline starts.
