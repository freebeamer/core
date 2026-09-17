# ADR 0001: Use mapdef as the canonical definition format

Date: 2026-09-02

Status: Accepted

## Decision

FreeBeamer uses `.mapdef`, a versioned JSON document, as its canonical persisted
calibration-definition format. The shared Go domain model in `pkg/types` is the
authoritative application model and owns only data structures plus model-local
behavior. The `pkg/mapdef` package owns validation and JSON persistence. Each
external format owns its conversion into the canonical model; `pkg/xdf`, for
example, implements the explicit `xdf.MapdefConverter` contract. Desktop, CLI,
and planned API adapters consume mapdef rather than exposing an external format
as their contract.

The model deliberately follows A2L concepts for characteristics, memory layout,
axes, conversions, units, and compatibility. It is not an A2L replacement and
FreeBeamer does not claim that a mapdef document is ASAM-compliant.

XDF is an interoperability input parsed and converted by `pkg/xdf`. A2L is the
next preferred import format and will own its equivalent converter. Export to
either external format is deferred until its fidelity can be demonstrated.

## Context

XDF is useful in offline and enthusiast calibration workflows, but it is a
TunerPro-native format with incomplete public specification coverage. FreeBeamer
currently understands only a documented subset. Making XDF the API contract
would expose tool-specific flags and layouts and would make versioning depend
on behavior outside this project.

A2L/ASAP2 is the dominant vendor-neutral ECU measurement and calibration
description standard. Adopting the complete standard as FreeBeamer's initial
model would also introduce measurements, online access protocols, complex
record layouts, and other capabilities beyond the current offline BIN scope.

## Alternatives considered

### Keep XDF canonical

This minimizes immediate conversion work but couples every product surface to
an incompletely understood external format. It also provides no controlled
contract for provenance, import diagnostics, checksum requirements, or API
migration.

### Use A2L canonical

This provides the best professional interoperability and vocabulary. Correct
support is substantially broader than current FreeBeamer semantics, however,
and real files include multiple versions and tool-specific extensions. A2L
remains a high-priority adapter and the conceptual reference for mapdef.

### Use an unversioned internal representation only

This avoids a new public file type but leaves desktop, CLI, and API persistence
without a portable, reviewable contract.

## Guarantees

- A valid document identifies itself as `freehorse.mapdef` and declares an
  explicitly supported semantic version.
- Version 1 supports scalar, flag, curve, and map parameters; integer storage;
  addressed or virtual axes; byte order; matrix order; strides; units; and the
  `freehorse-expression-v1` arithmetic subset.
- Import never implies complete support. Diagnostics identify constructs that
  were preserved but not interpreted.
- XDF conversion preserves the exact source document in the namespaced
  `extensions.xdf.rawDocument` member and records its SHA-256 digest. Consumers
  must not treat extension content as executable.
- XDF export is unsupported. A2L import and export are not yet implemented.
- Conversion does not change the copyright, confidentiality, or licence status
  of source material.

## Versioning and migration

`formatVersion` uses semantic versioning for the document contract. Readers
must reject unsupported major versions. New optional fields and enum values may
appear in minor releases only when older readers can reject or ignore them
without changing existing meaning. Patch releases clarify validation without
changing valid document meaning.

Migrations are explicit, deterministic transformations between adjacent
versions. They produce a new document, preserve provenance, append migration
diagnostics, and never overwrite the user's only copy. Desktop, CLI, and API
must use the same migration package. The API version and mapdef version are
independent and both are reported at their boundaries.

## Consequences

- The desktop will convert an opened XDF to the canonical model before exposing
  maps. The original source remains recoverable from the converted document.
- The CLI provides `freebeamer xdf convert INPUT --output OUTPUT.mapdef`.
- The planned API accepts and returns versioned mapdef DTOs rather than XDF XML.
- Contributors add conversion alongside each source format; they do not add
  source-format dependencies or conversion methods to `pkg/mapdef`.
- FreeBeamer owns schema stability and migration maintenance.
- The first version intentionally cannot express all A2L or XDF constructs.
  Unsupported input remains visible instead of being guessed.

## Legal and safety boundary

The parser and adapter are independently implemented for interoperability. No
TunerPro code or bundled third-party definitions are used. Only synthetic,
public-domain, user-owned, or explicitly licensed fixtures may be committed.
Questions about a particular definition's copyright, contract restrictions,
trade secrets, or redistribution require qualified legal review.
