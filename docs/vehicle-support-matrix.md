# Vehicle support and evidence matrix

Updated: 2026-09-16. First hardware target: the owner’s N55 + MHD WiFi +
Android, as confirmed by the owner. No physical vehicle has verified live-data
support yet.

“Mock tested” establishes software behavior only. “Unknown” means no recorded
validation; it is not a compatibility claim. Each future hardware row must name
an exact ECU/software and adapter/firmware combination, with a validation record.

| Evidence target | Transport | Monitor decoding | Physical units/scaling | Calibration definition match | Checksum correction |
|---|---|---|---|---|---|
| Synthetic MG1-format ECU over loopback; Android emulator and Go CLI | Mock tested | 19 fields, mock tested | Definition-derived; no vehicle comparison | Unknown | None established |
| Owner’s N55 + MHD WiFi + Android; installed software pending trial confirmation | Unknown | Unknown; current reader uses MG1 definition | Unknown | N55 editor baseline; installed firmware match unverified | Only if installed image matches the validated layout |
| BMW N55 / Bosch MEVD17.2.6, `75P9EJ0B` fixture | Unknown | Unknown; MG1 decoder does not establish support | Unknown | Recorded `00000FEF81A001.xdf` / 4 MiB BIN fixture | Exact calibration ADD16 layout independently validated |

Evidence:

- [Monitor implementation and derivation](mhd-live-monitor-v0-plan.md).
- [Recorded mobile mock/emulator trial](https://github.com/freebeamer/mobile/blob/main/docs/mobile-development.md).
- [Independent calibration decoder comparison](phase4-oracle-validation.md).
- [Exact firmware checksum validation](phase8-checksum-validation.md).
- [Hardware procedure and record template](vehicle-hardware-validation.md).

Do not combine the MG1 and N55 rows into a single supported vehicle. Add a
hardware row only with an evidence record, and mark each capability separately.
A software test pass or a manually selected profile cannot promote a hardware
capability to validated. Checksums alone do not establish suitability for flashing.
