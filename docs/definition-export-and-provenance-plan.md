# Definition export and provenance review plan

Date: 2026-11-03

Status: Proposed. This covers [mapdef backlog](mapdef-backlog.md) items 5
(external-format export) and 6 (provenance/licence review before export or
sharing). It defines the round-trip invariants item 5 requires and the review
item 6 requires. **XDF and A2L export stay disabled.** Nothing here enables
them. The plan only states what must hold before either could ship.

## Where things stand

- **No export path exists.** `xdf.Converter` and `a2l.Converter` only import.
  The CLI writes `.mapdef` files (`xdf convert`, `a2l convert`,
  `mapdef migrate`). The desktop saves BIN images only. The relay never
  carries definitions.
- **The only "sharing" is passing a `.mapdef` file to someone.** That file
  already carries more than its parameters:
  - `extensions.xdf.rawDocument` / `extensions.a2l.rawDocument` holds the
    **entire original source file, verbatim** (ADR 0001 requires this so the
    source stays recoverable).
  - `provenance.sourceSha256` identifies that source.
  - `provenance.author` comes from the XDF header when present. A2L never
    sets it.
  - `provenance.license` is **never set** by either converter. No tool
    collects it.

  So sharing any converted definition today redistributes the third-party
  source word for word, with no recorded licence. The existing redistribution
  policy ([xdf-compatibility.md](xdf-compatibility.md#redistribution-and-attribution))
  already says conversion grants no rights. Nothing in the tooling makes that
  visible at the moment it matters.

## Item 5: round-trip invariants

"Round trip" is ambiguous, so each invariant is named. Every invariant is
checked per advertised construct, against independently licensed or
synthetic fixtures. Physical values compare exactly, with no tolerance.
Float raws compare bit-for-bit.

| ID | Invariant | Meaning | Testable today? |
| --- | --- | --- | --- |
| I1 | Decode equivalence | For any BIN, decoding through the source format's own semantics (an independent oracle) and through the converted mapdef gives identical raw values, physical values and addresses. | Yes. `pkg/xdf/mapdef_test.go` already does this for the basic fixture, and [Phase 4](phase4-oracle-validation.md) did it against an independent exporter. A2L has fixture round trips but no independent oracle. |
| I2 | Edit equivalence | An edit through mapdef writes the same bytes an edit under source-format semantics would. Neighbouring bytes stay untouched. | Partly. Covered by engine tests, not by an external oracle. |
| I3 | Mapdef fixed point | `import(export(D))` equals `D`, except for declared non-semantic fields (`id`, `importDiagnostics`, `provenance.convertedBy`, `provenance.sourceSha256`, `extensions.*.rawDocument`). | No. Needs an exporter. |
| I4 | Source equivalence | For a document imported from format F, `export_F(import_F(S))` satisfies I1 against `S` on every BIN. Every construct the importer did not interpret is either carried through unchanged or named in a loss report. Byte-identical source text is **not** required. | No. Needs an exporter. |
| I5 | No silent loss | Export refuses, or emits a machine-readable loss report, for any mapdef feature the target cannot represent. The user sees the report before a file is written. | No. Defined here (see table below). |
| I6 | Version refusal | Export of a document with an unknown `formatVersion` or unknown extension fails rather than guessing. | Partly. `mapdef.Migrate` already rejects unknown versions. |

### What each target could represent

This is from the current compatibility matrices. "No" means export must
refuse or report a loss under I5.

| mapdef feature | XDF | A2L |
| --- | --- | --- |
| Integer `uint8`–`int32`, big/little endian, row/column major | Yes | Yes |
| `float32` (1.4) | Yes (`0x10000`, 32-bit cells; see [xdf-compatibility.md](xdf-compatibility.md#ieee-754-evidence)) | Yes |
| `float64` (1.4) | No (no evidenced 64-bit XDF cell) | Yes |
| `bitPosition` flags | Yes (`<mask>`) | No evidenced mapping (A2L `BIT_MASK` not researched) |
| `freehorse-expression-v1` affine conversions | Yes (`MATH`) | Yes (`LINEAR`/`RAT_FUNC`) |
| Non-affine expressions | Yes (`MATH`) | Partly (`FORM`, grammar subset) |
| `freehorse-lookup-table-v1` (1.3) | No | Yes (`TAB_INTP`/`TAB_NOINTP`) |
| `axis.staticValues` | Yes (`<LABEL>`) | Yes (`TAB_VERB`/`COMPU_VTAB`) |
| `axis.fixedPoints` (1.2) | No direct equivalent | Yes (`FIX_AXIS_PAR_LIST`) |
| Axis `strideBits` (1.2) | Unclear | Yes (`RES_AXIS`) |
| `display.min`/`max` | Yes | Yes (limits) |
| `categories` | Yes | Yes (`GROUP`) |
| `checksumRequirements` | No | No |
| `sourceMetadata` / unparsed source XML | Only back into the format it came from | Only back into the format it came from |

Two consequences:

1. Exporting into the format a document came from is the only case where
   carrying unparsed source through (I4) is possible at all. Cross-format
   export (XDF → mapdef → A2L) is always lossy and must say so.
2. mapdef follows A2L concepts (ADR 0001), so A2L can represent more of it.
   XDF is still the format enthusiast users actually exchange.

### Gate for any exporter

An export target may ship as **experimental, behind an explicit flag** only
when all of these hold:

- I1, I2, I5 and I6 pass for every construct it advertises, against
  independently licensed fixtures.
- I3 and I4 pass on those same fixtures.
- An independent oracle confirms I1 for that format. Phase 4's TunerPro
  exporter covers XDF; nothing comparable is known for A2L yet.
- The item 6 review below runs before the file is written.

**Recommendation:** do not build an exporter yet. The cheapest useful step is
a shared invariant harness for I1 that runs over every XDF and A2L fixture.
It protects import quality today and becomes the test bed for an exporter
later. If export is wanted, XDF goes first: an independent oracle exists for
it, and none is known for A2L.

## Item 6: provenance and licence review

### Goal

Before a definition leaves the user's machine, through any future export,
upload or share feature, or when a CLI user asks for it, show what the file
contains and what is known about the rights to it. The tool must never
create the impression that conversion granted rights. This is
engineering-level diligence, not legal advice; the open questions in
xdf-compatibility.md still need counsel.

### Proposed pieces

1. **Collect provenance at conversion time, never infer it.** Add
   `--author`, `--license` and `--source-url` to `xdf convert` and
   `a2l convert`. `--license` takes free text and suggests an SPDX expression
   or `unknown`. No value is guessed from the file's contents or where it
   was downloaded.
2. **Record the user's redistribution claim (mapdef 1.5, additive).** Add
   `provenance.redistribution` with values `"permitted"`, `"not-permitted"`
   and `"unknown"`. An absent value means `"unknown"`. This is the user's own
   assertion. Tools label it that way ("you declared…") and never present it
   as verified. A 1.4 → 1.5 migration is a version bump only and does not
   fill the field in.
3. **One shared review function in core:** `mapdef.ReviewForSharing(doc)`
   returns a structured review:
   - author, licence, source, digest and the redistribution claim;
   - whether a verbatim source is embedded, with its format and size;
   - free-text fields that may hold personal or vehicle data (`name`,
     `description`, `sourceMetadata` keys) for the user to check;
   - a severity: `ok` only when redistribution is `permitted` and a licence
     is recorded, otherwise `review-required`.

   Every share or export surface calls this one function. None
   re-implements it, matching how validation and migration are shared.
4. **A CLI surface:** `freebeamer mapdef review FILE` prints the review.
   `--require-shareable` makes it exit non-zero unless the severity is `ok`,
   for scripts and CI.
5. **An explicit way to drop the embedded source:**
   `freebeamer mapdef strip-source FILE --output OUT` writes a **new** file
   without `extensions.*.rawDocument`. It keeps `sourceSha256` and appends a
   diagnostic recording the removal. It never overwrites the input, since
   ADR 0001 wants the source recoverable from the user's own copy. The tool
   states that stripping does not make the result shareable; the parameters
   can still be derived from protected material.
6. **Gating rule for future features:** any export, upload or share flow
   shows the review and needs explicit confirmation when the severity is
   `review-required`. A hosted sharing service stays out of scope until the
   counsel review the existing policy already requires.

### Out of scope

- Detecting licences automatically, or looking up "known good" sources.
- Any network upload, sharing service or relay change.
- Legal judgements about specific definitions.

## Proposed delivery order

1. I1 invariant harness across all XDF and A2L fixtures (item 5, no export).
   **Still to do.**
2. **Done (2026-11-04)** — `mapdef.ReviewForSharing` and
   `freebeamer mapdef review [--require-shareable] [--json]`. Reports licence
   `unknown` and an embedded source for every converted file, severity `ok`
   only when redistribution is `permitted` and a licence is recorded.
3. **Done** — convert-time `--author`/`--license`/`--source-url` on
   `xdf convert` and `a2l convert`.
4. **Done** — mapdef 1.5 `provenance.redistribution` and its migration
   (`migrateV1_4_0ToV1_5_0`, version bump only).
5. **Done** — `freebeamer mapdef strip-source FILE --output OUT`.
6. Only then, if wanted: an experimental XDF exporter behind the gate above.
   **Not started; export stays disabled.**

## Decisions for the project owner

- Is external-format export wanted at all, and for which workflow (giving a
  definition back to a TunerPro user, or tool interoperability)? The answer
  decides whether step 6 happens.
- Should `strip-source` exist? It reduces verbatim redistribution but weakens
  the "source recoverable" property for any copy made with it.
- Should `xdf convert` / `a2l convert` warn by default when `--license` is
  missing, or stay silent until a share step?
