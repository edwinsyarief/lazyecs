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

// newQueryCache creates and initializes a new `queryCache`. It sets up the
// cache with the specified world and component mask and pre-allocates slices
// for matching archetypes and entities to reduce future allocations.
//
// Parameters:
//   - w: The `World` to associate with the cache.
//   - m: The `bitmask256` representing the component layout to be matched.
//
// Returns:
//   - An initialized `queryCache` instance.
func newQueryCache(w *World, m bitmask256) queryCache {
	return queryCache{
		world:          w,
		mask:           m,
		matchingArches: make([]*archetype, 0, 4),
		cachedEntities: nil,
	}
}

// updateMatching rebuilds the filter's list of archetypes that match its
// component mask. This is called automatically when the filter detects that
// the world's archetype layout has changed.
func (c *queryCache) updateMatching() {
	c.matchingArches = c.matchingArches[:0]
	isZeroMask := c.mask == bitmask256{}

	for _, a := range c.world.archetypes.archetypes {
		if a.size > 0 {
			if (isZeroMask && a.mask == c.mask) || (!isZeroMask && a.mask.contains(c.mask)) {
				c.matchingArches = append(c.matchingArches, a)
			}
		}
	}
	c.lastVersion = c.world.archetypes.archetypeVersion
}

// updateCachedEntities rebuilds the cached list of entities by collecting all
// entity IDs from the archetypes currently matching the filter's query. This
// method is called when the cache is stale to ensure the entity list is
// up-to-date with the world state. After rebuilding, it updates the cache's
// mutation version to match the world's current version.
func (c *queryCache) updateCachedEntities() {
	total := 0
	for _, a := range c.matchingArches {
		total += a.size
	}
	if c.cachedEntities == nil || cap(c.cachedEntities) < total {
		capacity := c.world.entities.capacity
		newCap := total
		if capacity > newCap {
			newCap = capacity
		}
		c.cachedEntities = make([]Entity, total, newCap)
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

// IsStale checks if the cache is out of sync with the world's state by
// comparing the cache's last known version numbers with the world's current
// versions. A cache is considered stale if either the archetype structure has
// changed (e.g., a new archetype was created) or if entities have been created
// or deleted.
//
// Returns:
//   - true if the cache is stale and needs to be updated, false otherwise.
func (c *queryCache) IsStale() bool {
	return c.world.archetypes.archetypeVersion != c.lastVersion || c.world.mutationVersion != c.lastMutationVersion
}

// Entities returns a slice of all entities that match the cached query. If the
// cache is detected as stale (i.e., out of sync with the world state), it will
// first update its internal lists of matching archetypes and entities before
// returning the result. This ensures the returned slice is always up-to-date.
//
// Returns:
//   - A slice of `Entity` objects that match the query.
func (c *queryCache) Entities() []Entity {
	if c.world.archetypes.archetypeVersion != c.lastVersion {
		c.updateMatching()
		c.updateCachedEntities()
	} else if c.cachedEntities == nil || c.world.mutationVersion != c.lastMutationVersion {
		c.updateCachedEntities()
	}
	return c.cachedEntities
}
