package render

import (
	"bufio"
	"io"
	"strings"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
)

// Terminal writes the symbol as text, two module rows per line, using the
// Unicode half blocks ▀ ▄ █ and spaces. By default dark modules are
// filled, which suits terminals with a light background; use WithInvert on
// dark-background terminals. Each line ends with a newline.
func Terminal(w io.Writer, c *qr.Code, opts ...Option) error {
	o, err := newOptions(opts)
	if err != nil {
		return err
	}
	q := o.quietZone
	filled := func(x, y int) bool { return c.At(x-q, y-q) != o.invert }
	units := c.Size + 2*q
	bw := bufio.NewWriter(w)
	for y := 0; y < units; y += 2 {
		for x := 0; x < units; x++ {
			top := filled(x, y)
			bottom := y+1 < units && filled(x, y+1)
			switch {
			case top && bottom:
				bw.WriteString("█")
			case top:
				bw.WriteString("▀")
			case bottom:
				bw.WriteString("▄")
			default:
				bw.WriteByte(' ')
			}
		}
		bw.WriteByte('\n')
	}
	return bw.Flush()
}

// TerminalString returns the Terminal rendering as a string.
func TerminalString(c *qr.Code, opts ...Option) (string, error) {
	var sb strings.Builder
	if err := Terminal(&sb, c, opts...); err != nil {
		return "", err
	}
	return sb.String(), nil
}
