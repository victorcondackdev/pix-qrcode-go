package render_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
	"github.com/victorcondackdev/pix-qrcode-go/render"
)

// TestWriteCorpus writes one PNG per version and level (plus mask and
// logo-area variants) with the expected text next to it, for an external
// decoder to check: see scripts/opencv_crosscheck.py. It only runs when
// QR_CORPUS_DIR is set.
func TestWriteCorpus(t *testing.T) {
	dir := os.Getenv("QR_CORPUS_DIR")
	if dir == "" {
		t.Skip("QR_CORPUS_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewSource(7))
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:abcdefghijklmnopqrstuvwxyz@#"
	write := func(name string, c *qr.Code, data string) {
		b, err := render.PNGBytes(c, render.WithModuleSize(4))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".png"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for v := 1; v <= 40; v++ {
		for l := qr.L; l <= qr.H; l++ {
			// Runs of digits, uppercase and lowercase so every symbol mixes
			// the three modes; the longest prefix that fits fills it.
			var sb strings.Builder
			for sb.Len() < 7200 {
				ch := chars[r.Intn(len(chars))]
				for k := r.Intn(12) + 1; k > 0; k-- {
					sb.WriteByte(ch)
					ch = chars[(strings.IndexByte(chars, ch)+1)%len(chars)]
				}
			}
			long := sb.String()
			lo, hi := 0, len(long)
			for lo < hi {
				mid := (lo + hi + 1) / 2
				if _, err := qr.Encode(long[:mid], l, qr.WithMinVersion(v), qr.WithMaxVersion(v)); err == nil {
					lo = mid
				} else {
					hi = mid - 1
				}
			}
			data := long[:lo]
			c, err := qr.Encode(data, l, qr.WithMinVersion(v), qr.WithMaxVersion(v), qr.WithMask((v+int(l))%8))
			if err != nil {
				t.Fatal(err)
			}
			write(fmt.Sprintf("v%02d-%s-m%d", v, l, c.Mask), c, data)
		}
	}
	// Pix payload at every mask, and with a cleared logo area.
	pix := "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"
	for m := 0; m < 8; m++ {
		c, _ := qr.Encode(pix, qr.M, qr.WithMask(m))
		write(fmt.Sprintf("pix-M-mask%d", m), c, pix)
	}
	for _, l := range []qr.Level{qr.Q, qr.H} {
		c, _ := qr.Encode(pix, l)
		side := int(0.2*float64(c.Size) + 0.5)
		if side%2 != c.Size%2 {
			side--
		}
		x := (c.Size - side) / 2
		if err := c.CheckClearArea(x, x, side, side); err != nil {
			t.Logf("%s: %v", l, err)
			continue
		}
		for yy := x; yy < x+side; yy++ {
			for xx := x; xx < x+side; xx++ {
				c.Modules[yy][xx] = false
			}
		}
		write(fmt.Sprintf("pix-%s-logo", l), c, pix)
	}
	// UTF-8 in byte mode.
	c, _ := qr.Encode("Padaria Pão Quente – São Paulo", qr.M)
	write("utf8", c, "Padaria Pão Quente – São Paulo")
}
