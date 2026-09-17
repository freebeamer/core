# A2L import — V0 plan

Date: 2026-09-14

Status: In progress. Follow-on slices after the initial scalar-only import:
multi-byte types with `BYTE_ORDER` support (see "Byte order evidence"),
`CURVE`/`MAP` characteristics with embedded `STD_AXIS` axes (see
"CURVE/MAP evidence"), shared axes via `COM_AXIS`/`AXIS_PTS` (see
"COM_AXIS / AXIS_PTS evidence"), value-to-text axis conversions via
`TAB_VERB`/`COMPU_VTAB`/`COMPU_VTAB_RANGE` (see "TAB_VERB / COMPU_VTAB
evidence"), computed/borrowed/rescale axes via `FIX_AXIS`/`CURVE_AXIS`/
`RES_AXIS` (see "FIX_AXIS / CURVE_AXIS / RES_AXIS evidence"), a
1-2 dimensional block characteristic with no axes at all via `VAL_BLK`
(see "VAL_BLK evidence"), category membership via `GROUP` (see "GROUP
evidence"), and the remaining numeric `COMPU_METHOD` conversion types —
`RAT_FUNC`, `FORM`, `TAB_INTP`, `TAB_NOINTP` (see "RAT_FUNC / FORM /
COMPU_TAB evidence").

Item 4 of [mapdef-backlog.md](mapdef-backlog.md) asks for "an A2L import
adapter against a legally available ASAM specification." Before writing any
parser, this plan records what's actually available and scopes a first,
intentionally small slice, the same way [xdf-v0.md](xdf-v0.md) and the
original `FreeBeamer_Codex_V0_Plan.md` scoped the XDF/BIN engine before
building the rest of it.

## What's legally available

The official ASAM MCD-2 MC (A2L/ASAP2) specification is **not** freely
published. `asam.net/standards/detail/mcd-2-mc/` gates the actual standard
document behind membership login or purchase; the public page lists keyword
names but not the grammar. No complete official text was used to write this
plan or the parser it describes.

Instead, like XDF, this relies on independent, redistributable evidence:

- `pyA2L` (MIT-licensed, <https://github.com/christoph2/pyA2L>) ships a
  substantial example file, `examples/ASAP2_Demo_V161.a2l`, covering scalar
  `CHARACTERISTIC`/`COMPU_METHOD`/`RECORD_LAYOUT` blocks with real,
  syntactically valid A2L text. This plan's understanding of exact keyword
  order and syntax comes from reading that file directly (fetched and
  inspected, not copied into this repository).
- Other independent open-source A2L implementations exist (`a2llib`,
  `A2LReader`, a Dart `a2l` library) as secondary corroboration for the
  general block structure, though this plan's syntax details were verified
  against `pyA2L`'s example specifically.

`pkg/a2l/testdata/basic.a2l` is authored from scratch for this project,
containing no text copied from `pyA2L` or any other source, matching how
`pkg/xdf/testdata/basic.xdf` was handled. This is engineering-level
diligence, not legal advice — see xdf-compatibility.md's "Evidence and
unresolved legal questions" section for the same caveat applied to XDF; it
applies here too, and more sharply, since even less of A2L is publicly
documented than XDF.

## Goal

The smallest useful A2L import slice: parse a single scalar
`CHARACTERISTIC` (A2L's `Type VALUE`) with an `IDENTICAL` or `LINEAR`
`COMPU_METHOD` and a single-byte `RECORD_LAYOUT`, and convert it to a mapdef
scalar parameter — reusing the existing `pkg/calibration` decode/edit engine
unchanged, exactly as the XDFCONSTANT and XDFFLAG work did.

No GUI/desktop wiring, no A2L export, no `MEASUREMENT`/`FUNCTION`
support. `CURVE`/`MAP` characteristics **are** supported for the embedded
(`STD_AXIS`), shared (`COM_AXIS`/`AXIS_PTS`), computed (`FIX_AXIS`), shared
rescale (`RES_AXIS`/`AXIS_PTS`), and borrowed (`CURVE_AXIS`) axis cases —
see "CURVE/MAP evidence", "COM_AXIS / AXIS_PTS evidence", and "FIX_AXIS /
CURVE_AXIS / RES_AXIS evidence" below for what that covers and what it
deliberately still rejects. `GROUP` (an A2L analog of XDF `<CATEGORY>`)
**is** supported — see "GROUP evidence" below.

## Non-goals for this slice

- `MEASUREMENT` — read-only telemetry channel declarations. These describe
  live/logged data, not calibration parameters; live data is an explicit
  FreeBeamer non-goal (see `FreeBeamer_Codex_V0_Plan.md`), so this is excluded
  on the same grounds axis "alarms/visibility/DAQ links" were in the XDF
  matrix, not merely deferred for lack of time. `MEASUREMENT`'s address
  (`ECU_ADDRESS`) is RAM, not the flash/ROM `CHARACTERISTIC.Address` this
  project's whole model reads from a static binary — supporting it for
  real would mean live ECU communication, a different subsystem entirely,
  not an extension of `pkg/calibration`.
- `FUNCTION` — groups characteristics/measurements by the software
  function that uses them, distinguishing ones a function *defines*
  (`DEF_CHARACTERISTIC`) from ones it only *depends on*
  (`REF_CHARACTERISTIC`/`IN_MEASUREMENT`/`OUT_MEASUREMENT`), plus nested
  `SUB_FUNCTION`s. That's a genuinely different relationship than `GROUP`'s
  (a flat "this parameter belongs to this tag" membership, the one
  `mapdef`'s `categories` field already represents for XDF `<CATEGORY>`)
  — folding both into one category tag would conflate distinct meanings,
  so `FUNCTION` remains unparsed pending its own design, not merely
  deferred for lack of time. See "GROUP evidence" below.
- `ASCII`/`CUBOID`/`CUBE_4`/`CUBE_5` characteristic types (strings and 3-5
  dimensional calibrations) — `CURVE`/`MAP` (1-2 dimensions, see "CURVE/MAP
  evidence" below) and `VAL_BLK` (a 1-2 dimensional block with no
  `AXIS_DESCR` at all, see "VAL_BLK evidence" below) are supported.
  `CUBOID`/`CUBE_4`/`CUBE_5` are genuinely 3-5 dimensional (`CUBOID` has
  real evidence of 3 real `AXIS_DESCR` blocks) and would need a real
  N-dimensional data model across `pkg/calibration`'s `DecodedTable`
  (`Z`/`Raw.Z`/`Addresses.Z` are hardcoded 2D today) and a mapdef schema
  change (`axes` only allows `x`/`y`) — a bigger, separate architectural
  decision, not a converter-only slice like every axis attribute above.
- Within a supported `CURVE`/`MAP`: `AXIS_PTS_X`/`AXIS_PTS_Y` `IndexMode`
  `INDEX_DECR` (descending storage order — decoding it correctly needs
  reversing the axis array, which this slice's addressing model doesn't
  do); any `RECORD_LAYOUT` `AddrType` other than `DIRECT` (pointer
  indirection). `STD_AXIS`, `COM_AXIS`, `FIX_AXIS`, `RES_AXIS`, and
  `CURVE_AXIS` are all supported — see "COM_AXIS / AXIS_PTS evidence" and
  "FIX_AXIS / CURVE_AXIS / RES_AXIS evidence" below — but a `CURVE_AXIS`
  chained onto another `CURVE_AXIS` (borrowing an already-borrowed axis) is
  rejected, and a `RES_AXIS` whose rescale pair count doesn't match its
  axis point count is rejected rather than guessed at (see that evidence
  section for why).
- `COMPU_METHOD` `ConversionType`s beyond `IDENTICAL`/`LINEAR`/`TAB_VERB`/
  `RAT_FUNC`/`FORM`/`TAB_INTP`/`TAB_NOINTP` — the real example file
  exercises exactly these seven, and this slice now implements all of
  them; see "TAB_VERB / COMPU_VTAB evidence" and "RAT_FUNC / FORM /
  COMPU_TAB evidence" below for how the latter five map onto mapdef.
  `TAB_VERB` used as a *characteristic's own* top-level `Conversion`
  (rather than an axis's) is explicitly rejected: mapdef's `staticValues`
  field lives only on `MapAxis`, not `MapParameter`, so a `CHARACTERISTIC`
  whose own Z data is `TAB_VERB`-labeled has no mapdef representation yet
  and needs a schema addition, not a silent drop, to support.
  `COMPU_VTAB_RANGE` (range-keyed text lookup, a distinct block shape from
  `COMPU_VTAB`'s single-value keying, that a `TAB_VERB` `COMPU_METHOD` can
  reference exactly the same way) is parsed and preserved, but a
  `Conversion` resolving to one is rejected for the same reason as
  characteristic-level `TAB_VERB`: no range-keyed representation exists in
  mapdef's `staticValues` yet — see "TAB_VERB / COMPU_VTAB evidence"
  below. A genuinely non-affine `RAT_FUNC`/`FORM` conversion (or `FORM`
  syntax outside `freehorse-expression-v1`'s small grammar) still decodes
  correctly but cannot be edited — the same precedent XDF's arbitrary
  `MATH` formulas already established, not a new limitation this slice
  introduces — see "RAT_FUNC / FORM / COMPU_TAB evidence" below.
- `FLOAT32_IEEE`/`FLOAT64_IEEE` — mapdef's `layout.dataType` enum has no
  float member (XDF import has the same limitation, for the same reason:
  deferred until an actual fixture needs it). `UWORD`/`SWORD`/`ULONG`/
  `SLONG` integer types **are** supported — see "Byte order evidence" below
  for why that was initially excluded here and isn't anymore.
- `A2ML`/`IF_DATA` (vendor extension schema/payload) — skipped as opaque,
  balanced `/begin ... /end` blocks, never interpreted. This mirrors how the
  XDF parser lets unknown XML elements pass through unread.
- Anything at `PROJECT`/`MODULE` level beyond `HEADER` (title/description)
  and the `CHARACTERISTIC`/`COMPU_METHOD`/`RECORD_LAYOUT`/`AXIS_PTS`/
  `COMPU_VTAB`/`COMPU_VTAB_RANGE`/`COMPU_TAB`/`GROUP` blocks this slice
  reads: `FUNCTION`, `UNIT`, `FRAME`, `USER_RIGHTS`, and multiple
  `MODULE`s in one file.

## Byte order evidence

The initial version of this slice excluded every multi-byte data type
because A2L's byte-order convention hadn't been researched yet, and this
project does not guess endianness. Two independent open-source A2L
implementations, read directly (never copied), corroborate both where
`BYTE_ORDER` lives and its legal values:

- `pyA2L`'s `pya2l/classes.py` defines a shared `Byteorder` enum —
  `LITTLE_ENDIAN`, `BIG_ENDIAN`, `MSB_LAST`, `MSB_FIRST`,
  `MSB_FIRST_MSW_LAST`, `MSB_LAST_MSW_FIRST` — and lists `BYTE_ORDER` as a
  recognized optional keyword not just on `MOD_COMMON` but directly on
  `CHARACTERISTIC`, `MEASUREMENT`, `AXIS_DESCR`, and `AXIS_PTS`:
  <https://github.com/christoph2/pyA2L/blob/master/pya2l/classes.py>
- `kallemooo/Asap2` (an independent C#/YACC implementation) confirms the
  same placement: `CHARACTERISTIC.cs` declares a `byte_order` field, and its
  grammar (`Asap2.Language.grammar.y`) has a `byte_order` production reduced
  into `characteristic_data`, `axis_descr`, `axis_pts_data`, `mod_common_data`,
  and `measurement_data` alike:
  <https://github.com/kallemooo/Asap2/blob/master/Asap2/Asap2Tree/CHARACTERISTIC.cs>,
  <https://github.com/kallemooo/Asap2/blob/master/Asap2/Asap2.Language.grammar.y>
- `pyA2L`'s own example file demonstrates typical real usage: a single
  `/begin MOD_COMMON "" DEPOSIT ABSOLUTE BYTE_ORDER MSB_LAST
  ALIGNMENT_BYTE 1 ALIGNMENT_WORD 2 ALIGNMENT_LONG 4
  ALIGNMENT_FLOAT32_IEEE 4 ALIGNMENT_FLOAT64_IEEE 4 /end MOD_COMMON` block
  declaring the module-wide default, with no per-item override present in
  that particular file.

This slice therefore supports `BYTE_ORDER` in the two places evidenced with
real examples — `MOD_COMMON` (module-wide default) and `CHARACTERISTIC`
(overrides the module default when present) — and maps only the
unambiguous values: `LITTLE_ENDIAN`/`MSB_LAST` → `"little"`,
`BIG_ENDIAN`/`MSB_FIRST` → `"big"`. `MSB_FIRST_MSW_LAST`/
`MSB_LAST_MSW_FIRST` (mixed word/byte order — the byte order within a word
differs from the word order within a 32-bit long) are recognized but
rejected: mapdef's `layout.byteOrder` enum has no representation for a
mixed order, and approximating one would be exactly the kind of guess this
project avoids. A multi-byte `CHARACTERISTIC` with no `BYTE_ORDER`
resolvable from either place is also rejected outright rather than assumed
to default to one endianness or the other, since no evidence pinned down
what A2L's default behavior is when the keyword is omitted entirely.
`MOD_COMMON`'s other keywords observed in the same real example
(`DEPOSIT`, `ALIGNMENT_BYTE`, `ALIGNMENT_WORD`, `ALIGNMENT_LONG`,
`ALIGNMENT_FLOAT32_IEEE`, `ALIGNMENT_FLOAT64_IEEE`) are parsed and
discarded — recognized only so a typical real `MOD_COMMON` block doesn't
fail to parse, not because this slice does anything with them.

## CURVE/MAP evidence

The initial version of this slice excluded `CURVE`/`MAP` entirely: they
need axis storage information the flat scalar `RECORD_LAYOUT` shape doesn't
have. Studying `pyA2L`'s example file directly (never copied) supplied both
the `CHARACTERISTIC`-level shape and the `RECORD_LAYOUT` shape needed to
decode the common case:

- A `CURVE`/`MAP` `CHARACTERISTIC` nests one (`CURVE`) or two (`MAP`, X then
  Y by declaration order) `/begin AXIS_DESCR ... /end AXIS_DESCR` blocks
  after its usual 9 positional fields:
  `<Attribute> <InputQuantity> <Conversion> <MaxAxisPoints> <LowerLimit>
  <UpperLimit>`, then optional keywords (`MONOTONY` is the only one this
  slice recognizes).
- `Attribute` `STD_AXIS` means the axis's breakpoints are stored directly in
  the enclosing `RECORD_LAYOUT`, addressed relative to the
  `CHARACTERISTIC`'s own `Address` — no separate lookup needed. The other
  attributes seen in the example (`COM_AXIS` via `AXIS_PTS_REF`, `FIX_AXIS`
  via `FIX_AXIS_PAR_DIST`/`FIX_AXIS_PAR_LIST`, `RES_AXIS`, `CURVE_AXIS` via
  `CURVE_AXIS_REF`) each need a mechanism this slice doesn't implement
  (a standalone top-level `AXIS_PTS` block, or computed/formula-based axis
  generation with no binary storage at all) and are rejected explicitly.
- A `STD_AXIS` `RECORD_LAYOUT` adds `AXIS_PTS_X`/`AXIS_PTS_Y` entries (one
  per axis) alongside `FNC_VALUES`, and optionally `NO_AXIS_PTS_X`/
  `NO_AXIS_PTS_Y` (a stored byte giving the *current*, possibly smaller,
  point count — this slice does not decode that field; every axis is always
  treated as exactly `MaxAxisPoints` long). Real example:
  `RL.MAP.SWORD.SBYTE.SBYTE.INCR` has `NO_AXIS_PTS_X 1 UBYTE`,
  `NO_AXIS_PTS_Y 2 UBYTE`, `AXIS_PTS_X 3 SBYTE INDEX_INCR DIRECT`,
  `AXIS_PTS_Y 4 SBYTE INDEX_INCR DIRECT`, `FNC_VALUES 5 SWORD ROW_DIR
  DIRECT`, with a comment confirming the memory layout this implies: "1x
  Byte for no of axis points, 8x Byte for axis points + 8x word for output
  values."

That comment is the key fact this slice relies on: each entry's leading
number (`1`, `2`, `3`, ...) is an **ordinal position**, not a byte offset.
The actual byte offset of an entry is the cumulative size of every
lower-positioned entry, where an axis entry's size is
`element size × that axis's MaxAxisPoints` and `FNC_VALUES`'s size is
`element size × (product of every axis's MaxAxisPoints)`. `pkg/a2l/converter.go`'s
`planRecordLayout` computes this explicitly rather than assuming any fixed
offset. This reuses mapdef's existing independently-addressed-axis shape
(the same one XDF's addressed X/Y axes use) — no calibration-engine changes
were needed, the same as every other A2L/XDF slice so far.

Two further restrictions, both because getting them wrong would silently
misalign data rather than just fail to parse: only `AXIS_PTS_X`/
`AXIS_PTS_Y` `IndexMode` `INDEX_INCR` is supported (`INDEX_DECR` — seen in
the same real example, e.g. `RL.CURVE.SWORD.SBYTE.DECR` — would need the
stored axis array read in reverse, which this slice's addressing model has
no way to express); only `AddrType` `DIRECT` is supported (no pointer
indirection).

Row/column convention: for a `MAP`, `layout.dimensions` is `[Y count, X
count]` (rows = Y, columns = X), the conventional Cartesian mapping (X
varies across columns, Y varies down rows). This is a stated convention,
not something A2L itself labels — unlike XDF's `id="x"`/`id="y"` attributes,
A2L identifies axes purely by `AXIS_DESCR` declaration order.

## COM_AXIS / AXIS_PTS evidence

`STD_AXIS` covers an axis stored alongside its characteristic's own data.
`COM_AXIS` is A2L's mechanism for a *shared* axis: several characteristics
(commonly several maps sharing the same RPM breakpoints) reference one
standalone, top-level `AXIS_PTS` block instead of each storing their own
copy. The real example file demonstrates the shape directly:

```text
/begin AXIS_PTS ASAM.C.AXIS_PTS.UBYTE_8
  "Common axis for "
  0x810340
  ASAM.M.SCALAR.SBYTE.IDENTICAL      /* will be overwritten by input quantity of AXIS_DESCR */
  RL.AXIS_PTS.SBYTE.DECR
  0
  CM.IDENTICAL                      /* will be overwritten by computation method of AXIS_DESCR */
  8                                  /* -- see the AXIS_DESCR side of this same comment below */
  -128 127
  DISPLAY_IDENTIFIER DI.ASAM.C.AXIS_PTS.UBYTE_8
/end AXIS_PTS
```

referenced by:

```text
/begin AXIS_DESCR
  COM_AXIS
  ASAM.M.SCALAR.SBYTE.IDENTICAL
  CM.IDENTICAL
  8                                  /* will be overwritten by max number of axis points of AXIS_PTS */
  -128 127
  AXIS_PTS_REF ASAM.C.AXIS_PTS.UBYTE_8
/end AXIS_DESCR
```

`AXIS_PTS` has the same 9-positional-field shape as `CHARACTERISTIC` minus
`Type`, plus `InputQuantity` and `MaxAxisPoints`:
`<Name> "<LongIdentifier>" <Address> <InputQuantity> <Deposit> <MaxDiff>
<Conversion> <MaxAxisPoints> <LowerLimit> <UpperLimit>`. Its `Deposit`
points to a `RECORD_LAYOUT` containing only an `AXIS_PTS_X` entry (real
example: `RL.AXIS.UBYTE` has just `AXIS_PTS_X 1 UBYTE INDEX_INCR DIRECT`) —
structurally axis-agnostic, always calling its one entry `AXIS_PTS_X`
regardless of whether the referencing `CHARACTERISTIC` uses it as an X or Y
axis. `pkg/a2l/converter.go`'s `planAxisPtsLayout` resolves this the same
way `planRecordLayout` resolves an embedded axis, just addressed relative
to the `AXIS_PTS`'s own `Address` instead of the characteristic's.

The two comments quoted above are the key fact this slice relies on for
which side is authoritative: the `AXIS_PTS`'s own `InputQuantity`/
`Conversion` are explicitly flagged as placeholders "overwritten by" the
referencing `AXIS_DESCR`, so this slice uses the `AXIS_DESCR`'s
`Conversion` (same as the embedded case). Symmetrically, the `AXIS_DESCR`'s
own `MaxAxisPoints` is flagged as "overwritten by" the `AXIS_PTS` — this
slice uses the `AXIS_PTS`'s `MaxAxisPoints`, `Address`, and `Deposit`
(unambiguous: they define where and how the shared data is actually
stored). `LowerLimit`/`UpperLimit` carry no such comment in this example
(both sides happen to have the same values, `-128 127`), so which one is
truly authoritative isn't nailed down by this evidence; this slice treats
the `AXIS_PTS`'s bounds as authoritative on the reasoning that a shared
axis's valid range is a property of the shared storage, not of any one
consumer — a considered choice, not blind copying, and flagged here as
such rather than silently assumed.

`FIX_AXIS` (computed via `FIX_AXIS_PAR_DIST`/`FIX_AXIS_PAR_LIST`, no binary
storage at all), `RES_AXIS` (a rescale axis with value/position pairs, a
differently-shaped `AXIS_PTS`/`RECORD_LAYOUT` than the one above), and
`CURVE_AXIS` (borrows another curve's own axis via `CURVE_AXIS_REF`) are
each a distinct mechanism, not a variation on `COM_AXIS` — see "FIX_AXIS /
CURVE_AXIS / RES_AXIS evidence" below for how each is supported.

## FIX_AXIS / CURVE_AXIS / RES_AXIS evidence

Each of these is a genuinely different mechanism from `COM_AXIS`, so each
got its own evidence-gathering pass against the real example file and,
where the file's own comments were silent, `pya2l/classes.py`'s class
definitions as a second independent source.

**`FIX_AXIS`** has no binary storage at all — its breakpoints are computed,
not read. The real example file shows two mechanisms, both under the
`FIX_AXIS` attribute:

```text
/begin AXIS_DESCR
  FIX_AXIS
  ASAM.M.SCALAR.SBYTE.IDENTICAL
  CM.IDENTICAL
  6
  -128 127
  FIX_AXIS_PAR_DIST 1 1 6
/end AXIS_DESCR
```

`FIX_AXIS_PAR_DIST <Offset> <Distance> <NumberPts>` computes breakpoint `i`
(for `i` in `[0, NumberPts)`) as `Offset + i*Distance`. The alternative,
`FIX_AXIS_PAR_LIST`, is a nested block listing the raw breakpoints
literally:

```text
/begin AXIS_DESCR
  FIX_AXIS
  ASAM.M.SCALAR.SBYTE.IDENTICAL
  CM.IDENTICAL
  6                                  /* shall match the number of axis points listed in FIX_AXIS_PAR_LIST */
  -128 127
  /begin FIX_AXIS_PAR_LIST
    -1 4 6 8 9 10
  /end FIX_AXIS_PAR_LIST
/end AXIS_DESCR
```

The comment on the second example is the key fact this slice relies on:
the count must match the number of points the chosen mechanism actually
produces, so `pkg/a2l/converter.go`'s `resolveAxisSpec` rejects a mismatch
rather than truncating or padding. Whichever mechanism is present, the
resulting raw values still go through the `AXIS_DESCR`'s own `Conversion`
exactly like any other axis (the comment "shall be the same computation as
used with the input quantity" applies here too) — computing the
breakpoints replaces reading them from a file address, nothing else. A
bare `FIX_AXIS_PAR` keyword (a third mechanism, `Offset`/`Shift`/`Numberapo`
per `pya2l/classes.py`, apparently a power-of-two-spaced variant of
`FIX_AXIS_PAR_DIST`) is listed in `pya2l/classes.py`'s grammar but never
actually used in the real example file, so it is not implemented — no
evidence exists for what its `Shift` field actually means at runtime.

Representing a formula/list-derived axis with no address at all needed a
mapdef schema addition: `axis.fixedPoints` (mapdef 1.2+), the axis's raw
breakpoints given directly rather than computed from `layout`. `pkg/a2l`
computes these once at conversion time; `pkg/calibration`'s `decodeAxis`
applies the axis's `Conversion` to each one directly, with no file read
and no address, when `Layout` is absent and `FixedPoints` is present.

**`CURVE_AXIS`** borrows another `CHARACTERISTIC`'s own axis outright:

```text
/begin CHARACTERISTIC ASAM.C.CURVE.CURVE_AXIS
  "Curve with curve axis"
  CURVE
  0x810380
  RL.FNC.SWORD.ROW_DIR
  0
  CM.IDENTICAL
  -32268 32267
  /begin AXIS_DESCR
    CURVE_AXIS
    ASAM.M.SCALAR.SBYTE.IDENTICAL
    NO_COMPU_METHOD                                    /* CURVE_AXIS have no input conversion */
    8                                                   /* will be overwritten by max number of axis points of AXIS_PTS */
    -128 127
    CURVE_AXIS_REF ASAM.C.CURVE_AXIS
  /end AXIS_DESCR
/end CHARACTERISTIC

/begin CHARACTERISTIC ASAM.C.CURVE_AXIS
  "Curve used as axis"
  CURVE
  0x810390
  RL.CURVE.SWORD.SBYTE.INCR
  0
  CM.IDENTICAL
  -32268 32267
  /begin AXIS_DESCR
    STD_AXIS
    ASAM.M.SCALAR.SBYTE.IDENTICAL
    CM.IDENTICAL
    8
    -128 127
  /end AXIS_DESCR
/end CHARACTERISTIC
```

The comment "`CURVE_AXIS` have no input conversion" on the *referencing*
`AXIS_DESCR`'s own `NO_COMPU_METHOD` is the key fact here — the opposite
convention from `COM_AXIS`, where the referencing side's `Conversion` is
authoritative. For `CURVE_AXIS` the referencing side's `Conversion` is
never real data, so `pkg/a2l/converter.go`'s `resolveAxisSpec` ignores it
entirely and uses the *referenced* `CHARACTERISTIC`'s own single axis
`Conversion` (and, per the "will be overwritten by" comment, its
`MaxAxisPoints`/bounds too) instead. The referenced axis must itself be
`STD_AXIS` or `COM_AXIS`: chaining `CURVE_AXIS` onto another `CURVE_AXIS`
(borrowing an already-borrowed axis) is rejected rather than resolved
recursively, since no real evidence shows this actually happens and
unbounded/cyclic reference chains are a real risk to guard against
regardless. Resolving a borrowed `STD_AXIS` needs the referenced
`CHARACTERISTIC`'s own `RECORD_LAYOUT` fully planned (its address depends
on the cumulative size of every lower-positioned entry in that other
characteristic's own layout, same as always) — `resolveAxisSpec` calls
`planRecordLayout` on the referenced characteristic to get it. No mapdef
schema change was needed: `CURVE_AXIS` resolves to the exact same
`axes.x`/`y.layout` shape `COM_AXIS` does, just sourced from a different
place.

**`RES_AXIS`** is referenced via `AXIS_PTS_REF` exactly like `COM_AXIS`,
but the standalone `AXIS_PTS` it points at stores rescale (position,
value) pairs instead of plain breakpoints:

```text
/begin RECORD_LAYOUT RL.AXIS_PTS.RES_AXIS
  NO_RESCALE_X      1 UBYTE
  RESERVED          2 BYTE                             /* to adapt the start of the rescale pairs to an even address */
  AXIS_RESCALE_X    3 UBYTE 5 INDEX_INCR DIRECT
/end RECORD_LAYOUT

/begin AXIS_PTS ASAM.C.AXIS_PTS.RESCALE
  "Rescale Axis"
  0x8103B0
  ASAM.M.SCALAR.SBYTE.IDENTICAL
  RL.AXIS_PTS.RES_AXIS
  0
  CM.IDENTICAL
  5
  0 255
/end AXIS_PTS
```

`AXIS_RESCALE_X`'s positional shape is `<Position> <DataType>
<MaxNumberOfRescalePairs> <IndexMode> <AddrType>` — one more field than
`AXIS_PTS_X`'s, confirmed independently by `pya2l/classes.py`'s
`AXIS_RESCALE_X` class (and its `AXIS_RESCALE_Y`/`_Z`/`_4`/`_5` siblings,
none of which this slice needs). What exactly a rescale pair's `position`
element means at runtime — whether every pair maps 1:1 to one axis point,
or a smaller set of pairs is meant to be piecewise-linearly interpolated
into a larger number of axis points — is **not** established by this
evidence: the only real example has `MaxNumberOfRescalePairs` (5) exactly
equal to the referencing `AXIS_DESCR`'s resolved `MaxAxisPoints` (5, via
the same "will be overwritten by" `COM_AXIS`-style authority rule), so it
cannot distinguish the two interpretations. Rather than guess at
interpolation semantics with no supporting evidence — a wrong guess here
would silently produce incorrect calibration values, not just a code
defect — this slice supports only the case the evidence actually confirms:
`AXIS_RESCALE_X`'s own `MaxNumberOfRescalePairs` must equal the resolved
axis's `MaxAxisPoints`, decoded as a direct 1:1 pairs-to-points mapping
(each point's raw value is its pair's second element; the first is
ignored). A pair-count mismatch is rejected explicitly
(`ErrRescalePairCountMismatch`), not misread as some other error or
silently truncated.

Reading only every other element (skipping each pair's first, position
element) needed a second mapdef schema addition: `layout.strideBits` (the
field existed since `staticValues`/`decimalPlaces` in 1.1.0 but was always
rejected whenever non-zero) now has real decode semantics for an axis —
an explicit per-element byte distance overriding the data type's natural
width, letting `pkg/calibration`'s `decodeAxis` step over the interleaved
position elements. `pkg/a2l/converter.go`'s `planAxisPtsRescaleLayout`
computes the resulting `Address` (past any `NO_RESCALE_X`/`RESERVED`
padding, then one more element to land on the first pair's own value) and
`StrideBits` (twice the natural element width). A parameter's own Z data
still rejects any non-zero stride unconditionally: no evidence needs that
yet, and this project doesn't add capabilities ahead of a concrete need.
`RESERVED`'s `BYTE` data type (the padding entry's real evidenced purpose:
"to adapt the start of the rescale pairs to an even address") is the only
`RESERVED` shape this slice recognizes.

## TAB_VERB / COMPU_VTAB evidence

`TAB_VERB` is a `COMPU_METHOD` conversion type for value-to-text lookup
(e.g. a raw gear index displaying as "Park"/"Reverse"/"Neutral"/"Drive"),
analogous to XDF's `<LABEL>` axis mechanism that mapdef's `staticValues`
field was originally added for. `pyA2L`'s example file and `pya2l/classes.py`
both show the same two-block shape: a `COMPU_METHOD` with conversion type
`TAB_VERB` pointing via `COMPU_TAB_REF` at a separate top-level `COMPU_VTAB`
block that carries the actual value/text pairs:

```text
/begin COMPU_METHOD CM.TAB_VERB.GEAR
  "Gear position labels"
  TAB_VERB "%3.0" ""
  COMPU_TAB_REF CM.TAB_VERB.GEAR.REF
/end COMPU_METHOD

/begin COMPU_VTAB CM.TAB_VERB.GEAR.REF
  "Gear position text"
  TAB_VERB 4
  0 "Park"
  1 "Reverse"
  2 "Neutral"
  3 "Drive"
  DEFAULT_VALUE "unknown gear"
/end COMPU_VTAB
```

The `COMPU_VTAB` block's own positional shape is `<Name> "<LongIdentifier>"
<ConversionType> <NumberValuePairs> (<Value> "<Text>")+`, followed by the
optional `DEFAULT_VALUE "<text>"` keyword for values with no matching entry.
Only the `TAB_VERB` shape of `COMPU_VTAB` (as opposed to `TAB_VERB_RANGE`,
used by the distinct `COMPU_VTAB_RANGE` block) is read; this slice's
`parseCompuVtab` diagnoses anything else rather than misreading it.

Because mapdef's `staticValues` field exists only on `MapAxis`, this slice
supports `TAB_VERB` fully when it is an `AXIS_DESCR`'s `Conversion` (each
`COMPU_VTAB` entry becomes one `types.StaticValue{Index, Value}`, with the
underlying raw axis point decoded normally — the lookup is purely a display
overlay, not a change to how the axis is stored or decoded) and explicitly
rejects it when it is a `CHARACTERISTIC`'s own top-level `Conversion`,
since there is nowhere in mapdef's `MapParameter` shape to put the
resulting `staticValues` yet. `DEFAULT_VALUE` is parsed and preserved but
has no mapdef equivalent to attach to today (no "no value matches" sentinel
in `StaticValues`), so it is dropped rather than guessed at.

A `TAB_VERB` `COMPU_METHOD`'s `COMPU_TAB_REF` is not guaranteed to point at
a `COMPU_VTAB` — the real example file shows the identical `COMPU_METHOD`
shape (`TAB_VERB "<Format>" "<Unit>"` plus `COMPU_TAB_REF`) used to
reference a `COMPU_VTAB_RANGE` block instead, for a range-keyed rather than
exact-value-keyed lookup:

```text
/begin COMPU_VTAB_RANGE CM.VTAB_RANGE.DEFAULT_VALUE.REF
   ""
   11
   0 1 "Zero_to_one"
   2 3 "two_to_three"
   ...
   DEFAULT_VALUE "out of range value"
/end COMPU_VTAB_RANGE
```

`COMPU_VTAB_RANGE`'s positional shape is `<Name> "<LongIdentifier>"
<NumberOfValueTriples> (<LowerValue> <UpperValue> "<Text>")+
[DEFAULT_VALUE "<Text>"]` — notably, unlike `COMPU_VTAB`, there is no
leading `TAB_VERB` literal keyword before the count; `pya2l/classes.py`'s
`COMPU_VTAB_RANGE` class definition confirms this (no `ConversionType`
attribute, unlike its `COMPU_VTAB` sibling which lists one), corroborating
the real example file's syntax with a second, independent source. This
slice parses `COMPU_VTAB_RANGE` (so it's recognized and its data preserved,
not skipped as opaque) but explicitly rejects any `TAB_VERB` `Conversion`
whose `COMPU_TAB_REF` resolves to one, with a dedicated error
(`ErrCompuVtabRangeUnsupported`) distinct from "not found": mapdef's
`staticValues` field is keyed by a single index, not an inclusive range, so
representing a `COMPU_VTAB_RANGE` correctly needs either a mapdef schema
addition for range-keyed labels or a documented, bounded expansion policy —
neither exists yet, so this is a "recognize and reject," not a guess.

## VAL_BLK evidence

`VAL_BLK` is a `CHARACTERISTIC` `Type` with no `AXIS_DESCR` blocks at
all — its shape comes from a `MATRIX_DIM` keyword instead:

```text
/begin CHARACTERISTIC ASAM.C.ARRAY.SWORD.MATRIX_DIM_3_4.ROW_DIR
  "Array 3x4 of SWORD"
  VAL_BLK
  0x810100
  RL.FNC.SWORD.ROW_DIR
  0
  CM.IDENTICAL
  -400 400
  EXTENDED_LIMITS -1000 1000
  MATRIX_DIM 3 4 1
  FORMAT "%8.4"
  DISPLAY_IDENTIFIER DI.ASAM.C.ARRAY.SWORD.MATRIX_DIM_3_4.ROW_DIR
/end CHARACTERISTIC

/begin CHARACTERISTIC ASAM.C.ARRAY.SWORD.MATRIX_DIM_6.ROW_DIR
  "Array 6 of SWORD"
  VAL_BLK
  0x810140
  RL.FNC.SWORD.ROW_DIR       /* COLUMN_DIR results here in the same memory
                                usage as only one dimension is used */
  0
  CM.IDENTICAL
  -400 400
  EXTENDED_LIMITS -1000 1000
  MATRIX_DIM 6 1 1
  FORMAT "%8.4"
  DISPLAY_IDENTIFIER DI.ASAM.C.ARRAY.SWORD.MATRIX_DIM_6.ROW_DIR
/end CHARACTERISTIC
```

`MATRIX_DIM <x> <y> <z>` gives the block's shape directly. Both real
examples have `z = 1` (a 1- or 2-dimensional block); the file's own
comment on the second example — that `ROW_DIR`/`COLUMN_DIR` "results in
the same memory usage" once a dimension is 1 — is the only comment
anywhere near `VAL_BLK`'s actual runtime semantics, and it's silent on
what a genuinely 3-dimensional `MATRIX_DIM` (all three values greater than
1) would mean for storage order. Rather than guess, this slice supports
only the evidenced case, `z == 1`, and rejects anything else
(`ErrUnsupportedCharacteristicType`) — the same "recognize and reject"
treatment `RES_AXIS`'s pair-count mismatch gets, for the same reason: no
real evidence to build a genuine 3D case on.

`RL.FNC.SWORD.ROW_DIR`/`RL.FNC.SWORD.COLUMN_DIR` are the same generic
single-`FNC_VALUES` `RECORD_LAYOUT` shape a scalar `VALUE` uses — nothing
about the `RECORD_LAYOUT` distinguishes `VAL_BLK`, only the
`CHARACTERISTIC`'s own `Type` and `MATRIX_DIM`. Since there's no
`AXIS_DESCR` to say which of `MATRIX_DIM`'s two used dimensions is a "row"
and which is a "column," this slice reuses the same positional convention
already established for `MAP` (`dimensions: [Y count, X count]`, `x`
varying across columns): `x` (`MATRIX_DIM`'s first value) is treated as the
column count and `y` (its second value) as the row count, and
`FNC_VALUES`'s own `IndexMode` (`ROW_DIR`/`COLUMN_DIR`) still governs
storage order exactly as it does for `CURVE`/`MAP`. No mapdef schema
change was needed: a `VAL_BLK` converts to a `map`-kind `MapParameter`
with `axes` entirely absent — mapdef's schema already makes `axes`
optional, and `pkg/calibration`'s decode/edit path already treats a
missing `x`/`y` axis as "no axis to decode," so a headless 2D grid was
already representable, just never produced by an existing converter until
now.

## GROUP evidence

`GROUP` is a top-level `MODULE` block, `pya2l/classes.py`'s own attrs
confirming its two positional fields match the real example file exactly:
`<GroupName> "<GroupLongIdentifier>"`. Its children — corroborated by the
same class's declared child list — are `ROOT` (a bare marker, no value),
and the `SUB_GROUP`/`REF_CHARACTERISTIC`/`REF_MEASUREMENT`/`FUNCTION_LIST`
blocks, each just a list of one or more identifiers:

```text
/begin GROUP Group_Type_All "contains all groups with special types"
  ROOT
  /begin SUB_GROUP
    Group_Type_Scalar
    Group_Type_Array
    ...
  /end SUB_GROUP
/end GROUP

/begin GROUP Group_Type_Scalar "Contains all scalar measurements and parameters"
  /begin REF_CHARACTERISTIC
    ASAM.C.SCALAR.FLOAT32_IEEE.IDENTICAL
    ASAM.C.SCALAR.FLOAT64_IEEE.IDENTICAL
    ...
  /end REF_CHARACTERISTIC
  /begin REF_MEASUREMENT
    ASAM.M.SCALAR.FLOAT32.IDENTICAL
    ...
  /end REF_MEASUREMENT
/end GROUP

/begin GROUP Group_Function_Virtual "Contains all functions, measurements and parameters used for virtual"
  /begin FUNCTION_LIST
    FunctionVirtualMeasurements
  /end FUNCTION_LIST
  /begin REF_CHARACTERISTIC
    ...
  /end REF_CHARACTERISTIC
/end GROUP
```

`GROUP` is A2L's analog of XDF's `<CATEGORY>`, and mapdef already has the
field XDF's category support built: a flat, document-level `categories`
list (`{id, name}` pairs, no parent/child) plus a per-parameter
`categories` array of IDs. `GROUP` converts onto exactly that — one
`MapCategory` per `GROUP`, `id` derived from `GroupName` (deduplicated the
same way parameter IDs already are), `name` from `GroupLongIdentifier`
(falling back to `GroupName` if empty) — with each `GROUP`'s own
`REF_CHARACTERISTIC` list becoming that category's membership on the
matching `MapParameter`s.

The one real mismatch is `SUB_GROUP`: `GROUP` supports a genuine
hierarchy (`ROOT` marks a top-level entry point, `SUB_GROUP` nests child
groups), but mapdef's `categories` has no parent/child relationship to
represent it. Rather than guess at flattening semantics — does a
characteristic in a subgroup implicitly belong to every ancestor group
too? nothing in the evidence says so, and the file's own note ("For GROUP
there is no special sorting defined... up to the user") suggests GROUP is
a user-organizational convenience, not a strict taxonomy — this slice
converts only each `GROUP`'s own direct `REF_CHARACTERISTIC` list.
`SubGroups`/`RefMeasurements`/`FunctionRefs` are preserved on the internal
`A2LGroup` model (parsed, not rejected) but have nowhere to go in
mapdef's category shape (`MapCategory` has no extension point), so they
aren't otherwise surfaced — recoverable only from
`extensions.a2l.rawDocument`, the same fallback every other
preserved-but-unmapped construct in this project uses.

`FUNCTION` and `ANNOTATION`/`IF_DATA` are valid `GROUP` children per
`pya2l/classes.py` but are not evidenced inside any real `GROUP` in the
example file, so this slice does not recognize them there — a `GROUP`
using either fails to parse with an explicit "unsupported" error rather
than being silently skipped.

## RAT_FUNC / FORM / COMPU_TAB evidence

The real example file's own comments group its `COMPU_METHOD` types by
which should be preferred: "the types IDENTICAL, LINEAR and RAT_FUNC
should be used as standard"; "type FORM should only be used if the
computation is complex and not feasible by RAT_FUNC"; "type TAB_INTP and
TAB_NOINTP should only be used if the computation is complex and not
feasible by RAT_FUNC". Each needed its own evidence pass since each has a
genuinely different shape.

**`RAT_FUNC`** carries six `COEFFS` values:

```text
/begin COMPU_METHOD CM.RAT_FUNC.DIV_10
  "rational function with parameter set for impl = f(phys) = phys * 10"
  RAT_FUNC "%3.1" "km/h"
  COEFFS 0 10 0 0 0 1
/end COMPU_METHOD
```

`COEFFS <a> <b> <c> <d> <e> <f>` means `phys = (a*int^2 + b*int + c) /
(d*int^2 + e*int + f)`, confirmed independently by `pya2l/classes.py`'s
`COEFFS` class (six `Float` attrs in that same `a`..`f` order). Every
`RAT_FUNC` in the real example file has `a=c=d=e=0`, reducing to a pure
scaling `phys = (b/f)*int` — evidence never exercises a genuinely
quadratic or divisor-with-`int` case. This slice still supports the
general form: when the denominator is a nonzero constant (`d=e=0,
f != 0`), the numerator's coefficients are divided through into a
simplified polynomial; otherwise both sides are rendered and divided
explicitly, as `(a*X*X+b*X+c)/(d*X*X+e*X+f)` in `freehorse-expression-v1`
(`X*X` stands in for `X^2` — the language has no exponent operator, but
repeated multiplication is exactly equivalent and is already correctly
detected as non-affine by `expression.Expression.Affine`). A denominator
that is always zero (`d=e=f=0`) is rejected outright, not silently
divided by zero.

**`FORM`** carries a nested `FORMULA` block instead of positional
coefficients:

```text
/begin COMPU_METHOD CM.FORM.X_PLUS_4
  ""
  FORM
  "%6.1"
  "rpm"
  /begin FORMULA
    "X1+4"
    FORMULA_INV "X1-4"
  /end FORMULA
/end COMPU_METHOD
```

`pya2l/classes.py`'s `FORMULA` class confirms the shape: one required
string attr (`F_x`, the forward formula) and an optional nested
`FORMULA_INV` (`G_x`, an inverse formula). This slice never reads
`FORMULA_INV`: exactly like every other conversion here, an edit-time
inverse is always derived algebraically from the forward expression
itself, never from a separately supplied formula — preserved on the
internal model, unused by the converter. The forward formula uses ASAM's
own variable convention (`X1` for the first, and only, input a
`CHARACTERISTIC`/`AXIS_DESCR` `Conversion` ever has); this slice
translates `X1` to `freehorse-expression-v1`'s `X` and rejects a formula
referencing any other variable (`X2`, `X3`, ...) outright, since this
project doesn't parse `MEASUREMENT`/`FUNCTION` and so has no other input
to resolve one against. The translated text is then fed through the same
`expression.Parse` every other conversion uses; since `FORM`'s own
purpose (per the file's comment above) is for computations "complex" and
"not feasible" any other way, real-world `FORM` formulas may well use
syntax (functions, exponents, ...) outside `freehorse-expression-v1`'s
deliberately small grammar — those are rejected with the underlying
parse error, not guessed at.

**`TAB_INTP`/`TAB_NOINTP`** reference a `COMPU_TAB` block — a *numeric*
value-to-value lookup table, a different block from `COMPU_VTAB` (text
output):

```text
/begin COMPU_METHOD CM.TAB_INTP.DEFAULT_VALUE
  ""
  TAB_INTP "%8.4" "U/  min  "
  COMPU_TAB_REF CM.TAB_INTP.DEFAULT_VALUE.REF
/end COMPU_METHOD

/begin COMPU_TAB CM.TAB_INTP.DEFAULT_VALUE.REF
   ""
   TAB_INTP
   12
   -3 98
   -1 99
   0 100
   ...
   13 111
   DEFAULT_VALUE_NUMERIC 300.56
/end COMPU_TAB
```

`pya2l/classes.py`'s `COMPU_TAB` class confirms the shape (`Name`,
`LongIdentifier`, `ConversionType` — `TAB_INTP` or `TAB_NOINTP` — and
`NumberValuePairs`, followed by that many pairs, matching the real file
exactly). `TAB_INTP` linearly interpolates between entries; `TAB_NOINTP`
("no interpolation") is evidenced only as far as its name and the
existence of `DEFAULT_VALUE_NUMERIC` go — nothing in either source
confirms whether an in-between value should be treated as an error, a
step/floor lookup, or something else. Rather than guess, this slice
treats `TAB_NOINTP` as exact-match-only: a raw value not equal to one of
the table's `InVal` entries falls back to `DEFAULT_VALUE_NUMERIC` if
present, else is an error — the most conservative reading, and the one
`DEFAULT_VALUE_NUMERIC`'s own existence (a fallback for values the table
doesn't cover) is most directly evidence for.

A numeric lookup table has no representation as a
`freehorse-expression-v1` arithmetic expression at all, so this needed a
second new conversion shape on top of `axis.fixedPoints`:
`freehorse-lookup-table-v1` (mapdef 1.3+) — `conversion.points` (an
explicit `{input, output}` array, not a string), `conversion.interpolated`,
and `conversion.default`. `pkg/calibration` gained a matching
`lookupTable` evaluator alongside `*expression.Expression`, both
satisfying a small internal `conversionEvaluator` interface so
decode/edit code doesn't need to branch on which conversion shape it's
looking at. Both directions are implemented: `Eval` (raw -> physical, for
decode) always works; `Invert` (physical -> raw, for `SetCell`/`SetIndex`
edits) additionally requires the table's `Output` values to be monotonic
in `Input` order — otherwise which raw value a given physical value
should invert to is genuinely ambiguous, so this reports a clear
"not invertible" error rather than picking one arbitrarily. This mirrors
the same "decode always works, edit may not" precedent `RAT_FUNC`/`FORM`
(and, before this slice, XDF's own arbitrary `MATH` formulas) already
established for non-affine expression conversions.

## Grammar subset for this slice

A2L is a custom keyword-block text format, not XML — `encoding/xml` doesn't
apply, so this needs a small hand-written lexer/parser
(`pkg/a2l/lexer.go`, `pkg/a2l/parser.go`).

Tokens: `/begin`, `/end`, bare identifiers (letters/digits/`._-`, e.g.
`CM.IDENTICAL`), double-quoted strings, numeric literals (decimal, `0x`-hex,
scientific notation, optional leading `-`), and `/* ... */` comments
(stripped, not nested — no nested-comment example was found in the
reference file).

```text
ASAP2_VERSION <major> <minor>
/begin PROJECT <Name> "<LongIdentifier>"
  /begin HEADER "<LongIdentifier>"
    ...                                  (ignored for V0)
  /end HEADER
  /begin MODULE <Name> "<LongIdentifier>"
    /begin MOD_COMMON "<LongIdentifier>"
      [DEPOSIT <ABSOLUTE|...>]           parsed, discarded
      [BYTE_ORDER <Value>]               module-wide default byte order
      [ALIGNMENT_BYTE <n>] [ALIGNMENT_WORD <n>] [ALIGNMENT_LONG <n>]
      [ALIGNMENT_FLOAT32_IEEE <n>] [ALIGNMENT_FLOAT64_IEEE <n>]
                                          all parsed, discarded; any other
                                          optional keyword here fails to parse
    /end MOD_COMMON

    /begin COMPU_METHOD <Name>
      "<LongIdentifier>"
      <ConversionType>                   IDENTICAL | LINEAR | TAB_VERB | RAT_FUNC |
                                          FORM | TAB_INTP | TAB_NOINTP (else: unsupported)
      "<Format>" "<Unit>"
      [COEFFS_LINEAR <a> <b>]            required and only meaningful for LINEAR: phys = a*int + b
      [COEFFS <a> <b> <c> <d> <e> <f>]   required and only meaningful for RAT_FUNC:
                                          phys = (a*int^2+b*int+c)/(d*int^2+e*int+f);
                                          d=e=f=0 (always-zero denominator) is rejected
      [/begin FORMULA
        "<F_x>"                          required and only meaningful for FORM: the
        [FORMULA_INV "<G_x>"]             forward formula (variable X1); FORMULA_INV
      /end FORMULA]                       is parsed, preserved, never used
      [COMPU_TAB_REF <Name>]             required and only meaningful for TAB_VERB
                                          (-> COMPU_VTAB/COMPU_VTAB_RANGE below) or
                                          TAB_INTP/TAB_NOINTP (-> COMPU_TAB below)
    /end COMPU_METHOD

    /begin COMPU_VTAB <Name>
      "<LongIdentifier>"
      TAB_VERB                           only this shape is supported, not TAB_VERB_RANGE
      <NumberValuePairs>
      (<Value> "<Text>")+                NumberValuePairs repetitions
      [DEFAULT_VALUE "<Text>"]           parsed, recorded, not yet used (no mapdef equivalent)
    /end COMPU_VTAB

    /begin COMPU_VTAB_RANGE <Name>       parsed and preserved; a TAB_VERB
      "<LongIdentifier>"                 Conversion resolving here is
      <NumberValueTriples>                rejected (ErrCompuVtabRangeUnsupported),
      (<LowerValue> <UpperValue> "<Text>")+  not silently dropped — see
      [DEFAULT_VALUE "<Text>"]            "TAB_VERB / COMPU_VTAB evidence"
    /end COMPU_VTAB_RANGE

    /begin COMPU_TAB <Name>              -> conversion.points (mapdef 1.3+,
      "<LongIdentifier>"                   "freehorse-lookup-table-v1"); see
      <ConversionType>                     "RAT_FUNC / FORM / COMPU_TAB
                                            evidence" — TAB_INTP interpolates,
                                            TAB_NOINTP is exact-match-only
      <NumberValuePairs>
      (<InVal> <OutVal>)+                 NumberValuePairs repetitions
      [DEFAULT_VALUE_NUMERIC <Value>]     -> conversion.default
    /end COMPU_TAB

    /begin RECORD_LAYOUT <Name>
      [NO_AXIS_PTS_X <Position> <DataType>]   the stored *current* point
      [NO_AXIS_PTS_Y <Position> <DataType>]   count; parsed, not decoded —
                                               every axis is always exactly
                                               its AXIS_DESCR's MaxAxisPoints
      [AXIS_PTS_X <Position> <DataType> <IndexMode> <AddrType>]
      [AXIS_PTS_Y <Position> <DataType> <IndexMode> <AddrType>]
                                          IndexMode must be INDEX_INCR;
                                          required exactly when the
                                          corresponding axis is STD_AXIS
                                          (embedded here); absent when it's
                                          COM_AXIS (see AXIS_PTS below)
      FNC_VALUES <Position> <DataType> <IndexMode> <AddrType>
                                          IndexMode ROW_DIR|COLUMN_DIR;
                                          DataType one of
                                          UBYTE|SBYTE|UWORD|SWORD|ULONG|SLONG
                                          (FLOAT32_IEEE/FLOAT64_IEEE and any
                                          other value: unsupported)
                                          Every AddrType above must be DIRECT.
                                          Position is an ordinal, not a byte
                                          offset — see "CURVE/MAP evidence".
      [NO_RESCALE_X <Position> <DataType>]    a RES_AXIS's AXIS_PTS only —
      [RESERVED <Position> BYTE]               see "FIX_AXIS / CURVE_AXIS /
      [AXIS_RESCALE_X <Position> <DataType>    RES_AXIS evidence"; RESERVED's
        <MaxNumberOfRescalePairs>               DataType must be the literal
        <IndexMode> <AddrType>]                 BYTE (only shape evidenced)
    /end RECORD_LAYOUT

    /begin CHARACTERISTIC <Name>
      "<LongIdentifier>"
      <Type>                             VALUE | CURVE | MAP | VAL_BLK (else: unsupported)
      <Address>                          hex or decimal
      <Deposit>                          RECORD_LAYOUT name reference
      <MaxDiff>                          parsed, recorded, not yet interpreted
      <Conversion>                       COMPU_METHOD name reference, or the
                                          literal NO_COMPU_METHOD (-> identity)
      <LowerLimit> <UpperLimit>          -> display.min / display.max
      [FORMAT "<Format>"]                overrides the COMPU_METHOD's Format;
                                          fractional digit count -> decimalPlaces
      [DISPLAY_IDENTIFIER <Name>]        parsed, recorded, not yet used
      [EXTENDED_LIMITS <low> <high>]     parsed, recorded, not yet used
      [/begin AXIS_DESCR                 0 for VALUE, 1 for CURVE, 2 (X, Y)
        <Attribute>                        for MAP; STD_AXIS | COM_AXIS |
                                            FIX_AXIS | RES_AXIS | CURVE_AXIS
        <InputQuantity>                    a MEASUREMENT reference; preserved,
                                            never resolved (MEASUREMENT isn't
                                            parsed)
        <Conversion>                       COMPU_METHOD reference or
                                            NO_COMPU_METHOD; authoritative for
                                            STD_AXIS, COM_AXIS, RES_AXIS, and
                                            FIX_AXIS (see the evidence
                                            sections below); always just a
                                            placeholder for CURVE_AXIS, whose
                                            referenced CHARACTERISTIC's own
                                            axis Conversion is authoritative
                                            instead. TAB_VERB here ->
                                            axis.staticValues (see "TAB_VERB /
                                            COMPU_VTAB evidence"); TAB_VERB
                                            directly on a CHARACTERISTIC's own
                                            Conversion is rejected, not
                                            silently dropped
        <MaxAxisPoints>                    authoritative for STD_AXIS and
                                            FIX_AXIS only; a COM_AXIS's or
                                            RES_AXIS's AXIS_PTS, or a
                                            CURVE_AXIS's referenced axis,
                                            overrides it
        <LowerLimit> <UpperLimit>          ditto
        [MONOTONY <Value>]                 parsed, not yet enforced
        [AXIS_PTS_REF <Name>]              required for, and only for,
                                            Attribute COM_AXIS or RES_AXIS
        [CURVE_AXIS_REF <Name>]            required for, and only for,
                                            Attribute CURVE_AXIS: the
                                            referenced CHARACTERISTIC.Name
        [FIX_AXIS_PAR_DIST <Offset>
          <Distance> <NumberPts>]          required for, and only for,
                                            Attribute FIX_AXIS, unless
                                            FIX_AXIS_PAR_LIST is used instead
        [/begin FIX_AXIS_PAR_LIST
          <AxisPtsValue>+
        /end FIX_AXIS_PAR_LIST]           the other FIX_AXIS mechanism;
                                            exactly one of the two is used
      /end AXIS_DESCR]
      [MATRIX_DIM <x> <y> <z>]           required for, and only for, Type
                                          VAL_BLK (which has no AXIS_DESCR
                                          of its own); only z == 1 is
                                          supported — see "VAL_BLK evidence"
      [BYTE_ORDER <Value>]               overrides MOD_COMMON's default;
                                          required (from here or MOD_COMMON)
                                          for any multi-byte DataType
    /end CHARACTERISTIC

    [/begin AXIS_PTS <Name>              a standalone, shareable axis a
      "<LongIdentifier>"                   COM_AXIS or RES_AXIS AXIS_DESCR
                                          references (never CURVE_AXIS,
                                          which borrows a CHARACTERISTIC's
                                          axis directly, not an AXIS_PTS)
      <Address>
      <InputQuantity>                    preserved, never resolved; not
                                          authoritative (see evidence above)
      <Deposit>                          RECORD_LAYOUT reference: either
                                          exactly one AXIS_PTS_X entry
                                          (optionally preceded by
                                          NO_AXIS_PTS_X) for COM_AXIS, or
                                          exactly one AXIS_RESCALE_X entry
                                          (optionally preceded by
                                          NO_RESCALE_X/RESERVED) for
                                          RES_AXIS — never both, no
                                          FNC_VALUES/AXIS_PTS_Y
      <MaxDiff>                          parsed, recorded, not interpreted
      <Conversion>                       preserved, never resolved; not
                                          authoritative (see evidence above)
      <MaxAxisPoints>                    authoritative — overrides the
                                          referencing AXIS_DESCR's
      <LowerLimit> <UpperLimit>          authoritative — see evidence above
      [DISPLAY_IDENTIFIER <Name>]        parsed, recorded, not yet used
    /end AXIS_PTS]

    [/begin GROUP <Name>                 -> one mapdef category; see
      "<LongIdentifier>"                   "GROUP evidence"
      [ROOT]                              bare marker, preserved, not used
                                            to filter which groups convert
      [/begin SUB_GROUP <Name>+
       /end SUB_GROUP]                   preserved, not flattened into
                                            category membership
      [/begin REF_CHARACTERISTIC <Name>+
       /end REF_CHARACTERISTIC]          -> this category's membership
      [/begin REF_MEASUREMENT <Name>+
       /end REF_MEASUREMENT]             preserved, never resolved
                                            (MEASUREMENT isn't parsed)
      [/begin FUNCTION_LIST <Name>+
       /end FUNCTION_LIST]               preserved, never resolved
                                            (FUNCTION isn't parsed)
    /end GROUP]
  /end MODULE
/end PROJECT
```

Any other block encountered at `MODULE` level (or nested inside a block this
slice does parse) is skipped by counting balanced `/begin`/`/end` pairs,
never interpreted, and the exact source text is preserved wholesale in
`extensions.a2l.rawDocument` on the converted document — the same
"recoverable, not guessed" guarantee XDF import makes.

## Mapping to mapdef

| A2L | mapdef |
| --- | --- |
| `CHARACTERISTIC` `Type VALUE`/`CURVE`/`MAP` | A `scalar`/`curve`/`map`-kind parameter, reusing the same shape `pkg/calibration` already decodes |
| `CHARACTERISTIC` `Type VAL_BLK` + `MATRIX_DIM x y 1` | A `map`-kind parameter with `axes` entirely absent; `layout.dimensions: [y, x]` (same `[Y count, X count]` convention `MAP` uses), sized and addressed exactly like a scalar's single `FNC_VALUES` entry — see "VAL_BLK evidence" |
| `Address` + the resolved `RECORD_LAYOUT` entry's computed byte offset (see "CURVE/MAP evidence") | `layout.address` (for `FNC_VALUES`) and each axis's own `layout.address` (for `AXIS_PTS_X`/`AXIS_PTS_Y`) |
| `RECORD_LAYOUT` entry data type | `layout.dataType` (`uint8`/`int8`/`uint16`/`int16`/`uint32`/`int32`); `dimensions: [1]` for a scalar, `[MaxAxisPoints]` for a curve/axis, `[Y count, X count]` for a map's Z; `order: "not-applicable"` except a map/curve's Z, which takes `FNC_VALUES`'s `ROW_DIR`/`COLUMN_DIR` |
| Effective `BYTE_ORDER` (`CHARACTERISTIC`'s own, else `MOD_COMMON`'s) | `layout.byteOrder`: `"not-applicable"` for single-byte types; `"little"`/`"big"` for multi-byte (`LITTLE_ENDIAN`/`MSB_LAST` or `BIG_ENDIAN`/`MSB_FIRST`) — see "Byte order evidence" above |
| `AXIS_DESCR` (`STD_AXIS`, `COM_AXIS`, or `RES_AXIS`) | `axes.x`/`axes.y`: `count`/bounds from the authoritative side (the `AXIS_DESCR` itself for `STD_AXIS`, the referenced `AXIS_PTS` for `COM_AXIS`/`RES_AXIS` — see "COM_AXIS / AXIS_PTS evidence" and "FIX_AXIS / CURVE_AXIS / RES_AXIS evidence"); `conversion`/`unit`/`decimalPlaces` always from the `AXIS_DESCR`'s own `Conversion` reference |
| `COM_AXIS`'s `AXIS_PTS_REF` -> resolved `AXIS_PTS` | `axes.x`/`axes.y.layout.address` (relative to the `AXIS_PTS`'s own `Address`, not the characteristic's) |
| `RES_AXIS`'s `AXIS_PTS_REF` -> resolved `AXIS_PTS` (rescale pairs) | `axes.x`/`axes.y.layout.address` (past padding, to the first pair's value element) and `layout.strideBits` (mapdef 1.2+; twice the natural element width, to skip each pair's position element) — see "FIX_AXIS / CURVE_AXIS / RES_AXIS evidence" |
| `FIX_AXIS`'s `FIX_AXIS_PAR_DIST`/`FIX_AXIS_PAR_LIST` | `axes.x`/`axes.y.fixedPoints` (mapdef 1.2+; the axis's raw breakpoints computed once, with no `layout` at all) — see "FIX_AXIS / CURVE_AXIS / RES_AXIS evidence" |
| `CURVE_AXIS`'s `CURVE_AXIS_REF` -> resolved `CHARACTERISTIC`'s own axis | `axes.x`/`axes.y.layout` (identical shape to `STD_AXIS`/`COM_AXIS`, just resolved from the referenced characteristic instead) — see "FIX_AXIS / CURVE_AXIS / RES_AXIS evidence" |
| `AXIS_DESCR`'s `InputQuantity`/`Attribute`/`Conversion` name, and (for `COM_AXIS`/`RES_AXIS`) the resolved `AXIS_PTS` name, or (for `CURVE_AXIS`) the resolved `CURVE_AXIS_REF` name | `axis.sourceMetadata.a2l.*`, preserved, not interpreted |
| `COMPU_METHOD` `IDENTICAL` | `freehorse-expression-v1` `"X"` |
| `COMPU_METHOD` `LINEAR` (`COEFFS_LINEAR a b`) | `freehorse-expression-v1` `"X*a+b"` |
| `COMPU_METHOD` `TAB_VERB` (`COMPU_TAB_REF` -> resolved `COMPU_VTAB` entries), on an axis only | `axis.staticValues` (`freehorse-expression-v1` `"X"` for the axis's own conversion — the raw value is decoded as-is; the lookup is display-only) — see "TAB_VERB / COMPU_VTAB evidence" |
| `COMPU_METHOD` `RAT_FUNC` (`COEFFS a b c d e f`) | `freehorse-expression-v1`, e.g. `"5*X"` (pure scaling, the only evidenced shape) or the general `"(a*X*X+b*X+c)/(d*X*X+e*X+f)"` — see "RAT_FUNC / FORM / COMPU_TAB evidence" |
| `COMPU_METHOD` `FORM` (`FORMULA`'s forward `F_x`) | `freehorse-expression-v1`, `X1` translated to `X` — see "RAT_FUNC / FORM / COMPU_TAB evidence" |
| `COMPU_METHOD` `TAB_INTP`/`TAB_NOINTP` (`COMPU_TAB_REF` -> resolved `COMPU_TAB` entries) | `freehorse-lookup-table-v1` (mapdef 1.3+): `conversion.points` (`{input, output}` from each entry's `InVal`/`OutVal`), `conversion.interpolated` (`true` for `TAB_INTP`, `false` for `TAB_NOINTP`), `conversion.default` from `DEFAULT_VALUE_NUMERIC` — see "RAT_FUNC / FORM / COMPU_TAB evidence" |
| `COMPU_METHOD`'s `Unit` | `unit` |
| `LowerLimit`/`UpperLimit` | `display.min`/`display.max` (mapdef 1.1's informational bounds field — the same one XDF `<min>`/`<max>` uses) |
| `Format` (characteristic-level overrides compu-method-level) | `display.decimalPlaces`, parsed from the fractional digit count of a `"%W.D"`-style format string |
| Everything else (`MaxDiff`, `DisplayIdentifier`, `ExtendedLimits`, raw block text) | `sourceMetadata.a2l.*`, preserved, not interpreted |
| `GROUP` (`GroupName`/`GroupLongIdentifier`) | A `MapCategory` (`id` from `GroupName`, deduplicated the same way parameter IDs already are; `name` from `GroupLongIdentifier`, falling back to `GroupName`) — see "GROUP evidence" |
| `GROUP`'s `REF_CHARACTERISTIC` list | `parameter.categories`: the referenced `MapCategory`'s `id`, for every `CHARACTERISTIC` named in the list |

This slice needed a genuine mapdef schema bump, to 1.2.0: `axis.fixedPoints`
is a new field (`FIX_AXIS`'s computed breakpoints with no on-disk storage
at all), and `layout.strideBits` — present in the schema since 1.1.0, but
always rejected as unsupported whenever non-zero — now has real decode
semantics for an axis (`RES_AXIS`'s interleaved rescale pairs). Both are
purely additive: an older reader ignores `fixedPoints` it doesn't
understand and would simply fail to decode that one axis, and no existing
document used a non-zero `strideBits`, so nothing is reinterpreted
retroactively. `pkg/mapdef/migrate.go`'s `migrateV1_1_0ToV1_2_0` step is a
version-number bump only — there is no previously-preserved lossy data to
promote, since neither capability existed before.

A follow-on bump, to 1.3.0, adds a second `conversion` shape entirely:
`language: "freehorse-lookup-table-v1"` alongside the existing
`"freehorse-expression-v1"`, with `points`/`interpolated`/`default`
instead of `expression` — `TAB_INTP`/`TAB_NOINTP`'s numeric lookup tables
have no representation as an arithmetic expression string at all. Again
purely additive (a 1.2.0 document only ever used
`"freehorse-expression-v1"`), so `migrateV1_2_0ToV1_3_0` is likewise a
version-number bump only.

## Delivery

1. `pkg/types/a2l.go` — normalized model (`A2LDefinition`, `A2LModule`,
   `A2LCharacteristic`, `A2LCompuMethod`, `A2LRecordLayout`, `A2LAxisPts`,
   `A2LCompuVtab`, `A2LCompuVtabRange`, `A2LCompuTab`, `A2LFixAxisParDist`,
   `A2LMatrixDim`, `A2LGroup`).
2. `pkg/a2l/lexer.go`, `pkg/a2l/parser.go` — tokenizer and recursive-descent
   parser for the grammar subset above; `pkg/a2l/testdata/basic.a2l` is a
   from-scratch synthetic fixture.
3. `pkg/a2l/converter.go` — `Converter.Convert`, mirroring
   `xdf.MapdefConverter`'s contract and using the same identifier/category
   helpers pattern where it makes sense.
4. `pkg/types/mapdef.go`, `schemas/mapdef-v1.schema.json` (and its embedded
   copy), `pkg/mapdef/validation.go`, `pkg/mapdef/migrate.go` —
   `axis.fixedPoints`, `formatVersion` 1.2.0/1.3.0, the
   `freehorse-lookup-table-v1` conversion shape, and their migration steps
   (version bumps only, see "Mapping to mapdef").
5. `pkg/calibration/decode.go`, `pkg/calibration/lookup_table.go` —
   `decodeAxis` now handles an axis with no `Layout` at all (`FixedPoints`)
   and an axis-specific stride override for `RES_AXIS`'s interleaved
   rescale pairs; `parseConversion` now returns a small
   `conversionEvaluator` interface satisfied by both
   `*expression.Expression` and the new `lookupTable` type (for
   `TAB_INTP`/`TAB_NOINTP`) — the only calibration engine changes any A2L
   or XDF slice has needed besides the earlier bit-position decode work
   for XDF flags.
6. `cmd/freebeamer/command_a2l.go` — `freebeamer a2l info FILE` and
   `freebeamer a2l convert FILE --output DEFINITION.mapdef`, mirroring the
   `xdf` command surface.
7. `docs/a2l-compatibility.md` — a compatibility matrix in the same shape as
   `xdf-compatibility.md`, covering exactly the constructs above and
   recording what's explicitly out of scope and why.

## Definition of this slice complete

```bash
freebeamer a2l info definition.a2l
freebeamer a2l convert definition.a2l --output definition.mapdef
```

on the synthetic fixture, producing a valid mapdef document whose scalar,
curve, and map parameters all decode/edit correctly through the existing,
unmodified `pkg/calibration` engine — proven by a fixture test that opens a binary
image and round-trips a value through the converted parameter, the same
acceptance shape `pkg/xdf/mapdef_test.go`'s conversion tests use.

Follow-on slices (`ASCII` strings; genuinely 3-5 dimensional `CUBOID`/
`CUBE_4`/`CUBE_5` shapes — needs a real N-dimensional data model decision
across `pkg/calibration` and the mapdef schema first, a bigger change than
any slice so far, see "VAL_BLK evidence"; `MEASUREMENT`, needing live ECU
communication rather than anything `pkg/calibration` does today;
`FUNCTION`, needing its own design since its `DEF_CHARACTERISTIC`/
`REF_CHARACTERISTIC` distinction doesn't fit `categories`, see "GROUP
evidence"; a `RES_AXIS` whose rescale pair count doesn't match its axis
point count — needs real interpolation evidence this project doesn't have
yet, see "FIX_AXIS / CURVE_AXIS / RES_AXIS evidence"; a `TAB_NOINTP`
between-points semantic other than exact-match, if real evidence ever
clarifies one, see "RAT_FUNC / FORM / COMPU_TAB evidence"; and a mapdef
schema addition to let a `CHARACTERISTIC`'s own `TAB_VERB` conversion, or a
`COMPU_VTAB_RANGE`-backed one on either an axis or a characteristic, carry
range- or index-keyed `staticValues`) are separate work, each starting
with its own evidence-gathering pass like this one.
