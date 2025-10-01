package lazyecs

// bitmask256 represents up to 256 component bits.
type bitmask256 [4]uint64

func (m *bitmask256) set(bit uint8) {
	i := bit >> 6
	o := bit & 63
	m[i] |= uint64(1) << uint64(o)
}
func (m *bitmask256) unset(bit uint8) {
	i := bit >> 6
	o := bit & 63
	m[i] &= ^(uint64(1) << uint64(o))
}

// check if all bits in sub are set in m
func (m bitmask256) contains(sub bitmask256) bool {
	return (m[0]&sub[0]) == sub[0] &&
		(m[1]&sub[1]) == sub[1] &&
		(m[2]&sub[2]) == sub[2] &&
		(m[3]&sub[3]) == sub[3]
}
