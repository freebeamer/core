# Vehicle profiles and bindings v1

This is a separate local document, not a change to mapdef or telemetry v2.
`format: freehorse-vehicle-profiles`, `version: 1`, `profiles: [...]` are stored
in the OS user configuration directory under `freebeamer/vehicle-profiles-v1.json`.
Saving is explicit, uses an atomic same-directory rename and mode 0600, and
never writes a BIN or a definition. Unknown versions/fields and malformed records
are rejected without overwriting the file. Profiles are not activated at startup.

A profile has an ID, label, ECU family/software assertions, evidence references,
eligible catalog ID/version pairs, exact workspace identity and axis bindings.
A binding names the canonical mapdef parameter ID, x/y axis, telemetry channel,
source/target physical meanings, source/target units and affine scale/offset.
Neither a map's display title nor its desktop integer index is a persisted key.

Workspace identity consists of definition ID, definition SHA-256, original BIN
SHA-256 and size. The original BIN digest is captured before any edits. The
definition digest is SHA-256 over `freehorse-definition-v1\n` followed by Go's
`encoding/json.Marshal` of the imported canonical `MapDefinition`, with ONLY
`Provenance.Source` cleared (filesystem location is not identity). The digest
includes source SHA-256, canonical parameter IDs/layouts/axes/conversions,
metadata/extensions and array order; Go JSON map keys are sorted. Even cosmetic
source changes may therefore require reassociation. This is conservative, not a
cross-language canonical-JSON standard. Future fingerprint algorithms need a
new format version. Paths and working-copy bytes are excluded.

Session fingerprints use `freehorse-session-v1\n` plus JSON of the binding
session DTO with its observed channel list omitted. They include a SHA-256 scope
of relay URL and client ID (never admin token), device/session IDs, catalog
ID/version, and the session's vehicle label/ECU/software assertions. A reconnect
or source selection also changes the active desktop scope token as applicable.
Saved profiles contain no session activation or connection credentials.

Compatibility is evaluated against current backend-owned workspace, session and
catalog data. Hard mismatches are `incompatible`; missing identity/context is
`unknown`; explicit compatible selection is `manual` (unverified). `validated`
requires a separately reviewed evidence record matching the complete profile
fingerprint AND current session fingerprint, with both telemetry and calibration
references. User-entered evidence notes are provenance, never authority. No
reviewed real-vehicle records ship in this phase. The manual UI cannot manufacture
validated status; synthetic tests supply isolated evidence records.

Only numeric, explicitly defined, decodable x/y axes with more than one point
are eligible. Scalar maps, implicit index axes, missing/duplicate parameter IDs,
unknown channels and dimension mismatches are refused. Meanings are explicit:
measured/requested load and absolute/gauge pressure remain distinct. Version 1
supports ordinary affine conversions within the same physical meaning and unit
dimension (rpm, load %, pressure, temperature, torque, vehicle speed, lambda).
It refuses cross-meaning conversions, including absolute-to-gauge pressure;
those need a future evidence-backed contextual conversion design.

Activation is in memory and tied to workspace generation, live display scope and
session fingerprint. Reopening/switching a workspace, reconnecting/selecting a
source, or changing session/catalog metadata requires explicit reassociation.
Editing working values or Save As does not change the original workspace identity.
Phase 6 must query compatibility again before drawing verified overlays; no
operating-point overlays are implemented here.
