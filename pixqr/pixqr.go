// Package pixqr turns Pix BR Codes ("copia e cola" payloads) into QR Code
// images.
//
// It checks that the input looks like a BR Code before encoding it: the
// payload must start with the EMV payload format indicator "000201" and
// end with a valid CRC field ("6304" plus the CRC-16/CCITT-FALSE of
// everything before it). It does not parse or lint the other fields; use
// github.com/victorcondackdev/brpay for that.
//
// Symbols use error correction level M by default, the usual choice for
// Pix, and the optimal segmentation of package qr: BR Codes are mostly
// digits and uppercase text, so mixing numeric, alphanumeric and byte
// segments takes fewer codewords than encoding the payload as bytes.
package pixqr

import (
	"errors"
	"fmt"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
	"github.com/victorcondackdev/pix-qrcode-go/render"
)

// MaxPayload is the longest payload accepted, in bytes. EMV MPM limits a
// payload to 512 characters.
const MaxPayload = 512

// Validation errors.
var (
	ErrNotBRCode = errors.New("pixqr: not a BR Code")
	ErrCRC       = errors.New("pixqr: CRC mismatch")
)

// Validate checks the BR Code envelope: length, the "000201" prefix and
// the trailing CRC field.
func Validate(payload string) error {
	n := len(payload)
	switch {
	case n > MaxPayload:
		return fmt.Errorf("%w: %d bytes, the limit is %d", ErrNotBRCode, n, MaxPayload)
	case n < 14 || payload[:6] != "000201":
		return fmt.Errorf("%w: missing payload format indicator 000201", ErrNotBRCode)
	case payload[n-8:n-4] != "6304":
		return fmt.Errorf("%w: does not end with a CRC field (6304XXXX)", ErrNotBRCode)
	}
	want := checksum(payload[:n-4])
	got, ok := parseHex16(payload[n-4:])
	if !ok || got != want {
		return fmt.Errorf("%w: field 63 is %s, payload checksum is %04X", ErrCRC, payload[n-4:], want)
	}
	return nil
}

// crcTable is the byte-wise table for CRC-16/CCITT-FALSE.
var crcTable = func() (t [256]uint16) {
	for i := range t {
		c := uint16(i) << 8
		for j := 0; j < 8; j++ {
			if c&0x8000 != 0 {
				c = c<<1 ^ 0x1021
			} else {
				c <<= 1
			}
		}
		t[i] = c
	}
	return t
}()

// checksum is CRC-16/CCITT-FALSE: polynomial 0x1021, initial value 0xFFFF,
// no reflection, no final XOR.
func checksum(s string) uint16 {
	crc := uint16(0xFFFF)
	for i := 0; i < len(s); i++ {
		crc = crc<<8 ^ crcTable[byte(crc>>8)^s[i]]
	}
	return crc
}

// AppendCRC appends "6304" and the checksum to a payload that has no CRC
// field yet. It is a convenience for tests and examples.
func AppendCRC(payload string) string {
	p := payload + "6304"
	return fmt.Sprintf("%s%04X", p, checksum(p))
}

func parseHex16(s string) (uint16, bool) {
	var v uint16
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			c -= '0'
		case c >= 'A' && c <= 'F':
			c -= 'A' - 10
		case c >= 'a' && c <= 'f':
			c -= 'a' - 10
		default:
			return 0, false
		}
		v = v<<4 | uint16(c)
	}
	return v, true
}

type config struct {
	level  qr.Level
	qr     []qr.Option
	render []render.Option
}

// Option configures this package's functions.
type Option func(*config)

// WithLevel sets the error correction level. Default qr.M.
func WithLevel(l qr.Level) Option { return func(c *config) { c.level = l } }

// WithQROptions passes options to qr.Encode, for example a minimum version
// to keep the symbol size stable.
func WithQROptions(opts ...qr.Option) Option {
	return func(c *config) { c.qr = append(c.qr, opts...) }
}

// WithRenderOptions passes options to the renderer (colours, quiet zone,
// logo area...).
func WithRenderOptions(opts ...render.Option) Option {
	return func(c *config) { c.render = append(c.render, opts...) }
}

func newConfig(opts []Option) *config {
	c := &config{level: qr.M}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Encode validates payload and encodes it as a QR Code.
func Encode(payload string, opts ...Option) (*qr.Code, error) {
	return newConfig(opts).encode(payload)
}

func (c *config) encode(payload string) (*qr.Code, error) {
	if err := Validate(payload); err != nil {
		return nil, err
	}
	return qr.Encode(payload, c.level, c.qr...)
}

// PNG returns a PNG image of the payload, exactly size×size pixels (see
// render.WithSize).
func PNG(payload string, size int, opts ...Option) ([]byte, error) {
	c := newConfig(opts)
	code, err := c.encode(payload)
	if err != nil {
		return nil, err
	}
	return render.PNGBytes(code, append(c.render, render.WithSize(size))...)
}

// SVG returns an SVG document of the payload.
func SVG(payload string, opts ...Option) (string, error) {
	c := newConfig(opts)
	code, err := c.encode(payload)
	if err != nil {
		return "", err
	}
	return render.SVGString(code, c.render...)
}

// Details describes how a payload is encoded and what optimal segmentation
// saved compared with a single byte-mode segment.
type Details struct {
	Version  int
	Size     int // modules per side, without quiet zone
	Level    qr.Level
	Segments []qr.Segment

	PayloadBytes int
	DataBits     int // segment bits with optimal segmentation
	// ByteModeBits and ByteModeVersion are the bit length and version a
	// single byte-mode segment would need at the same level.
	ByteModeBits    int
	ByteModeVersion int
	// BitsSaved is ByteModeBits - DataBits; BytesSaved rounds it down to
	// whole codewords.
	BitsSaved, BytesSaved int
}

// Info validates and encodes payload and reports the outcome.
func Info(payload string, opts ...Option) (Details, error) {
	c := newConfig(opts)
	code, err := c.encode(payload)
	if err != nil {
		return Details{}, err
	}
	naive, err := qr.Encode(payload, c.level, append(c.qr[:len(c.qr):len(c.qr)], qr.WithByteMode())...)
	if err != nil {
		return Details{}, err
	}
	saved := naive.DataBits - code.DataBits
	return Details{
		Version:         code.Version,
		Size:            code.Size,
		Level:           code.Level,
		Segments:        code.Segments,
		PayloadBytes:    len(payload),
		DataBits:        code.DataBits,
		ByteModeBits:    naive.DataBits,
		ByteModeVersion: naive.Version,
		BitsSaved:       saved,
		BytesSaved:      saved / 8,
	}, nil
}
