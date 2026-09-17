# FreeBeamer — V0 Codex Implementation Plan

**FreeBeamer** is an open-source ECU tuning and calibration platform.

V0 is intentionally narrow: build a trustworthy CLI/core for the supplied BMW N55 Bosch MEVD17.2.6 BIN/XDF pair before adding UI, cloud, realtime data, flashing, or support for additional ECUs.

## Goal

Build the smallest useful, testable core of an open-source ECU calibration editor for Linux, starting with the supplied BMW N55 Bosch MEVD17.2.6 files.

This first version has **no GUI, no cloud, no ECU communication, and no flashing**.

The first release is a Go library plus CLI that can:

1. Load an XDF.
2. Load a BIN.
3. Validate that the XDF's binary region is compatible with the BIN.
4. List XDF categories and maps.
5. Inspect a map and decode its values from the BIN.
6. Edit scalar/table values and encode them back into a working copy of the BIN.
7. Show an exact byte-level diff against the original.
8. Identify the supplied BMW/Bosch MEVD17.2.6 firmware.
9. Verify and recalculate its checksums once the MEVD17.2.6 checksum implementation has been independently validated.
10. Save a **new** BIN. Never overwrite the original.

The architecture must remain generic enough that future XDF/BIN pairs work without vehicle-specific editor code. Vehicle-specific code is allowed only for identification/checksum behavior.

## Architecture Principle

Keep the reusable editing path generic:

```text
XDF + BIN
   ↓
FreeBeamer Core
├── XDF parser
├── calibration model
├── binary image
├── expression engine
├── diff engine
└── checksum registry
```

Only checksum/identification providers may contain ECU-family-specific behavior.

For V0:

```text
Generic editor: any compatible XDF + BIN
Checksum target: BMW / Bosch / MEVD17.2.6 / N55
```


---

# Initial Regression Fixture

Use these files as the first integration fixture:

- `00000FEF81A001.xdf`
- `00000FEF81A001_original.bin`

Observed properties of the supplied fixture:

- XDF format: 1.70
- BIN size: 4,194,304 bytes / 4 MiB
- XDF declared binary region: `0x000000` through `0x3FFFFF`
- XDF tables: 962
- XDF flags: 7
- XDF categories: 73
- XDF checksum definitions: 0
- BIN SHA-256:
  `6509dc5257948091194ca9257b0d5763dcc67ca014e18df4d605d419738aadc8`
- ECU family identified from embedded firmware strings: Bosch MEVD17.2.6 / BMW N55

The original fixture must be treated as immutable test input.

Do not commit the user's original BIN to a public repository. Put local proprietary/personal fixtures under an ignored directory such as `testdata/private/`. Public CI should use synthetic fixtures or legally redistributable test binaries.

---

# Non-Goals for V0

Do not implement any of the following yet:

- GUI
- Wails
- React
- cloud API
- accounts
- project synchronization
- flashing
- OBD/J2534
- CAN communication
- MHD integration
- live data
- logging
- automatic tune generation
- AI
- map visualization
- 3D graphs
- interpolation tools
- A2L/DAMOS support
- generic checksum autodetection
- every XDF feature
- every BMW ECU

Keep the first milestone small.

---

# Technology

Use:

- Go 1.25+ or current stable Go
- Cobra for the CLI only if it materially simplifies command structure; otherwise standard `flag` is acceptable.
- Standard library XML parser for XDF.
- Standard Go testing.
- No database.
- No external service.
- No CGO.

Prefer standard library dependencies until there is a clear reason otherwise.

---

# Repository Layout

```text
freebeamer/
├── cmd/
│   └── freebeamer/
│       └── main.go
│
├── pkg/
│   ├── binfile/
│   │   ├── image.go
│   │   ├── read.go
│   │   ├── write.go
│   │   ├── diff.go
│   │   └── image_test.go
│   │
│   ├── xdf/
│   │   ├── model.go
│   │   ├── parser.go
│   │   ├── parser_test.go
│   │   └── testdata/
│   │
│   ├── calibration/
│   │   ├── model.go
│   │   ├── decode.go
│   │   ├── encode.go
│   │   ├── table.go
│   │   └── calibration_test.go
│   │
│   ├── expression/
│   │   ├── parser.go
│   │   ├── eval.go
│   │   ├── invert.go
│   │   └── expression_test.go
│   │
│   ├── identify/
│   │   ├── identify.go
│   │   └── identify_test.go
│   │
│   └── checksum/
│       ├── provider.go
│       ├── registry.go
│       ├── result.go
│       ├── common/
│       │   ├── crc32.go
│       │   ├── add16.go
│       │   └── add32.go
│       └── bmw/
│           └── mevd1726/
│               ├── provider.go
│               └── provider_test.go
│
├── testdata/
│   ├── public/
│   └── private/        # gitignored
│
├── .gitignore
├── go.mod
├── README.md
└── LICENSE
```

Do not introduce a generic "ECU framework" with dozens of abstractions before it is needed.

---

# Core Rule: Immutable Original

The binary object must always retain original bytes separately from working bytes.

Example:

```go
type Image struct {
    original []byte
    working  []byte
    path     string
}
```

Loading:

```go
img, err := binfile.Load(path)
```

Editing changes `working` only.

Expose:

```go
func (i *Image) Original() []byte
func (i *Image) Bytes() []byte
func (i *Image) Read(offset uint64, length int) ([]byte, error)
func (i *Image) Write(offset uint64, data []byte) error
func (i *Image) Diff() []Change
func (i *Image) Reset()
func (i *Image) SaveAs(path string) error
```

Never expose an API named simply `Save()` in V0. Force `SaveAs()`.

`Original()` and `Bytes()` should return defensive copies or otherwise prevent accidental external mutation.

---

# Phase 1 — BIN Engine

Implement BIN loading first.

Requirements:

- Reject empty files.
- Preserve exact size.
- Address bounds checks on all reads/writes.
- Support typed reads and writes:
  - uint8/int8
  - uint16/int16
  - uint32/int32
- Support little and big endian.
- Do not implement float until the fixture actually requires it.
- Keep byte writes deterministic.

Define:

```go
type Endian int

const (
    BigEndian Endian = iota
    LittleEndian
)
```

Diff result:

```go
type Change struct {
    Offset   uint64
    Original byte
    Current  byte
}
```

Acceptance tests:

- Loading the supplied BIN returns exactly 4,194,304 bytes.
- Its SHA-256 equals the known fixture hash.
- A write changes only the intended bytes.
- `Reset()` returns the working copy byte-for-byte to the original.
- Saving and reopening an untouched working copy produces an identical hash.
- Out-of-range reads/writes fail.

Commit after this phase.

---

# Phase 2 — Parse the Supplied XDF

Do not try to implement the entire theoretical XDF specification first.

Implement the subset used by `00000FEF81A001.xdf`, then generalize when another fixture requires it.

Parse at minimum:

## Header

- `XDFFORMAT.version`
- `XDFHEADER`
- `BASEOFFSET`
- `DEFAULTS`
- `REGION`
- `CATEGORY`

## Calibration entries

- `XDFTABLE`
- `XDFFLAG`

## Table fields

At minimum:

- title
- description
- unique ID if present
- category membership
- X/Y/Z axes
- units
- decimal precision / output formatting where present
- `EMBEDDEDDATA`
- `MATH`

For `EMBEDDEDDATA`, preserve raw attributes even when they are not interpreted yet.

Known attributes in the supplied fixture include:

- `mmedtypeflags`
- `mmedaddress`
- `mmedelementsizebits`
- `mmedrowcount`
- `mmedcolcount`
- `mmedmajorstridebits`
- `mmedminorstridebits`

Create a normalized XDF model rather than exposing raw XML everywhere.

Example:

```go
type Definition struct {
    Version    string
    Header     Header
    Categories []Category
    Tables     []Table
    Flags      []Flag
}

type Table struct {
    ID          string
    Title       string
    Description string
    Categories  []int
    X           Axis
    Y           Axis
    Z           Axis
}

type Axis struct {
    ID       string
    Units    string
    Embedded EmbeddedData
    Formula  string
}
```

Acceptance tests against supplied XDF:

- Version == `1.70`
- Category count == 73
- Table count == 962
- Flag count == 7
- Region size == `0x400000`
- Parsing produces no fatal errors.
- Unknown XML fields are ignored safely, not treated as fatal.

Add a CLI command:

```bash
freebeamer xdf info 00000FEF81A001.xdf
```

Expected useful output:

```text
XDF version: 1.70
Binary region: 0x000000-0x3FFFFF
Categories: 73
Tables: 962
Flags: 7
```

Add:

```bash
freebeamer maps list --xdf ...
```

Output:

```text
INDEX  CATEGORY  TITLE
...
```

Commit after this phase.

---

# Phase 3 — Expression Engine

The supplied XDF uses arithmetic formulas for raw-to-engineering-unit conversion.

Examples observed in the fixture include forms such as:

```text
X
X*1
X*0.1
X*0.01
X/10
X/4096*14.7
X*0.03125/3.6
X*3*0.145038
X*100/65536
```

V0 therefore only needs:

- variable `X`
- numeric literals
- `+`
- `-`
- `*`
- `/`
- parentheses
- unary `+` / `-`

Do not pull in a full scripting runtime.

Implement:

```go
func Parse(input string) (*Expression, error)
func (e *Expression) Eval(x float64) (float64, error)
```

For encoding edited values back to raw values, implement inversion for the arithmetic subset.

Prefer an AST-based affine simplifier if possible:

```text
display = a*X + b
raw     = (display-b)/a
```

If all current fixture expressions reduce to affine transforms, do not implement numerical root-finding in V0.

Add a fixture test that parses **every MATH expression in the supplied XDF**. The test should fail and print the unsupported equation if a new expression form appears.

Acceptance:

- Every expression in this XDF parses successfully.
- Decode/encode roundtrip remains within the quantization allowed by the raw datatype.

Commit after this phase.

---

# Phase 4 — Calibration Decoder

Connect XDF definitions to BIN bytes.

The calibration package should know:

```text
XDF definition
      +
BIN image
      ↓
decoded calibration values
```

Do not put XML logic here.

Implement XDF `mmedtypeflags`, element size, signedness, and byte order based on the actual XDF semantics.

Important:

Do not guess the meaning of a flag bit and silently decode with it.

For each type flag combination used by the supplied XDF:

1. determine its documented meaning;
2. create a named representation;
3. write a targeted unit test.

Element sizes required by the fixture include:

- 8 bit
- 16 bit
- 32 bit

Implement table dimensions from row/column metadata.

Expose:

```go
type DecodedTable struct {
    Definition xdf.Table
    X          []float64
    Y          []float64
    Z          [][]float64
}
```

Add CLI:

```bash
freebeamer maps show \
  --xdf 00000FEF81A001.xdf \
  --bin 00000FEF81A001_original.bin \
  --name "Performance gauge scaling"
```

Print:

- title
- category
- addresses
- dimensions
- units
- formula
- raw values
- decoded values

Do not optimize output formatting yet.

Acceptance:

- Every addressed map in the supplied XDF is bounds-checked against the 4 MiB BIN.
- Invalid definitions are reported, not panicked.
- At least several representative 1D/2D/3D tables are verified manually against TunerPro or another known-good decoder before proceeding.

Commit after this phase.

---

# Phase 5 — Editing

Add editing only after decoding is trustworthy.

CLI examples:

```bash
freebeamer maps set \
  --xdf definition.xdf \
  --bin original.bin \
  --map "Some map" \
  --row 2 \
  --col 3 \
  --value 1.25 \
  --output modified.bin
```

For 1D/scalar-like tables, allow:

```bash
--index N
```

Never write the original file.

Workflow:

1. Load original.
2. Decode target raw value.
3. Parse user engineering value.
4. Invert XDF equation.
5. Quantize to valid raw integer.
6. Verify raw value fits datatype.
7. Write exact bytes.
8. Decode from modified bytes again.
9. Print requested value, encoded raw value, and actual decoded value.
10. Save to `--output`.

If requested value cannot be represented exactly, report quantization:

```text
requested: 1.234
encoded:   1.230
raw:       123
```

Do not silently clamp overflow.

Add:

```bash
freebeamer diff original.bin modified.bin
```

and, when XDF is also supplied:

```bash
freebeamer diff \
  --xdf definition.xdf \
  original.bin modified.bin
```

The XDF-aware diff may additionally show which calibration map/cell owns each changed byte if straightforward.

Acceptance:

- Editing one cell changes exactly the expected byte range.
- Neighboring cells remain unchanged.
- Reloading the output decodes to the edited value.
- Original SHA-256 never changes.

Commit after this phase.

---

# Phase 6 — ECU Identification

Start with simple, explicit identification for the supplied target.

Create:

```go
type Identity struct {
    Manufacturer string
    ECUVendor    string
    ECUFamily    string
    Engine       string
    Software     []string
    Confidence   Confidence
}
```

For MEVD17.2.6, identify using multiple independent properties where available:

- 4 MiB size
- embedded `MEVD17.2.6` marker
- BMW/N55 firmware marker
- software identifiers found in the BIN

Do not identify solely by filename.

Add:

```bash
freebeamer identify original.bin
```

Expected result for the current fixture:

```text
Vendor: Bosch
ECU: MEVD17.2.6
Engine family: BMW N55
Confidence: high
```

Do not make exact chassis/model claims unless present in firmware or backed by a verified database.

Commit after this phase.

---

# Phase 7 — Checksum Framework

Create the provider interface before implementing MEVD17.2.6:

```go
type Provider interface {
    ID() string
    Detect(image []byte) Detection
    Verify(image []byte) ([]Result, error)
    Correct(image []byte) ([]Result, error)
}
```

Result:

```go
type Result struct {
    Name            string
    RegionStart     uint64
    RegionEnd       uint64
    StorageOffset   uint64
    Stored          uint64
    Calculated      uint64
    Valid           bool
    Corrected       bool
}
```

Registry:

```go
type Registry struct {
    providers []Provider
}
```

The CLI must refuse correction if zero or multiple equally confident providers match.

Commands:

```bash
freebeamer checksum verify original.bin
freebeamer checksum fix modified.bin --output modified_fixed.bin
```

`fix` must never overwrite input.

---

# Phase 8 — MEVD17.2.6 Checksum Research and Implementation

This is a validation gate, not a place to guess.

The supplied XDF contains no XDF checksum definitions, so MEVD17.2.6 integrity logic must be implemented independently.

Start by researching existing open implementations of Bosch MED17/EDC17 checksum block structures and determine whether the same structures/algorithms apply to this exact MEVD17.2.6 firmware.

A useful research lead is the open-source `medc17-checksum-tool`, which implements Bosch MED17/EDC17 block detection with CRC32, ADD32 and ADD16 handling. Do **not** assume it applies to MEVD17.2.6 merely because the family names are similar.

Validation procedure:

## A. Original must verify

Our provider is not accepted until:

```text
Verify(00000FEF81A001_original.bin) == all relevant regions valid
```

If it does not, stop. Do not "correct" the stock file to make the test pass.

## B. Controlled mutation

Modify exactly one known calibration byte through our calibration engine.

Then:

```text
Verify(modified.bin)
```

must show the expected integrity failure.

## C. Correction

Run:

```text
Correct(modified.bin)
```

Then:

```text
Verify(corrected.bin)
```

must return all relevant regions valid.

## D. Independent oracle

Before treating this checksum provider as flash-safe, compare our corrected result with at least one independent known-good checksum implementation/tool for the same MEVD17.2.6 software.

Matching byte-for-byte is ideal but not automatically required if the format permits multiple mathematically valid adjustment values. In that case independently verify the ECU's integrity conditions.

## E. More than one firmware

Do not label the provider as generally supporting every MEVD17.2.6 revision after testing only `75P9EJ0B`.

Initially register support as firmware-specific if necessary.

Example:

```text
bosch.meVD17.2.6.75P9EJ0B
```

Generalize to the full ECU family only after multiple firmware variants prove the logic is shared.

## F. No checksum success = no flashing claim

Until this phase is proven, README wording must say:

```text
BIN/XDF editing: experimental
MEVD17.2.6 checksum support: unverified
Do not flash generated files
```

Do not weaken this warning just because unit tests pass on synthetic data.

---

# Phase 9 — End-to-End Command

After all previous phases pass, add a simple project command:

```bash
freebeamer inspect \
  --xdf 00000FEF81A001.xdf \
  --bin 00000FEF81A001_original.bin
```

Output:

```text
ROM:
  size: 4194304
  sha256: ...

ECU:
  Bosch MEVD17.2.6
  BMW N55

XDF:
  version: 1.70
  region: 0x000000-0x3FFFFF
  categories: 73
  tables: 962
  flags: 7

Compatibility:
  XDF region fits BIN: yes
  addressed maps in bounds: ...
  parse errors: ...

Checksums:
  provider: ...
  status: valid / unsupported / unverified
```

This should become the first command a user runs on a new BIN/XDF pair.

---

# CLI V0 Surface

Keep the command set tiny:

```text
freebeamer identify BIN

freebeamer xdf info XDF

freebeamer inspect --xdf XDF --bin BIN

freebeamer maps list --xdf XDF
freebeamer maps show --xdf XDF --bin BIN --name NAME

freebeamer maps set \
  --xdf XDF \
  --bin BIN \
  --map NAME \
  [--row N --col N | --index N] \
  --value VALUE \
  --output OUTPUT

freebeamer diff OLD NEW

freebeamer checksum verify BIN
freebeamer checksum fix BIN --output OUTPUT
```

No interactive shell yet.

---

# Error Handling

Errors must be explicit and typed where useful.

Examples:

```text
XDF binary region expects 0x400000 bytes; BIN contains 0x200000.
```

```text
Map "Boost target" references 0x400010, outside BIN.
```

```text
Value 70000 cannot fit unsigned 16-bit raw representation.
```

```text
Checksum provider not found for this firmware.
```

Never panic for malformed user files.

---

# Safety Requirements

These are mandatory:

1. Original BIN is immutable.
2. Never overwrite input BIN.
3. Every write is bounds checked.
4. Every engineering value is range checked before raw encoding.
5. Every output can produce a byte diff.
6. Checksum correction requires a positive provider match.
7. `checksum fix` re-runs `verify` after correction.
8. A failed post-correction verify means no output is written unless a special developer-only debug flag is explicitly used.
9. Tests must include corruption cases.
10. No flashing code in this milestone.
11. Clearly label checksum support as unverified until independently validated.

---

# Testing Strategy

## Unit tests

- binary typed read/write
- endian handling
- diff
- XDF parser
- expression parser
- expression inversion
- datatype conversion
- address calculation
- checksum primitives
- provider detection

## Fixture tests

Using private current fixture:

```text
XDF loads
BIN loads
962 tables parsed
73 categories parsed
7 flags parsed
all MATH equations supported
all map addresses bounds checked
known maps decode
known map edits encode
original never changes
```

## Fuzz tests

Once the deterministic tests pass:

- fuzz XDF parsing
- fuzz expression parsing
- fuzz address/size combinations
- fuzz corrupted BIN inputs into checksum verifier

Checksum correction should not be the first area fuzzed; get known-good deterministic behavior first.

---

# Codex Working Rules

Codex should work one phase at a time.

For every phase:

1. Read this plan.
2. Implement only the current phase.
3. Add tests.
4. Run:
   ```bash
   go test ./...
   go vet ./...
   ```
5. Report:
   - files changed
   - tests added
   - commands run
   - current limitations
6. Do not begin the next phase until the current phase passes.

Do not introduce UI/cloud abstractions "for later."

Do not rewrite working code between phases without a concrete reason.

Do not fabricate XDF semantics. If an XDF flag meaning is unclear, document the uncertainty and research it before decoding it.

Do not fabricate MEVD17 checksum behavior.

---

# Recommended First Codex Task

Give Codex only this task first:

> Initialize the Go repository and implement Phase 1 only: the immutable BIN engine. Add loading, SHA-256 reporting, safe bounded reads/writes, typed integer reads/writes with explicit endian handling, byte-level diff, reset, and SaveAs. Use `00000FEF81A001_original.bin` from `testdata/private/` as a local integration fixture, but ensure `testdata/private/` is gitignored. Add tests proving the fixture is 4,194,304 bytes and has SHA-256 `6509dc5257948091194ca9257b0d5763dcc67ca014e18df4d605d419738aadc8`. Do not implement XDF parsing yet. Run `go test ./...` and `go vet ./...`.

That is intentionally small.

Once Phase 1 is green, give Codex Phase 2.

---

# Definition of V0 Complete

V0 is complete only when this sequence works:

```bash
freebeamer inspect \
  --xdf 00000FEF81A001.xdf \
  --bin 00000FEF81A001_original.bin

freebeamer maps list --xdf 00000FEF81A001.xdf

freebeamer maps show \
  --xdf 00000FEF81A001.xdf \
  --bin 00000FEF81A001_original.bin \
  --name "Performance gauge scaling"

freebeamer maps set \
  --xdf 00000FEF81A001.xdf \
  --bin 00000FEF81A001_original.bin \
  --map "Performance gauge scaling" \
  --row 0 \
  --col 0 \
  --value <TEST_VALUE> \
  --output /tmp/freebeamer_modified.bin

freebeamer diff \
  00000FEF81A001_original.bin \
  /tmp/freebeamer_modified.bin

freebeamer checksum verify /tmp/freebeamer_modified.bin

freebeamer checksum fix \
  /tmp/freebeamer_modified.bin \
  --output /tmp/freebeamer_fixed.bin

freebeamer checksum verify /tmp/freebeamer_fixed.bin
```

And the final verification is backed by independently validated MEVD17.2.6 checksum logic.

Only after this milestone should work begin on the desktop interface, real-time data, ECU communication, or cloud collaboration.
