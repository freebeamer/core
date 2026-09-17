# A2L compatibility and redistribution policy

FreeBeamer imports a documented, intentionally narrow subset of A2L (ASAM
MCD-2 MC / ASAP2). See [a2l-v0-plan.md](a2l-v0-plan.md) for how this subset
was scoped and why the official specification could not be used directly.
Import converts recognized data to `.mapdef`, retains the original source
text in a namespaced extension, and rejects anything it does not recognize
with an explicit error rather than guessing.

Unlike XDF import, A2L V0 rejects an unrecognized construct inside something
it does try to parse (an unsupported `CHARACTERISTIC` type, an unknown
`COMPU_METHOD` conversion type, a `RECORD_LAYOUT` with more than one
`FNC_VALUES` entry) rather than converting a partial result with a
diagnostic. This is a deliberate, temporary V0 simplification, not a
different safety philosophy: this slice's whole recognized grammar is small
enough that "reject and say why" is clearer than a diagnostic system for
constructs that don't have a partial-conversion story yet. Blocks this
parser does not read at all (`MEASUREMENT`, `A2ML`, `IF_DATA`, `FUNCTION`,
`MOD_PAR`, `UNIT`, and any other block, standard or vendor-specific) are
skipped as opaque, balanced `/begin`/`/end` blocks and never cause an
error — they are simply absent from the converted document, recoverable
only from the raw source preserved in `extensions.a2l.rawDocument`.
`COMPU_VTAB`, `COMPU_VTAB_RANGE` (the two lookup-table shapes a `TAB_VERB`
`COMPU_METHOD` can reference via `COMPU_TAB_REF`), `COMPU_TAB` (the
numeric lookup shape `TAB_INTP`/`TAB_NOINTP` reference the same way), and
`GROUP` are the exceptions: all four are read and their contents
preserved — `COMPU_VTAB`, `COMPU_TAB`, and `GROUP` convert successfully,
`COMPU_VTAB_RANGE` does not — see the matrix below.

## Compatibility matrix

| A2L construct | Parse | mapdef conversion | Decode/edit | Fidelity or limitation |
| --- | --- | --- | --- | --- |
| `ASAP2_VERSION` | Yes | Not retained | N/A | Read and discarded; no version-gated behavior in this slice |
| `PROJECT` name/description | Yes | Not retained | N/A | The `MODULE`, not the `PROJECT`, becomes the document identity/name |
| `HEADER` | No | — | N/A | Skipped as an opaque block |
| `MODULE` name/description | Yes | Identity/provenance | N/A | Preserved |
| `COMPU_METHOD` `IDENTICAL` | Yes | `freehorse-expression-v1` `"X"` | Yes | Supported |
| `COMPU_METHOD` `LINEAR` (`COEFFS_LINEAR a b`) | Yes | `freehorse-expression-v1` `"X*a+b"` | Yes | Supported |
| `COMPU_METHOD` `TAB_VERB` (`COMPU_TAB_REF` -> resolved `COMPU_VTAB`), used as an `AXIS_DESCR`'s `Conversion` | Yes | `axis.staticValues` (one `{index, value}` per `COMPU_VTAB` entry); the axis's own raw value still decodes normally, the lookup is display-only | Yes | Supported — see a2l-v0-plan.md's TAB_VERB / COMPU_VTAB evidence |
| `COMPU_METHOD` `TAB_VERB`, used as a `CHARACTERISTIC`'s own top-level `Conversion` | Recognized | Rejected | No | mapdef's `staticValues` field lives only on `MapAxis`, not `MapParameter`; needs a schema addition, not a silent drop |
| `COMPU_VTAB`'s `TAB_VERB_RANGE` shape | Recognized | Rejected | No | Only `COMPU_VTAB`'s single-value-keyed `TAB_VERB` shape is supported |
| `COMPU_VTAB_RANGE` block, and a `TAB_VERB` `Conversion` whose `COMPU_TAB_REF` resolves to one (real evidence shows the same `COMPU_METHOD` shape referencing either a `COMPU_VTAB` or a `COMPU_VTAB_RANGE`) | Yes (parsed, contents preserved on the internal model) | Rejected (`ErrCompuVtabRangeUnsupported`, distinct from "not found") | No | mapdef's `staticValues` is keyed by a single index, not an inclusive range; needs a schema addition or a documented expansion policy, neither of which exists yet |
| `COMPU_VTAB`/`COMPU_VTAB_RANGE`'s `DEFAULT_VALUE` | Yes | Not retained | No | Parsed, recorded on the internal model, not yet attached anywhere (no "unmatched value" sentinel in `staticValues`) |
| `COMPU_METHOD` `RAT_FUNC` (`COEFFS a b c d e f`) | Yes | `freehorse-expression-v1`: a simplified polynomial when the denominator is a nonzero constant (the only evidenced shape, e.g. `"5*X"`), else `"(a*X*X+b*X+c)/(d*X*X+e*X+f)"` (`X*X` standing in for `X^2`, which the language has no operator for) | Yes if affine, else decode-only | Supported — see a2l-v0-plan.md's RAT_FUNC / FORM / COMPU_TAB evidence |
| `COMPU_METHOD` `RAT_FUNC` with `d=e=f=0` (denominator always zero) | Recognized | Rejected | No | Not silently divided by zero |
| `COMPU_METHOD` `FORM` (`FORMULA`'s forward `F_x`, `X1` translated to `X`) | Yes, if the formula fits `freehorse-expression-v1`'s grammar | `freehorse-expression-v1` | Yes if affine, else decode-only | Supported for formulas expressible in this project's small arithmetic grammar — see a2l-v0-plan.md's RAT_FUNC / FORM / COMPU_TAB evidence |
| `COMPU_METHOD` `FORM` referencing a second variable (`X2`, ...), or using syntax (functions, exponents, ...) outside `freehorse-expression-v1`'s grammar | Recognized | Rejected | No | No other input to resolve a second variable against (`MEASUREMENT`/`FUNCTION` aren't parsed); unsupported syntax is rejected with the underlying parse error, not guessed at |
| `COMPU_METHOD` `FORM`'s `FORMULA_INV` | Yes | Not retained | No | Parsed, preserved on the internal model, never used — an edit-time inverse is always derived algebraically from the forward expression, like every other conversion here |
| `COMPU_METHOD` `TAB_INTP`/`TAB_NOINTP` (`COMPU_TAB_REF` -> resolved `COMPU_TAB`) | Yes | `freehorse-lookup-table-v1` (mapdef 1.3+): `conversion.points`, `conversion.interpolated`, `conversion.default` | Yes | Supported — `TAB_INTP` interpolates linearly between entries; `TAB_NOINTP` is exact-match-only (the most conservative reading available evidence supports — see a2l-v0-plan.md's RAT_FUNC / FORM / COMPU_TAB evidence) |
| A `freehorse-lookup-table-v1` conversion whose `points` aren't monotonic in `Output` | Yes (decode) | — | Decode yes, edit no | `SetCell`/`SetIndex` need an unambiguous reverse lookup; a non-monotonic table reports `ErrLookupTableNotInvertible` rather than picking one arbitrarily |
| `COMPU_TAB`'s `DEFAULT_VALUE_NUMERIC` | Yes | `conversion.default` | Yes | Used as the decoded value when a raw value is outside the table's range or (`TAB_NOINTP`) has no exact match |
| `COMPU_METHOD`'s `Format`/`Unit` | Yes | `unit`; fractional digit count of `Format` (or a `CHARACTERISTIC`-level override) -> `display.decimalPlaces` | Yes, where applicable | Preserved |
| `RECORD_LAYOUT`'s `FNC_VALUES`/`AXIS_PTS_X`/`AXIS_PTS_Y` data type (`UBYTE`/`SBYTE`/`UWORD`/`SWORD`/`ULONG`/`SLONG`) | Yes | `layout.dataType`; `layout.byteOrder: "not-applicable"` for 1-byte types, `"little"`/`"big"` for the rest (see `BYTE_ORDER` rows below) | Yes | Supported |
| `RECORD_LAYOUT` entry with `FLOAT32_IEEE`/`FLOAT64_IEEE` | Recognized | Rejected | No | `layout.dataType` has no float member (matches XDF import's same limitation) |
| `RECORD_LAYOUT`'s `NO_AXIS_PTS_X`/`NO_AXIS_PTS_Y` | Yes | Not retained (only its byte size, to correctly offset later entries) | No | The stored *current* axis point count is not decoded; every axis is always exactly its `AXIS_DESCR`'s `MaxAxisPoints` long |
| `RECORD_LAYOUT`'s `AXIS_PTS_X`/`AXIS_PTS_Y` `IndexMode` `INDEX_DECR` | Recognized | Rejected | No | Only `INDEX_INCR` is supported; decoding descending-stored axes needs reading the array in reverse, not implemented |
| Any `RECORD_LAYOUT` entry's `AddrType` other than `DIRECT` | Recognized | Rejected | No | No pointer indirection |
| A `RECORD_LAYOUT` missing an entry its characteristic's axis count requires, or with an entry role duplicated or present without a matching axis | No | Rejected | No | See a2l-v0-plan.md's CURVE/MAP evidence for exactly which entries each shape needs |
| `MOD_COMMON`'s `BYTE_ORDER` | Yes | Module-wide default for `layout.byteOrder` | Yes | `LITTLE_ENDIAN`/`MSB_LAST` -> `"little"`, `BIG_ENDIAN`/`MSB_FIRST` -> `"big"` |
| `MOD_COMMON`'s `DEPOSIT`/`ALIGNMENT_*` | Yes | Not retained | N/A | Parsed and discarded so a typical `MOD_COMMON` block still parses; any other optional keyword there fails to parse |
| `CHARACTERISTIC`'s own `BYTE_ORDER` | Yes | Overrides `MOD_COMMON`'s default for this parameter | Yes | Same value mapping as `MOD_COMMON`'s |
| A multi-byte `CHARACTERISTIC` with no `BYTE_ORDER` from either place, or `MSB_FIRST_MSW_LAST`/`MSB_LAST_MSW_FIRST` (mixed word/byte order) | Recognized | Rejected | No | No default is assumed when omitted; mixed order has no `layout.byteOrder` representation — see a2l-v0-plan.md's byte order evidence |
| `CHARACTERISTIC` `Type VALUE`/`CURVE`/`MAP` | Yes | `scalar`/`curve`/`map`-kind parameter | Yes | Supported; the `AXIS_DESCR` count must match (0/1/2) |
| `CHARACTERISTIC` `Type VAL_BLK` + `MATRIX_DIM x y 1` (no `AXIS_DESCR`) | Yes | `map`-kind parameter with `axes` entirely absent; `layout.dimensions: [y, x]` | Yes | Supported — see a2l-v0-plan.md's VAL_BLK evidence |
| `CHARACTERISTIC` `Type VAL_BLK` with an `AXIS_DESCR`, no `MATRIX_DIM`, or a `MATRIX_DIM` with `z != 1` | Recognized | Rejected | No | `VAL_BLK` has no axes of its own; only the evidenced 1-2 dimensional (`z == 1`) shape is supported — a genuinely 3D `MATRIX_DIM`'s storage order isn't established by any available evidence |
| `CHARACTERISTIC` `Type ASCII`/`CUBOID`/`CUBE_4`/`CUBE_5` | Recognized | Rejected | No | Strings and genuinely 3-5 dimensional calibrations need a real N-dimensional data model this project doesn't have yet (see a2l-v0-plan.md's VAL_BLK evidence) |
| `AXIS_DESCR` `Attribute STD_AXIS` (axis points stored in the enclosing `RECORD_LAYOUT`) | Yes | `axes.x`/`axes.y`: `count`, `layout` (address computed from the `RECORD_LAYOUT`'s entry offsets), `conversion`/`unit`/`decimalPlaces`, `display.min`/`max` | Yes | Supported |
| `AXIS_DESCR` `Attribute COM_AXIS` + its `AXIS_PTS_REF` | Yes | `axes.x`/`axes.y`: `count`/bounds from the referenced `AXIS_PTS` (authoritative), `layout.address` relative to the `AXIS_PTS`'s own `Address`, `conversion`/`unit`/`decimalPlaces` from the `AXIS_DESCR`'s own `Conversion` (authoritative) — see a2l-v0-plan.md's COM_AXIS / AXIS_PTS evidence | Yes | Supported |
| `AXIS_DESCR` `Attribute FIX_AXIS` + its `FIX_AXIS_PAR_DIST` (`Offset`/`Distance`/`NumberPts`) or `FIX_AXIS_PAR_LIST` block | Yes | `axes.x`/`axes.y.fixedPoints` (mapdef 1.2+; computed raw breakpoints with no `layout`/on-disk storage at all), through the `AXIS_DESCR`'s own `Conversion` like any other axis | Yes | Supported — see a2l-v0-plan.md's FIX_AXIS / CURVE_AXIS / RES_AXIS evidence |
| `AXIS_DESCR` `Attribute FIX_AXIS` with both or neither of `FIX_AXIS_PAR_DIST`/`FIX_AXIS_PAR_LIST`, a bare `FIX_AXIS_PAR` (a third, unevidenced mechanism), or a computed point count not matching `MaxAxisPoints` | Recognized | Rejected | No | No default is guessed when the mechanism is ambiguous or absent; a point-count mismatch is rejected rather than truncated/padded |
| `AXIS_DESCR` `Attribute CURVE_AXIS` + its `CURVE_AXIS_REF` (naming another `CHARACTERISTIC` whose own single axis is `STD_AXIS`/`COM_AXIS`) | Yes | `axes.x`/`axes.y`: `layout`/`count`/bounds/`conversion` all from the *referenced* `CHARACTERISTIC`'s own axis (authoritative — the referencing `AXIS_DESCR`'s own `Conversion` is always just a `NO_COMPU_METHOD` placeholder per real evidence) | Yes | Supported — see a2l-v0-plan.md's FIX_AXIS / CURVE_AXIS / RES_AXIS evidence |
| `AXIS_DESCR` `Attribute CURVE_AXIS` referencing a `CHARACTERISTIC` that doesn't exist, isn't a `CURVE` with exactly one `AXIS_DESCR`, or whose own axis is itself `CURVE_AXIS`/`FIX_AXIS`/`RES_AXIS` | Recognized | Rejected (`ErrCurveAxisNotFound` for "doesn't exist", `ErrUnsupportedAxisDescr` otherwise) | No | Chained `CURVE_AXIS` (borrowing an already-borrowed axis) is rejected rather than resolved recursively — no evidence it happens, and cyclic/unbounded chains are a real risk |
| `AXIS_DESCR` `Attribute RES_AXIS` + its `AXIS_PTS_REF`, when the referenced `AXIS_PTS`'s rescale pair count equals the resolved axis's `MaxAxisPoints` | Yes | `axes.x`/`axes.y`: `count`/bounds from the referenced `AXIS_PTS` (authoritative, same rule as `COM_AXIS`), `layout.address` past any padding to each pair's value element, `layout.strideBits` (mapdef 1.2+) to skip each pair's position element | Yes | Supported — see a2l-v0-plan.md's FIX_AXIS / CURVE_AXIS / RES_AXIS evidence |
| `AXIS_DESCR` `Attribute RES_AXIS` where the rescale pair count doesn't equal `MaxAxisPoints` | Recognized | Rejected (`ErrRescalePairCountMismatch`) | No | Whether a smaller pair count is meant to be piecewise-linearly interpolated into more axis points isn't established by any available evidence; guessing at interpolation semantics risks silently wrong calibration values, so this is rejected rather than guessed |
| `AXIS_DESCR`'s `InputQuantity` (a `MEASUREMENT` reference) | Yes | `axis.sourceMetadata.a2l.inputQuantity` | No | Preserved, never resolved — `MEASUREMENT` is not parsed |
| `AXIS_DESCR`'s `MONOTONY` | Yes | Not retained | No | Parsed, not yet enforced |
| `AXIS_DESCR`'s any other optional keyword (`MAX_GRAD`, ...) | No | Rejected | No | This slice only recognizes `MONOTONY`, `AXIS_PTS_REF`, `CURVE_AXIS_REF`, `FIX_AXIS_PAR_DIST`, `FIX_AXIS_PAR_LIST` |
| Top-level `AXIS_PTS` with exactly one `AXIS_PTS_X` `RECORD_LAYOUT` entry (optionally preceded by `NO_AXIS_PTS_X`), `IndexMode INDEX_INCR`, `AddrType DIRECT` | Yes | Resolved into a `COM_AXIS`'s `axes.x`/`axes.y` (see above) | Yes | Supported |
| Top-level `AXIS_PTS` with exactly one `AXIS_RESCALE_X` `RECORD_LAYOUT` entry (optionally preceded by `NO_RESCALE_X` and/or a `RESERVED` `BYTE` padding entry), `IndexMode INDEX_INCR`, `AddrType DIRECT` | Yes | Resolved into a `RES_AXIS`'s `axes.x`/`axes.y` (see above) | Yes | Supported |
| Top-level `AXIS_PTS`'s `DEPOSIT` (a `RECORD_LAYOUT`) with any other shape, or a mix of the `AXIS_PTS_X` and `AXIS_RESCALE_X` shapes, or `AXIS_PTS_Y`/`FNC_VALUES`/other roles present | Recognized | Rejected | No | This slice only supports the single-`AXIS_PTS_X` shape a `COM_AXIS` uses and the single-`AXIS_RESCALE_X` shape a `RES_AXIS` uses, never mixed |
| Top-level `AXIS_PTS`'s `DISPLAY_IDENTIFIER` | Yes | `sourceMetadata.a2l.axisPts`-adjacent provenance only (the `AXIS_PTS` name itself is preserved on the resolved axis) | No | Preserved, not interpreted |
| `CHARACTERISTIC`'s `Address` | Yes | `layout.address` | Yes | Supported |
| `CHARACTERISTIC`'s `Deposit` | Yes | Resolved to its `RECORD_LAYOUT` | Yes | Supported |
| `CHARACTERISTIC`'s `Conversion` (a `COMPU_METHOD` name, or the `NO_COMPU_METHOD` sentinel) | Yes | Resolved to its `COMPU_METHOD`, or `freehorse-expression-v1` `"X"` for the sentinel | Yes | Supported |
| `CHARACTERISTIC`'s `LowerLimit`/`UpperLimit` | Yes | `display.min`/`display.max` | No (informational only) | Same informational-only treatment as XDF's `<min>`/`<max>`; see xdf-compatibility.md's bounds evidence |
| `CHARACTERISTIC`'s `MaxDiff` | Yes | `sourceMetadata.a2l.maxDiff` | No | Preserved, not interpreted |
| `CHARACTERISTIC`'s `DISPLAY_IDENTIFIER` | Yes | `sourceMetadata.a2l.displayIdentifier` | No | Preserved, not interpreted |
| `CHARACTERISTIC`'s `EXTENDED_LIMITS` | Yes | `sourceMetadata.a2l.extendedLimits` | No | Preserved, not interpreted |
| `CHARACTERISTIC`'s any other optional keyword (`BIT_MASK`, `STEP_SIZE`, `READ_ONLY`, `CALIBRATION_ACCESS`, `DEPENDENT_CHARACTERISTIC`, `VIRTUAL_CHARACTERISTIC`, `SYMBOL_LINK`, `ANNOTATION`, ...) | No | Rejected | No | This slice only recognizes `FORMAT`, `DISPLAY_IDENTIFIER`, `EXTENDED_LIMITS`, `BYTE_ORDER`, `MATRIX_DIM`; any other optional keyword makes the whole `CHARACTERISTIC` fail to parse |
| `MEASUREMENT` | No | — | No | Read-only telemetry, not a calibration parameter; excluded as an explicit non-goal (live data — its address is RAM, needing live ECU communication `pkg/calibration` doesn't do), not merely deferred — see a2l-v0-plan.md |
| `GROUP` (`GroupName`/`GroupLongIdentifier`) + its `REF_CHARACTERISTIC` list | Yes | A `MapCategory` (`id` from `GroupName`, `name` from `GroupLongIdentifier`) plus `parameter.categories` membership for every named `CHARACTERISTIC` | Yes | Supported — see a2l-v0-plan.md's GROUP evidence |
| `GROUP`'s `ROOT`, `SUB_GROUP`, `REF_MEASUREMENT`, `FUNCTION_LIST` | Yes | Preserved on the internal model only — `SUB_GROUP` is not flattened into category membership (mapdef's `categories` has no parent/child relationship); `REF_MEASUREMENT`/`FUNCTION_LIST` reference constructs this slice doesn't parse | No | Parsed and preserved, not otherwise surfaced (`MapCategory` has no extension point) — see a2l-v0-plan.md's GROUP evidence |
| `GROUP`'s any other child (`ANNOTATION`, `IF_DATA`) | Recognized (per the A2L grammar) | Rejected | No | Not evidenced inside any real `GROUP` in the source file this slice was verified against |
| `FUNCTION` | No | — | No | Distinguishes characteristics a function *defines* from ones it only *depends on* — a different relationship than `GROUP`'s flat membership, needing its own design, not folded into `categories` — see a2l-v0-plan.md's GROUP evidence |
| `A2ML`/`IF_DATA` (outside `GROUP`) | No | — | No | Skipped as opaque; these use a brace-based sub-grammar this parser does not interpret |
| Everything else at `MODULE` level (`MOD_PAR`, `UNIT`, `FRAME`, `USER_RIGHTS`, `VARIANT_CODING`, a second `MODULE` in one file) — `FUNCTION` included | No | — | No | Skipped, not interpreted |

The synthetic fixture `pkg/a2l/testdata/basic.a2l` is authored for this
project, contains no production ECU data, and shares no text with any
third-party A2L file. Conversion tests cover identity, provenance, address,
layout, both supported conversion types, units, display bounds/decimal
places, source preservation, unsupported-construct rejection, and a full
decode/edit round trip through the unmodified `pkg/calibration` engine.

## Import/export policy

Same policy as [xdf-compatibility.md](xdf-compatibility.md#importexport-policy):
import only to the extent described by this matrix, unsupported information
is an explicit error (this slice does not yet have a diagnostic-and-preserve
path the way XDF import does), no A2L export, independent from and
unaffiliated with ASAM e.V.

## Redistribution and attribution

Same policy as
[xdf-compatibility.md](xdf-compatibility.md#redistribution-and-attribution),
applied with extra caution: even less of A2L's format is publicly documented
than XDF's, so the evidentiary basis for this parser (independent
open-source implementations, not the paywalled official specification) is
narrower. This is engineering-level diligence, not legal advice — see
[a2l-v0-plan.md](a2l-v0-plan.md#whats-legally-available) for what was and
was not used to write this parser, and xdf-compatibility.md's "Evidence and
unresolved legal questions" section for the caveats that apply here too.
