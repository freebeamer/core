# XDF semantics implemented in V0

FreeBeamer deliberately supports only the XDF memory representation needed by
the initial MEVD17.2.6 fixture.

The named `mmedtypeflags` bits are:

| Bit | Name | Meaning |
| ---: | --- | --- |
| `0x01` | signed | Interpret the integer using two's complement. |
| `0x02` | LSB first | Store the least-significant byte first. |
| `0x04` | column major | Populate table cells down columns before advancing columns. |
| `0x10000` | floating point | Recognized but rejected in V0. |

Evidence:

- TunerPro's official table-editor documentation defines signed source values,
  LSB-first byte order, and row-versus-column population:
  <https://tunerpro.net/WebHelp/source/xdftableeditor.htm>
- OpenEEC's XDF serializer identifies `0x01` as signed and `0x02` as LSB-first:
  <https://github.com/OpenEEC-Project/SAD806x/blob/master/SADXdf.cs>
- The open a2l2xdf generator uses `0x04` on Z data and `0x10000` for IEEE-754
  values:
  <https://github.com/bri3d/a2l2xdf/blob/master/a2l2xdf.py>
- MxT's independent implementation identifies bit 2 as column-major:
  <https://github.com/TracqiTechnology/MxT>

The initial `00000FEF81A001.xdf` uses addressed combinations `0x00`, `0x01`,
`0x02`, `0x03`, `0x06`, and `0x07`, with 8-, 16-, and 32-bit integers. Tests
cover each behavior and reject unknown bits.

Non-zero major/minor stride is not decoded. In the initial fixture, negative
major strides occur on virtual, unaddressed axes; these are safely skipped.
Any addressed axis using a non-zero stride produces an explicit error until its
layout is independently validated.

IEEE-754 data remains outside V0 as required by the implementation plan.

## XDFFLAG bit/mask semantics

`XDFFLAG` elements may carry a `<mask>` child element giving the bitmask
applied to the addressed byte. When that mask sets exactly one in-range bit,
FreeBeamer records it as `layout.bitPosition` and decodes/encodes that single
bit via `pkg/calibration`'s read-modify-write path, leaving sibling bits
(other flags packed into the same byte) untouched. See the XDFFLAG mask
evidence section in [xdf-compatibility.md](xdf-compatibility.md) for sourcing
and the cases that intentionally fall back to whole-word decoding.
