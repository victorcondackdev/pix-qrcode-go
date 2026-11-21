package qr

import (
	"bytes"
	"errors"
	"fmt"
	"math/bits"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/victorcondackdev/pix-qrcode-go/internal/decode"
)

// roundTrip decodes c with the independent decoder and checks every field.
func roundTrip(t testing.TB, c *Code, data string) {
	t.Helper()
	r, err := decode.Decode(c.Modules)
	if err != nil {
		t.Fatalf("v%d-%s mask %d: %v", c.Version, c.Level, c.Mask, err)
	}
	if string(r.Data) != data {
		t.Fatalf("v%d-%s: decoded %q, want %q", c.Version, c.Level, r.Data, data)
	}
	if r.Version != c.Version || r.Level != c.Level.String() || r.Mask != c.Mask {
		t.Fatalf("metadata: decoded v%d-%s mask %d, encoded v%d-%s mask %d",
			r.Version, r.Level, r.Mask, c.Version, c.Level, c.Mask)
	}
	if r.FormatErrors != 0 || r.VersionErrors != 0 {
		t.Fatalf("format/version info has bit errors: %d/%d", r.FormatErrors, r.VersionErrors)
	}
	if len(r.Segments) != len(c.Segments) {
		t.Fatalf("decoded %d segments, encoded %d", len(r.Segments), len(c.Segments))
	}
	for i, s := range c.Segments {
		if r.Segments[i].Mode != s.Mode.String() || r.Segments[i].Count != len(s.Data) {
			t.Fatalf("segment %d: decoded %+v, encoded %v %d", i, r.Segments[i], s.Mode, len(s.Data))
		}
	}
}

// Published worked examples. The 01234567 vector is the one in
// ISO/IEC 18004 Annex I; the HELLO WORLD one is the widely reproduced
// version 1-M example (Thonky QR tutorial).
func TestKnownCodewords(t *testing.T) {
	tests := []struct {
		data       string
		want, ecc  string
		wantSegsOf Mode
	}{
		{"01234567", "10200C56618 0EC11EC11EC11EC11EC11", "A524D4C1ED36C7872C55", Numeric},
		{"HELLO WORLD", "205B0B78D172DC4D4340EC11EC11EC11", "C4232777EBD7E7E25D17", Alphanumeric},
	}
	for _, tt := range tests {
		segs := optimalSegments(tt.data, 1)
		if len(segs) != 1 || segs[0].Mode != tt.wantSegsOf {
			t.Fatalf("%q: segments %v", tt.data, segs)
		}
		data := dataStream(segs, 1, M)
		if got := fmt.Sprintf("%X", data); got != strings.ReplaceAll(tt.want, " ", "") {
			t.Errorf("%q data codewords = %s, want %s", tt.data, got, tt.want)
		}
		all := interleave(data, 1, M)
		if got := fmt.Sprintf("%X", all[len(data):]); got != tt.ecc {
			t.Errorf("%q EC codewords = %s, want %s", tt.data, got, tt.ecc)
		}
	}
}

// Total codewords per version, ISO/IEC 18004 Table 1. rawDataModules
// derives them from the geometry, so this checks the function-pattern
// arithmetic against the standard.
var specTotalCodewords = [41]int{0,
	26, 44, 70, 100, 134, 172, 196, 242, 292, 346,
	404, 466, 532, 581, 655, 733, 815, 901, 991, 1085,
	1156, 1258, 1364, 1474, 1588, 1706, 1828, 1921, 2051, 2185,
	2323, 2465, 2611, 2761, 2876, 3034, 3196, 3362, 3532, 3706,
}

func TestCapacityTable(t *testing.T) {
	for v := 1; v <= 40; v++ {
		if got := totalCodewords(v); got != specTotalCodewords[v] {
			t.Errorf("v%d: %d codewords, spec %d", v, got, specTotalCodewords[v])
		}
		m := newMatrix(v)
		m.drawFunctionPatterns()
		free := 0
		for _, f := range m.function {
			if !f {
				free++
			}
		}
		if free != rawDataModules(v) {
			t.Errorf("v%d: %d free modules in the matrix, formula says %d", v, free, rawDataModules(v))
		}
		if rem := free - totalCodewords(v)*8; rem != [41]int{0, 0, 7, 7, 7, 7, 7, 0, 0, 0, 0, 0, 0, 0, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0}[v] {
			t.Errorf("v%d: %d remainder bits", v, rem)
		}
		for l := L; l <= H; l++ {
			ecc, nb := decode.ECBlocks(v, int(l))
			if ecc != int(eccPerBlock[l][v]) || nb != int(numBlocks[l][v]) {
				t.Errorf("v%d-%s: encoder and decoder tables disagree", v, l)
			}
			b := layout(v, l)
			if b.short < 1 || b.short > b.blocks || b.shortData < 1 {
				t.Errorf("v%d-%s: bad layout %+v", v, l, b)
			}
			if b.dataSize != dataCodewords(v, l) {
				t.Errorf("v%d-%s: layout data %d != %d", v, l, b.dataSize, dataCodewords(v, l))
			}
		}
	}
}

// Character capacities from ISO/IEC 18004 Table 7, checked end to end
// through Encode: the maximum fits, one more character does not.
func TestCharacterCapacity(t *testing.T) {
	tests := []struct {
		version int
		level   Level
		char    string
		n       int
	}{
		{1, L, "1", 41}, {1, L, "A", 25}, {1, L, "a", 17},
		{1, M, "1", 34}, {1, M, "A", 20}, {1, M, "a", 14},
		{1, Q, "1", 27}, {1, Q, "A", 16}, {1, Q, "a", 11},
		{1, H, "1", 17}, {1, H, "A", 10}, {1, H, "a", 7},
		{10, M, "a", 213}, {26, L, "a", 1367}, {27, L, "a", 1465},
		{40, L, "1", 7089}, {40, L, "A", 4296}, {40, L, "a", 2953},
		{40, M, "1", 5596}, {40, M, "A", 3391}, {40, M, "a", 2331},
		{40, Q, "1", 3993}, {40, Q, "A", 2420}, {40, Q, "a", 1663},
		{40, H, "1", 3057}, {40, H, "A", 1852}, {40, H, "a", 1273},
	}
	for _, tt := range tests {
		s := strings.Repeat(tt.char, tt.n)
		c, err := Encode(s, tt.level, WithMinVersion(tt.version), WithMaxVersion(tt.version))
		if err != nil {
			t.Errorf("%d x %q at %d-%s: %v", tt.n, tt.char, tt.version, tt.level, err)
			continue
		}
		roundTrip(t, c, s)
		_, err = Encode(s+tt.char, tt.level, WithMinVersion(tt.version), WithMaxVersion(tt.version))
		if !errors.Is(err, ErrTooLong) {
			t.Errorf("%d x %q at %d-%s: err = %v, want ErrTooLong", tt.n+1, tt.char, tt.version, tt.level, err)
		}
	}
}

func TestAlignmentPositions(t *testing.T) {
	for v := 1; v <= 40; v++ {
		got, want := alignmentPositions(v), decode.AlignmentCenters(v)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("v%d: formula %v, Annex E %v", v, got, want)
		}
	}
}

func TestFormatAndVersionInfo(t *testing.T) {
	// Spot values from the standard: L/mask 0 and M/mask 0.
	if got := formatInfo(L, 0); got != 0b111011111000100 {
		t.Errorf("format L0 = %015b", got)
	}
	if got := formatInfo(M, 0); got != 0b101010000010010 {
		t.Errorf("format M0 = %015b", got)
	}
	// Version 7 information from Annex D.
	if got := versionInfo(7); got != 0b000111110010010100 {
		t.Errorf("version 7 = %018b", got)
	}
	var words []uint32
	for l := L; l <= H; l++ {
		for m := 0; m < 8; m++ {
			w := formatInfo(l, m)
			if w != decode.FormatWord(int(l), m) {
				t.Errorf("%s%d: encoder %015b, decoder %015b", l, m, w, decode.FormatWord(int(l), m))
			}
			words = append(words, w)
		}
	}
	checkDistance(t, "format", words, 7)
	words = words[:0]
	for v := 7; v <= 40; v++ {
		w := versionInfo(v)
		if w != decode.VersionWord(v) || w>>12 != uint32(v) {
			t.Errorf("v%d: version word %018b", v, w)
		}
		words = append(words, w)
	}
	checkDistance(t, "version", words, 8)
}

// checkDistance verifies the minimum Hamming distance of a BCH code.
func checkDistance(t *testing.T, name string, words []uint32, want int) {
	t.Helper()
	for i := range words {
		for j := i + 1; j < len(words); j++ {
			if d := bits.OnesCount32(words[i] ^ words[j]); d < want {
				t.Errorf("%s words %d and %d at distance %d", name, i, j, d)
			}
		}
	}
}

func TestRSGenerator(t *testing.T) {
	// Degree 7, from the standard's Annex A: α exponents
	// 87, 229, 146, 149, 238, 102, 21.
	want := []byte{87, 229, 146, 149, 238, 102, 21}
	for i, g := range rsGenerators[7] {
		if gfLog[g] != want[i] {
			t.Fatalf("g7 = %v (log), want %v", rsGenerators[7], want)
		}
	}
	for d, g := range rsGenerators {
		if g == nil {
			continue
		}
		// Every α^i, i < d, must be a root.
		for i := 0; i < d; i++ {
			x := gfExp[i]
			s := byte(1)
			for _, c := range g {
				s = gfMul(s, x) ^ c
			}
			if s != 0 {
				t.Fatalf("degree %d: α^%d is not a root", d, i)
			}
		}
	}
}

var modeAlphabets = []struct {
	name, chars string
	bitsPerChar float64 // rough upper bound, to size near-capacity inputs
}{
	{"numeric", "0123456789", 10.0 / 3},
	{"alnum", alnumChars, 5.5},
	{"byte", "abcdefghijklmnopqrstuvwxyz{}~\x00\xff", 8},
	{"mixed", "0123456789ABCDEF.br.gov.bcb.pix-é", 8},
	{"pixlike", "0000000011112345BRASILIAbr.gov.bcb.pix***", 7},
	{"unicode", "aç日本語€", 8},
}

func randString(r *rand.Rand, alphabet string, n int) string {
	runes := []rune(alphabet)
	var b strings.Builder
	for b.Len() < n {
		b.WriteRune(runes[r.Intn(len(runes))])
	}
	return b.String()
}

// TestRoundTripAllVersions encodes, for every version and level, inputs of
// every character mix (rotating over the mixes to bound the run time) that
// nearly fill the symbol, plus a short input forced into that version, and
// decodes them with the independent decoder.
func TestRoundTripAllVersions(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	k := 0
	for v := 1; v <= 40; v++ {
		for l := L; l <= H; l++ {
			for rep := 0; rep < 2; rep++ {
				a := modeAlphabets[k%len(modeAlphabets)]
				k++
				full := int(float64(dataCodewords(v, l)*8-40) / a.bitsPerChar)
				for _, n := range []int{full, 1 + r.Intn(8)} {
					s := randString(r, a.chars, max(n, 1))
					for {
						c, err := Encode(s, l, WithMinVersion(v), WithMaxVersion(v))
						if errors.Is(err, ErrTooLong) {
							s = s[:len(s)*19/20]
							for len(s) > 0 && !utf8.ValidString(s) && s[len(s)-1] >= 0x80 {
								s = s[:len(s)-1]
							}
							continue
						}
						if err != nil {
							t.Fatal(err)
						}
						if c.Version != v {
							t.Fatalf("%s: version %d, want %d", a.name, c.Version, v)
						}
						roundTrip(t, c, s)
						break
					}
				}
			}
		}
	}
}

func TestAllMasks(t *testing.T) {
	data := "00020126330014br.gov.bcb.pix0111123456789015204000053039865802BR5913FULANO DE TAL6008BRASILIA62070503***6304"
	for _, v := range []int{1, 7, 21, 40} {
		for m := 0; m < 8; m++ {
			c, err := Encode(data[:min(len(data), 10)], Q, WithMinVersion(v), WithMask(m))
			if err != nil {
				t.Fatal(err)
			}
			if c.Mask != m {
				t.Fatalf("mask %d, want %d", c.Mask, m)
			}
			roundTrip(t, c, data[:min(len(data), 10)])
		}
	}
}

func TestBoostECL(t *testing.T) {
	c, err := Encode("HELLO", L, WithBoostECL())
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != 1 || c.Level != H {
		t.Fatalf("got %d-%s, want 1-H", c.Version, c.Level)
	}
	roundTrip(t, c, "HELLO")
	// 20 alphanumeric chars fill 1-M exactly; boosting must not exceed it.
	s := strings.Repeat("A", 20)
	c, _ = Encode(s, L, WithBoostECL())
	if c.Version != 1 || c.Level != M {
		t.Fatalf("got %d-%s, want 1-M", c.Version, c.Level)
	}
}

func TestByteModeOption(t *testing.T) {
	s := "0123456789"
	c, err := Encode(s, M, WithByteMode())
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Segments) != 1 || c.Segments[0].Mode != Byte || c.DataBits != 4+8+80 {
		t.Fatalf("segments %v, %d bits", c.Segments, c.DataBits)
	}
	roundTrip(t, c, s)
}

func TestEncodeErrors(t *testing.T) {
	tests := []struct {
		level Level
		opts  []Option
		want  error
	}{
		{Level(4), nil, ErrInvalidLevel},
		{M, []Option{WithMinVersion(0)}, ErrInvalidVersion},
		{M, []Option{WithMaxVersion(41)}, ErrInvalidVersion},
		{M, []Option{WithMinVersion(5), WithMaxVersion(4)}, ErrInvalidVersion},
		{M, []Option{WithMask(8)}, ErrInvalidMask},
		{M, []Option{WithMask(-2)}, ErrInvalidMask},
	}
	for _, tt := range tests {
		if _, err := Encode("x", tt.level, tt.opts...); !errors.Is(err, tt.want) {
			t.Errorf("err = %v, want %v", err, tt.want)
		}
	}
	_, err := Encode(strings.Repeat("a", 3000), L)
	if !errors.Is(err, ErrTooLong) || !strings.Contains(err.Error(), "version 40-L") {
		t.Errorf("err = %v", err)
	}
}

func TestLevelAndMode(t *testing.T) {
	for _, s := range []string{"L", "m", "Q", "h"} {
		l, err := ParseLevel(s)
		if err != nil || l.String() != strings.ToUpper(s) {
			t.Errorf("ParseLevel(%q) = %v, %v", s, l, err)
		}
	}
	for _, s := range []string{"", "X", "LM"} {
		if _, err := ParseLevel(s); !errors.Is(err, ErrInvalidLevel) {
			t.Errorf("ParseLevel(%q): %v", s, err)
		}
	}
	if Level(9).String() != "Level(9)" || Mode(3).String() != "Mode(3)" {
		t.Error("String of invalid values")
	}
	if Numeric.String() != "numeric" || Alphanumeric.String() != "alphanumeric" || Byte.String() != "byte" {
		t.Error("Mode.String")
	}
}

func TestAt(t *testing.T) {
	c, _ := Encode("A", L)
	if !c.At(0, 0) || c.At(-1, 0) || c.At(0, c.Size) || c.At(7, 0) {
		t.Fatal("At around the top-left finder")
	}
	for y := 0; y < c.Size; y++ {
		for x := 0; x < c.Size; x++ {
			if c.At(x, y) != c.Modules[y][x] {
				t.Fatal("At disagrees with Modules")
			}
		}
	}
}

// TestSegmentationOptimal compares the DP with exhaustive search over
// every per-character mode assignment.
func TestSegmentationOptimal(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	alpha := "0123456789ABZ:a"
	for trial := 0; trial < 300; trial++ {
		s := randString(r, alpha, 1+r.Intn(9))
		for _, v := range []int{1, 10, 27} {
			got := totalBits(optimalSegments(s, v), v)
			want := bruteForce(s, v)
			if got != want {
				t.Fatalf("%q v%d: DP %d bits (%v), brute force %d", s, v, got, optimalSegments(s, v), want)
			}
		}
	}
}

func bruteForce(s string, v int) int {
	best := -1
	modes := make([]Mode, len(s))
	var rec func(i int)
	rec = func(i int) {
		if i == len(s) {
			var segs []Segment
			for k := 0; k < len(s); {
				j := k
				for j < len(s) && modes[j] == modes[k] {
					j++
				}
				segs = append(segs, Segment{modes[k], s[k:j]})
				k = j
			}
			if n := totalBits(segs, v); best < 0 || n < best {
				best = n
			}
			return
		}
		for _, m := range []Mode{Numeric, Alphanumeric, Byte} {
			if m == Numeric && !isDigit(s[i]) || m == Alphanumeric && alnumValue[s[i]] < 0 {
				continue
			}
			modes[i] = m
			rec(i + 1)
		}
	}
	rec(0)
	return best
}

func TestSegmentationKeepsRunesWhole(t *testing.T) {
	s := "çé" + strings.Repeat("1", 20) + "日本" + strings.Repeat("A", 20) + "\xff\xfe" + "€"
	segs := optimalSegments(s, 1)
	var joined strings.Builder
	for _, sg := range segs {
		joined.WriteString(sg.Data)
		if !utf8.ValidString(strings.Trim(sg.Data, "\xff\xfe")) {
			t.Errorf("segment %q splits a character", sg.Data)
		}
	}
	if joined.String() != s || len(segs) != 5 {
		t.Fatalf("segments %v", segs)
	}
	c, err := Encode(s, M)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, c, s)
}

func TestLinePenalty(t *testing.T) {
	b := func(s string) []uint8 {
		l := make([]uint8, len(s))
		for i := range s {
			if s[i] == '1' {
				l[i] = 1
			}
		}
		return l
	}
	tests := []struct {
		line string
		want int
	}{
		{"10101", 0},
		{"11111", 3},
		{"1111111", 5},
		{"0000011111", 6},
		{"1011101", 80},                // quiet zone on both sides counts as light
		{"00001011101", 80},            // four light before, quiet zone after
		{"000001011101", 3 + 80},       // plus a run of five
		{"110111010", 40},              // light zone after reaches the edge
		{"1111011101111", 0},           // dark within four on both sides
		{"101110100001011101", 4 * 40}, // two patterns sharing the light run
	}
	for _, tt := range tests {
		if got := linePenalty(b(tt.line)); got != tt.want {
			t.Errorf("%s: %d, want %d", tt.line, got, tt.want)
		}
	}
}

func TestPenaltyBalance(t *testing.T) {
	n := 21
	g := make([]uint8, n*n)
	if p := penalty(g, g, n); p < 9*penaltyN4 {
		t.Fatalf("all-light symbol scored %d", p)
	}
	for i := range g {
		g[i] = uint8((i/n + i%n) % 2)
	}
	if p := penalty(g, g, n); p != 0 {
		t.Fatalf("checkerboard scored %d", p)
	}
}

func TestCheckClearArea(t *testing.T) {
	c, err := Encode(strings.Repeat("0002012658", 12), H)
	if err != nil {
		t.Fatal(err)
	}
	mid := c.Size / 2
	if err := c.CheckClearArea(mid-2, mid-2, 5, 5); err != nil {
		t.Errorf("small centre area: %v", err)
	}
	for _, r := range [][4]int{
		{0, 0, 3, 3},                     // finder
		{mid, 6, 1, 1},                   // timing
		{-1, 5, 3, 3},                    // outside
		{mid, mid, 0, 1},                 // empty
		{9, 9, c.Size - 18, c.Size - 18}, // too large
	} {
		if err := c.CheckClearArea(r[0], r[1], r[2], r[3]); !errors.Is(err, ErrUnsafeArea) {
			t.Errorf("%v: err = %v", r, err)
		}
	}
	low, _ := Encode(strings.Repeat("0002012658", 12), L)
	if err := low.CheckClearArea(low.Size/2-3, low.Size/2-3, 6, 6); !errors.Is(err, ErrUnsafeArea) {
		t.Errorf("level L tolerated a 6x6 hole: %v", err)
	}
}

func TestModulesAreRowsOfOneArray(t *testing.T) {
	c, _ := Encode("X", L)
	if len(c.Modules) != c.Size || cap(c.Modules[0]) != c.Size {
		t.Fatal("rows must not alias each other")
	}
	var buf bytes.Buffer
	for _, row := range c.Modules {
		for _, d := range row {
			if d {
				buf.WriteByte('#')
			} else {
				buf.WriteByte('.')
			}
		}
	}
	if !strings.HasPrefix(buf.String(), "#######.") {
		t.Fatal("finder pattern missing")
	}
}

func TestMaskTile(t *testing.T) {
	for p := 0; p < 8; p++ {
		for y := 0; y < 177; y++ {
			for x := 0; x < 177; x++ {
				if (maskTile[p][y%12][x%6] == 1) != maskBit(p, x, y) {
					t.Fatalf("mask %d tile differs at %d,%d", p, x, y)
				}
			}
		}
	}
}
