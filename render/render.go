// Package render draws QR Code symbols from package qr as PNG, SVG or
// terminal text, and wraps them in data URIs.
//
// All renderers add the quiet zone (4 modules by default, as the standard
// requires) and share one set of options; options that do not apply to a
// format are ignored by it.
package render

import (
	"errors"
	"fmt"
	"image/color"

	"github.com/victorcondackdev/pix-qrcode-go/qr"
)

// ErrOption is wrapped by errors about invalid options.
var ErrOption = errors.New("render: invalid option")

// Defaults.
const (
	DefaultQuietZone  = 4 // modules, the minimum ISO/IEC 18004 allows
	DefaultModuleSize = 8 // pixels per module for PNG, user units for SVG
)

type options struct {
	moduleSize int
	quietZone  int
	size       int
	fg, bg     color.Color
	invert     bool
	logo       float64
}

// Option configures a renderer.
type Option func(*options)

// WithModuleSize sets the size of a module in pixels (PNG) or the
// width/height scale of the SVG. Default 8.
func WithModuleSize(px int) Option { return func(o *options) { o.moduleSize = px } }

// WithQuietZone sets the light margin in modules. Default 4; smaller
// values are accepted for space-constrained layouts where the surrounding
// background is known to be light, but scanners may struggle.
func WithQuietZone(modules int) Option { return func(o *options) { o.quietZone = modules } }

// WithSize asks the PNG renderer for an image exactly px pixels wide and
// high. The module size is the largest integer that fits the symbol and
// its quiet zone; leftover pixels widen the margin, so modules stay sharp.
// It overrides WithModuleSize for PNG.
func WithSize(px int) Option { return func(o *options) { o.size = px } }

// WithForeground sets the colour of dark modules. Default black.
func WithForeground(c color.Color) Option { return func(o *options) { o.fg = c } }

// WithBackground sets the colour of light modules and the quiet zone.
// Default white. A fully transparent background is left out of SVG output.
func WithBackground(c color.Color) Option { return func(o *options) { o.bg = c } }

// WithInvert makes the terminal renderer draw light modules as filled
// blocks, which is what dark-background terminals need for the symbol to
// read dark-on-light.
func WithInvert() Option { return func(o *options) { o.invert = true } }

// WithLogoArea reserves a blank square in the centre of the SVG for a
// logo, side = fraction × symbol size (0 < fraction ≤ 0.3). It is only
// accepted for symbols at level Q or H, and only when the hidden modules
// fit the safety budget of [qr.Code.CheckClearArea]. Covering modules
// consumes error correction capacity that would otherwise absorb print
// and scan defects: prefer level H, keep the area small and test with the
// scanners your users have.
func WithLogoArea(fraction float64) Option { return func(o *options) { o.logo = fraction } }

func newOptions(opts []Option) (*options, error) {
	o := &options{
		moduleSize: DefaultModuleSize,
		quietZone:  DefaultQuietZone,
		fg:         color.Black,
		bg:         color.White,
	}
	for _, f := range opts {
		f(o)
	}
	switch {
	case o.moduleSize < 1 || o.moduleSize > 1024:
		return nil, fmt.Errorf("%w: module size %d", ErrOption, o.moduleSize)
	case o.quietZone < 0 || o.quietZone > 64:
		return nil, fmt.Errorf("%w: quiet zone %d", ErrOption, o.quietZone)
	case o.size < 0 || o.size > 1<<14:
		return nil, fmt.Errorf("%w: size %d", ErrOption, o.size)
	case o.fg == nil || o.bg == nil:
		return nil, fmt.Errorf("%w: nil colour", ErrOption)
	case o.logo < 0 || o.logo > 0.3:
		return nil, fmt.Errorf("%w: logo area %v (want 0 < fraction <= 0.3)", ErrOption, o.logo)
	}
	return o, nil
}

// logoBox returns the reserved centre square in symbol coordinates, or
// ok=false when no logo area was requested.
func (o *options) logoBox(c *qr.Code) (x, side int, ok bool, err error) {
	if o.logo == 0 {
		return 0, 0, false, nil
	}
	if c.Level < qr.Q {
		return 0, 0, false, fmt.Errorf("%w: logo area needs level Q or H, symbol is %s", ErrOption, c.Level)
	}
	side = int(o.logo*float64(c.Size) + 0.5)
	if side%2 != c.Size%2 { // same parity as the symbol, so it centres exactly
		side--
	}
	if side < 1 {
		return 0, 0, false, fmt.Errorf("%w: logo area too small for a %d-module symbol", ErrOption, c.Size)
	}
	x = (c.Size - side) / 2
	if err := c.CheckClearArea(x, x, side, side); err != nil {
		return 0, 0, false, fmt.Errorf("render: logo area: %w", err)
	}
	return x, side, true, nil
}
