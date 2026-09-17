# Live data and vehicle mapping implementation plan

Date: 2026-09-16
Status: Phase 1 preparation implemented; physical validation pending. Phase 2 software implementation and mock tests complete; remote deployment pending. Phase 3 (Go CLI and Flutter mobile, increments 3a/3b) software implementation and mock tests complete. Phase 4 increments 4a (Go/Wails backend) and 4b (Angular UI) implemented; software validation is recorded below. Phase 5 (vehicle profiles and map bindings) software implementation and mock tests are complete; there is no reviewed real-vehicle evidence registry yet, so "validated" status and Phase 6's operating-point overlays remain pending. Physical vehicle validation remains pending throughout.

Phase 1 artifacts: [support matrix](vehicle-support-matrix.md),
[hardware procedure and trial record](vehicle-hardware-validation.md), and CLI
`--target` / `--capture-file` options. Captures retain raw monitor replies and
request timing; they do not establish physical-vehicle compatibility.

## Goal

Let a client stream validated vehicle readings to FreeBeamer, let the desktop
user inspect that stream alongside a calibration workspace, and show the
current operating point on explicitly matched maps.

This milestone is read-only on the vehicle. ECU flashing, RAM writes, automatic
tune changes, general vehicle support, and production telemetry analytics are
outside its scope. Existing offline edits and Save As remain separate actions.

## Current baseline

- Go DoIP transport, a fixed MHD MG1 monitor decoder with 19 channels, and a
  CSV monitor command exist in `pkg/doip`, `pkg/mhd`, and `cmd/freebeamer`.
- Go and Flutter uploaders support authenticated delivery and offline buffering.
  Acquisition currently waits for delivery/backlog processing.
- The relay supports client management, SQLite storage, recent-sample backfill,
  and a WebSocket feed. Its retention is a rolling buffer, not an archive.
- The mobile README records Android emulator → mock ECU → relay → WebSocket
  validation. Real adapter/vehicle validation and iOS validation are outstanding.
- The desktop edits XDF/BIN workspaces; it has no relay consumer or live overlay.
  The core/CLI support `.mapdef` and a documented A2L subset, but desktop file
  opening still uses XDF + BIN.
- Established calibration/checksum evidence concerns BMW N55 / MEVD17.2.6,
  with checksum correction restricted to the `75P9EJ0B` layout. Live telemetry
  uses the separate MG1 definition. Neither establishes support for the other.
- Telemetry identifies a device and channel values, but has no explicit session,
  vehicle, firmware, or channel-definition association.
- `go test ./...` passed during the 2026-09-16 status review. Mobile validation
  above is recorded historical evidence, not a fresh run from that review.

## Delivery order

1. Establish the support matrix and hardware validation procedure.
2. Define session identity and channel metadata across Go, Dart, and the relay.
3. Separate acquisition from delivery and validate failure recovery.
4. Add the desktop live-data viewer.
5. Bind a vehicle profile and telemetry channels to a calibration workspace.
6. Add map operating-point overlays and complete the first vehicle trial.

Hardware validation starts in phase 1 and continues when hardware is available.
Phases 2–4 can proceed against mocks while it is pending. Vehicle-validated
status and real-vehicle overlays remain gated on the appropriate evidence.
Deliver each phase as a reviewable change with its own validation record.

## Phase 1 — Vehicle scope and hardware evidence

Record the first test vehicle's engine, ECU family, software identifier, adapter
model/firmware, phone OS, and relevant MHD configuration. The first target is the owner’s N55 + MHD WiFi + Android. Confirm installed
software and adapter firmware during the trial; do not infer compatibility from
a channel name. Keep identifying captures and proprietary definitions private.

Create a support matrix that distinguishes transport tested, monitor decoded,
channel scaling validated, calibration definition matched, and checksum support.
Track evidence separately for each exact combination. Start MG1 telemetry and
N55/MEVD17.2.6 calibration as separate rows with unknown links between them.

Hardware procedure:

1. Confirm adapter address and actual transport before relying on the current
   DoIP implementation. Record whether protocol changes are necessary.
2. Validate routing activation and the diagnostic target. The responding entity
   address is not automatically evidence of the correct DME target address.
3. Capture a monitor exchange and check response framing, length, and decoding.
4. Compare supported readings against an independent reference under repeatable
   conditions. Check units, signedness, boost pressure convention, and scaling;
   document reference timing and tolerances before recording a pass.
5. Measure sample intervals, latency, timeouts, and adapter disconnect behavior.
   Begin with stationary checks; any later road/load trial requires a separate
   test arrangement where the driver does not operate the tooling.

Acceptance: publish a redacted validation record with pass/fail/unknown entries
and fixture provenance. A mismatch stays unsupported until explained. Mocks
must not be described as physical-vehicle evidence.

## Phase 2 — Session and channel contracts

Delivered foundation: [v2 wire contract and rollout](telemetry-v2-contract.md),
Go/Dart models with shared fixtures, channel catalog, transactional legacy SQLite
migration, identity conflict checks/deduplication, server-assigned receipt time,
and identity-preserving live/backfill envelopes. CLI and Flutter acquisition now
negotiate relay capability, create fresh v2 sessions and increment sequences.
Shared negotiation fixtures, producer-loop tests, and retry/restart tests verify
legacy fallback and identity-preserving buffered delivery. V2 records are never
downgraded for an incompatible relay. Software acceptance is complete; remote
rollout and physical vehicle validation have not been performed.


Introduce a versioned telemetry contract shared by Go, Flutter, relay storage,
and desktop DTOs. Specify its rollout before changing producers or endpoints.

- Distinguish customer/client identity, device identity, vehicle profile,
  connection session, and ECU/software evidence. Unknown identity is explicit.
- Give each acquisition session a unique ID and samples a monotonic sequence
  number. Preserve acquisition time and relay receipt time separately.
- Define a versioned channel catalog: stable channel ID, display label, unit,
  physical meaning, decoder provenance, and validation status. Preserve source
  quirks as provenance while making display units explicit.
- Associate sessions with a catalog/profile version. Keep metadata at session
  level where practical rather than repeating it in every sample.
- Define replay, duplicate, gap, and stale-data semantics. Use session ID plus
  sequence for deduplication; device wall clocks alone cannot establish order.
- Specify how existing v1 producers remain usable during rollout, and migrate
  stored data without inventing missing vehicle or session evidence.

Acceptance: shared JSON fixtures pass in Go and Dart; legacy handling, unknown
versions, duplicate uploads, session changes, and clock skew have explicit tests.
The relay preserves identity and ordering through storage and WebSocket delivery.

## Phase 3 — Reliable acquisition and upload

Split sampling and delivery in both the CLI and mobile app. A slow request or
large backlog must not block the next scheduled acquisition.

Delivered foundation (increment 3a, Go CLI only): a bounded in-memory
acquisition queue (`pkg/telemetry.AcquisitionQueue`, drop-oldest-and-count on
overflow, never blocks acquisition), a byte-capped, mutex-guarded durable
spool (`pkg/telemetry.Spool`, `--buffer-max-bytes`, `--on-spool-full
drop|stop`) replacing the original unbounded buffer file, a cancellable
uploader with cached capability negotiation and bounded-delay retry/backoff
(`pkg/telemetry.UploaderService`; never gives up on a spooled sample), and
per-relay/client spool isolation (`telemetry.DestinationID` +
`telemetry.ResolveSpoolPath`) so reprovisioning cannot flush one
destination's buffered samples under another's credentials. `freebeamer mhd
monitor` now runs three concurrent stages (acquisition, spool-writer,
uploader) coordinated by one cancellable context and `sync.WaitGroup`, with
a new periodic stderr status line (queue/spool depth, upload state, measured
acquisition rate, catch-up progress, drop counts) replacing the old
per-sample CSV delivery column, which the decoupled design can no longer
report synchronously. `--drain-timeout` gives a finite (`--count N`) run a
bounded chance to actually deliver before exit without hanging against an
unreachable relay. A real concurrency bug in the original single-goroutine
`FlushBuffer` (its read-then-rewrite was only safe absent concurrent
appends) is fixed as part of this. Full design in
[MHD monitor](mhd-live-monitor-v0-plan.md)'s Phase 3a section.

Acceptance evidence (mock-tested, no physical hardware involved):
`go test ./... -race` passes clean, including new tests for the
concurrency-bug regression, bounded retry/backoff and recovery, per-relay
spool isolation, no-goroutine-leak across repeated start/stop, and
mid-stream adapter disconnect. Cadence was measured directly (not just
asserted): acquiring 8 samples at a configured 50ms interval against a relay
with an injected 250ms per-request delay, every acquisition gap stayed
within ~0.2ms of the configured interval (measured ~19.9 Hz against a 20 Hz
target) — delivery latency never leaked into acquisition timing. Total
process wall time (distinct from acquisition cadence) still reflects one
upfront capability-negotiation round trip and a bounded, documented grace
period that lets one in-flight delivery finish rather than aborting it
mid-request; this is a deliberate trade-off, not coupling.

Delivered foundation (increment 3b, Flutter mobile): the same architecture
mirrored in Dart — `ITelemetryUploadService`/`TelemetryUploadServiceImpl`
(`mobile/lib/core/telemetry/telemetry_upload_service.dart`) drives two
independent async loops (no `Isolate`; Dart's single-isolate cooperative
scheduling already decouples I/O-bound work as long as acquisition itself
never awaits it) draining a bounded `AcquisitionQueue` into a byte-capped
spool file (`defaultSpoolMaxBytes` 20 MiB, smaller than Go's 50 MiB given
mobile storage constraints; `drop`/`stop` overflow policy), then draining
that spool over HTTP with cached capability negotiation (5-minute TTL) and
the identical bounded-delay retry/backoff curve (1s→60s, ×2, ±20% jitter).
`destinationId`/`resolveSpoolPath` (`mobile/lib/core/telemetry/
destination_identity.dart`) mirror the Go side's SHA-256-fingerprint spool
isolation exactly, including the legacy-buffer quarantine-on-upgrade
behavior. `ConnectionCubit`'s read loop calls the new `enqueue` (sync,
non-blocking) instead of awaiting delivery; the old `UploadSample` use case
is removed as dead code, with `ITelemetryRepository.send`/`flushBuffer` kept
as unused-but-tested primitives, mirroring Go's own choice to leave
`Uploader.Send`/`FlushBuffer` unchanged. `StatusScreen` now actually
enforces the app's previously-documented-only foreground-only design via
`WidgetsBindingObserver`. One deliberate platform difference: `stop()`
cancels promptly without a CLI-style `--drain-timeout` wait, since a mobile
session's stop has no natural finite endpoint (bounded `--count` runs) to
wait against — anything undelivered stays safely spooled for the next
session. `flutter analyze` clean, `flutter test` green (new tests cover the
queue's own concurrency-safe `drained` accounting, destination isolation's
three cases, delivery/backoff/recovery, catch-up progress, and the
cross-destination isolation guarantee).

Validated live on the same Android emulator/mock-ECU/real-relay setup used
for the earlier mobile milestones. The live run found a real crash neither
`flutter analyze` nor `flutter test` caught: two independent async loops
(spool-writer, uploader) sharing one spool file — an interleaving pattern
this app never had before Phase 3b — could race such that deleting an
emptied spool file collided with a concurrent read's exists-check-then-read,
crashing the app after roughly two and a half minutes of continuous
streaming. Fixed with an async mutex serializing spool file access between
the two loops (`mobile/README.md`'s "Reliable acquisition and upload"
section has the full account) — the Dart analogue of the same
concurrent-append-during-drain class of bug fixed on the Go side, arising
here for the same underlying reason (two loops sharing durable state) even
though Dart has no OS threads. After the fix: over three minutes of
continuous live streaming with zero crashes and samples arriving at the
relay within a second of "now" throughout, and backgrounding the app
stopped delivery within the same second, confirming the new foreground
enforcement works.

Not yet done: physical-hardware validation of any of Phase 3 (Go or
Flutter) against a real relay deployment and a real vehicle/adapter.

- Use a bounded acquisition queue and a durable upload spool. Define capacity,
  disk limits, overflow behavior, and crash recovery before implementation;
  expose losses or a stopped recording explicitly, never silently discard data.
- Run a cancellable uploader with bounded retries/backoff. Preserve per-session
  sequence and distinguish historical replay from fresh readings.
- Prevent a backlog from appearing live merely because it arrived recently.
  Define the catch-up policy and show catch-up progress.
- Make start/stop/disconnect idempotent, prevent overlapping sessions, and
  isolate spool data by relay/client destination so reprovisioning cannot send
  one client's buffered samples using another client's credentials.
- Surface acquisition rate, upload state, queued samples, last fresh sample,
  and recording errors. Mobile remains foreground-only for this milestone.

Acceptance: delayed/unreachable relay tests preserve the configured acquisition
cadence within a documented tolerance. Recovery tests cover network loss,
backlog, duplicate delivery, process restart, disk failure/limit, reprovisioning,
adapter disconnect, and repeated start/stop. Record actual achievable rates;
do not advertise a rate based only on the configured delay.

## Phase 4 — Desktop live-data viewer

Implement relay HTTP/WebSocket consumption in the Go desktop adapter, exposing
typed snapshots/events through Wails. Keep connection credentials out of Angular
state and logs; specify credential persistence before adding saved connections.

- Add relay connection configuration, a client picker, and device/session
  selection. Scope subscriptions so changing selection cannot show old data.
- Add connection/error/reconnect state, capped exponential reconnect backoff,
  cancellation, stale-data indicators, and explicit backfill/catch-up status.
- Display channel values with units and selectable time-series plots. Keep
  chart history bounded and batch UI updates independently of acquisition.
- Show sample age and gaps. Do not draw a continuous trace across missing data
  or replace a fresh reading with an older replayed sample.
- Handle the backfill/live boundary without duplicates or missing samples,
  using the contract from phase 2.
- Allow monitoring without a loaded calibration workspace.

Acceptance: a reproducible mock ECU → mobile or CLI → relay → desktop test
displays known values. Tests cover authentication failure, selection changes,
disconnect/reconnect, backfill overlap, gaps, stale data, and bounded history.
Existing editor behavior and workspace switching continue to pass regression
checks. Record a sustained mock streaming run and its resource usage.

Delivered foundation (increment 4a, Go/Wails backend only): `internal/desktop`
gains `LiveData`, a sibling of `Service` (not nested inside it, so monitoring
works without a loaded workspace) that connects to a relay's `GET /v1/live`
WebSocket feed, using the same capped exponential reconnect backoff as
`pkg/telemetry.UploaderService` (1s→60s, ×2, ±20% jitter — the two backoff
implementations were unified into exported `telemetry.SleepWithJitter`/
`NextBackoff` rather than duplicated). A `401` at the WebSocket handshake
(bad admin token) is treated as terminal, not retried. Device/session
selection filters an already-connected stream client-side (the relay has no
subscription protocol below `client_id`); client selection reconnects.
A `generation` counter, mirroring `Service.generation`/`ApplyCellOperation`'s
existing stale-check pattern, guarantees a scope change can never surface
already-superseded data. Samples are deduped ((session, sequence), bounded
window), gap-annotated (missing sequence within a session), and batched into
one Wails event every 250ms (`live:samples`, separate from immediate
`live:status` transitions) — new presentation DTOs live in
`pkg/types/desktop_livedata.go`, never a re-export of the relay's own wire
envelope. The admin token is passed once per call and never appears in any
emitted event or DTO; credential persistence is explicitly out of scope for
this phase. `go test ./... -race` passes clean, including new tests for the
end-to-end known-values path, auth-failure terminality, reconnect/backoff
timing, stale-scope rejection, dedup, and gap detection. A sustained 15,000-
sample/3-session soak run (`internal/desktop/livedata_soak_test.go`, gated
behind a `soak` build tag rather than `-short` — this repo's own `go test
./...` convention doesn't pass `-short`, so a `-short`-only guard would have
silently made it part of every routine run) is recorded in
[the soak-test evidence doc](live-data-soak-test.md): the consumer kept up
with production throughout, the dedup window stayed exactly at its
configured cap, and profiling showed the relay's own SQLite writes — not
`LiveData`'s logic — dominating cost.

Delivered increment 4b (Angular UI): a workspace-independent **Live data** shell
view with relay/client connection controls, discovered device/session selectors,
channel values and catalog units/provenance status, and up to four independent
Plotly time-series charts. The root-scoped store keeps monitoring across editor /
live-view switches. Each selected source retains at most 2,000 samples. Sequence
gaps, missing channel values, and acquisition intervals over five seconds break
traces; readings older than five seconds are stale, and future device clocks are
explicitly flagged. Old replay cannot replace a newer reading. Session sequences remain authoritative
when the device clock moves backward; such chart transitions break the trace. Backfill and
connection/retry status remain visible separately from acquisition freshness.
Credentials are entered in a password input, cleared on Connect, and never saved
in Angular state or browser storage; another client/connection requires re-entry.

Integration exposed a 4a defect: device/session selection advanced the generation
that also guarded the WebSocket read loop, stopping consumption after selection.
Transport lifetime and display scope now have separate counters. Connect and
selection return the display generation, and status events include it, so Angular
can reject superseded events even when Wails delivery races an IPC response.
Disconnect invalidates pending batches and clears backend connection credentials.
A real-relay regression test verifies continued delivery after selection.

Validation: Go tests with the race detector and `go vet ./...`; 149 Angular tests,
including replay/overlap, stale/future clocks, bounded history, selection races,
authentication/retry status, listener cleanup, password clearing, chart gaps, and
workspace-independent navigation. The production frontend build passes with the
existing Plotly bundle/CommonJS and map-editor style budget warnings. Rendering
and streaming evidence is recorded in [the soak-test document](live-data-soak-test.md).
An opt-in reproducible mock DoIP → real CLI uploader → relay → Wails →
Angular/Plotly run verifies known RPM/load values and the 2,000-sample cap.
Physical hardware validation and remote rollout remain outstanding. This UI
acceptance run uses the CLI producer; mobile has its separately recorded tests.

## Phase 5 — Vehicle profiles and map bindings

Create a small explicit profile/binding model, separate from generic calibration
decoding. Define its versioned persistence format and exact fingerprint inputs
before implementation. Avoid a broad vehicle framework at this stage.

Delivered foundation: [the v1 profile/binding format](vehicle-profile-format.md),
`pkg/vehicleprofile` (fingerprinting, the compatibility state machine, and an
atomic, strictly-decoded local JSON store), and `internal/desktop/bindings.go`
wiring workspace identity, an observed live session, and the profile store
into new `App` methods (`GetBindingContext`, `SaveBindingProfile`,
`AssociateBindingProfile`, `ClearBindingProfile`, `BindingUnitConversion`).
The desktop's existing **Vehicle profiles** shell view
(`frontend/src/app/vehicle-profiles/`) lets an operator inspect the current
workspace/session identity, save a profile locally, declare axis bindings
against the definition's explicit numeric axes and the connected source's
observed channels, and explicitly (re)associate or clear it. Saving and
associating both re-evaluate compatibility against current backend-owned
identity — a save is refused outright if it is already incompatible with the
workspace/session it was drafted against. Workspace reopening immediately
clears any active association; a live scope or session-identity change
clears it lazily, on the next context fetch, with an explicit reason.

No reviewed real-vehicle evidence registry exists yet (see
[the format doc](vehicle-profile-format.md)'s `Evidence` discussion), so the
desktop UI can reach `manual` (unverified) association but never fabricate
`validated` status — this is verified directly by
`internal/desktop/bindings_test.go`, alongside context-derived numeric axis
eligibility, save-without-activating, contradicting-identity refusal, and
both invalidation paths (workspace reopen, live/session scope change).
`go test ./... -race` and `go vet ./...` pass. The Angular panel has its own
`frontend/src/app/vehicle-profiles/profile-panel.spec.ts` covering draft
seeding, clone-not-alias profile selection, meaning-mismatch refusal, the
gateway-reported unit conversion path, save/associate wiring, and the
scope-change reset; the production frontend build and the full Angular suite
(157 tests) pass. Physical hardware validation and the first real,
evidence-backed binding remain outstanding, as does Phase 6's overlay work.

- A profile records ECU/software evidence and eligible telemetry catalog
  versions. A workspace association records definition identity/fingerprint and
  original firmware identity; working-copy edits must not change that identity.
- Bind channels to stable parameter IDs and X/Y axes, never map titles alone.
  Validate dimensions and axis semantics against the loaded definition.
- Bindings declare unit conversion and physical meaning. Similar labels do not
  establish equivalence: measured versus requested load and absolute versus
  gauge pressure must remain distinct unless conversion is evidenced.
- Support explicit manual association when discovery is unavailable, but label
  it unverified. Manual selection alone must not upgrade compatibility status.
- Distinguish incompatible, unknown, manually associated, and validated states.
  Missing identity or incompatible definitions disables verified overlays while
  leaving the standalone live viewer usable.
- Invalidate associations when the original BIN, definition, session profile,
  or catalog changes. Never reuse bindings silently across firmware revisions.

Direct desktop `.mapdef` opening is a separate follow-on; it is not a prerequisite
for binding the canonical definition already created by XDF import. A2L
`MEASUREMENT` acquisition and arbitrary RAM-address polling remain out of scope.

Acceptance: synthetic fixtures prove exact matches, unknown identity, mismatched
firmware/definitions, duplicate titles, missing channels, and unit incompatibility
are handled correctly. Add the first real binding only when both its telemetry
and calibration definition have evidence for the same vehicle/software.

## Phase 6 — Operating-point overlays and vehicle acceptance

Add an opt-in operating-point marker to eligible 1D/2D map views. Calculate its
location from decoded breakpoints and the explicit phase 5 axis bindings.

- Start with a cursor and bracketing cells. Do not imply the highlighted cell is
  the ECU's internally selected table/cell or that its interpolation algorithm
  is known; describe it as a position derived from measured axis values.
- Cover ascending/descending axes and exact boundaries. Reject ambiguous
  repeated/non-monotonic axes initially, and show out-of-range values explicitly
  rather than silently clamping them to an edge.
- Use coherent samples for both axes. Clear or visibly mark the cursor when
  readings become stale, missing, incompatible, or belong to an old session.
- Keep live overlays independent of edit selection, undo history, and BIN bytes.
  Defer coverage heatmaps, tune suggestions, and automated edits.

Acceptance: deterministic replay places markers at expected breakpoints and
between cells; edge/stale/workspace-switch tests pass. Complete a physical
vehicle → mobile → relay → desktop trial with a matched definition, record
sample rate and latency distributions, and test temporary loss and recovery of
both adapter and internet connections. Preserve redacted evidence and unresolved
limitations. Declare support only for the tested combination.

## Documentation and completion

During implementation, update the root README, mobile README, and earlier plan
status sections to distinguish shipped software, mock-tested paths, and verified
vehicle support. Link this plan from those records where their old scope no
longer describes current work. Do not erase historical validation limitations.

For each phase, run relevant Go tests, Flutter analysis/tests when mobile changes,
and frontend tests/build when desktop UI changes. Use cross-language fixtures
and integration tests for contract changes; run the complete Go suite before
closing a phase that changes shared behavior. Hardware trials require their own
evidence and are not replaced by green software tests.

This milestone is complete when one explicitly identified vehicle/software and
adapter combination provides validated readings, sampling survives relay delays,
the desktop distinguishes fresh data from replay, and a matching calibration
workspace displays correctly bound operating points without modifying firmware.

Related records: [MHD monitor](mhd-live-monitor-v0-plan.md),
[relay](freebeamer-relay-v0-plan.md), [mobile](https://github.com/freebeamer/mobile/blob/main/docs/mobile-development.md),
[desktop](desktop-v1-plan.md), [mapdef backlog](mapdef-backlog.md), and
[checksum evidence](phase8-checksum-validation.md).
