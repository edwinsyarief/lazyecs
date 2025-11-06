package teishoku

import (
	"reflect"
	"unsafe"
)

// Filter provides a fast, cache-friendly iterator over all entities that have a
// specific set of components. It is the primary mechanism for implementing
// game logic (systems). The filter iterates directly over the component arrays
// within matching archetypes, providing maximum performance.
type Filter[T any] struct {
	curBase      unsafe.Pointer
	curEntityIDs []Entity
	queryCache
	curMatchIdx int // index into matchingArches
	curIdx      int // index into the current archetype's entity/component array
	compSize    uintptr
	curArchSize int
	compID      uint8
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
	w.mu.RLock()
	defer w.mu.RUnlock()
	id := w.getCompTypeID(reflect.TypeFor[T]())
	var m bitmask256
	m.set(id)
	f := &Filter[T]{
		queryCache:  newQueryCache(w, m),
		compID:      id,
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSize = w.components.compIDToSize[id]
	f.updateMatching([]uint8{id})
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component type, equivalent to calling `NewFilter`.
func (f *Filter[T]) New(w *World) *Filter[T] {
	return NewFilter[T](w)
}

// Reset rewinds the filter's iterator to the beginning. It must be called
// before re-iterating over a filter (e.g., in a loop). The filter will also
// automatically detect if new archetypes have been created since the last
// iteration and update its internal list accordingly.
func (f *Filter[T]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter[T]) doReset() {
	if f.isArchetypeStale() {
		f.updateMatching([]uint8{f.compID})
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBase = a.pointers[0]
		f.curEntityIDs = a.arch.entityIDs
		f.curArchSize = a.arch.size
	} else {
		f.curArchSize = 0
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
	f.curIdx++
	if f.curIdx < f.curArchSize {
		return true
	}
	f.curMatchIdx++
	if f.curMatchIdx >= len(f.matchingArches) {
		return false
	}
	a := f.matchingArches[f.curMatchIdx]
	f.curBase = a.pointers[0]
	f.curEntityIDs = a.arch.entityIDs
	f.curArchSize = a.arch.size
	f.curIdx = 0
	return true
}

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
//
// Returns:
//   - The current Entity.
func (f *Filter[T]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns a pointer to the component of type `T` for the current entity
// in the iteration. This should only be called after `Next()` has returned true.
//
// Returns:
//   - A pointer to the component data (*T).
func (f *Filter[T]) Get() *T {
	return (*T)(unsafe.Add(f.curBase, uintptr(f.curIdx)*f.compSize))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory, making it highly
// performant for clearing large groups of entities.
//
// After this operation, the filter will be empty.
func (f *Filter[T]) RemoveEntities() {
	f.world.mu.Lock()
	defer f.world.mu.Unlock()
	if f.IsStale() {
		f.updateMatching([]uint8{f.compID})
	}
	for _, fa := range f.matchingArches {
		for i := 0; i < fa.arch.size; i++ {
			ent := fa.arch.entityIDs[i]
			meta := &f.world.entities.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.entities.freeIDs = append(f.world.entities.freeIDs, ent.ID)
		}
		fa.arch.size = 0
	}
	f.world.mutationVersion.Add(1)
	f.doReset()
}

// Entities returns a slice containing all entities that match the filter's
// query. This method retrieves a cached list of entities, which is updated only
// when the filter is reset or detects that the world's archetypes have changed.
//
// The returned slice is owned by the filter and will be invalidated if the
// world is modified (e.g., by creating or deleting entities) or when the filter
// is reset. If you need to retain the list of entities for long-term use, make
// a copy of the slice.
//
// Returns:
//   - A slice of matching entities.
func (f *Filter[T]) Entities() []Entity {
	return f.queryCache.Entities([]uint8{f.compID})
}

// Query returns a new iterator snapshot for the filter, optimized for allocation-free iteration.
// Assume no world mutations during the Query's lifetime.
type Query[T any] struct {
	matchingArches []*filterArch
	curBase        unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSize       uintptr
	curArchSize    int
	compID         uint8
}

// Query creates a new Query iterator from the Filter.
func (f *Filter[T]) Query() Query[T] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching([]uint8{f.compID})
	}
	q := Query[T]{
		matchingArches: f.matchingArches, // share, no alloc
		compID:         f.compID,
		compSize:       f.compSize,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		q.curBase = a.pointers[0]
		q.curEntityIDs = a.arch.entityIDs
		q.curArchSize = a.arch.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query[T]) Next() bool {
	q.curIdx++
	if q.curIdx < q.curArchSize {
		return true
	}
	q.curMatchIdx++
	if q.curMatchIdx >= len(q.matchingArches) {
		return false
	}
	a := q.matchingArches[q.curMatchIdx]
	q.curBase = a.pointers[0]
	q.curEntityIDs = a.arch.entityIDs
	q.curArchSize = a.arch.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query[T]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns a pointer to the component T for the current entity.
func (q *Query[T]) Get() *T {
	return (*T)(unsafe.Add(q.curBase, uintptr(q.curIdx)*q.compSize))
}

// Filter0 provides a fast, cache-friendly iterator over all entities that have a
// no components.
type Filter0 struct {
	curEntityIDs []Entity
	queryCache
	curMatchIdx int // index into matchingArches
	curIdx      int // index into the current archetype's entity/component array
	curArchSize int
}

// NewFilter0 creates a new `Filter` that iterates over all entities possessing
// no components.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter0`.
func NewFilter0(w *World) *Filter0 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var m bitmask256
	f := &Filter0{
		queryCache:  newQueryCache(w, m),
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.updateMatching(nil)
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component type, equivalent to calling `NewFilter`.
func (f *Filter0) New(w *World) *Filter0 {
	return NewFilter0(w)
}

// Reset rewinds the filter's iterator to the beginning. It must be called
// before re-iterating over a filter (e.g., in a loop). The filter will also
// automatically detect if new archetypes have been created since the last
// iteration and update its internal list accordingly.
func (f *Filter0) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter0) doReset() {
	if f.isArchetypeStale() {
		f.updateMatching(nil)
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curEntityIDs = a.arch.entityIDs
		f.curArchSize = a.arch.size
	} else {
		f.curArchSize = 0
	}
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete. This method must
// be called before accessing the entity or its components.
//
// Example:
//
//	query := teishoku.NewFilter0(world)
//	for query.Next() {
//	    // ... process entity
//	}
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter0) Next() bool {
	f.curIdx++
	if f.curIdx < f.curArchSize {
		return true
	}
	f.curMatchIdx++
	if f.curMatchIdx >= len(f.matchingArches) {
		return false
	}
	a := f.matchingArches[f.curMatchIdx]
	f.curEntityIDs = a.arch.entityIDs
	f.curArchSize = a.arch.size
	f.curIdx = 0
	return true
}

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
//
// Returns:
//   - The current Entity.
func (f *Filter0) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory, making it highly
// performant for clearing large groups of entities.
//
// After this operation, the filter will be empty.
func (f *Filter0) RemoveEntities() {
	f.world.mu.Lock()
	defer f.world.mu.Unlock()
	if f.IsStale() {
		f.updateMatching(nil)
	}
	for _, fa := range f.matchingArches {
		for i := 0; i < fa.arch.size; i++ {
			ent := fa.arch.entityIDs[i]
			meta := &f.world.entities.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.entities.freeIDs = append(f.world.entities.freeIDs, ent.ID)
		}
		fa.arch.size = 0
	}
	f.world.mutationVersion.Add(1)
	f.doReset()
}

// Entities returns a slice containing all entities that match the filter's
// query. This method retrieves a cached list of entities, which is updated only
// when the filter is reset or detects that the world's archetypes have changed.
//
// The returned slice is owned by the filter and will be invalidated if the
// world is modified (e.g., by creating or deleting entities) or when the filter
// is reset. If you need to retain the list of entities for long-term use, make
// a copy of the slice.
//
// Returns:
//   - A slice of matching entities.
func (f *Filter0) Entities() []Entity {
	return f.queryCache.Entities(nil)
}

// Query0 is an allocation-free iterator snapshot for Filter0.
type Query0 struct {
	matchingArches []*filterArch
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	curArchSize    int
}

// Query returns a new Query0 iterator from the Filter0.
func (f *Filter0) Query() Query0 {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching(nil)
	}
	q := Query0{
		matchingArches: f.matchingArches,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		q.curEntityIDs = a.arch.entityIDs
		q.curArchSize = a.arch.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query0) Next() bool {
	q.curIdx++
	if q.curIdx < q.curArchSize {
		return true
	}
	q.curMatchIdx++
	if q.curMatchIdx >= len(q.matchingArches) {
		return false
	}
	a := q.matchingArches[q.curMatchIdx]
	q.curEntityIDs = a.arch.entityIDs
	q.curArchSize = a.arch.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query0) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}
