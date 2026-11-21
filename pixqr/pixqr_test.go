package pixqr

import (
	"bytes"
	"errors"
	"image/png"
	"strings"
	"testing"

	"github.com/victorcondackdev/pix-qrcode-go/internal/decode"
	"github.com/victorcondackdev/pix-qrcode-go/qr"
	"github.com/victorcondackdev/pix-qrcode-go/render"
)

// Sample payloads. The keys, names and URLs are fictitious; every payload
// has a valid CRC. The README savings table is generated from these and
// checked by TestSavings.
var samples = []struct {
	name    string
	payload string
	// Byte-mode bits and version vs optimal bits and version, at level M.
	byteBits, byteVersion, bits, version int
}{
	{"static, random key (EVP), mixed-case name",
		"00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D",
		1108, 8, 865, 7},
	{"static, phone key, amount and txid",
		"00020126360014br.gov.bcb.pix0114+5521987654321520400005303986540512.505802BR5918Padaria Pao Quente6007Niteroi62120508PEDIDO4263048559",
		1076, 8, 814, 6},
	{"static, CPF key, uppercase name and city",
		"00020126330014br.gov.bcb.pix0111123456789095204000053039865406150.005802BR5912LOJA EXEMPLO6009SAO PAULO62140510PEDIDO123463047DEC",
		1044, 8, 688, 5},
	{"dynamic, location URL",
		"00020101021226810014br.gov.bcb.pix2559pix.example.com/qr/v2/cobv/9d36b84fc70b478fb95c12729b90ca255204000053039865802BR5912LOJA EXEMPLO6009SAO PAULO62070503***6304BBB9",
		1340, 9, 1065, 8},
	{"static, e-mail key, 25-character txid",
		"00020126440014br.gov.bcb.pix0122financeiro@example.com5204000053039865406100.005802BR5920MERCADO BOM PRECO LT6014RIO DE JANEIRO62290525PEDIDO20261121A0001XPTO996304790A",
		1356, 9, 1000, 8},
}

func TestSavings(t *testing.T) {
	for _, s := range samples {
		d, err := Info(s.payload)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if d.ByteModeBits != s.byteBits || d.ByteModeVersion != s.byteVersion || d.DataBits != s.bits || d.Version != s.version {
			t.Errorf("%s: byte %d bits v%d, optimal %d bits v%d; table says %d v%d, %d v%d",
				s.name, d.ByteModeBits, d.ByteModeVersion, d.DataBits, d.Version, s.byteBits, s.byteVersion, s.bits, s.version)
		}
		if d.BitsSaved != d.ByteModeBits-d.DataBits || d.BytesSaved != d.BitsSaved/8 || d.Level != qr.M ||
			d.Size != 4*d.Version+17 || d.PayloadBytes != len(s.payload) {
			t.Errorf("%s: inconsistent details %+v", s.name, d)
		}
		c, err := Encode(s.payload)
		if err != nil {
			t.Fatal(err)
		}
		r, err := decode.Decode(c.Modules)
		if err != nil || string(r.Data) != s.payload {
			t.Fatalf("%s: round trip: %v", s.name, err)
		}
	}
}

func TestValidate(t *testing.T) {
	good := samples[0].payload
	tests := []struct {
		in   string
		want error
	}{
		{good, nil},
		{strings.ToLower(good[:len(good)-4]) + strings.ToLower(good[len(good)-4:]), ErrCRC}, // body changed
		{good[:len(good)-4] + "1d3d", nil},                                                  // hex is case-insensitive
		{good[:len(good)-1] + "E", ErrCRC},
		{good[:len(good)-1] + "G", ErrCRC},
		{"", ErrNotBRCode},
		{"000201", ErrNotBRCode},
		{"010201" + good[6:], ErrNotBRCode},
		{good[:len(good)-8] + "6305" + good[len(good)-4:], ErrNotBRCode},
		{AppendCRC("000201" + strings.Repeat("9", MaxPayload)), ErrNotBRCode},
	}
	for _, tt := range tests {
		err := Validate(tt.in)
		if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
			t.Errorf("Validate(%.20q...) = %v, want %v", tt.in, err, tt.want)
		}
	}
	if AppendCRC("00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***") != good {
		t.Error("AppendCRC does not reproduce the reference checksum")
	}
}

func TestPNG(t *testing.T) {
	b, err := PNG(samples[2].payload, 300)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if r := img.Bounds(); r.Dx() != 300 || r.Dy() != 300 {
		t.Fatalf("image is %v", r)
	}
	if _, err := PNG(samples[2].payload, 10); !errors.Is(err, render.ErrOption) {
		t.Fatalf("tiny size: %v", err)
	}
	if _, err := PNG("nope", 300); !errors.Is(err, ErrNotBRCode) {
		t.Fatalf("invalid payload: %v", err)
	}
}

func TestSVGAndOptions(t *testing.T) {
	p := samples[0].payload
	s, err := SVG(p, WithLevel(qr.H), WithRenderOptions(render.WithLogoArea(0.2)), WithQROptions(qr.WithMinVersion(12)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "<svg") || !strings.Contains(s, `viewBox="0 0 73 73"`) { // v12 = 65 + 8
		t.Fatalf("unexpected SVG head: %.120s", s)
	}
	if _, err := SVG("x"); !errors.Is(err, ErrNotBRCode) {
		t.Fatal(err)
	}
	if _, err := SVG(p, WithRenderOptions(render.WithLogoArea(0.2))); err == nil {
		t.Fatal("logo area accepted at level M")
	}
	if _, err := Info("x"); !errors.Is(err, ErrNotBRCode) {
		t.Fatal(err)
	}
	if _, err := Info(p, WithQROptions(qr.WithMaxVersion(2))); !errors.Is(err, qr.ErrTooLong) {
		t.Fatal(err)
	}
	if _, err := Info(p, WithQROptions(qr.WithMaxVersion(7))); !errors.Is(err, qr.ErrTooLong) {
		t.Fatal("byte-mode comparison should fail at v7 for this payload:", err)
	}
	if _, err := PNG(p, 300, WithQROptions(qr.WithMaxVersion(2))); !errors.Is(err, qr.ErrTooLong) {
		t.Fatal(err)
	}
}
