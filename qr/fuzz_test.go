package qr

import (
	"errors"
	"testing"
	"unicode/utf8"
)

// FuzzEncode encodes arbitrary input with arbitrary options and checks
// that the independent decoder reads back exactly the input, version,
// level and mask, and that the segmentation is no worse than byte mode.
func FuzzEncode(f *testing.F) {
	f.Add("00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D", uint8(1), uint8(0), int8(-1), false)
	f.Add("HELLO WORLD", uint8(2), uint8(134), int8(3), true)
	f.Add("0123456789012345678901234567890", uint8(3), uint8(154), int8(7), false)
	f.Add("日本語 \xff\x00 é", uint8(0), uint8(137), int8(0), true)
	f.Add("", uint8(0), uint8(167), int8(5), false)
	f.Fuzz(func(t *testing.T, data string, level, minVersion uint8, mask int8, boost bool) {
		if len(data) > 3000 {
			return
		}
		v := 1 // half the inputs use the smallest version that fits
		if minVersion >= 128 {
			v = int(minVersion%40) + 1
		}
		opts := []Option{WithMinVersion(v), WithMask(int(mask))}
		if mask < -1 || mask > 7 {
			opts[1] = WithMask(-1)
		}
		if boost {
			opts = append(opts, WithBoostECL())
		}
		c, err := Encode(data, Level(level%4), opts...)
		if errors.Is(err, ErrTooLong) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		roundTrip(t, c, data)
		if c.Level < Level(level%4) {
			t.Fatalf("level lowered from %s to %s", Level(level%4), c.Level)
		}
		if data != "" {
			if byteBits := segmentBits(Segment{Byte, data}, c.Version); c.DataBits > byteBits {
				t.Fatalf("segmentation %d bits, byte mode %d", c.DataBits, byteBits)
			}
		}
		if utf8.ValidString(data) {
			for _, s := range c.Segments {
				if !utf8.ValidString(s.Data) {
					t.Fatalf("segment %q splits a character", s.Data)
				}
			}
		}
	})
}
