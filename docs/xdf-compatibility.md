# XDF compatibility and redistribution policy

FreeBeamer imports a documented subset of XDF 1.70 for interoperability. XDF is
not the canonical application or API model. Import converts recognized data to
`.mapdef`, retains the original XML in a namespaced extension, and emits
diagnostics for known fidelity limits.

“Parsed” means that FreeBeamer captured a field; it does not mean that the field
is supported for decoding or editing.

## Compatibility matrix

| XDF construct | Parse | mapdef conversion | Decode/edit | Fidelity or limitation |
| --- | --- | --- | --- | --- |
| `XDFFORMAT version` | Yes | Provenance | N/A | Preserved |
| Header title/description/author | Yes | Identity/provenance | N/A | Preserved |
| `BASEOFFSET` offset/subtract | Yes | Effective signed base offset | Yes | Preserved semantically; raw XML retained |
| `DEFAULTS` size, signed, endian, float | Yes | Applied when axis fields omit flags/size | Integer subset | Float is diagnosed unsupported |
| `REGION` | Yes | Memory segment and candidate image size | Compatibility check | Attributes retained in source XML |
| `CATEGORY` / `CATEGORYMEM` | Yes | Stable category references | Display/filter | Missing category declarations get deterministic IDs |
| `XDFTABLE` identity/text | Yes | Parameter | Yes | Duplicate IDs receive deterministic suffixes |
| X/Y/Z axes | Yes | Parameter layout and axes | Addressed X/Y/Z; virtual X/Y retained | Unknown axis IDs are not interpreted; source XML retained |
| 8/16/32-bit integer storage | Yes | Explicit data type | Yes | Supported |
| Signed values | Yes | Explicit data type | Yes | Supported |
| MSB/LSB-first | Yes | Explicit byte order | Yes | Supported |
| Row/column-major Z layout | Yes | Explicit order | Yes | Supported |
| Non-zero major/minor strides | Yes | Preserved and warned | No | Explicit `xdf.unsupported-stride` diagnostic |
| IEEE-754 storage | Recognized | Marked `unknown` and warned | No | Explicit diagnostic |
| Unknown `mmedtypeflags` bits | Preserved | Marked `unknown` and warned | No | Explicit diagnostic |
| Units, decimal places, output type | Yes | Unit/display metadata | Yes, where applicable | Preserved |
| `MATH equation` | Yes | `freehorse-expression-v1` | `+ - * /`, unary signs, parentheses, numeric literals, `X` | Other XDF expressions prevent semantic validation; source remains available |
| `XDFFLAG` | Partial | Flag parameter; `<mask>` resolves to `layout.bitPosition` | Yes, for a single in-range bit | A missing, zero, multi-bit, or out-of-range mask falls back to treating the whole addressed word as the raw value, with a diagnostic |
| XDF constants/scalars outside tables (`XDFCONSTANT`) | Yes | Scalar parameter | Yes | Same layout/formula handling as a table's Z axis |
| Embedded/static axis labels (`<LABEL>`) | Yes | `axis.staticValues` (mapdef 1.1+) | N/A (presentational) | See axis label evidence below |
| Display bounds (`<min>`/`<max>`) | Yes | `display.min`/`display.max` (mapdef 1.1+) | No (informational only) | Not enforced as edit-time range checks; see bounds evidence below |
| `DALINK`, `embedinfo type="3"` axis linking | No | Source XML only | No | Purpose not confirmed by available evidence; not interpreted |
| Unknown attributes | Selected elements | Source metadata/raw XML | No | Known-element attributes are maps; exact XML is retained |
| Unknown elements and ordering | No semantic parse | Exact raw XML extension | No | Recoverable, not interpreted |

### Axis label evidence

TunerPro's "Label Source" axis setting includes "External (Manual)", where a
user types values directly into "Row Labels"/"Column Labels" boxes rather
than computing them from an address and formula. Two independent open-source
XDF implementations agree these become `<LABEL index="" value="">` child
elements of `<XDFAXIS>`:

- OpenEEC's `SADXdf.cs` models `XdfAxis.xdfLabels` as an array of `XdfLabel`,
  each with `index`/`value` attributes:
  <https://github.com/OpenEEC-Project/SAD806x/blob/master/SADXdf.cs>
- `tunerpro-xdf-bin-cli-map-editor`'s exporter finds `.//LABEL` under each
  axis and reads its `value` attribute:
  <https://github.com/KingAiCodeForge/tunerpro-xdf-bin-cli-map-editor>

FreeBeamer parses `<LABEL>` entries (`pkg/xdf/normalize_axis.go`) and converts
them to `axis.staticValues`, an ordered `{index, value}` list, mapdef 1.1's
typed home for them (`pkg/mapdef/schema/mapdef-v1.schema.json`'s `staticValue`
definition). Adding this field required a genuine `formatVersion` bump — ADR
0001 requires new optional fields to arrive in a minor release, not be added
silently under an unchanged `"1.0.0"` const — which is why it waited for the
adjacent-version migration framework, item 2 of
[mapdef-backlog.md](mapdef-backlog.md). A mapdef 1.0.0 document converted
before that framework existed still only has `<LABEL>` data in
`sourceMetadata.xdf.labels`; `mapdef.Migrate` promotes it into
`staticValues` on upgrade (see `pkg/mapdef/migrate.go`). The raw
`sourceMetadata.xdf.labels` copy is kept even in a fresh 1.1 conversion, for
provenance alongside every other `xdf.*` raw-attribute preservation in this
matrix.

`embedinfo type="3" linkobjid="..."` (axis breakpoints linked to another
table by unique ID, used by some MS42/MS43-style XDFs) is a separate,
unrelated mechanism and remains unparsed; it is not a static-label feature.

### Bounds evidence

OpenEEC's `SADXdf.cs` lists `min`/`max` (both plain, non-attribute members)
on `XdfAxis`, and `min`/`max`/`rangelow`/`rangehigh` on the `XdfScalar` model
that also backs `XDFCONSTANT`:
<https://github.com/OpenEEC-Project/SAD806x/blob/master/SADXdf.cs>. FreeBeamer
implements `<min>`/`<max>` only, on both axes and constants: `rangelow`/
`rangehigh` also appear in that same field list, but their relationship to
`min`/`max` (a distinct valid-value range versus the same display bound
under another name, for instance) isn't established from a second source, so
they're left unparsed rather than guessed. Like axis labels, `<min>`/`<max>`
convert to typed fields — `display.min`/`display.max` (mapdef 1.1+), on the
table's Z axis, each X/Y axis, and `XDFCONSTANT` scalars — with the raw
value also kept in `sourceMetadata.xdf.bounds` for provenance. A 1.0.0
document that only has the `sourceMetadata` copy gets `display.min`/`max`
populated by `mapdef.Migrate`. They are informational only: FreeBeamer does
not reject or clamp an edit that falls outside them, since XDF gives no
signal for whether such a bound is a hard ECU limit or just a display
suggestion, and this project does not silently clamp values it cannot
justify.

**What this project decided *not* to build here:** the original compatibility
matrix entry for this row read "bounds, alarms, visibility, DAQ links." Only
`min`/`max` bounds turned out to be corroborated by available evidence.
`DALINK` exists on `XdfAxis` (per the same OpenEEC source, a plain `index`/
`objidhash` pair defaulting to `index="0"`) but its purpose is not documented
anywhere found during this research pass; it is left as unparsed raw XML
rather than modeled speculatively. No "alarm" or "visibility" field was found
in `XDFAXIS`/`XDFCONSTANT` at all — those terms describe TunerPro's live
data-monitoring gauges, a separate runtime feature, not an XDF file
construct — and live data/logging are explicit FreeBeamer V0 non-goals, so
that part of the original row description does not describe real XDF import
work and has been removed rather than carried forward as open backlog.

### XDFCONSTANT evidence

TunerPro's help pages describe table editing but not `XDFCONSTANT`'s exact
XML shape. Two independent open-source XDF implementations agree it is a
hybrid of `XDFTABLE`'s Z axis and a table/flag's identity fields: `uniqueid`
attribute; `title`/`description`/`CATEGORYMEM` children (as `XDFTABLE` and
`XDFFLAG` have); `EMBEDDEDDATA` and `MATH` children (as any axis has); and
`units`/`decimalpl`/`outputtype` children directly on the element (as an
axis has, but without an intervening `XDFAXIS` wrapper, since a constant has
no X/Y/Z structure):

- OpenEEC's `SADXdf.cs` models it as `XdfScalar`, listing exactly these
  members: <https://github.com/OpenEEC-Project/SAD806x/blob/master/SADXdf.cs>
- The A2L-to-XDF generator `a2l2xdf.py`'s `xdf_constant_with_root` builds
  `title`/`description`/`EMBEDDEDDATA`/`MATH` directly on `<XDFCONSTANT>`:
  <https://github.com/bri3d/a2l2xdf/blob/master/a2l2xdf.py>

FreeBeamer converts `XDFCONSTANT` the same way it converts a scalar
`XDFTABLE` (a 1x1 table with no axes): one `scalar`-kind mapdef parameter
using the same `convertLayout` path, so it inherits the same data-type,
byte-order, and stride support and limits.

### XDFFLAG mask evidence

TunerPro's own help pages do not document the `<mask>` element's XML shape.
Two independent open-source XDF implementations agree that `<XDFFLAG>` carries
a `<mask>` child element holding the bitmask applied to the addressed byte,
and that a flag's boolean value is `(byte & mask) != 0`:

- OpenEEC's `SADXdf.cs` models `XdfFlag.mask` as a plain (non-attribute) XML
  member: <https://github.com/OpenEEC-Project/SAD806x/blob/master/SADXdf.cs>
- The independent `TunerPro-XDF-BIN-Universal-Exporter` project's
  `_extract_flags` locates a nested `<mask>` element, defaulting to `0x01`
  when absent: <https://github.com/KingAiCodeForge/TunerPro-XDF-BIN-Universal-Exporter>

FreeBeamer follows both sources for the `<mask>` element shape and the
"single set bit" case, converting it to `layout.bitPosition`. It deliberately
does **not** adopt the "default to 0x01 when `<mask>` is absent" convention
both sources share, since that default is unverified against an authoritative
TunerPro source; an absent mask is left uninterpreted instead. A mask with
more than one bit set (TunerPro's UI describes this as a multi-value
"bitmask" selection distinct from a boolean flag) is also left uninterpreted,
since FreeBeamer does not yet model multi-bit selections.

The synthetic fixture `pkg/xdf/testdata/basic.xdf` is authored for this project
and is legally redistributable. Conversion tests cover identity, provenance,
addresses, integer layout, axes, orientation, units, equations, raw values,
decoded values, source preservation, and diagnostics. Private fixtures under
`testdata/private/` are optional and must remain untracked.

## Import/export policy

- XDF import is supported only to the extent described by this matrix.
- Unsupported information produces an error or diagnostic; it is never guessed.
- XDF export is not implemented and has no compatibility guarantee.
- Future export must remain experimental until independently licensed fixtures
  demonstrate lossless round trips for every advertised construct.
- FreeBeamer is independent from and unaffiliated with TunerPro or Creational
  Technologies LLC. Product names are used only to describe compatibility.

## Redistribution and attribution

Users are responsible for rights to access, convert, and redistribute their
definitions. Conversion does not grant a new licence or remove source
attribution. A mapdef created from an XDF must not be published merely because
conversion succeeded.

The repository and official distributions must never contain third-party XDFs
or converted definitions without explicit permission compatible with that
distribution. Record author, source, licence, and source digest whenever known.
Do not assume that a publicly downloadable file is public domain.

Local import is the default. Any future hosted upload or sharing service needs
separate terms, privacy/retention rules, and notice-and-action procedures.

This is an engineering policy, not legal advice. Qualified counsel should
review specification-licensing questions, hosted sharing, disputed ownership,
confidential definitions, and trademark presentation before those features
ship.

## Evidence and unresolved legal questions

The technical interpretation uses TunerPro's public help pages, including the
[XDF table editor documentation](https://tunerpro.net/WebHelp/source/xdftableeditor.htm),
which describes addresses, element sizes, byte order, signedness, row/column
population, axes, and conversions. TunerPro's
[definition download page](https://www.tunerpro.net/downloadBinDefs.htm) says
definitions come from many users and sources; availability there is not treated
as a reusable software licence. No official complete XDF grammar or explicit
general permission to redistribute arbitrary XDF definitions was located during
the 2026-09-02 review.

The repository audit found one tracked XDF, `pkg/xdf/testdata/basic.xdf`. It is
a synthetic fixture authored for FreeBeamer and labels itself legally
redistributable. No downloaded third-party definition is bundled.

EU Directive 2009/24/EC recognizes a limited interoperability boundary for
independently created software, and US 17 USC 1201(f) contains a conditional
interoperability exception. Those provisions do not clear the copyright,
contract, confidentiality, or trade-secret status of definition content. Open
questions for counsel are:

- whether any non-public material used to infer additional XDF semantics is
  subject to contract or confidentiality restrictions;
- what notices and terms a future hosted conversion or sharing service needs;
- when publishing converted descriptions would reproduce protectable source
  expression or confidential calibration research; and
- the appropriate trademark wording if compatibility marketing expands.
