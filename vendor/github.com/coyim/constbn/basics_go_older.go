// +build !go1.12

package constbn

func mul31Lo(x, y Base) Base {
	return Base(x*y) & mask31
}
