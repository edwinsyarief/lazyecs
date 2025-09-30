package lazyecs

// bitmask256 represents up to 256 component bits.
type bitmask256 [4]uint64

func (m *bitmask256) set(bit uint8) {
	i := bit / 64
	o := bit % 64
	m[i] |= 1 << o
}
func (m *bitmask256) unset(bit uint8) {
	i := bit / 64
	o := bit % 64
	m[i] &= ^(1 << o)
}

// check if all bits in sub are set in m
func (m bitmask256) contains(sub bitmask256) bool {
	return (m[0]&sub[0]) == sub[0] &&
		(m[1]&sub[1]) == sub[1] &&
		(m[2]&sub[2]) == sub[2] &&
		(m[3]&sub[3]) == sub[3]
}

// intersects checks if this bitmask has any bits in common with another bitmask.
func (m bitmask256) intersects(other bitmask256) bool {
	return (m[0]&other[0] != 0) ||
		(m[1]&other[1] != 0) ||
		(m[2]&other[2] != 0) ||
		(m[3]&other[3] != 0)
}
