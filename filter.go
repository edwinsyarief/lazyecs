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
	world          *World
	matchingArches []*archetype
	mask           bitmask256
	curMatchIdx    int // index into matchingArches
	curIdx         int // index into the current archetype's entity/component array
	curEnt         Entity
	lastVersion    uint32 // world.archetypeVersion when matchingArches was last updated
	compID         uint8
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
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	var m bitmask256
	m.set(id)
	f := &Filter[T]{world: w, mask: m, compID: id, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
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

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times. The filter
// will also automatically detect if new archetypes have been created since the
// last iteration and update its internal list accordingly.
func (f *Filter[T]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
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
		if f.curMatchIdx >= len(f.matchingArches) {
			return false
		}
		a := f.matchingArches[f.curMatchIdx]
		if f.curIdx >= a.size {
			f.curMatchIdx++
			f.curIdx = -1
			continue
		}
		f.curEnt = a.entityIDs[f.curIdx]
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
	a := f.matchingArches[f.curMatchIdx]
	ptr := unsafe.Pointer(uintptr(a.compPointers[f.compID]) + uintptr(f.curIdx)*a.compSizes[f.compID])
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
	f.Reset()
}

// Entities returns all entities that match the filter.
func (f *Filter[T]) Entities() []Entity {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	total := 0
	for _, a := range f.matchingArches {
		total += a.size
	}
	ents := make([]Entity, total)
	idx := 0
	for _, a := range f.matchingArches {
		copy(ents[idx:idx+a.size], a.entityIDs[:a.size])
		idx += a.size
	}
	return ents
}
