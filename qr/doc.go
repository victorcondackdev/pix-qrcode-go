// Package qr is a QR Code encoder (ISO/IEC 18004:2015, Model 2) written
// from scratch on the standard library.
//
// [Encode] turns text or bytes into a [Code]: a square grid of modules
// with its version (1 to 40), error correction level ([L], [M], [Q], [H])
// and mask. Rendering lives in package
// [github.com/victorcondackdev/pix-qrcode-go/render].
//
// # Segmentation
//
// QR Codes can switch encoding mode mid-stream. Numeric mode stores three
// digits in 10 bits, alphanumeric mode (digits, uppercase letters and
// " $%*+-./:") two characters in 11 bits, and byte mode one byte in 8
// bits; every switch costs a 4-bit mode indicator plus a character count
// of 8 to 16 bits. Encode picks the split with the fewest total bits by
// dynamic programming over the input, with a state per mode and per
// partial group (numeric length mod 3, alphanumeric length mod 2), which
// makes every per-character cost exact and the result optimal for each
// version range (1-9, 10-26, 27-40, where count widths differ). The
// smallest version that fits is chosen afterwards. Multi-byte UTF-8
// characters stay whole in byte segments. [WithByteMode] turns this off.
//
// # Construction
//
// Data codewords are split into the Reed–Solomon blocks of ISO/IEC 18004
// Table 9 and each block gets its EC codewords from a generator polynomial
// over GF(2^8) (field polynomial 0x11D), computed at start-up rather than
// tabulated. Blocks are interleaved and placed in the standard zigzag
// order around the finder, timing and alignment patterns; alignment
// positions come from the closed form behind Annex E. All eight masks are
// scored with the four penalty rules of section 7.8.3 and the lowest wins
// unless [WithMask] forces one. Format information is protected by a
// BCH(15,5) code and, from version 7, version information by BCH(18,6).
//
// # Verification
//
// The test suite decodes every symbol it makes with an independent
// decoder (internal/decode) that checks function patterns, format and
// version information, Reed–Solomon syndromes, segment syntax and padding,
// across all versions, levels, masks and modes, and under fuzzing.
// Published codeword vectors, the capacity tables of the standard and
// BCH distances are checked separately.
package qr
