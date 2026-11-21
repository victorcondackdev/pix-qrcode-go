// Command pixqr renders a Pix BR Code ("copia e cola" payload) as a QR
// Code: PNG, SVG or text for the terminal.
//
// Run "pixqr -h" for usage.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/victorcondackdev/pix-qrcode-go/pixqr"
	"github.com/victorcondackdev/pix-qrcode-go/qr"
	"github.com/victorcondackdev/pix-qrcode-go/render"
)

// Exit codes.
const (
	exitOK      = 0
	exitInvalid = 1
	exitUsage   = 2
)

const usage = `pixqr renders a Pix BR Code as a QR Code.

Usage:
  pixqr [flags] [payload | -]

The payload is read from standard input when it is omitted or "-";
surrounding whitespace is ignored. Without -o, the code is printed to
the terminal (or, with --svg, the SVG document is written to standard
output).

Flags:
  -o FILE        write the image to FILE (PNG, or SVG with --svg or a .svg name)
  --svg          produce SVG instead of PNG
  --term         print the code to the terminal (also with -o)
  --invert       for --term: draw light modules filled (dark terminals)
  --level L      error correction level L, M, Q or H (default M)
  --size PX      PNG width and height in pixels (default 512)
  --quiet N      quiet zone in modules (default 4)
  --info         print version, size, segments and savings vs byte mode
  --no-validate  encode the input even if it is not a valid BR Code

Exit status: 0 success, 1 invalid input, 2 usage error.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pixqr", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	out := fs.String("o", "", "")
	svg := fs.Bool("svg", false, "")
	term := fs.Bool("term", false, "")
	invert := fs.Bool("invert", false, "")
	levelFlag := fs.String("level", "M", "")
	size := fs.Int("size", 512, "")
	quiet := fs.Int("quiet", render.DefaultQuietZone, "")
	info := fs.Bool("info", false, "")
	noValidate := fs.Bool("no-validate", false, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return exitOK
		}
		fmt.Fprintf(stderr, "pixqr: %v\n\n%s", err, usage)
		return exitUsage
	}
	level, err := qr.ParseLevel(*levelFlag)
	if err != nil {
		fmt.Fprintf(stderr, "pixqr: --level must be L, M, Q or H\n")
		return exitUsage
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "pixqr: expected one payload, got %d arguments (quote it)\n", fs.NArg())
		return exitUsage
	}

	payload := fs.Arg(0)
	if payload == "" || payload == "-" {
		b, err := io.ReadAll(io.LimitReader(stdin, 64<<10))
		if err != nil {
			fmt.Fprintf(stderr, "pixqr: reading stdin: %v\n", err)
			return exitInvalid
		}
		payload = string(b)
	}
	payload = strings.TrimSpace(payload)
	if payload == "" {
		fmt.Fprintf(stderr, "pixqr: no payload\n\n%s", usage)
		return exitUsage
	}

	if !*noValidate {
		if err := pixqr.Validate(payload); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return exitInvalid
		}
	}
	code, err := qr.Encode(payload, level)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitInvalid
	}
	ropts := []render.Option{render.WithQuietZone(*quiet)}

	if *info {
		naive, err := qr.Encode(payload, level, qr.WithByteMode())
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return exitInvalid
		}
		fmt.Fprintf(stdout, "version   %d (%dx%d modules)\nlevel     %s\nmask      %d\n", code.Version, code.Size, code.Size, code.Level, code.Mask)
		fmt.Fprintf(stdout, "payload   %d bytes\ndata      %d bits (byte mode: %d bits, version %d)\n", len(payload), code.DataBits, naive.DataBits, naive.Version)
		for _, s := range code.Segments {
			fmt.Fprintf(stdout, "segment   %-12s %3d  %s\n", s.Mode, len(s.Data), s.Data)
		}
	}

	if *out != "" {
		var buf bytes.Buffer
		if *svg || strings.HasSuffix(strings.ToLower(*out), ".svg") {
			err = render.SVG(&buf, code, ropts...)
		} else {
			err = render.PNG(&buf, code, append(ropts, render.WithSize(*size))...)
		}
		if err != nil {
			fmt.Fprintf(stderr, "pixqr: %v\n", err)
			return exitUsage
		}
		if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
			fmt.Fprintf(stderr, "pixqr: %v\n", err)
			return exitInvalid
		}
	} else if *svg {
		if err := render.SVG(stdout, code, ropts...); err != nil {
			fmt.Fprintf(stderr, "pixqr: %v\n", err)
			return exitUsage
		}
		fmt.Fprintln(stdout)
		return exitOK
	}

	if *term || *out == "" && !*info {
		if *invert {
			ropts = append(ropts, render.WithInvert())
		}
		if err := render.Terminal(stdout, code, ropts...); err != nil {
			fmt.Fprintf(stderr, "pixqr: %v\n", err)
			return exitUsage
		}
	}
	return exitOK
}
