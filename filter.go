package teishoku

import (
	"reflect"
	"unsafe"
)

// Filter provides a fast, cache-friendly iterator over all entities that have a
// specific set of components. It is the primary mechanism for implementing
// game logic (systems). The filter iterates directly over the component arrays
// within matching archetypes, providing maximum performance.
//
// This is the filter for entities with one component. Generated filters for
// multiple components (e.g., Filter2, Filter3) follow a similar pattern.
type Filter[T any] struct {
	world               *World
	curBase             unsafe.Pointer
	matchingArches      []*archetype
	curEntityIDs        []Entity
	cachedEntities      []Entity
	mask                bitmask256
	curMatchIdx         int // index into matchingArches
	curIdx              int // index into the current archetype's entity/component array
	compSize            uintptr
	curArchSize         int
	curEnt              Entity
	lastVersion         uint32 // world.archetypeVersion when matchingArches was last updated
	lastMutationVersion uint32 // world.mutationVersion when cachedEntities was last updated
	compID              uint8
}

// NewFilter creates a new `Filter` that iterates over all entities possessing
// at least the component of type `T`. The filter automatically discovers and
// caches the archetypes that match this component signature.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter[T]`.
func NewFilter[T any](w *World) *Filter[T] {
	id := w.getCompTypeID(reflect.TypeFor[T]())
	var m bitmask256
	m.set(id)
	f := &Filter[T]{world: w, mask: m, compID: id, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.compSize = w.compIDToSize[id]
	f.updateMatching()
	f.updateCachedEntities()
	f.Reset()
	return f
}

// New is a convenience function that creates a new filter instance.
func (f *Filter[T]) New(w *World) *Filter[T] {
	return NewFilter[T](w)
}

// updateMatching rebuilds the filter's list of archetypes that match its
// component mask. This is called automatically when the filter detects that
// the world's archetype layout has changed.
func (f *Filter[T]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// updateCachedEntities rebuilds the cached list of entities.
func (f *Filter[T]) updateCachedEntities() {
	total := 0
	for _, a := range f.matchingArches {
		total += a.size
	}
	if cap(f.cachedEntities) < total {
		f.cachedEntities = make([]Entity, total)
	} else {
		f.cachedEntities = f.cachedEntities[:total]
	}
	idx := 0
	for _, a := range f.matchingArches {
		copy(f.cachedEntities[idx:idx+a.size], a.entityIDs[:a.size])
		idx += a.size
	}
	f.lastMutationVersion = f.world.mutationVersion
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times. The filter
// will also automatically detect if new archetypes have been created since the
// last iteration and update its internal list accordingly.
func (f *Filter[T]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
		f.updateCachedEntities()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBase = a.compPointers[f.compID]
		f.curEntityIDs = a.entityIDs
		f.curArchSize = a.size
	}
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete. This method must
// be called before accessing the entity or its components.
//
// Example:
//
//	query := teishoku.NewFilter[Position](world)
//	for query.Next() {
//	    // ... process entity
//	}
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter[T]) Next() bool {
	for {
		f.curIdx++
		if f.curIdx >= f.curArchSize {
			f.curMatchIdx++
			if f.curMatchIdx >= len(f.matchingArches) {
				return false
			}
			a := f.matchingArches[f.curMatchIdx]
			f.curBase = a.compPointers[f.compID]
			f.curEntityIDs = a.entityIDs
			f.curArchSize = a.size
			f.curIdx = -1
			continue
		}
		f.curEnt = f.curEntityIDs[f.curIdx]
		return true
	}
}

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
//
// Returns:
//   - The current Entity.
func (f *Filter[T]) Entity() Entity {
	return f.curEnt
}

// Get returns a pointer to the component of type `T` for the current entity
// in the iteration. This should only be called after `Next()` has returned true.
//
// Returns:
//   - A pointer to the component data (*T).
func (f *Filter[T]) Get() *T {
	ptr := unsafe.Pointer(uintptr(f.curBase) + uintptr(f.curIdx)*f.compSize)
	return (*T)(ptr)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory, making it highly
// performant for clearing large groups of entities.
//
// After this operation, the filter will be empty.
func (f *Filter[T]) RemoveEntities() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := range a.size {
			ent := a.entityIDs[i]
			meta := &f.world.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.freeIDs = append(f.world.freeIDs, ent.ID)
		}
		a.size = 0
	}
	f.world.mutationVersion++
	f.Reset()
}

// Entities returns all entities that match the filter.
// Note: The returned slice is owned by the Filter and may be invalidated on next Entities call or world mutation. Copy if needed for long-term use.
func (f *Filter[T]) Entities() []Entity {
	if f.world.archetypeVersion != f.lastVersion || f.world.mutationVersion != f.lastMutationVersion {
		f.updateMatching()
		f.updateCachedEntities()
	}
	return f.cachedEntities
}
