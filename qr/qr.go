package qr

import (
	"errors"
	"fmt"
)

// Level is an error correction level. Higher levels recover from more
// damage at the cost of capacity.
type Level uint8

// Error correction levels, in increasing order of redundancy.
const (
	L Level = iota // recovers about 7% of codewords
	M              // about 15%
	Q              // about 25%
	H              // about 30%
)

// String returns "L", "M", "Q" or "H".
func (l Level) String() string {
	if l <= H {
		return string("LMQH"[l])
	}
	return fmt.Sprintf("Level(%d)", uint8(l))
}

// ParseLevel parses "L", "M", "Q" or "H" (case-insensitive).
func ParseLevel(s string) (Level, error) {
	if len(s) == 1 {
		switch s[0] | 0x20 {
		case 'l':
			return L, nil
		case 'm':
			return M, nil
		case 'q':
			return Q, nil
		case 'h':
			return H, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", ErrInvalidLevel, s)
}

// formatBits is the 2-bit level indicator used in the format information.
func (l Level) formatBits() uint32 { return [4]uint32{1, 0, 3, 2}[l] }

// Errors returned by Encode. Errors about the input wrap ErrTooLong; the
// others report invalid arguments.
var (
	ErrTooLong        = errors.New("qr: data too long")
	ErrInvalidLevel   = errors.New("qr: invalid error correction level")
	ErrInvalidVersion = errors.New("qr: invalid version range")
	ErrInvalidMask    = errors.New("qr: invalid mask")
)

// MinVersion and MaxVersion bound the symbol versions (sizes 21 to 177).
const (
	MinVersion = 1
	MaxVersion = 40
)

// Code is an encoded QR Code symbol.
type Code struct {
	Version int   // 1 to 40
	Level   Level // error correction level actually used
	Mask    int   // mask pattern, 0 to 7
	Size    int   // modules per side: 4*Version + 17
	// Modules[y][x] is true for a dark module. The quiet zone is not
	// included; renderers add it.
	Modules [][]bool
	// Segments is the segmentation that was encoded.
	Segments []Segment
	// DataBits is the length of the encoded segments, headers included,
	// before the terminator and padding.
	DataBits int
}

// At reports whether the module at column x, row y is dark. Coordinates
// outside the symbol (the quiet zone) are light.
func (c *Code) At(x, y int) bool {
	if x < 0 || y < 0 || x >= c.Size || y >= c.Size {
		return false
	}
	return c.Modules[y][x]
}

type config struct {
	minVersion, maxVersion int
	mask                   int
	boost                  bool
	byteMode               bool
}

// Option configures Encode.
type Option func(*config)

// WithMinVersion sets the smallest version Encode may choose. Use it to
// get a stable symbol size across payloads of slightly different lengths.
func WithMinVersion(v int) Option { return func(c *config) { c.minVersion = v } }

// WithMaxVersion sets the largest version Encode may choose; data that
// does not fit fails with ErrTooLong.
func WithMaxVersion(v int) Option { return func(c *config) { c.maxVersion = v } }

// WithMask forces a mask pattern (0 to 7) instead of choosing the one with
// the lowest penalty score. Pass -1 for automatic selection.
func WithMask(m int) Option { return func(c *config) { c.mask = m } }

// WithBoostECL raises the error correction level as far as possible
// without increasing the version: the symbol keeps its size and gains
// redundancy for free.
func WithBoostECL() Option { return func(c *config) { c.boost = true } }

// WithByteMode disables optimal segmentation and encodes the whole input
// as a single byte-mode segment, the way most simple encoders do. It
// exists for comparison and for readers with poor mode-switch support.
func WithByteMode() Option { return func(c *config) { c.byteMode = true } }

// Encode encodes data (UTF-8 text or arbitrary bytes) in the smallest
// symbol that holds it at the given error correction level, between the
// configured minimum and maximum versions.
//
// The input is split into numeric, alphanumeric and byte segments with the
// fewest total bits (see the package documentation), so mostly-uppercase
// and digit payloads such as Pix BR Codes need fewer codewords than in
// byte mode. Byte segments carry the input bytes unchanged, without an ECI
// header.
func Encode(data string, level Level, opts ...Option) (*Code, error) {
	cfg := config{minVersion: MinVersion, maxVersion: MaxVersion, mask: -1}
	for _, o := range opts {
		o(&cfg)
	}
	if level > H {
		return nil, fmt.Errorf("%w: %d", ErrInvalidLevel, level)
	}
	if cfg.minVersion < MinVersion || cfg.maxVersion > MaxVersion || cfg.minVersion > cfg.maxVersion {
		return nil, fmt.Errorf("%w: %d to %d", ErrInvalidVersion, cfg.minVersion, cfg.maxVersion)
	}
	if cfg.mask < -1 || cfg.mask > 7 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidMask, cfg.mask)
	}

	// Segmentations depend only on the version group (count indicator
	// widths), so compute at most three.
	var cache [3][]Segment
	var cached [3]bool
	segmentsFor := func(v int) []Segment {
		g := 0
		if v >= 27 {
			g = 2
		} else if v >= 10 {
			g = 1
		}
		if !cached[g] {
			cached[g] = true
			switch {
			case data == "":
			case cfg.byteMode:
				cache[g] = []Segment{{Mode: Byte, Data: data}}
			default:
				cache[g] = optimalSegments(data, v)
			}
		}
		return cache[g]
	}

	version, bits := 0, 0
	var segs []Segment
	for v := cfg.minVersion; v <= cfg.maxVersion; v++ {
		s := segmentsFor(v)
		if n := totalBits(s, v); n >= 0 && n <= dataCodewords(v, level)*8 {
			version, bits, segs = v, n, s
			break
		}
	}
	if version == 0 {
		v := cfg.maxVersion
		return nil, fmt.Errorf("%w: needs %d bits, version %d-%s holds %d",
			ErrTooLong, totalBits(segmentsFor(v), v), v, level, dataCodewords(v, level)*8)
	}
	if cfg.boost {
		for l := level + 1; l <= H; l++ {
			if bits <= dataCodewords(version, l)*8 {
				level = l
			}
		}
	}

	m := newMatrix(version)
	m.drawFunctionPatterns()
	m.drawCodewords(interleave(dataStream(segs, version, level), version, level))
	mask := cfg.mask
	if mask < 0 {
		mask = m.bestMask(level)
	}
	m.applyMask(mask)
	m.drawFormat(level, mask)

	return &Code{
		Version:  version,
		Level:    level,
		Mask:     mask,
		Size:     m.size,
		Modules:  m.rows(),
		Segments: segs,
		DataBits: bits,
	}, nil
}

// dataStream returns the data codewords for segs: the segments, a
// terminator of up to four zero bits, zero padding to a byte boundary and
// alternating 0xEC/0x11 pad codewords up to the symbol's data capacity.
func dataStream(segs []Segment, version int, level Level) []byte {
	capacity := dataCodewords(version, level) * 8
	bb := bitBuffer{b: make([]byte, 0, capacity/8)}
	for _, s := range segs {
		bb.appendSegment(s, version)
	}
	bb.append(0, min(4, capacity-bb.n))
	bb.append(0, (8-bb.n%8)%8)
	for pad := uint32(0xEC); bb.n < capacity; pad ^= 0xEC ^ 0x11 {
		bb.append(pad, 8)
	}
	return bb.b
}

// blockLayout describes how a symbol's codewords are split into blocks.
type blockLayout struct {
	blocks, ecc     int
	short           int // number of short blocks (they come first)
	shortData       int // data codewords in a short block; long ones have one more
	total, dataSize int
}

func layout(version int, level Level) blockLayout {
	b := blockLayout{
		blocks: int(numBlocks[level][version]),
		ecc:    int(eccPerBlock[level][version]),
		total:  totalCodewords(version),
	}
	b.short = b.blocks - b.total%b.blocks
	b.shortData = b.total/b.blocks - b.ecc
	b.dataSize = b.total - b.blocks*b.ecc
	return b
}

func (b blockLayout) dataLen(i int) int {
	if i < b.short {
		return b.shortData
	}
	return b.shortData + 1
}

// interleave splits data into blocks, appends each block's EC codewords
// and interleaves them: data codewords column by column across blocks,
// then EC codewords the same way.
func interleave(data []byte, version int, level Level) []byte {
	b := layout(version, level)
	gen := rsGenerators[b.ecc]
	out := make([]byte, b.total)
	ecc := make([]byte, b.ecc)
	off := 0
	for i := 0; i < b.blocks; i++ {
		n := b.dataLen(i)
		blk := data[off : off+n]
		off += n
		for j, c := range blk {
			k := j * b.blocks
			if j == b.shortData {
				k = b.shortData*b.blocks + (i - b.short) // only long blocks reach here
			} else {
				k += i
			}
			out[k] = c
		}
		rsRemainder(blk, gen, ecc)
		for j, c := range ecc {
			out[b.dataSize+j*b.blocks+i] = c
		}
	}
	return out
}
