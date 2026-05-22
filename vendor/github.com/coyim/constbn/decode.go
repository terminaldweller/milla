package constbn

/*
 * Decode an integer from its big-endian unsigned representation. The
 * "true" bit length of the integer is computed and set in the encoded
 * announced bit length (x[0]), but all words of x[] corresponding to
 * the full 'len' bytes of the source are set.
 *
 * CT: value or length of x does not leak.
 */

func simpleDecode(src []byte) []Base {
	result := make([]Base, (len(src)/2)+2)
	Decode(result, src)
	return result
}

// Decode will decode the given number from a big endian unsigned byte array. The given result
// should be an array with sufficient size for the number
func Decode(x []Base, src []byte) {
	u := len(src)
	v := 1
	acc := uint(0)
	accLen := uint(0)
	for u > 0 {
		u--
		b := src[u]
		acc |= uint(Base(b) << accLen)
		accLen += 8
		if accLen >= 31 {
			x[v] = Base(acc) & mask31
			v++
			accLen -= 31
			acc = uint(b) >> (8 - accLen)
		}
	}
	if accLen != 0 {
		x[v] = Base(acc)
		v++
	}
	x[0] = bitLength(x[1:], v-1)
}
