package render_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
	"github.com/victorcondackdev/pix-qrcode-go/render"
)

// A version 1 symbol drawn with half blocks, inverted for a dark terminal
// and with a one-module quiet zone to keep the example short (use the
// default of four in real output).
func ExampleTerminal() {
	c, err := qr.Encode("PIX", qr.L)
	if err != nil {
		panic(err)
	}
	if err := render.Terminal(os.Stdout, c, render.WithInvert(), render.WithQuietZone(1)); err != nil {
		panic(err)
	}
	// Output:
	// █▀▀▀▀▀▀▀██▀█▀▀█▀▀▀▀▀▀▀█
	// █ █▀▀▀█ █  █▄▀█ █▀▀▀█ █
	// █ █   █ █▀▄█▄▀█ █   █ █
	// █ ▀▀▀▀▀ █ █▀▄ █ ▀▀▀▀▀ █
	// █▀▀█▀██▀▀▄  ▄▄█▀▀▀█▀▀██
	// ██ ▄ ▄█▀▄▄▀▀█▀▀▄██▄ █ █
	// █▄▄▄▀ █▀▀▀▀ ▄▀█▀█▀▄▀▀▀█
	// █▀▀▀▀▀▀▀█  ▀▄█▀█ ▄ ▀▀▀█
	// █ █▀▀▀█ ██▀█ ▄▄▀  ▄▀▀▀█
	// █ █   █ █▄  ▄▀███▀▄▄▄ █
	// █ ▀▀▀▀▀ █ ▄▀▀▀▄▄█ ▀ █▄█
	// ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀
}

func ExampleSVGString() {
	c, err := qr.Encode("HELLO WORLD", qr.Q)
	if err != nil {
		panic(err)
	}
	svg, err := render.SVGString(c, render.WithModuleSize(4))
	if err != nil {
		panic(err)
	}
	fmt.Println(svg[:strings.Index(svg, ">")+1])
	// Output:
	// <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 29 29" width="116" height="116" shape-rendering="crispEdges">
}
