package qr

// Block structure of every symbol, from ISO/IEC 18004:2015 Table 9.
//
// Only the number of error correction codewords per block and the number of
// blocks are stored. The split into short and long blocks and the number of
// data codewords follow from the total codeword count, which is itself
// derived from the symbol geometry (see rawDataModules): all blocks carry
// the same number of EC codewords and long blocks have one data codeword
// more than short ones, with the short blocks first.
//
// Rows are indexed by Level (L, M, Q, H) and columns by version; column 0
// is unused.
var eccPerBlock = [4][41]int8{
	{-1, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28},
	{-1, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	{-1, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
}

var numBlocks = [4][41]int8{
	{-1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},
	{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},
	{-1, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},
	{-1, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81},
}

// misdecodeProtection is the number p of EC codewords per block that the
// standard reserves for error detection instead of correction (Table 9,
// footnote). It is non-zero only for the smallest symbols.
func misdecodeProtection(version int, level Level) int {
	switch {
	case version == 1 && level == L:
		return 3
	case version == 1 && level == M, version == 2 && level == L:
		return 2
	case version == 1, version == 3 && level == L:
		return 1
	}
	return 0
}

// rawDataModules returns the number of modules available for codewords and
// remainder bits in a symbol of the given version: everything that is not
// a finder pattern, separator, timing pattern, alignment pattern, format
// information or version information.
func rawDataModules(version int) int {
	n := (16*version+128)*version + 64 // size² minus finders, separators, timing, format, dark module
	if version >= 2 {
		a := version/7 + 2
		n -= (25*a-10)*a - 55 // alignment patterns, minus overlaps with timing
		if version >= 7 {
			n -= 36 // two 6x3 version information blocks
		}
	}
	return n
}

// totalCodewords is the number of 8-bit codewords in a symbol; the
// remaining rawDataModules%8 modules are remainder bits.
func totalCodewords(version int) int { return rawDataModules(version) / 8 }

// dataCodewords is the number of data codewords for a version and level.
func dataCodewords(version int, level Level) int {
	return totalCodewords(version) - int(eccPerBlock[level][version])*int(numBlocks[level][version])
}

// alignmentPositions returns the row/column coordinates of the alignment
// pattern centres. The standard tabulates them (Annex E); the table follows
// a closed form: the first is 6, the last is size-7, and the ones in
// between are evenly spaced by an even step, rounded up, except for
// version 32 whose step is 26.
func alignmentPositions(version int) []int {
	if version == 1 {
		return nil
	}
	n := version/7 + 2
	var step int
	if version == 32 {
		step = 26
	} else {
		step = (version*4 + n*2 + 1) / (n*2 - 2) * 2
	}
	pos := make([]int, n)
	pos[0] = 6
	for i, p := n-1, version*4+10; i >= 1; i, p = i-1, p-step {
		pos[i] = p
	}
	return pos
}
