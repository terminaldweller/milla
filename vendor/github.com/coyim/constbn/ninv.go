package constbn

/*
 * Compute -(1/x) mod 2^31. If x is even, then this function returns 0.
 */

// Ninv calculates -(1/x) mod 2^31 of the given number
func Ninv(x Base) Base {
	two := Base(2)
	y := two - x
	y *= two - y*x
	y *= two - y*x
	y *= two - y*x
	y *= two - y*x
	return mux(x&one, -y, zero) & mask31
}
