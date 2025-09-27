package lazyecs

// removePair is a helper struct used for sorting entities to be removed
// during batch operations. It ensures that removals from archetypes happen
// correctly without invalidating indices.
type removePair struct {
	index int
	e     Entity
}
