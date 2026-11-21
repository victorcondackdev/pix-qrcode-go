package render

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"io"
	"sync"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
)

// geometry computes the module scale, the margin before the first module
// and the image side, in pixels.
func geometry(c *qr.Code, o *options) (scale, margin, side int, err error) {
	units := c.Size + 2*o.quietZone
	if o.size == 0 {
		return o.moduleSize, o.quietZone * o.moduleSize, units * o.moduleSize, nil
	}
	scale = o.size / units
	if scale < 1 {
		return 0, 0, 0, fmt.Errorf("%w: %d px cannot hold %d modules with the quiet zone", ErrOption, o.size, units)
	}
	return scale, (o.size - c.Size*scale) / 2, o.size, nil
}

// Image returns the symbol as a two-colour paletted image (index 0 is the
// background, 1 the foreground), for callers that compose it with other
// graphics. PNG does not go through it.
func Image(c *qr.Code, opts ...Option) (*image.Paletted, error) {
	o, err := newOptions(opts)
	if err != nil {
		return nil, err
	}
	scale, margin, side, err := geometry(c, o)
	if err != nil {
		return nil, err
	}
	img := image.NewPaletted(image.Rect(0, 0, side, side), color.Palette{o.bg, o.fg})
	for y := 0; y < side; y++ {
		my := (y - margin) / scale
		if y < margin || my >= c.Size {
			continue
		}
		line := img.Pix[y*img.Stride : y*img.Stride+side]
		for x := margin; x < margin+c.Size*scale; x++ {
			if c.Modules[my][(x-margin)/scale] {
				line[x] = 1
			}
		}
	}
	return img, nil
}

// PNG writes the symbol as a PNG image: 1 bit per pixel with a two-entry
// palette (and a tRNS chunk if a colour is translucent). The encoder is
// built on compress/zlib and hash/crc32 and packs pixel rows directly from
// the modules, which is several times faster than image/png for this kind
// of image.
func PNG(w io.Writer, c *qr.Code, opts ...Option) error {
	o, err := newOptions(opts)
	if err != nil {
		return err
	}
	scale, margin, side, err := geometry(c, o)
	if err != nil {
		return err
	}

	stride := 1 + (side+7)/8 // filter byte, then packed pixels
	blank := make([]byte, stride)
	row := make([]byte, stride)

	z := zlibPool.Get().(*zlibBuf)
	defer zlibPool.Put(z)
	z.buf.Reset()
	z.w.Reset(&z.buf)
	for y := 0; y < side; y++ {
		my := (y - margin) / scale
		if y < margin || my >= c.Size {
			z.w.Write(blank)
			continue
		}
		if y == margin || (y-margin)%scale == 0 { // first pixel row of a module row
			clear(row)
			for x, d := range c.Modules[my] {
				if !d {
					continue
				}
				for px := margin + x*scale; px < margin+(x+1)*scale; px++ {
					row[1+px>>3] |= 0x80 >> uint(px&7)
				}
			}
		}
		z.w.Write(row)
	}
	if err := z.w.Close(); err != nil {
		return err
	}

	bg := color.NRGBAModel.Convert(o.bg).(color.NRGBA)
	fg := color.NRGBAModel.Convert(o.fg).(color.NRGBA)
	var out bytes.Buffer
	out.Grow(z.buf.Len() + 128)
	out.WriteString("\x89PNG\r\n\x1a\n")
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], uint32(side))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(side))
	ihdr[8], ihdr[9] = 1, 3 // bit depth 1, indexed colour; compression, filter, interlace 0
	chunk(&out, "IHDR", ihdr[:])
	chunk(&out, "PLTE", []byte{bg.R, bg.G, bg.B, fg.R, fg.G, fg.B})
	if bg.A != 0xFF || fg.A != 0xFF {
		chunk(&out, "tRNS", []byte{bg.A, fg.A})
	}
	chunk(&out, "IDAT", z.buf.Bytes())
	chunk(&out, "IEND", nil)
	_, err = w.Write(out.Bytes())
	return err
}

func chunk(b *bytes.Buffer, typ string, data []byte) {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(data)))
	b.Write(n[:])
	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(data)
	b.WriteString(typ)
	b.Write(data)
	binary.BigEndian.PutUint32(n[:], crc.Sum32())
	b.Write(n[:])
}

type zlibBuf struct {
	buf bytes.Buffer
	w   *zlib.Writer
}

// zlibPool reuses compressors: a fresh one allocates about 1 MB, far more
// than the image.
var zlibPool = sync.Pool{New: func() any {
	z := &zlibBuf{}
	z.w, _ = zlib.NewWriterLevel(&z.buf, zlib.DefaultCompression) // the level is valid
	return z
}}

// PNGBytes returns the symbol as PNG-encoded bytes.
func PNGBytes(c *qr.Code, opts ...Option) ([]byte, error) {
	var buf bytes.Buffer
	if err := PNG(&buf, c, opts...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// PNGDataURI returns the PNG as a "data:image/png;base64,..." URI, ready
// for an <img src> attribute.
func PNGDataURI(c *qr.Code, opts ...Option) (string, error) {
	b, err := PNGBytes(c, opts...)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b), nil
}
