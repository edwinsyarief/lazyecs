package teishoku

// queryCache provides a reusable mechanism for caching the results of a filter
// query. It stores a list of matching archetypes and entities, and tracks the
// world's version numbers to detect when the cache needs to be updated. This
// avoids redundant archetype lookups and entity list construction, improving
// performance for frequently used filters.
type queryCache struct {
	world               *World
	matchingArches      []*archetype
	cachedEntities      []Entity
	mask                bitmask256
	lastVersion         uint32 // world.archetypes.archetypeVersion when matchingArches was last updated
	lastMutationVersion uint32 // world.mutationVersion when cachedEntities was last updated
}

// newQueryCache creates a new query cache.
func newQueryCache(w *World, m bitmask256) queryCache {
	return queryCache{
		world:          w,
		mask:           m,
		matchingArches: make([]*archetype, 0, 4),
	}
}

// updateMatching rebuilds the filter's list of archetypes that match its
// component mask. This is called automatically when the filter detects that
// the world's archetype layout has changed.
func (c *queryCache) updateMatching() {
	c.matchingArches = c.matchingArches[:0]
	for _, a := range c.world.archetypes.archetypes {
		if a.mask.contains(c.mask) {
			c.matchingArches = append(c.matchingArches, a)
		}
	}
	c.lastVersion = c.world.archetypes.archetypeVersion
}

// updateCachedEntities rebuilds the cached list of entities.
func (c *queryCache) updateCachedEntities() {
	total := 0
	for _, a := range c.matchingArches {
		total += a.size
	}
	if cap(c.cachedEntities) < total {
		c.cachedEntities = make([]Entity, total)
	} else {
		c.cachedEntities = c.cachedEntities[:total]
	}
	idx := 0
	for _, a := range c.matchingArches {
		copy(c.cachedEntities[idx:idx+a.size], a.entityIDs[:a.size])
		idx += a.size
	}
	c.lastMutationVersion = c.world.mutationVersion
}

// IsStale checks if the cache is out of sync with the world state.
func (c *queryCache) IsStale() bool {
	return c.world.archetypes.archetypeVersion != c.lastVersion || c.world.mutationVersion != c.lastMutationVersion
}

// Entities returns all entities that match the filter.
func (c *queryCache) Entities() []Entity {
	if c.IsStale() {
		c.updateMatching()
		c.updateCachedEntities()
	}
	return c.cachedEntities
}
