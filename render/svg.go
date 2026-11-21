package render

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"image/color"
	"io"
	"strconv"
	"strings"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
)

// SVG writes the symbol as a standalone SVG document. One module is one
// user unit; the width and height attributes are scaled by the module
// size. Dark modules are drawn as a single <path> whose subpaths are
// rectangles: horizontal runs of dark modules, merged downwards with
// identical runs in the following rows. shape-rendering="crispEdges"
// keeps edges sharp at any scale.
func SVG(w io.Writer, c *qr.Code, opts ...Option) error {
	o, err := newOptions(opts)
	if err != nil {
		return err
	}
	lx, ls, logo, err := o.logoBox(c)
	if err != nil {
		return err
	}
	dark := func(x, y int) bool {
		if logo && x >= lx && x < lx+ls && y >= lx && y < lx+ls {
			return false
		}
		return c.Modules[y][x]
	}

	units := c.Size + 2*o.quietZone
	px := units * o.moduleSize
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" shape-rendering="crispEdges">`,
		units, units, px, px)
	if hex, opaque := svgColor(o.bg); opaque {
		fmt.Fprintf(bw, `<path fill="%s" d="M0 0h%dv%dH0z"/>`, hex, units, units)
	} else if a := alpha(o.bg); a > 0 {
		fmt.Fprintf(bw, `<path fill="%s" fill-opacity="%s" d="M0 0h%dv%dH0z"/>`, hex, opacity(a), units, units)
	}
	bw.WriteString(`<path`)
	if hex, opaque := svgColor(o.fg); hex != "#000" || !opaque {
		fmt.Fprintf(bw, ` fill="%s"`, hex)
		if !opaque {
			fmt.Fprintf(bw, ` fill-opacity="%s"`, opacity(alpha(o.fg)))
		}
	}
	bw.WriteString(` d="`)
	writeRects(bw, c.Size, dark, o.quietZone)
	bw.WriteString(`"/></svg>`)
	return bw.Flush()
}

// rect is a w×h block of dark modules with top-left corner (x, y).
type rect struct{ x, y, w, h int }

// writeRects emits the merged rectangles as path data. Each row is split
// into maximal horizontal runs; a run identical in position and width to
// a rectangle open in the row above extends it downwards, anything else
// closes the rectangle. Both lists are sorted by x, so one merge pass per
// row suffices.
func writeRects(w *bufio.Writer, n int, dark func(x, y int) bool, off int) {
	var buf []byte
	emit := func(r rect) {
		buf = buf[:0]
		buf = append(buf, 'M')
		buf = strconv.AppendInt(buf, int64(r.x+off), 10)
		buf = append(buf, ' ')
		buf = strconv.AppendInt(buf, int64(r.y+off), 10)
		buf = append(buf, 'h')
		buf = strconv.AppendInt(buf, int64(r.w), 10)
		buf = append(buf, 'v')
		buf = strconv.AppendInt(buf, int64(r.h), 10)
		buf = append(buf, 'h', '-')
		buf = strconv.AppendInt(buf, int64(r.w), 10)
		buf = append(buf, 'z')
		w.Write(buf)
	}
	open := make([]rect, 0, n)
	next := make([]rect, 0, n)
	for y := 0; y <= n; y++ {
		next = next[:0]
		i := 0 // index into open
		for x := 0; y < n && x < n; {
			if !dark(x, y) {
				x++
				continue
			}
			s := x
			for x < n && dark(x, y) {
				x++
			}
			// Close open rectangles entirely left of, or overlapping
			// without matching, this run.
			for i < len(open) && open[i].x < s {
				emit(open[i])
				i++
			}
			if i < len(open) && open[i].x == s && open[i].w == x-s {
				r := open[i]
				r.h++
				next = append(next, r)
				i++
			} else {
				next = append(next, rect{s, y, x - s, 1})
			}
		}
		for ; i < len(open); i++ {
			emit(open[i])
		}
		open, next = next, open
	}
}

// svgColor formats c as #rgb or #rrggbb (straight alpha) and reports
// whether it is fully opaque.
func svgColor(c color.Color) (string, bool) {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	if n.R>>4 == n.R&0xF && n.G>>4 == n.G&0xF && n.B>>4 == n.B&0xF {
		return fmt.Sprintf("#%x%x%x", n.R&0xF, n.G&0xF, n.B&0xF), n.A == 0xFF
	}
	return fmt.Sprintf("#%02x%02x%02x", n.R, n.G, n.B), n.A == 0xFF
}

func alpha(c color.Color) float64 {
	return float64(color.NRGBAModel.Convert(c).(color.NRGBA).A) / 255
}

func opacity(a float64) string { return strconv.FormatFloat(a, 'g', 3, 64) }

// SVGString returns the SVG document as a string.
func SVGString(c *qr.Code, opts ...Option) (string, error) {
	var sb strings.Builder
	if err := SVG(&sb, c, opts...); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// SVGDataURI returns the SVG as a "data:image/svg+xml;base64,..." URI.
func SVGDataURI(c *qr.Code, opts ...Option) (string, error) {
	s, err := SVGString(c, opts...)
	if err != nil {
		return "", err
	}
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(s)), nil
}
