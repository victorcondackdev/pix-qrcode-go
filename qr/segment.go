package qr

import (
	"fmt"
	"unicode/utf8"
)

// Mode is a data encoding mode. Its value is the 4-bit mode indicator.
type Mode uint8

// Encoding modes supported by the encoder. Kanji and ECI are not used:
// byte mode carries UTF-8, which every mainstream reader assumes.
const (
	Numeric      Mode = 0x1 // digits 0-9, 10 bits per 3 characters
	Alphanumeric Mode = 0x2 // 0-9 A-Z space $ % * + - . / :, 11 bits per 2 characters
	Byte         Mode = 0x4 // arbitrary bytes, 8 bits each
)

// String returns the mode name.
func (m Mode) String() string {
	switch m {
	case Numeric:
		return "numeric"
	case Alphanumeric:
		return "alphanumeric"
	case Byte:
		return "byte"
	}
	return fmt.Sprintf("Mode(%d)", uint8(m))
}

// Segment is a run of input encoded in a single mode.
type Segment struct {
	Mode Mode
	Data string // the input bytes covered by the segment
}

// countBits returns the width of the character count indicator.
func countBits(m Mode, version int) int {
	g := 0 // version group: 1-9, 10-26, 27-40
	if version >= 27 {
		g = 2
	} else if version >= 10 {
		g = 1
	}
	switch m {
	case Numeric:
		return [3]int{10, 12, 14}[g]
	case Alphanumeric:
		return [3]int{9, 11, 13}[g]
	default:
		return [3]int{8, 16, 16}[g]
	}
}

// segmentBits returns the exact number of bits a segment occupies,
// header included.
func segmentBits(s Segment, version int) int {
	n := len(s.Data)
	bits := 4 + countBits(s.Mode, version)
	switch s.Mode {
	case Numeric:
		bits += n/3*10 + [3]int{0, 4, 7}[n%3]
	case Alphanumeric:
		bits += n/2*11 + n%2*6
	default:
		bits += n * 8
	}
	return bits
}

// totalBits returns the bit length of segs in the given version, or -1 if
// a segment holds more characters than its count indicator can express.
func totalBits(segs []Segment, version int) int {
	total := 0
	for _, s := range segs {
		if len(s.Data) >= 1<<countBits(s.Mode, version) {
			return -1
		}
		total += segmentBits(s, version)
	}
	return total
}

const alnumChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"

// alnumValue maps a byte to its alphanumeric value, or -1.
var alnumValue = func() (t [256]int8) {
	for i := range t {
		t[i] = -1
	}
	for i := 0; i < len(alnumChars); i++ {
		t[alnumChars[i]] = int8(i)
	}
	return t
}()

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// DP states for optimal segmentation. A numeric segment is tracked by its
// length modulo 3 and an alphanumeric one by its length modulo 2, because
// that is what decides the cost of the next character exactly:
//
//	numeric:      a character opening a group costs 4 bits, the 2nd and 3rd 3 more (4, 7, 10)
//	alphanumeric: an odd character costs 6 bits, an even one 5 more (6, 11)
//	byte:         8 bits per byte
//
// Starting a segment costs the 4-bit mode indicator plus the character
// count indicator. With exact per-character costs, the shortest path
// through this 6-state graph is the shortest possible bit stream over
// numeric, alphanumeric and byte segments.
const (
	stN1 = iota // numeric, length ≡ 1 (mod 3)
	stN2        // numeric, length ≡ 2
	stN0        // numeric, length ≡ 0, non-empty
	stA1        // alphanumeric, odd length
	stA0        // alphanumeric, even length, non-empty
	stB         // byte
	numStates
)

func stateMode(s int) Mode {
	switch {
	case s <= stN0:
		return Numeric
	case s <= stA0:
		return Alphanumeric
	}
	return Byte
}

const inf = int(^uint(0) >> 2)

// optimalSegments splits data into the segmentation with the fewest bits
// for the version group of version. It works on characters: a multi-byte
// UTF-8 sequence (or an invalid byte) always goes to byte mode as a unit,
// so segments never split a character.
func optimalSegments(data string, version int) []Segment {
	if data == "" {
		return nil
	}
	hdr := [3]int{
		4 + countBits(Numeric, version),
		4 + countBits(Alphanumeric, version),
		4 + countBits(Byte, version),
	}
	type step struct {
		start, size int
		from        [numStates]int8 // predecessor state, -1 for the start
	}
	steps := make([]step, 0, len(data))
	cost := [numStates]int{inf, inf, inf, inf, inf, inf}
	first := true
	for i := 0; i < len(data); {
		size := 1
		if data[i] >= utf8.RuneSelf {
			_, size = utf8.DecodeRuneInString(data[i:])
		}
		next := [numStates]int{inf, inf, inf, inf, inf, inf}
		st := step{start: i, size: size}
		relax := func(to, from, c int) {
			if c < next[to] {
				next[to] = c
				st.from[to] = int8(from)
			}
		}
		// best[m] is the cheapest state not in mode m (or the start), for
		// opening a new segment of mode m.
		bestOutside := func(lo, hi int) (int, int) {
			if first {
				return 0, -1
			}
			bc, bs := inf, -1
			for s := 0; s < numStates; s++ {
				if (s < lo || s > hi) && cost[s] < bc {
					bc, bs = cost[s], s
				}
			}
			return bc, bs
		}
		c := data[i]
		if size == 1 && isDigit(c) {
			relax(stN2, stN1, cost[stN1]+3)
			relax(stN0, stN2, cost[stN2]+3)
			relax(stN1, stN0, cost[stN0]+4)
			if bc, bs := bestOutside(stN1, stN0); bc < inf {
				relax(stN1, bs, bc+hdr[0]+4)
			}
		}
		if size == 1 && alnumValue[c] >= 0 {
			relax(stA0, stA1, cost[stA1]+5)
			relax(stA1, stA0, cost[stA0]+6)
			if bc, bs := bestOutside(stA1, stA0); bc < inf {
				relax(stA1, bs, bc+hdr[1]+6)
			}
		}
		relax(stB, stB, cost[stB]+8*size)
		if bc, bs := bestOutside(stB, stB); bc < inf {
			relax(stB, bs, bc+hdr[2]+8*size)
		}
		steps = append(steps, st)
		cost = next
		first = false
		i += size
	}
	end := 0
	for s := 1; s < numStates; s++ {
		if cost[s] < cost[end] {
			end = s
		}
	}
	// Walk back, collecting each character's mode.
	modes := make([]Mode, len(steps))
	for k, s := len(steps)-1, end; k >= 0; k-- {
		modes[k] = stateMode(s)
		s = int(steps[k].from[s])
	}
	var segs []Segment
	for k := 0; k < len(steps); {
		j := k
		for j < len(steps) && modes[j] == modes[k] {
			j++
		}
		last := steps[j-1]
		segs = append(segs, Segment{Mode: modes[k], Data: data[steps[k].start : last.start+last.size]})
		k = j
	}
	return segs
}

// bitBuffer is an append-only, MSB-first bit sequence.
type bitBuffer struct {
	b []byte
	n int // number of bits
}

func (bb *bitBuffer) append(v uint32, bits int) {
	for i := bits - 1; i >= 0; i-- {
		if bb.n%8 == 0 {
			bb.b = append(bb.b, 0)
		}
		if v>>uint(i)&1 != 0 {
			bb.b[bb.n/8] |= 0x80 >> uint(bb.n%8)
		}
		bb.n++
	}
}

func (bb *bitBuffer) appendSegment(s Segment, version int) {
	bb.append(uint32(s.Mode), 4)
	bb.append(uint32(len(s.Data)), countBits(s.Mode, version))
	d := s.Data
	switch s.Mode {
	case Numeric:
		for i := 0; i < len(d); i += 3 {
			n := len(d) - i
			if n > 3 {
				n = 3
			}
			v := uint32(0)
			for j := i; j < i+n; j++ {
				v = v*10 + uint32(d[j]-'0')
			}
			bb.append(v, n*3+1)
		}
	case Alphanumeric:
		for i := 0; i+1 < len(d); i += 2 {
			bb.append(uint32(alnumValue[d[i]])*45+uint32(alnumValue[d[i+1]]), 11)
		}
		if len(d)%2 == 1 {
			bb.append(uint32(alnumValue[d[len(d)-1]]), 6)
		}
	default:
		for i := 0; i < len(d); i++ {
			bb.append(uint32(d[i]), 8)
		}
	}
}
