package qr

// Penalty weights from ISO/IEC 18004:2015 section 7.8.3.
const (
	penaltyN1 = 3
	penaltyN2 = 3
	penaltyN3 = 40
	penaltyN4 = 10
)

// Rule 3 patterns as 11-bit windows, oldest module first: four light
// modules followed by dark-light-dark-dark-dark-light-dark, and the
// mirror image.
const (
	finderBefore = 0b00001011101
	finderAfter  = 0b10111010000
)

// runScore[k] is what the k-th module of a run adds under rule 1: a run
// of five scores N1 and each further module one more.
var runScore = func() (t [178]int) {
	t[5] = penaltyN1
	for k := 6; k < len(t); k++ {
		t[k] = 1
	}
	return t
}()

// blockScore[s] scores a 2x2 block whose dark-module count is s.
var blockScore = [5]int{penaltyN2, 0, 0, 0, penaltyN2}

// maskTile[p][y%12][x%6] is mask pattern p at (x, y). Every pattern
// repeats with period 6 horizontally and 12 vertically (the conditions
// use x mod 2, 3 and 6, y mod 2, 3, 4 and 6), so one tile covers all
// symbol sizes.
var maskTile = func() (t [8][12][6]uint8) {
	for p := range t {
		for y := range t[p] {
			for x := range t[p][y] {
				t[p][y][x] = b2u8(maskBit(p, x, y))
			}
		}
	}
	return t
}()

// bestMask evaluates all eight masks, each with its format information,
// and returns the one with the lowest penalty; ties go to the lower
// number. It works on byte copies of the matrix so the inner loops are
// branch-free on module data.
func (m *matrix) bestMask(level Level) int {
	n := m.size
	base := make([]uint8, n*n)   // modules before masking
	isData := make([]uint8, n*n) // 1 where the mask applies
	for i, d := range m.dark {
		base[i] = b2u8(d)
		isData[i] = b2u8(!m.function[i])
	}
	g := make([]uint8, n*n)
	t := make([]uint8, n*n)
	first, second := formatPositions(n)
	best, bestScore := 0, int(^uint(0)>>1)
	for p := 0; p < 8; p++ {
		for y := 0; y < n; y++ {
			row := &maskTile[p][y%12]
			for x, k := 0, 0; x < n; x, k = x+1, k+1 {
				if k == 6 {
					k = 0
				}
				i := y*n + x
				g[i] = base[i] ^ isData[i]&row[k]
			}
		}
		bits := formatInfo(level, p)
		for i := 0; i < 15; i++ {
			g[first[i]] = uint8(bits >> uint(i) & 1)
			g[second[i]] = uint8(bits >> uint(i) & 1)
		}
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				t[x*n+y] = g[y*n+x]
			}
		}
		if s := penalty(g, t, n); s < bestScore {
			best, bestScore = p, s
		}
	}
	return best
}

// penalty scores a symbol given row-major modules g and their transpose t
// (1 = dark) with the four rules of section 7.8.3:
//
//  1. each run of five or more same-coloured modules in a row or column
//     scores N1 plus one per module beyond five;
//  2. each 2x2 block of one colour scores N2 (blocks may overlap);
//  3. each dark-light-dark-dark-dark-light-dark (1:1:3:1:1) sequence in a
//     row or column scores N3 for each side on which it is followed by
//     four light modules; the quiet zone counts as light;
//  4. the proportion of dark modules scores N4 for every full 5% step
//     away from 50%.
func penalty(g, t []uint8, n int) int {
	score := 0
	for a := 0; a < n; a++ {
		score += linePenalty(g[a*n : (a+1)*n])
		score += linePenalty(t[a*n : (a+1)*n])
	}
	dark := 0
	for y := 0; y < n; y++ {
		row := g[y*n : (y+1)*n]
		for _, c := range row {
			dark += int(c)
		}
		if y+1 < n {
			next := g[(y+1)*n : (y+2)*n]
			for x := 0; x+1 < n; x++ {
				score += blockScore[row[x]+row[x+1]+next[x]+next[x+1]]
			}
		}
	}
	total := n * n
	k := (abs(dark*20-total*10)+total-1)/total - 1
	return score + k*penaltyN4
}

// linePenalty applies rules 1 and 3 to one row or column.
func linePenalty(line []uint8) int {
	score := 0
	run := 0
	prev := uint8(2) // matches neither colour
	var w uint32     // last 11 modules; starts as light quiet zone
	for _, d := range line {
		same := int(1 - (d^prev)&1)
		if prev == 2 {
			same = 0
		}
		run = run*same + 1
		score += runScore[run]
		prev = d
		w = (w<<1 | uint32(d)) & 0x7FF
		if w == finderBefore || w == finderAfter {
			score += penaltyN3
		}
	}
	for i := 0; i < 4; i++ { // light quiet zone after the line
		w = w << 1 & 0x7FF
		if w == finderAfter {
			score += penaltyN3
		}
	}
	return score
}

func b2u8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
