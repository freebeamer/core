# MHD live monitor — V0 plan

Date: 2026-09-15

Status: `pkg/doip` (DoIP transport), `pkg/mhd` (MHD live-monitor decode), and
`pkg/telemetry` (relay upload + offline buffering) are implemented and
tested against mock servers, not yet against real hardware. `freebeamer mhd
monitor` (`cmd/freebeamer/command_mhd.go`) is the CLI entry point for testing
against a real MHD adapter.

Follow-on work is tracked in the [live-data plan](live-data-and-vehicle-mapping-plan.md).
The [hardware procedure](vehicle-hardware-validation.md) documents CLI raw captures
and explicit diagnostic target selection for the first physical trial.

## Why this exists

`docs/a2l-v0-plan.md` and `FreeBeamer_Codex_V0_Plan.md` list live ECU data as
an explicit non-goal, on the grounds that it needs real-time bus
communication rather than static `.bin` file editing — a different subsystem
than the rest of this project. That framing was about scope at the time, not
a permanent stance: the project's actual owner already relies on MHD for
live data today and plans to add dedicated hardware (a custom device, or a
generic ENET cable) later. This plan scopes the first concrete step: a
FreeBeamer-owned Go client that reads live data over the same DoIP/ENET
transport MHD's own WiFi adapter uses, independent of MHD's app, so it can
eventually be driven by FreeBeamer's own tooling (a future mobile app and
backend API) instead of going through TunerPro RT or MHD's own software.

## What's legally/practically available

MHD Tuning's app itself is closed-source, and its Terms of Service withhold
reverse-engineering rights for the app/protocol. This plan avoids that
category of evidence entirely. Instead it's built from:

- **DoIP wire protocol** (ISO 13400 is not freely published, same situation
  as A2L/ASAP2): read directly from
  [`jacobschaer/python-doipclient`](https://github.com/jacobschaer/python-doipclient)
  (MIT-licensed), an independent, real, working DoIP client's actual source
  (`messages.py`/`client.py`), not a paraphrase or secondhand summary.
- **The MHD-specific monitor exchange**: MHD's own published
  `Real time tuning/MG1.adx` from
  [`dmacpro91/BMW-XDFs`](https://github.com/dmacpro91/BMW-XDFs) (fetched
  directly from `raw.githubusercontent.com/dmacpro91/BMW-XDFs/master/Real%20time%20tuning/MG1.adx`),
  a plain-text TunerPro RT ADX file authored by MHD Tuning
  (`<author>MHD Tuning</author>`). This is materially different from
  reverse-engineering MHD's app: it's MHD's own published definition file,
  the same evidentiary category as reading a vendor's published XDF/A2L
  file, not a decompiled binary.
- **That the ADX file is genuine and generically usable**: confirmed via
  `dmacpro91/BMW-XDFs`'s own README, which links MHD's official "MHD+
  Real-Time Tuning Guide" (a freely published Google Doc). That guide states
  outright: *"You can use other ENET devices / cables with TunerPro RT, but
  will be unable to log w/ MHD at the same time"* — i.e. MHD's own
  documentation confirms the monitor exchange works over generic/DIY ENET
  hardware, not only MHD's specific adapter.
- **What's explicitly NOT evidenced**: the write/live-tuning (calibration
  RAM patching) mechanism. The guide describes that workflow only at the
  TunerPro RT UI level (Initialize Emulation Hardware, Verify Emulator RAM
  Contents, Enable Live Tuning, Commit Changes and Upload); the actual
  byte-level UDS mechanism lives inside the closed `MHD.dll` TunerPro RT
  plugin, which was deliberately not decompiled or inspected. Building that
  would need independent evidence this plan doesn't have — a different,
  harder-risk undertaking than the read-only monitor path built here.

## Goal for this slice

A read-only live-data path: connect to a DoIP entity over TCP, perform
routing activation, repeatedly send MG1.adx's fixed monitor request, and
decode the fixed-format reply into named, scaled scalar values — usable
today against the user's own MHD WiFi-ENET adapter (real hardware, MHD+
Super License already owned), and, per the evidence above, expected to work
against any compatible generic ENET adapter later.

- `pkg/doip`: a minimal ISO 13400 client — TCP dial, routing activation
  (payload types `0x0005`/`0x0006`), and diagnostic messaging (`0x8001`/
  `0x8002`/`0x8003`). No UDP vehicle discovery, no TLS, nothing on DoIP's ECU
  reprogramming/emulation side.
- `pkg/mhd`: MG1.adx's `MONITORMACRO` exchange specifically — the fixed
  3-byte request, the fixed 38-byte reply (2-byte header + 36-byte body),
  and the 19 named scalar fields within it, each decoded to an engineering
  value via the same `pkg/expression` affine-expression engine already used
  throughout `pkg/mapdef`/A2L/XDF conversions.
- `freebeamer mhd monitor HOST[:PORT]`: a CLI entry point that streams
  decoded samples as CSV to stdout, so the user can run this against their
  real adapter today.

## Non-goals for this slice

- **Writing/live tuning** (calibration RAM patching). Not evidenced outside
  `MHD.dll` — see above. A future slice, if pursued, needs its own
  evidence-gathering (e.g. published UDS RAM-write/routine-control
  conventions for BMW MG1 DMEs) or an explicit reverse-engineering-risk
  decision, not an extension of this one.
- **UDP vehicle discovery / DoIP entity announcement** (ISO 13400's
  `0x0001`-`0x0004` payload types). MHD's adapter's TCP address is already
  known by the user (its own AP, fixed IP); discovery isn't needed to use
  it.
- **TLS-secured DoIP** (`0x50`xx payload types, ISO 13400:2019). Not
  evidenced as relevant to BMW's ENET-generation hardware.
- **The mobile app / backend API** that will eventually consume this
  package. This slice is deliberately just the hardware-facing transport
  and decode layer, usable from Go today (the CLI) and, per the user's
  stated direction, intended as the shared foundation regardless of what
  the mobile app and relay API end up being built in.
- **CSV log import** (parsing files MHD's app has already exported,
  reverse-engineered by the open-source `UltraLog` project). A different,
  independent piece of work — see `live_ecu_data_direction` project memory
  — not part of this live-read slice.

## DoIP evidence

`pkg/doip`'s header framing, routing activation, and diagnostic messaging
were read directly from `jacobschaer/python-doipclient`'s source (not a
summary or secondhand description):

- **Generic header** (8 bytes): protocol version (1 byte), its bitwise
  complement (1 byte, `0xFF ^ protocol_version` — a self-check the receiver
  validates), payload type (2 bytes, big-endian), payload length (4 bytes,
  big-endian), followed by the payload itself.
- **Routing activation request** (payload type `0x0005`): source address (2
  bytes, big-endian — the client's own chosen logical address) + activation
  type (1 byte) + a 4-byte reserved field (zero).
- **Routing activation response** (payload type `0x0006`): client address (2
  bytes) + the responding entity's own logical address (2 bytes) + a
  1-byte response code + an optional reserved/OEM-specific tail this
  package ignores.
- **Diagnostic messaging**: `0x8001` (DiagnosticMessage: source address (2
  bytes) + target address (2 bytes) + raw UDS payload), `0x8002`
  (DoIP-level positive acknowledgement), `0x8003` (DoIP-level negative
  acknowledgement) — both acks shaped as source (2 bytes) + target (2
  bytes) + code (1 byte) + echoed prior message data.

Two protocol values are **not** independently confirmed against a real BMW
capture, and are flagged as such in `pkg/doip`'s own doc comments:

- **Routing activation's `activation_type`**: `pkg/doip.ActivationDefault`
  (`0x00`) is ISO 13400's own specification default, used because no
  BMW-specific source confirms what value BMW's ENET adapters actually
  expect — it's the best-evidenced fallback, not a confirmed fact.
- **The DME's own logical target address**: rather than hardcode a guessed
  value, `pkg/doip.Client.Activate` discovers it dynamically from the
  routing activation response itself (the responding entity's own logical
  address) and uses that as the default diagnostic target. This sidesteps
  needing to guess/confirm a specific address ahead of time; callers who
  discover (from a real capture, or a negative acknowledgement) that a
  different address is needed can override it with `SetTargetAddress`.

Both should be validated against the user's real adapter — see "Next steps"
below.

## MHD monitor exchange evidence

Fetched and read `Real time tuning/MG1.adx` directly (not summarized) from
`dmacpro91/BMW-XDFs`. It's a plain-text TunerPro RT ADX file defining a
`MONITORMACRO` that sends `MONITORREQUEST` and listens for
`MONITORREPLY`:

```xml
<ADXCSENDCOMMAND id="MONITORREQUEST" ...>
  <bytestring size="0x3">2C8801</bytestring>
</ADXCSENDCOMMAND>

<ADXCLISTENPACKET id="MONITORREPLY" ... flags="0x00000005">
  <listentimeout>100</listentimeout>
  <packetbodylength>36</packetbodylength>
  <packetoffsetinbody>0</packetoffsetinbody>
  <packetsize>36</packetsize>
  <headerstring size="2">6C88</headerstring>
</ADXCLISTENPACKET>
```

- The request, `2C 88 01`, is UDS (ISO 14229) service `0x2C`
  (`DynamicallyDefineDataIdentifier`) with sub-parameters `0x88 0x01`.
- The reply is headed `6C 88` — `0x6C` is the standard ISO 14229 positive
  response ID for service `0x2C` (`service | 0x40`) — followed by a fixed
  36-byte body (`pkg/mhd.MonitorReplyBodyLength`). `packetoffset` for every
  field in the file is relative to this body, not to the 2-byte header
  (`pkg/mhd.MonitorReplyHeader`); the last field (`MHDMAFCALC`, offset
  `0x22`, 16 bits) ends exactly at body byte 36, matching
  `<packetbodylength>36</packetbodylength>` exactly. `pkg/mhd.MonitorReplyLength`
  (38) is the full reply length this project decodes: header + body.
- `<DEFAULTS ... lsbfirst="1" .../>` — every multi-byte field is
  little-endian, which is how `pkg/mhd.Field.decodeRaw` reads them
  (`encoding/binary.LittleEndian`).

### Field table

All 19 `<ADXVALUE>` entries, transcribed into `pkg/mhd.Fields`
(`pkg/mhd/livemonitor.go`) with their exact declared offset, size,
equation, and units:

| Field | Offset | Bits | Signed | Equation | Units (as declared) |
|---|---|---|---|---|---|
| RPM | 0x00 | 16 | no | `X` | `l/min` (see below) |
| LOAD | 0x02 | 16 | **yes** | `X*0.01` | `%` |
| BOOST | 0x04 | 16 | no | `X*0.001133107328125` | `psi` |
| BOOSTTARGET | 0x06 | 16 | no | `X*0.0018129717` | `psi` |
| BOOSTDEVIATION | 0x08 | 16 | **yes** | `X*0.0018129717` | `psi` |
| BOOSTDEVGRAD | 0x0A | 16 | **yes** | `X*0.0018129717` | `psi` |
| ENGINETEMP | 0x0C | 16 | **yes** | `X*0.01` | `°C` |
| GEAR | 0x0E | 8 | no | `X` | (none declared) |
| ETHANOLCONTENT | 0x0F | 8 | no | `X` | `%` |
| WGDISTRIBFAC | 0x10 | 16 | no | `X/16384` | `-` |
| TURBOMASSFLOW | 0x12 | 16 | no | `X*0.0390625/3.6` | `g/s` |
| TURBOMASSFLOW2 | 0x14 | 16 | no | `X*0.03125/3.6` | `g/s` |
| CMPRMASSFLOW | 0x16 | 16 | no | `X*0.0625/3.6` | `g/s` |
| CMPRMASSFLOW2 | 0x18 | 16 | no | `X*0.03125/3.6` | `g/s` |
| BOOSTSETPOINTF | 0x1A | 16 | no | `X/8192` | `-` |
| SPEED | 0x1C | 16 | no | `X` | `km/h` |
| AMBIENTTEMP | 0x1E | 16 | **yes** | `X/10` | `°C` |
| INTAKEAIRTEMP | 0x20 | 16 | **yes** | `X/10` | `°C` |
| MHDMAFCALC ("MHD+ MAF") | 0x22 | 16 | no | `X*0.03472225` | `g/s` |

RPM's declared `<units>l/min</units>` (liters per minute) is very likely an
authoring mistake in MHD's own file — RPM is an engine speed, not a flow
rate, and every other field's units are sensible. It's transcribed as
declared anyway (in `Field.Units`), not silently corrected, since it's a
faithful transcription of the source, not a claim about physical meaning.

### Signedness — evidence and derivation

MG1.adx declares one blanket default, `<DEFAULTS ... signed="0" .../>`, and
**no** `<ADXVALUE>` in the file carries its own `signed="..."` override —
so read purely literally, every field would be unsigned. That contradicts
several fields' own declared `<range>`, which can only be reached if the
raw value is actually signed. `Field.Signed` (`pkg/mhd/livemonitor.go`)
corrects those specific fields, derived by cross-checking each field's
declared range against what its equation produces over the full raw domain,
both interpretations:

- **Unsigned 16-bit**: raw ∈ [0, 65535]
- **Signed 16-bit**: raw ∈ [-32768, 32767]
- (8-bit fields: [0, 255] vs. [-128, 127])

| Field | Declared range | Unsigned-domain result | Signed-domain result | Conclusion |
|---|---|---|---|---|
| LOAD | -327.68..327.67 | 0..655.35 | -327.68..327.67 | **signed** (exact match) |
| BOOSTDEVIATION | -59.40..59.40 | 0..118.81 | -59.41..59.41 | **signed** (matches within digcount rounding) |
| BOOSTDEVGRAD | -59.40..59.40 | 0..118.81 | -59.41..59.41 | **signed** (same as above) |
| ENGINETEMP | -327.68..327.67 | 0..655.35 | -327.68..327.67 | **signed** (exact match) |
| AMBIENTTEMP | -3276.80..3276.70 | 0..6553.50 | -3276.80..3276.70 | **signed** (exact match) |
| INTAKEAIRTEMP | -3276.80..3276.70 | 0..6553.50 | -3276.80..3276.70 | **signed** (exact match) |
| BOOST | 0..74.25 | 0..74.26 | -37.13..37.13 | unsigned (matches unsigned domain) |
| BOOSTTARGET | 0..118.81 | 0..118.81 | -59.41..59.41 | unsigned (matches unsigned domain) |
| GEAR | 0..255 | 0..255 | -128..127 | unsigned (matches unsigned domain) |
| WGDISTRIBFAC | 0..4.00 | 0..4.00 | -2.00..2.00 | unsigned (matches unsigned domain) |
| TURBOMASSFLOW | 0..711.10 | 0..711.10 | -355.56..355.54 | unsigned (matches unsigned domain) |
| TURBOMASSFLOW2 | 0..568.88 | 0..568.88 | -284.44..284.44 | unsigned (matches unsigned domain) |
| CMPRMASSFLOW2 | 0..568.88 | 0..568.88 | -284.44..284.44 | unsigned (matches unsigned domain) |
| BOOSTSETPOINTF | 0..8.00 | 0..8.00 | -4.00..4.00 | unsigned (matches unsigned domain) |
| MHDMAFCALC | 0..2275.52 | 0..2275.52 | -1137.78..1137.74 | unsigned (matches unsigned domain) |
| RPM | 0..32767 | 0..65535 | 0..32767 (upper bound only) | ambiguous — kept unsigned (file default); non-negative, no contradiction |
| ETHANOLCONTENT | 0..100 | 0..255 | -128..127 | ambiguous — kept unsigned (file default); non-negative, no contradiction |
| SPEED | 0..255 | 0..65535 | -32768..32767 | ambiguous — kept unsigned (file default); non-negative, no contradiction |
| CMPRMASSFLOW | 0..1.00 | 0..1137.76 | -568.89..568.87 | ambiguous — kept unsigned (file default); non-negative, no contradiction |

For the four "ambiguous" rows, the declared range doesn't span either
raw domain fully — it's a narrower, plausible physical cap that's
consistent with either signedness. Since none of them require a negative
raw value to be reachable, they're left at the file's own literal default
(unsigned) rather than guessed into signed. This is an evidence-derived
inference from the file's own internally-consistent data (matching the same
discipline `docs/a2l-v0-plan.md` and prior slices used for other formats),
not independently confirmed against a real packet capture.

## Telemetry relay (`pkg/telemetry`)

This project's live-data model is client-facing, not just the project
owner's own car: FreeBeamer's actual product needs to let clients connect
their own vehicle and have the data reach the project owner remotely for
tuning. That surfaced a real hardware constraint during scoping: most
phones/tablets have a single WiFi radio, so joining the MHD/ENET adapter's
own WiFi AP drops any other WiFi-based internet route (e.g. a phone
hotspot) — a WiFi-only tablet genuinely cannot be on the adapter's network
and an internet-providing WiFi network at the same time. Devices with their
own independent connection (cellular data, wired Ethernet) don't hit this;
WiFi and cellular coexist fine, and Android/iOS both automatically fall
back to cellular when the connected WiFi network has no internet route.

Decided approach: don't try to detect a device's connectivity situation
up front (unreliable, platform-specific). Instead, `pkg/telemetry.Uploader`
always attempts live delivery first and falls back to a local buffer file
on any failure — which handles "has both," "has neither," and "spotty
in-between" uniformly, since attempting delivery and catching failure *is*
the detection:

- `Uploader.Send`/`FlushBuffer`/`post` (unchanged since their introduction)
  remain the low-level, synchronous delivery primitives: POST a JSON
  `Sample` to `Endpoint`, falling back to a JSON-Lines `BufferPath` on
  failure. They stay covered by their own original tests and are no longer
  called directly by `freebeamer mhd monitor` — see below.

This defines the initial client→relay HTTP contract (FreeBeamer owns both
ends; no relay API exists yet, so this isn't conforming to a pre-existing
shape). Explicitly deferred, per the project owner ("too early for custom
hardware"): a dedicated cellular-connected hardware gateway — the pattern
commercial fleet-telematics/remote-tuning dongles use to sidestep this
constraint entirely by not depending on a client's phone at all. Worth
revisiting once the software-only path above is proven with real clients.

**Phase 3a superseded the synchronous Send-per-sample loop above** (see
[live data and vehicle mapping plan](live-data-and-vehicle-mapping-plan.md)'s
Phase 3 section) — acquisition (reading the adapter) is now decoupled from
delivery, so a slow or unreachable relay never stalls the next scheduled
DoIP read:

- `pkg/telemetry.AcquisitionQueue` is a small bounded in-memory handoff
  (default capacity 32) between the DoIP read loop and a durable
  `pkg/telemetry.Spool` file on disk; a full queue drops the oldest queued
  sample and counts it rather than blocking acquisition.
- `pkg/telemetry.Spool` replaces the old unbounded buffer file with a
  byte-capped (`--buffer-max-bytes`, default 50 MiB), mutex-guarded JSONL
  file supporting concurrent append (from a spool-writer goroutine) and
  drain (from the uploader). `--on-spool-full` chooses `drop` (default:
  drop new samples, count them, keep recording) or `stop` (stop recording)
  once the cap is reached.
- `pkg/telemetry.UploaderService` continuously drains the spool with cached
  capability negotiation (5-minute TTL, replacing the old
  per-sample re-negotiation) and bounded-delay retry/backoff (1s initial,
  ×2, capped at 60s, ±20% jitter) — it never gives up on a spooled sample,
  since abandoning one would silently discard durable data.
- `pkg/telemetry.Spool.Path` is resolved per relay/client destination via
  `telemetry.DestinationID` + `telemetry.ResolveSpoolPath`, so reprovisioning
  a device to a different relay/API key can never flush its old buffered
  samples under the new credentials — a pre-existing buffer file with
  unknown destination provenance (predating this) is quarantined once,
  renamed to `<path>.legacy-unknown-destination`, never deleted.
- The CSV `delivery` column (`live`/`buffered`) is gone — acquisition no
  longer learns the delivery outcome synchronously. It's replaced by
  `queue-status` (`queued`/`queue-dropped`, known at print time) plus a new
  periodic stderr status line (`--status-interval`, default 5s) reporting
  measured acquisition rate, queue/spool depth, upload state, catch-up
  progress, and drop counts.
- `--drain-timeout` (default 10s) gives a finite (`--count N`) run a real
  chance to actually deliver before exiting, without hanging indefinitely
  against an unreachable relay; Ctrl-C skips this grace period and exits
  immediately, leaving anything not yet delivered safely in the spool for
  the next run.
- Full design rationale, the concurrency-bug fix this required (the
  original single-goroutine `FlushBuffer`'s read-then-rewrite was only
  safe because nothing appended concurrently with a drain), and the
  acceptance-test mapping (cadence under a delayed relay, network loss,
  backlog/catch-up, duplicate delivery, process restart, disk
  failure/limit, reprovisioning, adapter disconnect, repeated start/stop)
  are in the Phase 3 section of
  [live data and vehicle mapping plan](live-data-and-vehicle-mapping-plan.md).
- Flutter mobile acquisition/upload now mirrors this same architecture
  (Increment 3b) — see `mobile/README.md`'s "Reliable acquisition and
  upload" section and `mobile/lib/core/telemetry/`.

## Testing

- `pkg/doip/frame_test.go`, `pkg/doip/client_test.go`: header packing,
  routing activation (success/denied), diagnostic messaging (positive
  ack/negative ack/response), and a full round trip against a mock TCP
  server sending exactly MG1.adx's `2C 88 01` request and a `6C 88`-headed
  reply.
- `pkg/mhd/livemonitor_test.go`: `DecodeMonitorReply` against a crafted
  38-byte reply covering every field (including a negative raw value for
  every signed field, cross-checked against each field's own equation
  computed independently of `pkg/expression`), reply length/header
  validation, and a `Monitor.Read` round trip against a mock DoIP server.
- No test here uses real hardware. `go build ./... && go vet ./... &&
  gofmt -l cmd pkg && go test ./... -count=1` all pass as of this writing.

## Next steps

- **Validate against real hardware.** The user has an MHD WiFi-ENET
  adapter and a Super License (paid tier enabling MHD+ Real-Time Tuning)
  and a real vehicle. Running `freebeamer mhd monitor <adapter-ip>` while
  connected to the adapter's WiFi is the next concrete step — it will
  confirm or correct the two unconfirmed DoIP details (`ActivationDefault`
  and dynamic target-address discovery) and the four ambiguous-signedness
  fields above (a negative reading on any of RPM/ETHANOLCONTENT/SPEED/
  CMPRMASSFLOW would immediately prove that field needs `Signed: true`
  after all).
- **Mobile app / backend API.** Out of scope here by design — see
  "Non-goals" above — but this package (`pkg/doip` + `pkg/mhd`) is meant as
  the shared, hardware-facing foundation those components build on, per the
  user's own stated direction.
