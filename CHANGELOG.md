# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-11-21

First public release.

### Added

- `qr`: QR Code encoder for versions 1-40 and levels L, M, Q, H with
  numeric, alphanumeric and byte modes; optimal segmentation by dynamic
  programming; Reed–Solomon over GF(256) with computed generator
  polynomials; all eight masks with the four penalty rules; BCH format and
  version information. Options `WithMinVersion`, `WithMaxVersion`,
  `WithMask`, `WithBoostECL`, `WithByteMode`. `Code.At` and
  `Code.CheckClearArea`.
- `render`: 1-bit PNG writer, SVG with a single run-merged `<path>`,
  Unicode half-block terminal output, data URIs, exact-size PNGs and a
  guarded logo area for SVG.
- `pixqr`: BR Code envelope validation (prefix and CRC-16/CCITT-FALSE),
  `PNG`, `SVG`, `Encode` and `Info` with savings against byte mode.
- `cmd/pixqr`: command-line interface (PNG, SVG, terminal, `--info`,
  stdin).
- `examples/server`: `GET /qr.svg` handler with ETag and immutable caching.
- `internal/decode`: independent strict decoder used by the round-trip and
  fuzz tests; `scripts/opencv_crosscheck.py` for an optional external
  check.

[Unreleased]: https://github.com/victorcondackdev/pix-qrcode-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/victorcondackdev/pix-qrcode-go/releases/tag/v0.1.0
