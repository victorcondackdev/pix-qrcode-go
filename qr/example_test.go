package qr_test

import (
	"fmt"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
)

func ExampleEncode() {
	c, err := qr.Encode("00020126330014br.gov.bcb.pix0111123456789095204000053039865406150.005802BR"+
		"5912LOJA EXEMPLO6009SAO PAULO62140510PEDIDO123463047DEC", qr.M)
	if err != nil {
		panic(err)
	}
	fmt.Printf("version %d, %dx%d modules, level %s, mask %d\n", c.Version, c.Size, c.Size, c.Level, c.Mask)
	for _, s := range c.Segments {
		fmt.Println(s.Mode, len(s.Data))
	}
	// Output:
	// version 5, 37x37 modules, level M, mask 0
	// numeric 14
	// byte 14
	// numeric 37
	// alphanumeric 64
}

// Boosting keeps the smallest version and spends its spare capacity on
// error correction.
func ExampleWithBoostECL() {
	c, err := qr.Encode("https://example.com/", qr.L, qr.WithBoostECL())
	if err != nil {
		panic(err)
	}
	fmt.Println(c.Version, c.Level)
	// Output:
	// 2 Q
}

func ExampleCode_At() {
	c, _ := qr.Encode("1", qr.L)
	// The top-left finder pattern: a dark 7x7 ring around a 3x3 core.
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			if c.At(x, y) {
				fmt.Print("#")
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println()
	}
	// Output:
	// #######
	// #.....#
	// #.###.#
	// #.###.#
	// #.###.#
	// #.....#
	// #######
}
