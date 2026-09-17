# First vehicle hardware validation

Status: Procedure ready; first target is the owner’s N55, MHD WiFi adapter,
and Android phone. Physical trial pending.
Scope: phase 1 of the [implementation plan](live-data-and-vehicle-mapping-plan.md).

## Trial record

Copy this template into an ignored directory under `testdata/private/` for the
trial. Use a pseudonymous vehicle label; omit VINs, registration numbers, keys,
and customer details from any published report.

| Item | Recorded value |
|---|---|
| Trial ID/date and operator | Pending |
| Vehicle label/model/year/engine | Owner’s N55; model/year not required for preparation |
| ECU family and software ID; evidence source | MEVD17.2.6 editor baseline; installed software to confirm during trial |
| Adapter model/firmware and actual transport | MHD WiFi; firmware and actual transport to record during trial |
| Phone model/OS/app version (when applicable) | Android; model/app version to record during trial |
| MHD configuration/version/license relevant to monitoring | Unknown |
| FreeBeamer commit and local changes | Pending |
| Adapter IP/port, source, routing responder, diagnostic target | Unknown |
| Evidence supporting target/activation settings | Unknown |
| BIN and definition fingerprints/provenance, if applicable | Unknown |
| Independent reference/tool version and timing method | Pending |
| Trial conditions and predeclared comparison tolerances | Pending |

## Procedure

1. Confirm the adapter's actual transport and endpoint using adapter evidence.
   The current CLI tries DoIP, default port 13400; this is not proof that the
   selected vehicle/adapter uses it. If incompatible, record that result and
   stop this procedure rather than trying arbitrary target addresses/services.
2. Start stationary. Document ignition/engine state and adapter connection.
   Identify an independent reference and establish tolerances for each channel
   before comparison. If reference tooling cannot run simultaneously, use
   repeatable stationary conditions and record the timing limitation.
3. Build the CLI and record the revision (`git rev-parse HEAD`) plus whether
   there are local changes (`git status --short`). Create a new private directory
   per run. Substitute the evidenced endpoint in the example below. Flags must
   precede the host because this command uses Go's standard flag parser.

   ```sh
   go build -o /tmp/freebeamer-validation ./cmd/freebeamer
   mkdir -p testdata/private/hardware/trial-001
   /tmp/freebeamer-validation mhd monitor \
     --count 20 --interval 200ms --timeout 2s \
     --device-id trial-001 \
     --capture-file testdata/private/hardware/trial-001/exchanges.jsonl \
     --buffer-file testdata/private/hardware/trial-001/samples.jsonl \
     ADAPTER_IP:13400 \
     > testdata/private/hardware/trial-001/readings.csv \
     2> testdata/private/hardware/trial-001/diagnostics.txt
   ```

   Use a fresh trial directory to avoid overwriting CSV/diagnostics. Raw capture
   creation refuses overwrite and uses owner-only permissions. It records the
   routing responder separately from the target. Only if evidence identifies a
   different DME target, add `--target 0xNNNN` before the host. The CLI continues
   to use activation type `0x00`; do not interpret a failed activation as proof
   of unsupported monitor decoding.
4. Inspect the routing record and each exchange. The capture includes the UDS
   request/reply hex, request start time, elapsed nanoseconds, sequence, and any
   transport/decode error. Malformed replies are retained before decoding fails.
   Expected decoder input is `6c88` plus 36 bytes. A response with that shape is
   not yet evidence that all physical meanings/scales are correct.
5. Compare every available channel with the independent reference. Explicitly
   check signed fields, temperature units, RPM display units (the source labels
   RPM `l/min`), and absolute versus gauge boost. Mark channels that cannot be
   exercised as unknown. Do not infer B58/S58 compatibility from channel labels.
6. Measure actual cadence from successive request start times, and request
   latency from `elapsed_ns`. Record count, duration, median/p95/max interval,
   median/p95/max request latency, errors, and trial conditions. The configured
   interval is an added delay, not a promised sample period; current delivery
   and buffering also take place between acquisitions.
7. In a separate stationary run, disconnect the adapter and record the observed
   failure and recovery procedure. The current CLI stops on read error and must
   be restarted; automatic reconnect is not yet implemented. Internet loss and
   catch-up trials belong to the later delivery phases, not this transport test.

Captures are application-level evidence, not a packet trace: activation failure
is reported in stderr, routing records are written only after successful
activation, and partial transport frames are not retained. Use a separate private
packet capture if those details are needed. No credentials are added to the raw
capture, but captured vehicle data still requires review before sharing.

## Results template

Use `pass`, `fail`, or `unknown`; include a reason and evidence path for each.

| Gate | Result | Evidence / reason |
|---|---|---|
| Adapter transport/endpoint confirmed | Unknown | |
| Routing activation confirmed | Unknown | |
| DME diagnostic target confirmed independently | Unknown | |
| Monitor framing/length decoded | Unknown | |
| Per-channel scaling/reference comparison completed | Unknown | |
| Cadence/latency measured | Unknown | |
| Adapter disconnect behavior recorded | Unknown | |
| Calibration definition matches this ECU/software | Unknown | |

Per-channel comparison: channel ID, raw value, decoded value, source unit,
reference value/unit, reference timestamp, accepted tolerance, deviation,
pass/fail/unknown, evidence. A protocol failure leaves downstream gates unknown,
not passed. A mock run must be labelled mock throughout.

Publish a redacted result with fixture provenance and hashes for retained
artifacts, then update the [support matrix](vehicle-support-matrix.md). Retain
unresolved differences explicitly. Physical validation remains open until an
actual vehicle trial supplies this evidence; tooling and synthetic tests alone
complete only the preparation portion of phase 1.
