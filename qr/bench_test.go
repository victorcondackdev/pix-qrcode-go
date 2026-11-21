package qr

import (
	"strings"
	"testing"
)

// benchPayload is a 153-byte static Pix payload (EVP key, amount, txid).
const benchPayload = "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
	"520400005303986540549.905802BR5913Fulano de Tal6008BRASILIA62140510PEDIDO20266304EFFF"

func BenchmarkEncodePix(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(benchPayload, M); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodePixByteMode(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(benchPayload, M, WithByteMode()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodePixFixedMask(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(benchPayload, M, WithMask(0)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeVersion40(b *testing.B) {
	s := strings.Repeat("a", 2953)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(s, L); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSegmentation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		optimalSegments(benchPayload, 7)
	}
}
