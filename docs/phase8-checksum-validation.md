# Phase 8 checksum validation

Status: Completed

Validation was performed on 2026-09-01 against the exact 4 MiB BMW
MEVD17.2.6 `75P9EJ0B` stock image used by the private integration tests:

```text
SHA-256 6509dc5257948091194ca9257b0d5763dcc67ca014e18df4d605d419738aadc8
```

The independent oracle was Connor Howell's
[`medc17-checksum-tool`](https://github.com/ConnorHowell/medc17-checksum-tool)
at commit `1e962dcf4766ac786dd00d6be28a9ba5f8155e88`. The XDF and stock image
came from the public
[`dmacpro91/BMW-XDFs`](https://github.com/dmacpro91/BMW-XDFs) repository.

## Firmware layout

FreeBeamer requires the exact image size, software marker, and all six checksum
block headers before selecting this provider.

| File offset | Block ID | Block size | Descriptors | Trailer |
|---:|---:|---:|---:|---|
| `0x000000` | `0x00000030` | `0x00fc04` | 2 | zero |
| `0x014000` | `0x00800020` | `0x003f00` | 1 | `DEADBEEF` |
| `0x018000` | `0x00000010` | `0x007f00` | 2 | `DEADBEEF` |
| `0x020000` | `0x00000040` | `0x15fc04` | 3 | zero |
| `0x180000` | `0x00000060` | `0x07fc04` | 2 | zero |
| `0x220000` | `0x00000050` | `0x1dfc04` | 1 | zero |

The upstream oracle's automatic detector recognizes only the two blocks with
`DEADBEEF` trailers. For the comparison, the four zero-trailer block
descriptors were supplied manually to the oracle; its checksum, address, and
correction algorithms were not changed.

## Results

- The stock image validates all 11 descriptors in both implementations.
- Changing the first cell of `Performance gauge scaling` from raw `0` to raw
  `12` at file offset `0x18171c` invalidates exactly the calibration dataset's
  ADD16 checksum: computed `0xCAFEB00A`, expected `0xCAFEAFFE`.
- FreeBeamer and the independent oracle both restore all 11 checksums by changing
  the calibration compensation word only.
- The independently corrected binary has SHA-256
  `698dc5eb0a18923464d978a41880883789355c0d42f25096945e61b080f6a737`,
  which is asserted by the private integration test.

## Safety boundary

Correction is enabled only for the calibration dataset ADD16 checksum in this
exact `75P9EJ0B` layout. FreeBeamer refuses to correct code, CRC, other dataset,
unknown-layout, or unknown-software failures. It writes transactionally and
re-verifies every descriptor before returning output.

This validation establishes checksum equivalence for the tested edits. It is
not a general MEVD17 checksum claim and does not establish that an output is
safe to flash.
