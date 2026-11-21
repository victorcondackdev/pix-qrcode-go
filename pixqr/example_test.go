package pixqr_test

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"

	"github.com/victorcondackdev/pix-qrcode-go/pixqr"
)

func ExampleInfo() {
	payload := "00020126330014br.gov.bcb.pix0111123456789095204000053039865406150.005802BR" +
		"5912LOJA EXEMPLO6009SAO PAULO62140510PEDIDO123463047DEC"
	d, err := pixqr.Info(payload)
	if err != nil {
		panic(err)
	}
	fmt.Printf("version %d (%dx%d), level %s\n", d.Version, d.Size, d.Size, d.Level)
	fmt.Printf("%d bits instead of %d in byte mode (version %d)\n", d.DataBits, d.ByteModeBits, d.ByteModeVersion)
	for _, s := range d.Segments {
		fmt.Printf("%-12s %q\n", s.Mode, s.Data)
	}
	// Output:
	// version 5 (37x37), level M
	// 688 bits instead of 1044 in byte mode (version 8)
	// numeric      "00020126330014"
	// byte         "br.gov.bcb.pix"
	// numeric      "0111123456789095204000053039865406150"
	// alphanumeric ".005802BR5912LOJA EXEMPLO6009SAO PAULO62140510PEDIDO123463047DEC"
}

func ExampleValidate() {
	err := pixqr.Validate("00020126330014br.gov.bcb.pix0111123456789095204000053039865406150.005802BR" +
		"5912LOJA EXEMPLO6009SAO PAULO62140510PEDIDO123463040000")
	fmt.Println(err)
	// Output:
	// pixqr: CRC mismatch: field 63 is 0000, payload checksum is 7DEC
}

func ExamplePNG() {
	data, err := pixqr.PNG("00020126330014br.gov.bcb.pix0111123456789095204000053039865406150.005802BR"+
		"5912LOJA EXEMPLO6009SAO PAULO62140510PEDIDO123463047DEC", 256)
	if err != nil {
		panic(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%dx%d, %d-colour palette\n", cfg.Width, cfg.Height, len(cfg.ColorModel.(color.Palette)))
	// Output:
	// 256x256, 2-colour palette
}
