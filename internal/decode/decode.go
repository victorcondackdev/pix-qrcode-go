// Package decode reads back the symbols produced by package qr. It is a
// verification tool, not a camera decoder: it expects a perfect module
// grid and checks it strictly instead of correcting errors.
//
// It is written independently of the encoder (separate tables, separate
// placement and parsing code) so that round-trip tests catch mistakes that
// a shared implementation would hide. Every check that a real reader
// performs on a clean symbol is done here and failure is an error:
// format and version information within BCH distance, function patterns,
// zero Reed–Solomon syndromes in every block, well-formed segments,
// terminator and pad codewords.
package decode

import (
	"errors"
	"fmt"
	"math/bits"
)

// Segment is a decoded segment.
type Segment struct {
	Mode  string // "numeric", "alphanumeric" or "byte"
	Count int    // characters (bytes for byte mode)
}

// Result is a decoded symbol.
type Result struct {
	Version  int
	Level    string // "L", "M", "Q" or "H"
	Mask     int
	Data     []byte
	Segments []Segment
	// FormatErrors and VersionErrors are the bit errors corrected in the
	// best format and version information copies; a symbol straight from
	// the encoder has zero.
	FormatErrors, VersionErrors int
}

// ErrInvalid is wrapped by every decoding error.
var ErrInvalid = errors.New("decode: invalid symbol")

func fail(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// ecTable holds, per version, the (EC codewords per block, block count)
// pairs for levels L, M, Q, H, from ISO/IEC 18004 Table 9.
var ecTable = [41][4][2]int{
	{},
	{{7, 1}, {10, 1}, {13, 1}, {17, 1}},
	{{10, 1}, {16, 1}, {22, 1}, {28, 1}},
	{{15, 1}, {26, 1}, {18, 2}, {22, 2}},
	{{20, 1}, {18, 2}, {26, 2}, {16, 4}},
	{{26, 1}, {24, 2}, {18, 4}, {22, 4}},
	{{18, 2}, {16, 4}, {24, 4}, {28, 4}},
	{{20, 2}, {18, 4}, {18, 6}, {26, 5}},
	{{24, 2}, {22, 4}, {22, 6}, {26, 6}},
	{{30, 2}, {22, 5}, {20, 8}, {24, 8}},
	{{18, 4}, {26, 5}, {24, 8}, {28, 8}},
	{{20, 4}, {30, 5}, {28, 8}, {24, 11}},
	{{24, 4}, {22, 8}, {26, 10}, {28, 11}},
	{{26, 4}, {22, 9}, {24, 12}, {22, 16}},
	{{30, 4}, {24, 9}, {20, 16}, {24, 16}},
	{{22, 6}, {24, 10}, {30, 12}, {24, 18}},
	{{24, 6}, {28, 10}, {24, 17}, {30, 16}},
	{{28, 6}, {28, 11}, {28, 16}, {28, 19}},
	{{30, 6}, {26, 13}, {28, 18}, {28, 21}},
	{{28, 7}, {26, 14}, {26, 21}, {26, 25}},
	{{28, 8}, {26, 16}, {30, 20}, {28, 25}},
	{{28, 8}, {26, 17}, {28, 23}, {30, 25}},
	{{28, 9}, {28, 17}, {30, 23}, {24, 34}},
	{{30, 9}, {28, 18}, {30, 25}, {30, 30}},
	{{30, 10}, {28, 20}, {30, 27}, {30, 32}},
	{{26, 12}, {28, 21}, {30, 29}, {30, 35}},
	{{28, 12}, {28, 23}, {28, 34}, {30, 37}},
	{{30, 12}, {28, 25}, {30, 34}, {30, 40}},
	{{30, 13}, {28, 26}, {30, 35}, {30, 42}},
	{{30, 14}, {28, 28}, {30, 38}, {30, 45}},
	{{30, 15}, {28, 29}, {30, 40}, {30, 48}},
	{{30, 16}, {28, 31}, {30, 43}, {30, 51}},
	{{30, 17}, {28, 33}, {30, 45}, {30, 54}},
	{{30, 18}, {28, 35}, {30, 48}, {30, 57}},
	{{30, 19}, {28, 37}, {30, 51}, {30, 60}},
	{{30, 19}, {28, 38}, {30, 53}, {30, 63}},
	{{30, 20}, {28, 40}, {30, 56}, {30, 66}},
	{{30, 21}, {28, 43}, {30, 59}, {30, 70}},
	{{30, 22}, {28, 45}, {30, 62}, {30, 74}},
	{{30, 24}, {28, 47}, {30, 65}, {30, 77}},
	{{30, 25}, {28, 49}, {30, 68}, {30, 81}},
}

// alignTable lists alignment pattern centre coordinates (Annex E).
var alignTable = [41][]int{
	2: {6, 18}, 3: {6, 22}, 4: {6, 26}, 5: {6, 30}, 6: {6, 34},
	7: {6, 22, 38}, 8: {6, 24, 42}, 9: {6, 26, 46}, 10: {6, 28, 50},
	11: {6, 30, 54}, 12: {6, 32, 58}, 13: {6, 34, 62},
	14: {6, 26, 46, 66}, 15: {6, 26, 48, 70}, 16: {6, 26, 50, 74},
	17: {6, 30, 54, 78}, 18: {6, 30, 56, 82}, 19: {6, 30, 58, 86},
	20: {6, 34, 62, 90},
	21: {6, 28, 50, 72, 94}, 22: {6, 26, 50, 74, 98}, 23: {6, 30, 54, 78, 102},
	24: {6, 28, 54, 80, 106}, 25: {6, 32, 58, 84, 110}, 26: {6, 30, 58, 86, 114},
	27: {6, 34, 62, 90, 118},
	28: {6, 26, 50, 74, 98, 122}, 29: {6, 30, 54, 78, 102, 126},
	30: {6, 26, 52, 78, 104, 130}, 31: {6, 30, 56, 82, 108, 134},
	32: {6, 34, 60, 86, 112, 138}, 33: {6, 30, 58, 86, 114, 142},
	34: {6, 34, 62, 90, 118, 146},
	35: {6, 30, 54, 78, 102, 126, 150}, 36: {6, 24, 50, 76, 102, 128, 154},
	37: {6, 28, 54, 80, 106, 132, 158}, 38: {6, 32, 58, 84, 110, 136, 162},
	39: {6, 26, 54, 82, 110, 138, 166}, 40: {6, 30, 58, 86, 114, 142, 170},
}

// AlignmentCenters returns the Annex E alignment coordinates for version.
func AlignmentCenters(version int) []int { return append([]int(nil), alignTable[version]...) }

// ECBlocks returns the EC codewords per block and the block count for
// version and level index (0=L, 1=M, 2=Q, 3=H).
func ECBlocks(version, level int) (ecc, blocks int) {
	e := ecTable[version][level]
	return e[0], e[1]
}

// polyMod returns the remainder of v (of degree < n+deg) modulo the binary
// polynomial g of degree deg, by long division.
func polyMod(v, g uint32, deg int) uint32 {
	for i := bits.Len32(v) - 1; i >= deg; i-- {
		if v>>uint(i)&1 != 0 {
			v ^= g << uint(i-deg)
		}
	}
	return v
}

// FormatWord returns the masked 15-bit format word for level index
// (0=L..3=H) and mask, computed by polynomial division.
func FormatWord(level, mask int) uint32 {
	ind := [4]uint32{0b01, 0b00, 0b11, 0b10}[level]
	d := ind<<3 | uint32(mask)
	return (d<<10 | polyMod(d<<10, 0b10100110111, 10)) ^ 0b101010000010010
}

// VersionWord returns the 18-bit version word.
func VersionWord(version int) uint32 {
	d := uint32(version)
	return d<<12 | polyMod(d<<12, 0b1111100100101, 12)
}

type grid struct {
	n   int
	m   [][]bool
	fn  [][]bool
	err error
}

func (g *grid) at(x, y int) bool { return g.m[y][x] }

func (g *grid) mark(x0, y0, w, h int) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if x >= 0 && y >= 0 && x < g.n && y < g.n {
				g.fn[y][x] = true
			}
		}
	}
}

// expect checks a function module colour.
func (g *grid) expect(x, y int, dark bool, what string) {
	if g.err == nil && g.m[y][x] != dark {
		g.err = fail("%s module at (%d,%d) is wrong", what, x, y)
	}
}

// Decode reads a symbol given as modules[y][x] (true = dark), without
// quiet zone.
func Decode(modules [][]bool) (*Result, error) {
	n := len(modules)
	if n < 21 || n > 177 || (n-17)%4 != 0 {
		return nil, fail("size %d is not a QR Code size", n)
	}
	for _, row := range modules {
		if len(row) != n {
			return nil, fail("grid is not square")
		}
	}
	version := (n - 17) / 4
	g := &grid{n: n, m: modules, fn: make([][]bool, n)}
	for i := range g.fn {
		g.fn[i] = make([]bool, n)
	}

	// Finder patterns and separators.
	for _, c := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		for dy := -1; dy <= 7; dy++ {
			for dx := -1; dx <= 7; dx++ {
				x, y := c[0]+dx, c[1]+dy
				if x < 0 || y < 0 || x >= n || y >= n {
					continue
				}
				ring := max(abs(dx-3), abs(dy-3))
				g.expect(x, y, ring <= 1 || ring == 3, "finder")
			}
		}
		g.mark(c[0]-1, c[1]-1, 9, 9)
	}
	// Timing patterns.
	for i := 8; i < n-8; i++ {
		g.expect(i, 6, i%2 == 0, "timing")
		g.expect(6, i, i%2 == 0, "timing")
	}
	g.mark(0, 6, n, 1)
	g.mark(6, 0, 1, n)
	// Alignment patterns.
	al := alignTable[version]
	for _, cy := range al {
		for _, cx := range al {
			if cx < 9 && cy < 9 || cx >= n-8 && cy < 9 || cx < 9 && cy >= n-8 {
				continue // would overlap a finder pattern
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					g.expect(cx+dx, cy+dy, max(abs(dx), abs(dy)) != 1, "alignment")
				}
			}
			g.mark(cx-2, cy-2, 5, 5)
		}
	}
	// Dark module and format areas.
	g.expect(8, n-8, true, "dark")
	g.mark(0, 8, 9, 1)
	g.mark(8, 0, 1, 9)
	g.mark(n-8, 8, 8, 1)
	g.mark(8, n-8, 1, 8)
	if g.err != nil {
		return nil, g.err
	}

	res := &Result{Version: version}

	// Format information: read both copies, most significant bit first,
	// and take the closest valid word.
	var f1, f2 uint32
	for _, p := range formatPath1(n) {
		f1 = f1<<1 | b2u(g.at(p[0], p[1]))
	}
	for _, p := range formatPath2(n) {
		f2 = f2<<1 | b2u(g.at(p[0], p[1]))
	}
	level, mask, best := -1, -1, 99
	for l := 0; l < 4; l++ {
		for k := 0; k < 8; k++ {
			w := FormatWord(l, k)
			d := min(bits.OnesCount32(w^f1), bits.OnesCount32(w^f2))
			if d < best {
				level, mask, best = l, k, d
			}
		}
	}
	if best > 3 {
		return nil, fail("format information unreadable")
	}
	res.Level, res.Mask, res.FormatErrors = string("LMQH"[level]), mask, best

	if version >= 7 {
		var v1, v2 uint32
		// Bottom-left block: 6 columns x 3 rows at rows n-11..n-9; bit
		// 17 is at column 5, row n-9.
		for i := 17; i >= 0; i-- {
			v1 = v1<<1 | b2u(g.at(i/3, n-11+i%3))
			v2 = v2<<1 | b2u(g.at(n-11+i%3, i/3))
		}
		bestV, bestD := 0, 99
		for v := 7; v <= 40; v++ {
			w := VersionWord(v)
			if d := min(bits.OnesCount32(w^v1), bits.OnesCount32(w^v2)); d < bestD {
				bestV, bestD = v, d
			}
		}
		if bestD > 3 || bestV != version {
			return nil, fail("version information does not match size")
		}
		res.VersionErrors = bestD
		g.mark(0, n-11, 6, 3)
		g.mark(n-11, 0, 3, 6)
	}

	// Read the data region in placement order, unmasking as we go.
	var stream []bool
	x, up := n-1, true
	for x > 0 {
		if x == 6 {
			x--
		}
		for k := 0; k < n; k++ {
			y := k
			if up {
				y = n - 1 - k
			}
			for _, cx := range [2]int{x, x - 1} {
				if !g.fn[y][cx] {
					stream = append(stream, g.at(cx, y) != maskAt(mask, cx, y))
				}
			}
		}
		x -= 2
		up = !up
	}
	ecc, nb := ECBlocks(version, level)
	total := len(stream) / 8
	for _, b := range stream[total*8:] {
		if b {
			return nil, fail("remainder bits are not zero")
		}
	}
	cw := make([]byte, total)
	for i := range cw {
		for j := 0; j < 8; j++ {
			cw[i] = cw[i]<<1 | byte(b2u(stream[i*8+j]))
		}
	}

	// De-interleave.
	dataTotal := total - ecc*nb
	longBlocks := dataTotal % nb
	shortLen := dataTotal / nb
	blocks := make([][]byte, nb)
	k := 0
	for i := 0; i <= shortLen; i++ {
		for j := 0; j < nb; j++ {
			if i < shortLen || j >= nb-longBlocks {
				blocks[j] = append(blocks[j], cw[k])
				k++
			}
		}
	}
	for i := 0; i < ecc; i++ {
		for j := 0; j < nb; j++ {
			blocks[j] = append(blocks[j], cw[k])
			k++
		}
	}
	var data []byte
	for j, b := range blocks {
		for i := 0; i < ecc; i++ {
			if s := syndrome(b, i); s != 0 {
				return nil, fail("block %d: syndrome S%d = %#02x", j, i, s)
			}
		}
		data = append(data, b[:len(b)-ecc]...)
	}

	out, segs, err := parse(data, version)
	if err != nil {
		return nil, err
	}
	res.Data, res.Segments = out, segs
	return res, nil
}

// formatPath1 lists the modules of the first format copy, bit 14 first.
func formatPath1(n int) [][2]int {
	return [][2]int{
		{0, 8}, {1, 8}, {2, 8}, {3, 8}, {4, 8}, {5, 8}, {7, 8}, {8, 8},
		{8, 7}, {8, 5}, {8, 4}, {8, 3}, {8, 2}, {8, 1}, {8, 0},
	}
}

// formatPath2 lists the modules of the second copy, bit 14 first: down the
// bottom-left column, then along the top-right row.
func formatPath2(n int) [][2]int {
	p := make([][2]int, 0, 15)
	for y := n - 1; y >= n-7; y-- {
		p = append(p, [2]int{8, y})
	}
	for x := n - 8; x < n; x++ {
		p = append(p, [2]int{x, 8})
	}
	return p
}

func maskAt(p, x, y int) bool {
	// Written in the standard's notation: i is the row, j the column.
	i, j := y, x
	switch p {
	case 0:
		return (i+j)%2 == 0
	case 1:
		return i%2 == 0
	case 2:
		return j%3 == 0
	case 3:
		return (i+j)%3 == 0
	case 4:
		return (i/2+j/3)%2 == 0
	case 5:
		return (i*j)%2+(i*j)%3 == 0
	case 6:
		return ((i*j)%2+(i*j)%3)%2 == 0
	}
	return ((i+j)%2+(i*j)%3)%2 == 0
}

// GF(256) arithmetic by shift-and-add, deliberately table-free.
func mul(a, b byte) byte {
	var p byte
	for b != 0 {
		if b&1 != 0 {
			p ^= a
		}
		hi := a & 0x80
		a <<= 1
		if hi != 0 {
			a ^= 0x1D
		}
		b >>= 1
	}
	return p
}

// mulTable caches mul for the syndrome loop; it is filled from mul itself.
var mulTable = func() *[256][256]byte {
	t := new([256][256]byte)
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			t[a][b] = mul(byte(a), byte(b))
		}
	}
	return t
}()

// syndrome evaluates the codeword polynomial at α^i by Horner's rule.
func syndrome(block []byte, i int) byte {
	x := byte(1)
	for k := 0; k < i; k++ {
		x = mul(x, 2)
	}
	var s byte
	row := &mulTable[x]
	for _, c := range block {
		s = row[s] ^ c
	}
	return s
}

const alnum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"

type bitReader struct {
	b   []byte
	pos int
}

func (r *bitReader) left() int { return len(r.b)*8 - r.pos }

func (r *bitReader) read(n int) int {
	v := 0
	for i := 0; i < n; i++ {
		v = v<<1 | int(r.b[r.pos/8]>>(7-uint(r.pos%8))&1)
		r.pos++
	}
	return v
}

func parse(data []byte, version int) ([]byte, []Segment, error) {
	r := &bitReader{b: data}
	g := 0
	if version >= 27 {
		g = 2
	} else if version >= 10 {
		g = 1
	}
	out := []byte{}
	var segs []Segment
	for r.left() >= 4 {
		mode := r.read(4)
		if mode == 0 {
			break
		}
		var cb int
		switch mode {
		case 1:
			cb = [3]int{10, 12, 14}[g]
		case 2:
			cb = [3]int{9, 11, 13}[g]
		case 4:
			cb = [3]int{8, 16, 16}[g]
		default:
			return nil, nil, fail("unsupported mode indicator %04b", mode)
		}
		if r.left() < cb {
			return nil, nil, fail("truncated count")
		}
		count := r.read(cb)
		need := 0
		switch mode {
		case 1:
			need = count/3*10 + [3]int{0, 4, 7}[count%3]
		case 2:
			need = count/2*11 + count%2*6
		case 4:
			need = count * 8
		}
		if r.left() < need {
			return nil, nil, fail("segment overruns data")
		}
		switch mode {
		case 1:
			for left := count; left > 0; {
				k := min(left, 3)
				v := r.read(k*3 + 1)
				if v >= [4]int{1, 10, 100, 1000}[k] {
					return nil, nil, fail("numeric group %d out of range", v)
				}
				out = append(out, fmt.Sprintf("%0*d", k, v)...)
				left -= k
			}
			segs = append(segs, Segment{"numeric", count})
		case 2:
			for left := count; left > 0; {
				if left >= 2 {
					v := r.read(11)
					if v >= 45*45 {
						return nil, nil, fail("alphanumeric pair out of range")
					}
					out = append(out, alnum[v/45], alnum[v%45])
					left -= 2
				} else {
					v := r.read(6)
					if v >= 45 {
						return nil, nil, fail("alphanumeric char out of range")
					}
					out = append(out, alnum[v])
					left--
				}
			}
			segs = append(segs, Segment{"alphanumeric", count})
		case 4:
			for i := 0; i < count; i++ {
				out = append(out, byte(r.read(8)))
			}
			segs = append(segs, Segment{"byte", count})
		}
	}
	// After the terminator: zero bits to the byte boundary, then pad
	// codewords 0xEC, 0x11, ... A terminator may be cut short (or absent)
	// only when the data ends within four bits of capacity.
	for r.pos%8 != 0 {
		if r.read(1) != 0 {
			return nil, nil, fail("non-zero padding bits")
		}
	}
	for i := 0; r.left() > 0; i++ {
		want := [2]int{0xEC, 0x11}[i%2]
		if v := r.read(8); v != want {
			return nil, nil, fail("pad codeword %d is %#02x, want %#02x", i, v, want)
		}
	}
	return out, segs, nil
}

func b2u(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
