# mapdef implementation backlog

These issue-ready work items follow ADR 0001. They are recorded here because
this repository has no configured issue tracker automation.

1. **Done.** `pkg/mapdef.ValidateSchema` compiles the published Draft 2020-12
   schema (`schemas/mapdef-v1.schema.json`, embedded read-only at
   `pkg/mapdef/schema/`) and reports every failure located by JSON Pointer.
   `Decode`/`Load` run it against the raw bytes before struct-decoding, and
   `Validate` runs it against any in-memory document (so `Encode`/`Save` and
   `xdf.Converter` get it too). The CLI exposes it directly via
   `freebeamer mapdef validate DEFINITION.mapdef`. Desktop does not yet open
   `.mapdef` files directly (it opens XDF+BIN), so there is no desktop file-open
   path to wire up yet; when one is added it should call the same
   `pkg/mapdef` functions rather than re-implementing validation.
2. **Done.** `pkg/mapdef.Migrate` (`pkg/mapdef/migrate.go`) walks a document
   forward through adjacent `formatVersion`s to `CurrentVersion`, cloning
   rather than mutating its input, validating the result before returning
   it, and rejecting a version with no known step
   (`mapdef.ErrUnknownFormatVersion`). The CLI exposes it via
   `freebeamer mapdef migrate DEFINITION.mapdef --output OUTPUT.mapdef`
   (same desktop caveat as item 1: no desktop mapdef-open flow exists yet to
   wire it into). Its first migration, `1.0.0` → `1.1.0`
   (`migrateV1_0_0ToV1_1_0`), is a real, golden-tested transformation, not a
   placeholder: it promotes XDF axis labels/bounds that a 1.0.0 document
   could only preserve in `sourceMetadata` into the typed fields item 3
   introduced. See `pkg/mapdef/migrate_test.go` for the golden case.
3. Expand XDF import fidelity using independently licensed synthetic
   fixtures:
   - **Done** — flag bit/mask semantics: `<mask>` resolves to
     `layout.bitPosition` for a single in-range bit, decoded/encoded through
     `pkg/calibration`'s bit-level read-modify-write. See
     [xdf-compatibility.md](xdf-compatibility.md#xdfflag-mask-evidence).
   - **Done** — `XDFCONSTANT` scalars outside tables: converted the same way
     as a scalar `XDFTABLE`. See
     [xdf-compatibility.md](xdf-compatibility.md#xdfconstant-evidence).
   - **Done** — embedded/static axis labels (`<LABEL>`): convert to
     `axis.staticValues` (mapdef 1.1+, item 2). A 1.0.0 document that only
     has the old `sourceMetadata.xdf.labels` preservation is upgraded by
     `mapdef.Migrate`. See
     [xdf-compatibility.md](xdf-compatibility.md#axis-label-evidence).
   - **Done** — display bounds (`<min>`/`<max>`, on the table Z axis, each
     X/Y axis, and constants): convert to `display.min`/`display.max`
     (mapdef 1.1+, item 2), informational only — not enforced as edit-time
     range checks, since XDF gives no signal for whether a bound is a hard
     ECU limit or a display suggestion, and this project does not clamp
     values it cannot justify. `rangelow`/`rangehigh` remain unparsed
     pending a second source clarifying their relationship to `min`/`max`.
     See [xdf-compatibility.md](xdf-compatibility.md#bounds-evidence).
   - **Closed as not applicable** — "alarms/visibility/DAQ links" from this
     item's original wording: no such `XDFAXIS`/`XDFCONSTANT` construct was
     found in available evidence (alarms/visibility describe TunerPro's live
     data-monitoring gauges, not the XDF file format), and live data/logging
     are explicit FreeBeamer V0 non-goals regardless. `DALINK`/
     `embedinfo type="3"` axis-linking constructs do exist but their purpose
     is unconfirmed, so they remain unparsed raw XML rather than modeled
     speculatively; revisit only if a concrete need or better source
     surfaces. See
     [xdf-compatibility.md](xdf-compatibility.md#bounds-evidence) (final
     paragraph).

   Item 3 is now fully addressed to the extent current evidence supports.
4. **In progress — V0 slice done.** The official ASAM MCD-2 MC (A2L)
   specification is not freely available (membership/purchase gated, no
   public grammar), so this proceeds on the same evidentiary basis as XDF:
   independent open-source implementations, not the official text. See
   [a2l-v0-plan.md](a2l-v0-plan.md) for what was and was not used, and
   [a2l-compatibility.md](a2l-compatibility.md) for the compatibility matrix.
   `pkg/a2l` parses and converts a scalar (`Type VALUE`) `CHARACTERISTIC`
   with an `IDENTICAL`/`LINEAR` `COMPU_METHOD` and a single-byte
   `RECORD_LAYOUT` to a mapdef scalar parameter, decoding/editing through
   the unmodified `pkg/calibration` engine — no calibration changes needed,
   the same pattern XDFCONSTANT used. Exposed via `freebeamer a2l info A2L`
   and `freebeamer a2l convert A2L --output DEFINITION.mapdef`.
   **Follow-on done:** multi-byte integer types (`UWORD`/`SWORD`/`ULONG`/
   `SLONG`) via `BYTE_ORDER` support on `MOD_COMMON` (module default) and
   `CHARACTERISTIC` (override), evidenced by two independent open-source
   A2L implementations — see a2l-v0-plan.md's byte order evidence section.
   **Follow-on done:** `CURVE`/`MAP` characteristics with an embedded
   `STD_AXIS` axis (or two, X then Y) per axis — `pkg/a2l`'s
   `planRecordLayout` computes each entry's byte offset from
   `RECORD_LAYOUT`'s ordinal `NO_AXIS_PTS_X`/`NO_AXIS_PTS_Y`/`AXIS_PTS_X`/
   `AXIS_PTS_Y`/`FNC_VALUES` positions, verified against a real example
   file's own documented memory layout — see a2l-v0-plan.md's CURVE/MAP
   evidence section. `INDEX_DECR`-stored axes and non-`DIRECT` `AddrType`
   remain explicit rejections.
   **Follow-on done:** shared axes via `COM_AXIS`/top-level `AXIS_PTS`
   (several characteristics referencing one standalone axis definition
   instead of each storing their own copy) — `pkg/a2l`'s `planAxisPtsLayout`
   resolves an `AXIS_PTS`'s own `RECORD_LAYOUT` the same way an embedded
   axis is resolved, addressed relative to the `AXIS_PTS`'s own `Address`.
   Which side (the `AXIS_DESCR` or the referenced `AXIS_PTS`) is
   authoritative for which field was resolved from an explicit "will be
   overwritten by" comment in the real evidence file for `Conversion`/
   `MaxAxisPoints`, and by reasoned inference (documented as such, not
   silently assumed) for bounds — see a2l-v0-plan.md's COM_AXIS / AXIS_PTS
   evidence section.
   **Follow-on done:** the remaining `AXIS_DESCR` attributes —
   `FIX_AXIS` (breakpoints computed from `FIX_AXIS_PAR_DIST`/
   `FIX_AXIS_PAR_LIST`, no on-disk storage at all), `CURVE_AXIS`
   (breakpoints borrowed outright from another `CHARACTERISTIC`'s own
   `STD_AXIS`/`COM_AXIS`, never chained onto another `CURVE_AXIS`), and
   `RES_AXIS` (a shared, standalone `AXIS_PTS` storing rescale
   (position, value) pairs, supported only when the pair count equals the
   resolved axis's `MaxAxisPoints` — real evidence doesn't establish
   whether a smaller pair count is meant to be interpolated into more
   axis points, and guessing at that risks silently wrong calibration
   values, so a mismatch is rejected, not guessed). `FIX_AXIS` needed a
   genuine mapdef schema addition (`axis.fixedPoints`, formatVersion
   1.2.0) since it has no address at all; `RES_AXIS` needed
   `layout.strideBits` (present since 1.1.0 but always rejected until
   now) to actually mean something during decode, and a small,
   general-purpose `pkg/calibration` extension (`decodeAxis` now honors a
   single non-zero per-axis stride) — the only calibration engine changes
   any A2L or XDF slice has needed besides the earlier bit-position work
   for XDF flags. See a2l-v0-plan.md's FIX_AXIS / CURVE_AXIS / RES_AXIS
   evidence section.
   **Follow-on done:** value-to-text axis conversions via `TAB_VERB`
   `COMPU_METHOD`s and the `COMPU_VTAB` block they reference through
   `COMPU_TAB_REF` — each `COMPU_VTAB` value/text entry becomes one
   `axis.staticValues` entry (the same mapdef field XDF `<LABEL>` axes use),
   with the underlying raw axis point still decoding normally since the
   lookup is display-only, evidenced by `pyA2L`'s example file and
   `pya2l/classes.py` — see a2l-v0-plan.md's TAB_VERB / COMPU_VTAB evidence
   section. `TAB_VERB` used as a `CHARACTERISTIC`'s own top-level
   `Conversion` (rather than an axis's) is rejected, not silently dropped:
   mapdef's `staticValues` field lives only on `MapAxis`, so this needs a
   schema addition, not this slice. `DEFAULT_VALUE` remains unattached.
   **Follow-on done:** `COMPU_VTAB_RANGE` (range-keyed text lookup — real
   evidence shows a `TAB_VERB` `COMPU_METHOD`'s `COMPU_TAB_REF` can point at
   either a `COMPU_VTAB` or a `COMPU_VTAB_RANGE`, indistinguishably from the
   `COMPU_METHOD` side, corroborated by `pya2l/classes.py`'s class
   definitions) is now parsed and its contents preserved rather than
   skipped as opaque, but a `Conversion` resolving to one is rejected with
   a dedicated error (`ErrCompuVtabRangeUnsupported`) rather than silently
   dropped or misdiagnosed as "not found": mapdef's `staticValues` is
   keyed by a single index, not an inclusive range, and no schema addition
   or expansion policy for that exists yet.
   **Follow-on done:** `VAL_BLK` — a `CHARACTERISTIC` type with no
   `AXIS_DESCR` at all, shaped instead by a `MATRIX_DIM x y z` keyword.
   Both real examples have `z = 1` (1-2 dimensional), so this slice
   supports only that evidenced case, converting to a `map`-kind parameter
   with `axes` entirely absent (mapdef's schema already makes `axes`
   optional and `pkg/calibration` already treats a missing axis as
   nothing to decode, so no schema or engine change was needed) and
   rejecting a `MATRIX_DIM` with `z != 1` rather than guessing at
   undocumented 3D storage order — see a2l-v0-plan.md's VAL_BLK evidence
   section. `ASCII`/`CUBOID`/`CUBE_4`/`CUBE_5` remain out of scope:
   `CUBOID` has real evidence of 3 genuine `AXIS_DESCR` blocks, so
   supporting it (or `CUBE_4`/`CUBE_5`) for real needs an actual
   N-dimensional data model decision across `pkg/calibration`'s
   `DecodedTable` and the mapdef schema's `axes` object — a bigger,
   separate architectural change, not a converter-only slice like every
   other follow-on in this item.
   **Follow-on done:** `GROUP` — A2L's analog of XDF's `<CATEGORY>`.
   `pya2l/classes.py`'s own attrs/children corroborate the real example
   file's shape: `GROUP <GroupName> "<GroupLongIdentifier>"`, an optional
   `ROOT` marker, and `SUB_GROUP`/`REF_CHARACTERISTIC`/`REF_MEASUREMENT`/
   `FUNCTION_LIST` identifier-list children. Converts to mapdef's existing
   `categories` field (already built for XDF's `<CATEGORY>` support, so no
   schema change was needed) using each `GROUP`'s own direct
   `REF_CHARACTERISTIC` list; `SUB_GROUP`'s real hierarchy is preserved on
   the internal model but not flattened into transitive membership, since
   mapdef's flat `categories` has no parent/child relationship to
   represent it correctly and no evidence establishes that flattening is
   even the intended semantic. `FUNCTION` was scoped out of this
   follow-on: it distinguishes characteristics a function *defines*
   (`DEF_CHARACTERISTIC`) from ones it only *depends on*
   (`REF_CHARACTERISTIC`/`IN_MEASUREMENT`/`OUT_MEASUREMENT`), a genuinely
   different relationship than `GROUP`'s flat membership that would
   conflate distinct meanings if folded into the same `categories` tag —
   see a2l-v0-plan.md's GROUP evidence section.
   **Follow-on done:** the remaining numeric `COMPU_METHOD` types —
   `RAT_FUNC` (`COEFFS a b c d e f`, `phys = (a*int^2+b*int+c)/
   (d*int^2+e*int+f)`; every real-evidenced use reduces to pure scaling,
   but the general form converts too, as a `freehorse-expression-v1`
   expression using `X*X` in place of an exponent the language doesn't
   have), `FORM` (a nested `FORMULA` block using ASAM's `X1` variable
   convention, translated to `X` and rejected if it references a second
   variable or uses syntax outside this project's small expression
   grammar), and `TAB_INTP`/`TAB_NOINTP` (a numeric lookup table via a
   `COMPU_TAB` block — a different, value-to-*number* block from
   `COMPU_VTAB`'s value-to-*text*). The first two reuse the existing
   `freehorse-expression-v1` conversion shape and, like XDF's arbitrary
   `MATH` formulas before them, decode correctly even when genuinely
   non-affine but can only be *edited* when they are (no guessed
   inverse). `TAB_INTP`/`TAB_NOINTP` needed a second conversion shape
   entirely — `freehorse-lookup-table-v1` (mapdef 1.3+, `points`/
   `interpolated`/`default` instead of an `expression` string) — plus a
   small `pkg/calibration` extension (`parseConversion` now returns a
   `conversionEvaluator` interface satisfied by both the existing
   expression evaluator and a new lookup-table one, supporting
   interpolated and exact-match-only lookups in both directions, the
   reverse direction requiring the table's output to be monotonic). See
   a2l-v0-plan.md's RAT_FUNC / FORM / COMPU_TAB evidence section.
   Still explicitly out of scope: `FLOAT32_IEEE`/`FLOAT64_IEEE` (no float
   member in `layout.dataType`, matching XDF), `ASCII`/`CUBOID`/`CUBE_4`/
   `CUBE_5` characteristics (string/3-5 dimensional, see above),
   `MEASUREMENT` (live data, an explicit non-goal — its address is RAM,
   needing live ECU communication, not an extension of `pkg/calibration`),
   `FUNCTION` (see above), and `A2ML`/`IF_DATA`. Each remaining item is a
   follow-on slice with its own evidence-gathering pass, same as XDF's
   incremental fidelity work in item 3.
5. Evaluate external-format export only after import coverage and round-trip
   invariants are defined. Keep XDF and A2L export disabled until then.
6. Add user-facing provenance/licence review before exporting or sharing a
   converted definition.
