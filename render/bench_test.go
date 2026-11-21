package render

import (
	"image/png"
	"io"
	"testing"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
)

const benchPayload = "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
	"520400005303986540549.905802BR5913Fulano de Tal6008BRASILIA62140510PEDIDO20266304EFFF"

func benchCode(b *testing.B) *qr.Code {
	c, err := qr.Encode(benchPayload, qr.M)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	return c
}

func BenchmarkPNG(b *testing.B) {
	c := benchCode(b)
	for i := 0; i < b.N; i++ {
		if err := PNG(io.Discard, c, WithSize(512)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSVG(b *testing.B) {
	c := benchCode(b)
	for i := 0; i < b.N; i++ {
		if err := SVG(io.Discard, c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTerminal(b *testing.B) {
	c := benchCode(b)
	for i := 0; i < b.N; i++ {
		if err := Terminal(io.Discard, c); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPNGStdlib is the same image through image/png, for comparison.
func BenchmarkPNGStdlib(b *testing.B) {
	c := benchCode(b)
	for i := 0; i < b.N; i++ {
		img, err := Image(c, WithSize(512))
		if err != nil {
			b.Fatal(err)
		}
		if err := png.Encode(io.Discard, img); err != nil {
			b.Fatal(err)
		}
	}
}
