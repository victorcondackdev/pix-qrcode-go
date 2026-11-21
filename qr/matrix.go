package qr

// matrix is the working representation of a symbol: flat row-major module
// colours plus a mask of function modules, which are excluded from data
// placement and masking.
type matrix struct {
	size     int
	dark     []bool
	function []bool
}

func newMatrix(version int) *matrix {
	n := version*4 + 17
	return &matrix{size: n, dark: make([]bool, n*n), function: make([]bool, n*n)}
}

func (m *matrix) version() int { return (m.size - 17) / 4 }

// setFunction sets a function module at column x, row y.
func (m *matrix) setFunction(x, y int, dark bool) {
	m.dark[y*m.size+x] = dark
	m.function[y*m.size+x] = true
}

func (m *matrix) rows() [][]bool {
	r := make([][]bool, m.size)
	for y := range r {
		r[y] = m.dark[y*m.size : (y+1)*m.size : (y+1)*m.size]
	}
	return r
}

func (m *matrix) drawFunctionPatterns() {
	n := m.size
	for i := 0; i < n; i++ {
		m.setFunction(6, i, i%2 == 0)
		m.setFunction(i, 6, i%2 == 0)
	}
	m.drawFinder(3, 3)
	m.drawFinder(n-4, 3)
	m.drawFinder(3, n-4)

	pos := alignmentPositions(m.version())
	last := len(pos) - 1
	for i, y := range pos {
		for j, x := range pos {
			if i == 0 && j == 0 || i == 0 && j == last || i == last && j == 0 {
				continue // these overlap the finder patterns
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					m.setFunction(x+dx, y+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}

	// Reserve the format areas (drawn for real after masking) and draw the
	// version information, which does not depend on the mask.
	m.drawFormat(0, 0)
	m.drawVersion()
}

// drawFinder draws a finder pattern centred at (cx, cy) with its light
// separator ring, clipped to the symbol.
func (m *matrix) drawFinder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || y < 0 || x >= m.size || y >= m.size {
				continue
			}
			d := max(abs(dx), abs(dy))
			m.setFunction(x, y, d != 2 && d != 4)
		}
	}
}

// formatInfo returns the 15-bit format information: 5 data bits (level
// indicator and mask), 10 BCH(15,5) check bits with generator 0x537, and
// the 0x5412 XOR mask that keeps the word from being all zeros.
func formatInfo(level Level, mask int) uint32 {
	data := level.formatBits()<<3 | uint32(mask)
	rem := data
	for i := 0; i < 10; i++ {
		rem = rem<<1 ^ (rem>>9)*0x537
	}
	return (data<<10 | rem) ^ 0x5412
}

// versionInfo returns the 18-bit version information: 6 version bits and
// 12 BCH(18,6) check bits with generator 0x1F25.
func versionInfo(version int) uint32 {
	rem := uint32(version)
	for i := 0; i < 12; i++ {
		rem = rem<<1 ^ (rem>>11)*0x1F25
	}
	return uint32(version)<<12 | rem
}

// formatPositions returns the flat indices of the two copies of the
// format information, bit i of the 15-bit word at index i of each array.
func formatPositions(n int) (first, second [15]int) {
	at := func(x, y int) int { return y*n + x }
	// First copy, around the top-left finder.
	for i := 0; i <= 5; i++ {
		first[i] = at(8, i)
	}
	first[6] = at(8, 7)
	first[7] = at(8, 8)
	first[8] = at(7, 8)
	for i := 9; i < 15; i++ {
		first[i] = at(14-i, 8)
	}
	// Second copy, split between the top-right and bottom-left finders.
	for i := 0; i < 8; i++ {
		second[i] = at(n-1-i, 8)
	}
	for i := 8; i < 15; i++ {
		second[i] = at(8, n-15+i)
	}
	return first, second
}

func (m *matrix) drawFormat(level Level, mask int) {
	bits := formatInfo(level, mask)
	first, second := formatPositions(m.size)
	for i := 0; i < 15; i++ {
		d := bits>>uint(i)&1 != 0
		m.dark[first[i]], m.function[first[i]] = d, true
		m.dark[second[i]], m.function[second[i]] = d, true
	}
	n := m.size
	m.setFunction(8, n-8, true) // the dark module, always set
}

func (m *matrix) drawVersion() {
	v := m.version()
	if v < 7 {
		return
	}
	bits := versionInfo(v)
	for i := 0; i < 18; i++ {
		dark := bits>>uint(i)&1 != 0
		a, b := m.size-11+i%3, i/3
		m.setFunction(a, b, dark) // top right
		m.setFunction(b, a, dark) // bottom left
	}
}

// forEachDataModule visits the non-function modules in placement order:
// two-module-wide columns from the right edge, alternating upwards and
// downwards, skipping the vertical timing pattern column.
func (m *matrix) forEachDataModule(f func(i int)) {
	n := m.size
	for right := n - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		upward := (right+1)&2 == 0
		for vert := 0; vert < n; vert++ {
			y := vert
			if upward {
				y = n - 1 - vert
			}
			for j := 0; j < 2; j++ {
				i := y*n + right - j
				if !m.function[i] {
					f(i)
				}
			}
		}
	}
}

// drawCodewords places the codeword bits, most significant first. The
// remainder modules after the last codeword stay light (zero bits).
func (m *matrix) drawCodewords(cw []byte) {
	k := 0
	total := len(cw) * 8
	m.forEachDataModule(func(i int) {
		if k < total {
			m.dark[i] = cw[k>>3]>>(7-uint(k&7))&1 != 0
			k++
		}
	})
}

// maskBit reports whether mask pattern p inverts the module at (x, y).
func maskBit(p, x, y int) bool {
	switch p {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (x/3+y/2)%2 == 0
	case 5:
		return x*y%2+x*y%3 == 0
	case 6:
		return (x*y%2+x*y%3)%2 == 0
	default:
		return ((x+y)%2+x*y%3)%2 == 0
	}
}

// applyMask XORs mask pattern p onto the data modules; applying it twice
// restores the original.
func (m *matrix) applyMask(p int) {
	n := m.size
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			i := y*n + x
			if !m.function[i] && maskBit(p, x, y) {
				m.dark[i] = !m.dark[i]
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
