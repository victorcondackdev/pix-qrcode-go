package render

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/color"
	"image/png"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/victorcondackdev/pix-qrcode-go/internal/decode"
	"github.com/victorcondackdev/pix-qrcode-go/qr"
)

const pix = "00020126330014br.gov.bcb.pix0111123456789095204000053039865406150.005802BR" +
	"5912LOJA EXEMPLO6009SAO PAULO62140510PEDIDO123463047DEC"

func mustEncode(t testing.TB, s string, l qr.Level, opts ...qr.Option) *qr.Code {
	t.Helper()
	c, err := qr.Encode(s, l, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// sample reads modules back from a rendered image by sampling module
// centres, then decodes them.
func TestPNGRoundTrip(t *testing.T) {
	c := mustEncode(t, pix, qr.M)
	for _, opts := range [][]Option{
		nil,
		{WithModuleSize(3), WithQuietZone(1)},
		{WithSize(333)},
		{WithSize(c.Size + 8), WithForeground(color.RGBA{0, 0, 128, 255}), WithBackground(color.RGBA{255, 255, 200, 255})},
	} {
		b, err := PNGBytes(c, opts...)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		o, _ := newOptions(opts)
		w := img.Bounds().Dx()
		scale, margin := o.moduleSize, o.quietZone*o.moduleSize
		if o.size > 0 {
			if w != o.size {
				t.Fatalf("width %d, want %d", w, o.size)
			}
			scale = o.size / (c.Size + 2*o.quietZone)
			margin = (o.size - c.Size*scale) / 2
		} else if w != (c.Size+2*o.quietZone)*o.moduleSize {
			t.Fatalf("width %d", w)
		}
		fr, fg, fb, _ := o.fg.RGBA()
		grid := make([][]bool, c.Size)
		for y := range grid {
			grid[y] = make([]bool, c.Size)
			for x := range grid[y] {
				r, g, b, _ := img.At(margin+x*scale+scale/2, margin+y*scale+scale/2).RGBA()
				grid[y][x] = r == fr && g == fg && b == fb
			}
		}
		// Corners of the margin must be background.
		if r, _, _, _ := img.At(0, 0).RGBA(); r == fr && o.quietZone > 0 {
			t.Fatal("quiet zone is not light")
		}
		res, err := decode.Decode(grid)
		if err != nil || string(res.Data) != pix {
			t.Fatalf("decoded %q, %v", res.Data, err)
		}
	}
}

func TestPNGIsOneBit(t *testing.T) {
	b, err := PNGBytes(mustEncode(t, "HELLO", qr.L))
	if err != nil {
		t.Fatal(err)
	}
	// IHDR: width, height, then bit depth (byte 24) and colour type 3.
	if b[24] != 1 || b[25] != 3 {
		t.Fatalf("bit depth %d, colour type %d", b[24], b[25])
	}
	uri, err := PNGDataURI(mustEncode(t, "HELLO", qr.L))
	if err != nil || !strings.HasPrefix(uri, "data:image/png;base64,iVBORw0KGgo") {
		t.Fatalf("%.40s %v", uri, err)
	}
}

var rectRE = regexp.MustCompile(`M(\d+) (\d+)h(\d+)v(\d+)h-(\d+)z`)

// svgModules rasterises the path back into modules.
func svgModules(t *testing.T, svg string, size, quiet int) [][]bool {
	t.Helper()
	i := strings.LastIndex(svg, ` d="`)
	path := svg[i+4 : strings.LastIndex(svg, `"`)]
	grid := make([][]bool, size)
	for y := range grid {
		grid[y] = make([]bool, size)
	}
	if rest := rectRE.ReplaceAllString(path, ""); rest != "" {
		t.Fatalf("unexpected path data %q", rest)
	}
	for _, m := range rectRE.FindAllStringSubmatch(path, -1) {
		v := make([]int, 5)
		for k := range v {
			v[k], _ = strconv.Atoi(m[k+1])
		}
		if v[2] != v[4] {
			t.Fatalf("rectangle does not close: %s", m[0])
		}
		for y := v[1]; y < v[1]+v[3]; y++ {
			for x := v[0]; x < v[0]+v[2]; x++ {
				if grid[y-quiet][x-quiet] {
					t.Fatalf("overlapping rectangles at %d,%d", x, y)
				}
				grid[y-quiet][x-quiet] = true
			}
		}
	}
	return grid
}

func TestSVGRoundTrip(t *testing.T) {
	for _, l := range []qr.Level{qr.L, qr.H} {
		c := mustEncode(t, pix, l)
		s, err := SVGString(c)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(s, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `) ||
			!strings.Contains(s, `shape-rendering="crispEdges"`) || !strings.HasSuffix(s, "</svg>") ||
			strings.Count(s, "<path") != 2 || !strings.Contains(s, `<path fill="#fff" d="M0 0h`) {
			t.Fatalf("unexpected document %.200s", s)
		}
		grid := svgModules(t, s, c.Size, DefaultQuietZone)
		for y := range grid {
			for x := range grid[y] {
				if grid[y][x] != c.Modules[y][x] {
					t.Fatalf("module %d,%d differs", x, y)
				}
			}
		}
		// Merging must beat one rectangle per horizontal run.
		runs := 0
		for _, row := range c.Modules {
			for x, d := range row {
				if d && (x == 0 || !row[x-1]) {
					runs++
				}
			}
		}
		if n := strings.Count(s, "z"); n >= runs {
			t.Fatalf("%d rectangles for %d runs", n, runs)
		}
	}
}

func TestSVGColours(t *testing.T) {
	c := mustEncode(t, "A", qr.L)
	s, _ := SVGString(c, WithForeground(color.RGBA{0x12, 0x34, 0x56, 0xFF}), WithBackground(color.Transparent), WithQuietZone(0), WithModuleSize(2))
	if strings.Count(s, "<path") != 1 || !strings.Contains(s, `fill="#123456"`) || !strings.Contains(s, `width="42"`) {
		t.Fatalf("%.200s", s)
	}
	s, _ = SVGString(c, WithForeground(color.NRGBA{0, 0, 0, 0x80}), WithBackground(color.NRGBA{0xFF, 0, 0, 0x80}))
	if !strings.Contains(s, `fill="#f00" fill-opacity="0.502"`) || !strings.Contains(s, `fill="#000" fill-opacity="0.502"`) {
		t.Fatalf("%.300s", s)
	}
	uri, err := SVGDataURI(c)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "data:image/svg+xml;base64,"))
	if !strings.HasPrefix(string(raw), "<svg") {
		t.Fatal("data URI does not wrap the SVG")
	}
}

func TestSVGLogoArea(t *testing.T) {
	c := mustEncode(t, pix, qr.H)
	s, err := SVGString(c, WithLogoArea(0.2))
	if err != nil {
		t.Fatal(err)
	}
	grid := svgModules(t, s, c.Size, DefaultQuietZone)
	side := int(0.2*float64(c.Size) + 0.5)
	if side%2 != c.Size%2 {
		side--
	}
	x0 := (c.Size - side) / 2
	cleared := 0
	for y := range grid {
		for x := range grid[y] {
			in := x >= x0 && x < x0+side && y >= x0 && y < x0+side
			switch {
			case in && grid[y][x]:
				t.Fatalf("module %d,%d inside the logo area is drawn", x, y)
			case !in && grid[y][x] != c.Modules[y][x]:
				t.Fatalf("module %d,%d outside the logo area differs", x, y)
			case in && c.Modules[y][x]:
				cleared++
			}
		}
	}
	if cleared == 0 {
		t.Fatal("nothing cleared")
	}
	for _, tt := range []struct {
		c    *qr.Code
		frac float64
	}{
		{mustEncode(t, pix, qr.M), 0.1},  // level too low
		{c, 0.31},                        // too large
		{c, -0.1},                        // negative
		{mustEncode(t, "1", qr.H), 0.01}, // rounds to nothing
		{mustEncode(t, pix, qr.Q), 0.3},  // over the EC budget
	} {
		if _, err := SVGString(tt.c, WithLogoArea(tt.frac)); err == nil {
			t.Errorf("logo %v accepted for %d-%s", tt.frac, tt.c.Version, tt.c.Level)
		}
	}
	if _, err := SVGString(mustEncode(t, pix, qr.Q), WithLogoArea(0.3)); !errors.Is(err, qr.ErrUnsafeArea) {
		t.Errorf("want ErrUnsafeArea, got %v", err)
	}
}

func TestTerminal(t *testing.T) {
	c := mustEncode(t, "A", qr.L)
	s, err := TerminalString(c, WithQuietZone(1))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) != 12 { // 23 rows, two per line
		t.Fatalf("%d lines", len(lines))
	}
	for _, l := range lines {
		if n := len([]rune(l)); n != 23 {
			t.Fatalf("line has %d columns", n)
		}
	}
	// Top line: quiet row (light) over the finder's top edge (dark).
	if !strings.HasPrefix(lines[0], " ▄▄▄▄▄▄▄ ") {
		t.Fatalf("first line %q", lines[0])
	}
	inv, _ := TerminalString(c, WithQuietZone(1), WithInvert())
	if !strings.HasPrefix(inv, "█▀▀▀▀▀▀▀█") {
		t.Fatalf("inverted first line %q", strings.SplitN(inv, "\n", 2)[0])
	}
	// The last line holds only the bottom quiet row: top halves.
	if last := lines[len(lines)-1]; strings.Trim(last, " ") != "" {
		t.Fatalf("last line %q", last)
	}
	if l := strings.Split(inv, "\n")[11]; l != strings.Repeat("▀", 23) {
		t.Fatalf("inverted last line %q", l)
	}
}

func TestOptionErrors(t *testing.T) {
	c := mustEncode(t, "A", qr.L)
	for _, o := range []Option{
		WithModuleSize(0), WithModuleSize(2000), WithQuietZone(-1), WithQuietZone(65),
		WithSize(-1), WithSize(1 << 15), WithForeground(nil), WithBackground(nil), WithLogoArea(0.5),
	} {
		if _, err := PNGBytes(c, o); !errors.Is(err, ErrOption) {
			t.Errorf("PNG accepted an invalid option: %v", err)
		}
		if _, err := SVGString(c, o); !errors.Is(err, ErrOption) {
			t.Errorf("SVG accepted an invalid option: %v", err)
		}
		if _, err := TerminalString(c, o); !errors.Is(err, ErrOption) {
			t.Errorf("Terminal accepted an invalid option: %v", err)
		}
		if _, err := PNGDataURI(c, o); err == nil {
			t.Error("PNGDataURI")
		}
		if _, err := SVGDataURI(c, o); err == nil {
			t.Error("SVGDataURI")
		}
	}
	if _, err := Image(c, WithSize(20)); !errors.Is(err, ErrOption) {
		t.Errorf("20 px for 29 modules: %v", err)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriteErrors(t *testing.T) {
	c := mustEncode(t, "A", qr.L)
	if PNG(failWriter{}, c) == nil || SVG(failWriter{}, c) == nil || Terminal(failWriter{}, c) == nil {
		t.Fatal("write errors must propagate")
	}
}

// TestPNGMatchesImage checks the hand-written PNG encoder against
// image/png decoding, pixel by pixel, for several geometries and colours.
func TestPNGMatchesImage(t *testing.T) {
	c := mustEncode(t, pix, qr.Q)
	for _, opts := range [][]Option{
		nil,
		{WithModuleSize(1), WithQuietZone(0)},
		{WithModuleSize(3), WithQuietZone(1)},
		{WithSize(301)},
		{WithSize(c.Size + 8)},
		{WithForeground(color.NRGBA{10, 20, 30, 255}), WithBackground(color.Transparent), WithModuleSize(2)},
		{WithForeground(color.NRGBA{200, 0, 0, 128}), WithModuleSize(2)},
	} {
		b, err := PNGBytes(c, opts...)
		if err != nil {
			t.Fatal(err)
		}
		got, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		want, err := Image(c, opts...)
		if err != nil {
			t.Fatal(err)
		}
		if got.Bounds() != want.Bounds() {
			t.Fatalf("bounds %v, want %v", got.Bounds(), want.Bounds())
		}
		for y := 0; y < want.Bounds().Dy(); y++ {
			for x := 0; x < want.Bounds().Dx(); x++ {
				g := color.NRGBAModel.Convert(got.At(x, y))
				w := color.NRGBAModel.Convert(want.At(x, y))
				if g != w {
					t.Fatalf("pixel %d,%d: %v, want %v", x, y, g, w)
				}
			}
		}
	}
}
