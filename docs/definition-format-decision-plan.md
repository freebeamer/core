# Definition format decision plan

Date: 2026-09-02
Status: Completed — `.mapdef` selected by ADR 0001

## Goal

Decide whether FreeBeamer should use XDF as its primary definition format or
introduce its own open, versioned format while retaining XDF compatibility.

FreeBeamer uses `.mapdef` as the canonical model for the desktop application,
CLI, and planned API. XDF remains an interoperability and import format behind
a separate adapter. The decision and its consequences are recorded in
[ADR 0001](adr/0001-mapdef-canonical-format.md).

## Morning decision criteria

Evaluate both options against the same criteria:

1. Legal and licensing risk.
2. Compatibility with existing calibration definitions.
3. Fidelity and lossless round-tripping.
4. Suitability as a stable public API contract.
5. Versioning and forward compatibility.
6. Ease of implementation, validation, contribution, and tooling.
7. Ability to preserve provenance and prevent accidental redistribution of
   third-party data.

## Work session

### 1. Confirm the legal boundary

- Check for an official XDF specification and explicit format or
  redistribution terms.
- Distinguish implementing format compatibility from copying TunerPro code,
  branding, documentation, or bundled definitions.
- Confirm that FreeBeamer contains no third-party XDF files without permission.
- Record questions that require qualified legal advice instead of treating
  technical conclusions as legal clearance.

Expected output: a short compatibility and redistribution policy with any
remaining legal questions clearly identified.

### 2. Audit the current XDF implementation

- Inventory every supported XDF element, attribute, flag, equation, address
  mode, axis layout, and data type.
- Identify ignored, partially interpreted, and unsupported information.
- Run representative public-domain, permissively licensed, or user-owned XDF
  fixtures through the parser.
- Determine which information would be lost during import and export.
- Compare the findings with the V0 scope in [xdf-v0.md](xdf-v0.md).

Expected output: an XDF compatibility matrix and a list of round-trip risks.

### 3. Draft the canonical FreeBeamer model

Define the smallest useful version of `.mapdef`, including:

- Schema and format version.
- Definition identity, ECU compatibility, author, license, source, and
  provenance.
- Scalars, flags, one-dimensional tables, and two-dimensional tables.
- Axes, dimensions, units, addresses, byte order, signedness, strides, and raw
  data types.
- Conversion equations and their supported operations.
- Checksums and compatibility requirements without embedding executable code.
- Validation rules and explicit unsupported states.
- An extension mechanism for future capabilities.
- Storage for unknown imported metadata where practical, so an import does not
  silently discard information.

Use a versioned JSON document with a published JSON Schema for the first draft.
The in-memory Go domain model remains authoritative while serializers and
importers adapt external formats to it.

Expected output: an example `.mapdef` file and an initial JSON Schema.

### 4. Set the XDF compatibility policy

The proposed initial policy is:

- Support XDF import for interoperability.
- Keep XDF parsing isolated from the canonical model and public API.
- Mark XDF export experimental, or defer it entirely, until lossless
  round-trip behavior can be demonstrated.
- Preserve unknown XDF information where feasible and warn when information is
  discarded.
- Never bundle or redistribute third-party definitions without explicit
  permission.
- State clearly that FreeBeamer is independent from and unaffiliated with
  TunerPro.

Expected output: a documented import/export support policy suitable for the
README and contributor documentation.

### 5. Build a narrow proof of concept

Exercise this path with a small, legally usable fixture:

```text
XDF -> XDF adapter -> canonical Go model -> .mapdef
                                      |
                                      +-> desktop / CLI / planned API
```

Verify that the conversion preserves:

- Definition identity and provenance.
- Addresses and byte layout.
- Axes and matrix orientation.
- Raw and decoded values.
- Units and equations.
- Unsupported or unknown data warnings.

Do not implement broad XDF export as part of this proof of concept.

Expected output: conversion tests and a documented fidelity result.

### 6. Record the decision

Write an architecture decision record containing:

- The chosen canonical format.
- Alternatives considered.
- Legal and technical constraints.
- Benefits and drawbacks of the decision.
- XDF import and export guarantees.
- Versioning and migration policy.
- Consequences for the desktop application, CLI, API, and contributors.

Then create implementation issues for schema validation, import fidelity,
migrations, documentation, and any future export work.

## Decision gate

Adopt `.mapdef` as the canonical format only if the prototype demonstrates that
it can represent all currently supported calibration semantics without loss
and gives the planned API a stable, versioned contract.

Keep XDF as the canonical format only if the audit shows that maintaining a
second format provides no meaningful legal, architectural, or interoperability
benefit. If important licensing facts remain unclear, retain XDF import but do
not expand export or redistribute definitions until those questions are
resolved.

## Definition of done

The format question is settled when the repository contains:

- A written architecture decision record.
- An XDF compatibility matrix.
- A documented XDF redistribution and attribution policy.
- A draft `.mapdef` example and JSON Schema, if selected.
- Passing conversion and fidelity tests for a legally usable fixture.
- A versioning and migration policy for the desktop, CLI, and planned API.

## Outputs

- [ADR 0001: Use mapdef as the canonical definition format](adr/0001-mapdef-canonical-format.md)
- [XDF compatibility and redistribution policy](xdf-compatibility.md)
- [mapdef v1 JSON Schema](../schemas/mapdef-v1.schema.json)
- [Synthetic mapdef example](../examples/synthetic.mapdef)
- Conversion implementation and contract/fidelity tests in `pkg/xdf`; canonical
  model validation and I/O tests in `pkg/mapdef`
- [Implementation backlog](mapdef-backlog.md)

The decision gate passed for FreeBeamer's currently decoded integer table
semantics: the synthetic conversion test independently decodes the converted
layout and compares raw and physical values with the existing XDF calibration
decoder. Unsupported XDF constructs remain source-preserved and diagnosed;
their presence is not represented as supported behavior.
