# Phase 4 independent decoder validation

Date: 2026-09-01
Status: Completed

FreeBeamer calibration decoding was compared with the independent KingAI
TunerPro XDF + BIN Universal Exporter v3.6.0 at commit
`18697503fd5b1208045f667affa7c50803f6f894`.

Inputs:

- Exact `00000FEF81A001.xdf` from the public BMW-XDFs repository.
- A deterministic synthetic 4 MiB BIN. No proprietary firmware was used.
- Explicit little-endian patterns written only at the maps' declared addresses.

Results:

| Shape | Map | Raw values | Independently decoded values |
| --- | --- | --- | --- |
| 1×1 | `Zeitkonstante zum Abschalten des Massenstromregler Hub` | `2000` | `0.9765626000000001` |
| 1×6 | `Performance gauge scaling X (autogen)` | `100, 200, …, 600` | `10, 20, …, 60` |
| 6×6 | `Performance gauge scaling` | signed `-1800` through `1700` by 100 | `-180` through `170` by 10, in identical row order |

FreeBeamer produced the same raw interpretation, dimensions, row order, signed
values, and engineering-unit values for all three cases. The exact expected
matrices are retained in `pkg/calibration/fixture_test.go`.

The independent exporter is available at:
<https://github.com/KingAiCodeForge/TunerPro-XDF-BIN-Universal-Exporter>

This comparison validates the decoding path but does not validate stock
firmware contents, because the original BIN is not present. The stock-BIN hash
and checksum gates remain pending.
