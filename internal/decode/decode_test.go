package decode

import (
	"errors"
	"math/rand"
	"strings"
	"testing"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
)

func makeGrid(n int, f func(x, y int) bool) [][]bool {
	g := make([][]bool, n)
	for y := range g {
		g[y] = make([]bool, n)
		for x := range g[y] {
			g[y][x] = f(x, y)
		}
	}
	return g
}

func TestRejects(t *testing.T) {
	for _, g := range [][][]bool{
		nil,
		makeGrid(20, func(x, y int) bool { return false }),
		makeGrid(22, func(x, y int) bool { return false }),
		append(makeGrid(21, func(x, y int) bool { return false })[:20], make([]bool, 3)),
		makeGrid(21, func(x, y int) bool { return true }),
		makeGrid(21, func(x, y int) bool { return rand.Intn(2) == 0 }),
	} {
		if _, err := Decode(g); !errors.Is(err, ErrInvalid) {
			t.Errorf("err = %v", err)
		}
	}
}

func TestTables(t *testing.T) {
	for v := 2; v <= 40; v++ {
		a := AlignmentCenters(v)
		if len(a) != v/7+2 || a[0] != 6 || a[len(a)-1] != 4*v+10 {
			t.Errorf("v%d alignment %v", v, a)
		}
	}
	if FormatWord(1, 0) != 0x5412 {
		t.Error("M0 format word")
	}
	if mul(0x80, 2) != 0x1D || mul(3, 7) != 9 || mul(0, 5) != 0 {
		t.Error("GF(256) multiply")
	}
}

// FuzzDecode damages valid symbols (flipping modules, including function
// patterns and format information) and feeds them to the decoder: it must
// return a result or an error, never panic, and a clean symbol must
// decode to its input.
func FuzzDecode(f *testing.F) {
	f.Add("HELLO", uint8(1), []byte{})
	f.Add("00020126330014br.gov.bcb.pix", uint8(7), []byte{1, 2, 3})
	f.Add("0123456789", uint8(30), []byte{200, 17})
	f.Fuzz(func(t *testing.T, data string, version uint8, flips []byte) {
		c, err := qr.Encode(data, qr.M, qr.WithMinVersion(int(version%40)+1))
		if err != nil {
			return
		}
		n := c.Size
		for i := 0; i+1 < len(flips); i += 2 {
			x, y := int(flips[i])%n, int(flips[i+1])%n
			c.Modules[y][x] = !c.Modules[y][x]
		}
		r, err := Decode(c.Modules)
		if len(flips) < 2 && (err != nil || string(r.Data) != data) {
			t.Fatalf("clean symbol: %v", err)
		}
	})
}

func encode(t *testing.T, data string, opts ...qr.Option) *qr.Code {
	t.Helper()
	c, err := qr.Encode(data, qr.Q, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDecodeModesVersionsMasks(t *testing.T) {
	data := "0123456789HELLO WORLD $%*+-./:hello, wörld 0123"
	for _, v := range []int{1, 7, 10, 27} {
		for m := 0; m < 8; m++ {
			c := encode(t, data[:min(len(data), 4*v+10)], qr.WithMinVersion(v), qr.WithMask(m))
			r, err := Decode(c.Modules)
			if err != nil || string(r.Data) != data[:min(len(data), 4*v+10)] || r.Mask != m || r.Version != c.Version {
				t.Fatalf("v%d mask %d: %v %q", v, m, err, r.Data)
			}
		}
	}
}

func TestDecodeDamage(t *testing.T) {
	flip := func(c *qr.Code, pts ...[2]int) [][]bool {
		g := make([][]bool, c.Size)
		for y := range g {
			g[y] = append([]bool(nil), c.Modules[y]...)
		}
		for _, p := range pts {
			g[p[1]][p[0]] = !g[p[1]][p[0]]
		}
		return g
	}
	c := encode(t, "HELLO", qr.WithMinVersion(7))
	n := c.Size
	var formatBoth [][2]int
	for _, p := range formatPath1(n)[:4] {
		formatBoth = append(formatBoth, p)
	}
	for _, p := range formatPath2(n)[:4] {
		formatBoth = append(formatBoth, p)
	}
	var versionBoth [][2]int
	for i := 0; i < 4; i++ {
		versionBoth = append(versionBoth, [2]int{i / 3, n - 11 + i%3}, [2]int{n - 11 + i%3, i / 3})
	}
	// A single error in one format copy is corrected and reported.
	if r, err := Decode(flip(c, formatPath1(n)[0])); err != nil || r.FormatErrors != 0 {
		t.Fatalf("one damaged copy: %v %+v", err, r)
	}
	if r, err := Decode(flip(c, formatPath1(n)[0], formatPath2(n)[0])); err != nil || r.FormatErrors != 1 {
		t.Fatalf("one error in both copies: %v", err)
	}
	for name, g := range map[string][][]bool{
		"timing":    flip(c, [2]int{10, 6}),
		"finder":    flip(c, [2]int{3, 3}),
		"alignment": flip(c, [2]int{22, 22}),
		"dark":      flip(c, [2]int{8, n - 8}),
		"format":    flip(c, formatBoth...),
		"version":   flip(c, versionBoth...),
		"data":      flip(c, [2]int{n - 1, n - 1}),
	} {
		if _, err := Decode(g); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s damage not detected: %v", name, err)
		}
	}
	// Remainder bits: version 2 has 7, at the end of the placement order
	// (bottom-left area, left of the timing column's neighbour).
	c2 := encode(t, "A", qr.WithMinVersion(2), qr.WithMaxVersion(2))
	found := false
	for y := c2.Size - 1; y > 8 && !found; y-- { // the last column pair holds the remainder modules
		for x := 0; x < 2 && !found; x++ {
			g := flip(c2, [2]int{x, y})
			if _, err := Decode(g); err != nil && strings.Contains(err.Error(), "remainder") {
				found = true
			}
		}
	}
	if !found {
		t.Error("remainder bits are not checked")
	}
}

func TestParse(t *testing.T) {
	bits := func(s string) []byte { // "0101..." with spaces ignored, zero-padded to bytes
		s = strings.ReplaceAll(s, " ", "")
		b := make([]byte, (len(s)+7)/8)
		for i, c := range s {
			if c == '1' {
				b[i/8] |= 0x80 >> (i % 8)
			}
		}
		return b
	}
	tests := []struct {
		name, stream string
		version      int
		want         string // decoded data, or error substring prefixed by "!"
	}{
		{"numeric", "0001 0000000100 1111100111 0101 0000", 1, "9995"},
		{"byte v10", "0100 0000000000000001 01000001 0000", 10, "A"},
		{"numeric v27", "0001 00000000000010 0001100 0000", 27, "12"},
		{"alnum v27", "0010 0000000000001 100011 0000", 27, "Z"},
		{"bad mode", "0111 0000", 1, "!unsupported mode"},
		{"truncated count", "0001 0000", 1, "!truncated count"},
		{"overrun", "0100 11111111 0000", 1, "!overruns"},
		{"numeric range", "0001 0000000011 1111101000 0000 0000", 1, "!numeric group"},
		{"alnum pair range", "0010 000000010 11111111111 0000", 1, "!alphanumeric pair"},
		{"alnum char range", "0010 000000001 111111 0000 00", 1, "!alphanumeric char"},
		{"padding bits", "0001 0000000001 0001 0000 01", 1, "!padding bits"},
		{"pad codeword", "0100 00000001 01000001 0000 11101100 11101100", 1, "!pad codeword 1"},
		{"no terminator room", "0100 00000001 01000001 0", 1, "A"},
	}
	for _, tt := range tests {
		out, _, err := parse(bits(tt.stream), tt.version)
		if strings.HasPrefix(tt.want, "!") {
			if err == nil || !strings.Contains(err.Error(), tt.want[1:]) {
				t.Errorf("%s: err = %v", tt.name, err)
			}
			continue
		}
		if err != nil || string(out) != tt.want {
			t.Errorf("%s: %q, %v", tt.name, out, err)
		}
	}
}
