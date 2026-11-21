package qr

// Reed–Solomon error correction over GF(2^8) with the QR Code field
// polynomial x^8 + x^4 + x^3 + x^2 + 1 (0x11D) and generator element α = 2.

const gfPoly = 0x11D

var (
	gfExp [512]byte // gfExp[i] = α^i, doubled so gfExp[a+b] needs no modulo
	gfLog [256]byte // gfLog[α^i] = i; gfLog[0] is unused
)

// rsGenerators[d] is the generator polynomial of degree d, built at init.
// Only degrees used by the block table are populated.
var rsGenerators [31][]byte

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfLog[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= gfPoly
		}
	}
	for i := 255; i < len(gfExp); i++ {
		gfExp[i] = gfExp[i-255]
	}
	for lvl := range eccPerBlock {
		for v := 1; v <= 40; v++ {
			d := eccPerBlock[lvl][v]
			if rsGenerators[d] == nil {
				rsGenerators[d] = rsGenerator(int(d))
			}
		}
	}
}

// gfMul multiplies two field elements.
func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// rsGenerator returns the monic polynomial (x-α^0)(x-α^1)...(x-α^(d-1)),
// without its leading coefficient, highest degree first.
func rsGenerator(degree int) []byte {
	g := make([]byte, degree)
	g[degree-1] = 1 // start from the constant polynomial 1
	root := byte(1)
	for i := 0; i < degree; i++ {
		// Multiply by (x - root); subtraction is XOR in GF(2^8).
		for j := 0; j < degree; j++ {
			g[j] = gfMul(g[j], root)
			if j+1 < degree {
				g[j] ^= g[j+1]
			}
		}
		root = gfMul(root, 2)
	}
	return g
}

// rsRemainder writes into ecc the remainder of data·x^len(ecc) divided by
// the generator gen, i.e. the EC codewords. len(ecc) must equal len(gen).
func rsRemainder(data, gen, ecc []byte) {
	for i := range ecc {
		ecc[i] = 0
	}
	n := len(ecc)
	for _, b := range data {
		factor := b ^ ecc[0]
		copy(ecc, ecc[1:])
		ecc[n-1] = 0
		if factor == 0 {
			continue
		}
		lf := int(gfLog[factor])
		for i, g := range gen {
			if g != 0 {
				ecc[i] ^= gfExp[int(gfLog[g])+lf]
			}
		}
	}
}
