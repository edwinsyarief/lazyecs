package teishoku

// bitmask256 represents a set of up to 256 component IDs. It is used to
// uniquely identify archetypes. Each bit corresponds to a component ID, and if
// the bit is set, it indicates that the component is present in the archetype.
type bitmask256 [4]uint64

// set enables the bit corresponding to the given component ID.
func (m *bitmask256) set(bit uint8) {
	i := bit >> 6 // (bit / 64) to find the uint64 index
	o := bit & 63 // (bit % 64) to find the bit offset
	m[i] |= uint64(1) << uint64(o)
}

// unset disables the bit corresponding to the given component ID.
func (m *bitmask256) unset(bit uint8) {
	i := bit >> 6
	o := bit & 63
	m[i] &= ^(uint64(1) << uint64(o))
}

// contains checks if all the bits set in the `sub` bitmask are also set in the
// receiver bitmask `m`. This is used to determine if an archetype's component
// set is a superset of a filter's required components.
//
// Parameters:
//   - sub: The bitmask representing the subset of components to check for.
//
// Returns:
//   - true if the receiver contains all components from the subset, false otherwise.
func (m bitmask256) contains(sub bitmask256) bool {
	return (m[0]&sub[0]) == sub[0] &&
		(m[1]&sub[1]) == sub[1] &&
		(m[2]&sub[2]) == sub[2] &&
		(m[3]&sub[3]) == sub[3]
}

// containsBit checks if a specific bit is set in the mask.
func (m bitmask256) containsBit(bit uint8) bool {
	i := bit >> 6
	o := bit & 63
	return (m[i] & (uint64(1) << uint64(o))) != 0
}
