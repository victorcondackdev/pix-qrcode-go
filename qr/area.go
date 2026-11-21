package qr

import (
	"errors"
	"fmt"
)

// ErrUnsafeArea is returned by CheckClearArea.
var ErrUnsafeArea = errors.New("qr: clearing this area is unsafe")

// CheckClearArea reports whether the w×h module rectangle with top-left
// corner (x, y) can be blanked (for example to overlay a logo) while the
// symbol stays comfortably decodable. It returns nil when:
//
//   - the rectangle lies inside the symbol and does not touch the finder
//     patterns and their separators, the timing patterns, the format
//     information or the version information. Alignment patterns may be
//     covered: readers locate the grid from the remaining ones, but small
//     symbols with a single alignment pattern are more fragile, which is
//     one more reason to keep the area small;
//   - in every Reed–Solomon block, the number of codewords with at least
//     one module in the rectangle is at most a quarter of the block's EC
//     codewords (half of what the block can correct). The other half is
//     left for print defects, glare and perspective.
//
// A blanked module is an error, not an erasure, for a reader: it cannot
// know which modules were hidden. The check is therefore conservative,
// but it is still about a perfect print of the remaining modules; test
// real-world scans before relying on a logo.
func (c *Code) CheckClearArea(x, y, w, h int) error {
	if w <= 0 || h <= 0 || x < 0 || y < 0 || x+w > c.Size || y+h > c.Size {
		return fmt.Errorf("%w: rectangle (%d,%d) %dx%d is outside the %dx%d symbol", ErrUnsafeArea, x, y, w, h, c.Size, c.Size)
	}
	m := newMatrix(c.Version)
	m.drawFunctionPatterns()
	align := make([]bool, len(m.function))
	pos := alignmentPositions(c.Version)
	last := len(pos) - 1
	for i, cy := range pos {
		for j, cx := range pos {
			if i == 0 && j == 0 || i == 0 && j == last || i == last && j == 0 {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					if cx+dx != 6 && cy+dy != 6 { // timing modules stay protected
						align[(cy+dy)*c.Size+cx+dx] = true
					}
				}
			}
		}
	}
	inside := func(i int) bool {
		px, py := i%c.Size, i/c.Size
		return px >= x && px < x+w && py >= y && py < y+h
	}
	for i, f := range m.function {
		if f && !align[i] && inside(i) {
			return fmt.Errorf("%w: rectangle covers a finder, timing, format or version module at (%d,%d)",
				ErrUnsafeArea, i%c.Size, i/c.Size)
		}
	}

	// Map each data module to its codeword, and each codeword to a block.
	b := layout(c.Version, c.Level)
	owner := make([]int, b.total)
	for i := 0; i < b.blocks; i++ {
		for j := 0; j < b.dataLen(i); j++ {
			k := j*b.blocks + i
			if j == b.shortData {
				k = b.shortData*b.blocks + i - b.short
			}
			owner[k] = i
		}
		for j := 0; j < b.ecc; j++ {
			owner[b.dataSize+j*b.blocks+i] = i
		}
	}
	hit := make([]bool, b.total)
	bit := 0
	m.forEachDataModule(func(i int) {
		if k := bit / 8; k < b.total && inside(i) {
			hit[k] = true
		}
		bit++
	})
	perBlock := make([]int, b.blocks)
	for k, h := range hit {
		if h {
			perBlock[owner[k]]++
		}
	}
	budget := (b.ecc - misdecodeProtection(c.Version, c.Level)) / 2 / 2
	for i, n := range perBlock {
		if n > budget {
			return fmt.Errorf("%w: block %d loses %d codewords, budget is %d at %d-%s",
				ErrUnsafeArea, i, n, budget, c.Version, c.Level)
		}
	}
	return nil
}
