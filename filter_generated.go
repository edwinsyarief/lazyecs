package teishoku

import (
	"reflect"
	"unsafe"
)

// Filter2 provides a fast, cache-friendly iterator over all entities that
// have the 2 components: T1, T2.
type Filter2[T1 any, T2 any] struct {
	queryCache
	curBases     [2]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [2]uintptr
	curArchSize  int
	ids          [2]uint8
}

// NewFilter2 creates a new `Filter` that iterates over all entities
// possessing at least the 2 components: T1, T2.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter2`.
func NewFilter2[T1 any, T2 any](w *World) *Filter2[T1, T2] {
	w.mu.RLock()
	defer w.mu.RUnlock()
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())

	if id2 == id1 {
		panic("ecs: duplicate component types in Filter2")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)

	f := &Filter2[T1, T2]{
		queryCache:  newQueryCache(w, m),
		ids:         [2]uint8{id1, id2},
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSizes[0] = w.components.compIDToSize[id1]
	f.compSizes[1] = w.components.compIDToSize[id2]

	f.updateMatching()
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter2`.
func (f *Filter2[T1, T2]) New(w *World) *Filter2[T1, T2] {
	return NewFilter2[T1, T2](w)
}

// Reset rewinds the filter's iterator to the beginning. It must be called
// before re-iterating over a filter (e.g., in a loop). The filter will also
// automatically detect if new archetypes have been created since the last
// iteration and update its internal list accordingly.
func (f *Filter2[T1, T2]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter2[T1, T2]) doReset() {
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curEntityIDs = a.entityIDs
		f.curArchSize = a.size
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
//	query := teishoku.NewFilter2[Position, Velocity](world)
//	for query.Next() {
//	    // ... process entity
//	}
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter2[T1, T2]) Next() bool {
	f.curIdx++
	if f.curIdx < f.curArchSize {
		return true
	}
	f.curMatchIdx++
	if f.curMatchIdx >= len(f.matchingArches) {
		return false
	}
	a := f.matchingArches[f.curMatchIdx]
	f.curBases[0] = a.compPointers[f.ids[0]]
	f.curBases[1] = a.compPointers[f.ids[1]]
	f.curEntityIDs = a.entityIDs
	f.curArchSize = a.size
	f.curIdx = 0
	return true
}

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
//
// Returns:
//   - The current Entity.
func (f *Filter2[T1, T2]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the 2 components (T1, T2) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2).
func (f *Filter2[T1, T2]) Get() (*T1, *T2) {
	return (*T1)(unsafe.Add(f.curBases[0], uintptr(f.curIdx)*f.compSizes[0])),
		(*T2)(unsafe.Add(f.curBases[1], uintptr(f.curIdx)*f.compSizes[1]))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory, making it highly
// performant for clearing large groups of entities.
//
// After this operation, the filter will be empty.
func (f *Filter2[T1, T2]) RemoveEntities() {
	f.world.mu.Lock()
	defer f.world.mu.Unlock()
	if f.IsStale() {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
			ent := a.entityIDs[i]
			meta := &f.world.entities.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.entities.freeIDs = append(f.world.entities.freeIDs, ent.ID)
		}
		a.size = 0
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
func (f *Filter2[T1, T2]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Query2 is an allocation-free iterator snapshot for Filter2.
type Query2[T1 any, T2 any] struct {
	matchingArches []*archetype
	curBases       [2]unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSizes      [2]uintptr
	curArchSize    int
	ids            [2]uint8
}

// Query returns a new Query2 iterator from the Filter2.
func (f *Filter2[T1, T2]) Query() Query2[T1, T2] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	q := Query2[T1, T2]{
		matchingArches: f.matchingArches,
		ids:            f.ids,
		compSizes:      f.compSizes,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		q.curBases[0] = a.compPointers[q.ids[0]]
		q.curBases[1] = a.compPointers[q.ids[1]]
		q.curEntityIDs = a.entityIDs
		q.curArchSize = a.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query2[T1, T2]) Next() bool {
	q.curIdx++
	if q.curIdx < q.curArchSize {
		return true
	}
	q.curMatchIdx++
	if q.curMatchIdx >= len(q.matchingArches) {
		return false
	}
	a := q.matchingArches[q.curMatchIdx]
	q.curBases[0] = a.compPointers[q.ids[0]]
	q.curBases[1] = a.compPointers[q.ids[1]]
	q.curEntityIDs = a.entityIDs
	q.curArchSize = a.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query2[T1, T2]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns pointers to T1, T2 for the current entity.
func (q *Query2[T1, T2]) Get() (*T1, *T2) {
	return (*T1)(unsafe.Add(q.curBases[0], uintptr(q.curIdx)*q.compSizes[0])),
		(*T2)(unsafe.Add(q.curBases[1], uintptr(q.curIdx)*q.compSizes[1]))
}

// Filter3 provides a fast, cache-friendly iterator over all entities that
// have the 3 components: T1, T2, T3.
type Filter3[T1 any, T2 any, T3 any] struct {
	queryCache
	curBases     [3]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [3]uintptr
	curArchSize  int
	ids          [3]uint8
}

// NewFilter3 creates a new `Filter` that iterates over all entities
// possessing at least the 3 components: T1, T2, T3.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter3`.
func NewFilter3[T1 any, T2 any, T3 any](w *World) *Filter3[T1, T2, T3] {
	w.mu.RLock()
	defer w.mu.RUnlock()
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in Filter3")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)

	f := &Filter3[T1, T2, T3]{
		queryCache:  newQueryCache(w, m),
		ids:         [3]uint8{id1, id2, id3},
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSizes[0] = w.components.compIDToSize[id1]
	f.compSizes[1] = w.components.compIDToSize[id2]
	f.compSizes[2] = w.components.compIDToSize[id3]

	f.updateMatching()
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter3`.
func (f *Filter3[T1, T2, T3]) New(w *World) *Filter3[T1, T2, T3] {
	return NewFilter3[T1, T2, T3](w)
}

// Reset rewinds the filter's iterator to the beginning. It must be called
// before re-iterating over a filter (e.g., in a loop).
func (f *Filter3[T1, T2, T3]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter3[T1, T2, T3]) doReset() {
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curEntityIDs = a.entityIDs
		f.curArchSize = a.size
	} else {
		f.curArchSize = 0
	}
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete. This method must
// be called before accessing the entity or its components.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter3[T1, T2, T3]) Next() bool {
	f.curIdx++
	if f.curIdx < f.curArchSize {
		return true
	}
	f.curMatchIdx++
	if f.curMatchIdx >= len(f.matchingArches) {
		return false
	}
	a := f.matchingArches[f.curMatchIdx]
	f.curBases[0] = a.compPointers[f.ids[0]]
	f.curBases[1] = a.compPointers[f.ids[1]]
	f.curBases[2] = a.compPointers[f.ids[2]]
	f.curEntityIDs = a.entityIDs
	f.curArchSize = a.size
	f.curIdx = 0
	return true
}

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
//
// Returns:
//   - The current Entity.
func (f *Filter3[T1, T2, T3]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the 3 components (T1, T2, T3) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3).
func (f *Filter3[T1, T2, T3]) Get() (*T1, *T2, *T3) {
	return (*T1)(unsafe.Add(f.curBases[0], uintptr(f.curIdx)*f.compSizes[0])),
		(*T2)(unsafe.Add(f.curBases[1], uintptr(f.curIdx)*f.compSizes[1])),
		(*T3)(unsafe.Add(f.curBases[2], uintptr(f.curIdx)*f.compSizes[2]))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter3[T1, T2, T3]) RemoveEntities() {
	f.world.mu.Lock()
	defer f.world.mu.Unlock()
	if f.IsStale() {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
			ent := a.entityIDs[i]
			meta := &f.world.entities.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.entities.freeIDs = append(f.world.entities.freeIDs, ent.ID)
		}
		a.size = 0
	}
	f.world.mutationVersion.Add(1)
	f.doReset()
}

// Entities returns all entities that match the filter.
func (f *Filter3[T1, T2, T3]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Query3 is an allocation-free iterator snapshot for Filter3.
type Query3[T1 any, T2 any, T3 any] struct {
	matchingArches []*archetype
	curBases       [3]unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSizes      [3]uintptr
	curArchSize    int
	ids            [3]uint8
}

// Query returns a new Query3 iterator from the Filter3.
func (f *Filter3[T1, T2, T3]) Query() Query3[T1, T2, T3] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	q := Query3[T1, T2, T3]{
		matchingArches: f.matchingArches,
		ids:            f.ids,
		compSizes:      f.compSizes,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		q.curBases[0] = a.compPointers[q.ids[0]]
		q.curBases[1] = a.compPointers[q.ids[1]]
		q.curBases[2] = a.compPointers[q.ids[2]]
		q.curEntityIDs = a.entityIDs
		q.curArchSize = a.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query3[T1, T2, T3]) Next() bool {
	q.curIdx++
	if q.curIdx < q.curArchSize {
		return true
	}
	q.curMatchIdx++
	if q.curMatchIdx >= len(q.matchingArches) {
		return false
	}
	a := q.matchingArches[q.curMatchIdx]
	q.curBases[0] = a.compPointers[q.ids[0]]
	q.curBases[1] = a.compPointers[q.ids[1]]
	q.curBases[2] = a.compPointers[q.ids[2]]
	q.curEntityIDs = a.entityIDs
	q.curArchSize = a.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query3[T1, T2, T3]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns pointers to T1, T2, T3 for the current entity.
func (q *Query3[T1, T2, T3]) Get() (*T1, *T2, *T3) {
	return (*T1)(unsafe.Add(q.curBases[0], uintptr(q.curIdx)*q.compSizes[0])),
		(*T2)(unsafe.Add(q.curBases[1], uintptr(q.curIdx)*q.compSizes[1])),
		(*T3)(unsafe.Add(q.curBases[2], uintptr(q.curIdx)*q.compSizes[2]))
}

// Filter4 provides a fast, cache-friendly iterator over all entities that
// have the 4 components: T1, T2, T3, T4.
type Filter4[T1 any, T2 any, T3 any, T4 any] struct {
	queryCache
	curBases     [4]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [4]uintptr
	curArchSize  int
	ids          [4]uint8
}

// NewFilter4 creates a new `Filter` that iterates over all entities
// possessing at least the 4 components: T1, T2, T3, T4.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter4`.
func NewFilter4[T1 any, T2 any, T3 any, T4 any](w *World) *Filter4[T1, T2, T3, T4] {
	w.mu.RLock()
	defer w.mu.RUnlock()
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 {
		panic("ecs: duplicate component types in Filter4")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)
	m.set(id4)

	f := &Filter4[T1, T2, T3, T4]{
		queryCache:  newQueryCache(w, m),
		ids:         [4]uint8{id1, id2, id3, id4},
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSizes[0] = w.components.compIDToSize[id1]
	f.compSizes[1] = w.components.compIDToSize[id2]
	f.compSizes[2] = w.components.compIDToSize[id3]
	f.compSizes[3] = w.components.compIDToSize[id4]

	f.updateMatching()
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter4`.
func (f *Filter4[T1, T2, T3, T4]) New(w *World) *Filter4[T1, T2, T3, T4] {
	return NewFilter4[T1, T2, T3, T4](w)
}

// Reset rewinds the filter's iterator to the beginning. It must be called
// before re-iterating over a filter (e.g., in a loop).
func (f *Filter4[T1, T2, T3, T4]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter4[T1, T2, T3, T4]) doReset() {
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		f.curEntityIDs = a.entityIDs
		f.curArchSize = a.size
	} else {
		f.curArchSize = 0
	}
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete. This method must
// be called before accessing the entity or its components.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter4[T1, T2, T3, T4]) Next() bool {
	f.curIdx++
	if f.curIdx < f.curArchSize {
		return true
	}
	f.curMatchIdx++
	if f.curMatchIdx >= len(f.matchingArches) {
		return false
	}
	a := f.matchingArches[f.curMatchIdx]
	f.curBases[0] = a.compPointers[f.ids[0]]
	f.curBases[1] = a.compPointers[f.ids[1]]
	f.curBases[2] = a.compPointers[f.ids[2]]
	f.curBases[3] = a.compPointers[f.ids[3]]
	f.curEntityIDs = a.entityIDs
	f.curArchSize = a.size
	f.curIdx = 0
	return true
}

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
//
// Returns:
//   - The current Entity.
func (f *Filter4[T1, T2, T3, T4]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the 4 components (T1, T2, T3, T4) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4).
func (f *Filter4[T1, T2, T3, T4]) Get() (*T1, *T2, *T3, *T4) {
	return (*T1)(unsafe.Add(f.curBases[0], uintptr(f.curIdx)*f.compSizes[0])),
		(*T2)(unsafe.Add(f.curBases[1], uintptr(f.curIdx)*f.compSizes[1])),
		(*T3)(unsafe.Add(f.curBases[2], uintptr(f.curIdx)*f.compSizes[2])),
		(*T4)(unsafe.Add(f.curBases[3], uintptr(f.curIdx)*f.compSizes[3]))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter4[T1, T2, T3, T4]) RemoveEntities() {
	f.world.mu.Lock()
	defer f.world.mu.Unlock()
	if f.IsStale() {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
			ent := a.entityIDs[i]
			meta := &f.world.entities.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.entities.freeIDs = append(f.world.entities.freeIDs, ent.ID)
		}
		a.size = 0
	}
	f.world.mutationVersion.Add(1)
	f.doReset()
}

// Entities returns all entities that match the filter.
func (f *Filter4[T1, T2, T3, T4]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Query4 is an allocation-free iterator snapshot for Filter4.
type Query4[T1 any, T2 any, T3 any, T4 any] struct {
	matchingArches []*archetype
	curBases       [4]unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSizes      [4]uintptr
	curArchSize    int
	ids            [4]uint8
}

// Query returns a new Query4 iterator from the Filter4.
func (f *Filter4[T1, T2, T3, T4]) Query() Query4[T1, T2, T3, T4] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	q := Query4[T1, T2, T3, T4]{
		matchingArches: f.matchingArches,
		ids:            f.ids,
		compSizes:      f.compSizes,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		q.curBases[0] = a.compPointers[q.ids[0]]
		q.curBases[1] = a.compPointers[q.ids[1]]
		q.curBases[2] = a.compPointers[q.ids[2]]
		q.curBases[3] = a.compPointers[q.ids[3]]
		q.curEntityIDs = a.entityIDs
		q.curArchSize = a.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query4[T1, T2, T3, T4]) Next() bool {
	q.curIdx++
	if q.curIdx < q.curArchSize {
		return true
	}
	q.curMatchIdx++
	if q.curMatchIdx >= len(q.matchingArches) {
		return false
	}
	a := q.matchingArches[q.curMatchIdx]
	q.curBases[0] = a.compPointers[q.ids[0]]
	q.curBases[1] = a.compPointers[q.ids[1]]
	q.curBases[2] = a.compPointers[q.ids[2]]
	q.curBases[3] = a.compPointers[q.ids[3]]
	q.curEntityIDs = a.entityIDs
	q.curArchSize = a.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query4[T1, T2, T3, T4]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns pointers to T1, T2, T3, T4 for the current entity.
func (q *Query4[T1, T2, T3, T4]) Get() (*T1, *T2, *T3, *T4) {
	return (*T1)(unsafe.Add(q.curBases[0], uintptr(q.curIdx)*q.compSizes[0])),
		(*T2)(unsafe.Add(q.curBases[1], uintptr(q.curIdx)*q.compSizes[1])),
		(*T3)(unsafe.Add(q.curBases[2], uintptr(q.curIdx)*q.compSizes[2])),
		(*T4)(unsafe.Add(q.curBases[3], uintptr(q.curIdx)*q.compSizes[3]))
}

// Filter5 provides a fast, cache-friendly iterator over all entities that
// have the 5 components: T1, T2, T3, T4, T5.
type Filter5[T1 any, T2 any, T3 any, T4 any, T5 any] struct {
	queryCache
	curBases     [5]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [5]uintptr
	curArchSize  int
	ids          [5]uint8
}

// NewFilter5 creates a new `Filter` that iterates over all entities
// possessing at least the 5 components: T1, T2, T3, T4, T5.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter5`.
func NewFilter5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World) *Filter5[T1, T2, T3, T4, T5] {
	w.mu.RLock()
	defer w.mu.RUnlock()
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())
	id5 := w.getCompTypeID(reflect.TypeFor[T5]())

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 {
		panic("ecs: duplicate component types in Filter5")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)
	m.set(id4)
	m.set(id5)

	f := &Filter5[T1, T2, T3, T4, T5]{
		queryCache:  newQueryCache(w, m),
		ids:         [5]uint8{id1, id2, id3, id4, id5},
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSizes[0] = w.components.compIDToSize[id1]
	f.compSizes[1] = w.components.compIDToSize[id2]
	f.compSizes[2] = w.components.compIDToSize[id3]
	f.compSizes[3] = w.components.compIDToSize[id4]
	f.compSizes[4] = w.components.compIDToSize[id5]

	f.updateMatching()
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter5`.
func (f *Filter5[T1, T2, T3, T4, T5]) New(w *World) *Filter5[T1, T2, T3, T4, T5] {
	return NewFilter5[T1, T2, T3, T4, T5](w)
}

// Reset rewinds the filter's iterator to the beginning. It must be called
// before re-iterating over a filter (e.g., in a loop).
func (f *Filter5[T1, T2, T3, T4, T5]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter5[T1, T2, T3, T4, T5]) doReset() {
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		f.curBases[4] = a.compPointers[f.ids[4]]
		f.curEntityIDs = a.entityIDs
		f.curArchSize = a.size
	} else {
		f.curArchSize = 0
	}
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete. This method must
// be called before accessing the entity or its components.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter5[T1, T2, T3, T4, T5]) Next() bool {
	f.curIdx++
	if f.curIdx < f.curArchSize {
		return true
	}
	f.curMatchIdx++
	if f.curMatchIdx >= len(f.matchingArches) {
		return false
	}
	a := f.matchingArches[f.curMatchIdx]
	f.curBases[0] = a.compPointers[f.ids[0]]
	f.curBases[1] = a.compPointers[f.ids[1]]
	f.curBases[2] = a.compPointers[f.ids[2]]
	f.curBases[3] = a.compPointers[f.ids[3]]
	f.curBases[4] = a.compPointers[f.ids[4]]
	f.curEntityIDs = a.entityIDs
	f.curArchSize = a.size
	f.curIdx = 0
	return true
}

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
//
// Returns:
//   - The current Entity.
func (f *Filter5[T1, T2, T3, T4, T5]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the 5 components (T1, T2, T3, T4, T5) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5).
func (f *Filter5[T1, T2, T3, T4, T5]) Get() (*T1, *T2, *T3, *T4, *T5) {
	return (*T1)(unsafe.Add(f.curBases[0], uintptr(f.curIdx)*f.compSizes[0])),
		(*T2)(unsafe.Add(f.curBases[1], uintptr(f.curIdx)*f.compSizes[1])),
		(*T3)(unsafe.Add(f.curBases[2], uintptr(f.curIdx)*f.compSizes[2])),
		(*T4)(unsafe.Add(f.curBases[3], uintptr(f.curIdx)*f.compSizes[3])),
		(*T5)(unsafe.Add(f.curBases[4], uintptr(f.curIdx)*f.compSizes[4]))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter5[T1, T2, T3, T4, T5]) RemoveEntities() {
	f.world.mu.Lock()
	defer f.world.mu.Unlock()
	if f.IsStale() {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
			ent := a.entityIDs[i]
			meta := &f.world.entities.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.entities.freeIDs = append(f.world.entities.freeIDs, ent.ID)
		}
		a.size = 0
	}
	f.world.mutationVersion.Add(1)
	f.doReset()
}

// Entities returns all entities that match the filter.
func (f *Filter5[T1, T2, T3, T4, T5]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Query5 is an allocation-free iterator snapshot for Filter5.
type Query5[T1 any, T2 any, T3 any, T4 any, T5 any] struct {
	matchingArches []*archetype
	curBases       [5]unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSizes      [5]uintptr
	curArchSize    int
	ids            [5]uint8
}

// Query returns a new Query5 iterator from the Filter5.
func (f *Filter5[T1, T2, T3, T4, T5]) Query() Query5[T1, T2, T3, T4, T5] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	q := Query5[T1, T2, T3, T4, T5]{
		matchingArches: f.matchingArches,
		ids:            f.ids,
		compSizes:      f.compSizes,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		q.curBases[0] = a.compPointers[q.ids[0]]
		q.curBases[1] = a.compPointers[q.ids[1]]
		q.curBases[2] = a.compPointers[q.ids[2]]
		q.curBases[3] = a.compPointers[q.ids[3]]
		q.curBases[4] = a.compPointers[q.ids[4]]
		q.curEntityIDs = a.entityIDs
		q.curArchSize = a.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query5[T1, T2, T3, T4, T5]) Next() bool {
	q.curIdx++
	if q.curIdx < q.curArchSize {
		return true
	}
	q.curMatchIdx++
	if q.curMatchIdx >= len(q.matchingArches) {
		return false
	}
	a := q.matchingArches[q.curMatchIdx]
	q.curBases[0] = a.compPointers[q.ids[0]]
	q.curBases[1] = a.compPointers[q.ids[1]]
	q.curBases[2] = a.compPointers[q.ids[2]]
	q.curBases[3] = a.compPointers[q.ids[3]]
	q.curBases[4] = a.compPointers[q.ids[4]]
	q.curEntityIDs = a.entityIDs
	q.curArchSize = a.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query5[T1, T2, T3, T4, T5]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns pointers to T1, T2, T3, T4, T5 for the current entity.
func (q *Query5[T1, T2, T3, T4, T5]) Get() (*T1, *T2, *T3, *T4, *T5) {
	return (*T1)(unsafe.Add(q.curBases[0], uintptr(q.curIdx)*q.compSizes[0])),
		(*T2)(unsafe.Add(q.curBases[1], uintptr(q.curIdx)*q.compSizes[1])),
		(*T3)(unsafe.Add(q.curBases[2], uintptr(q.curIdx)*q.compSizes[2])),
		(*T4)(unsafe.Add(q.curBases[3], uintptr(q.curIdx)*q.compSizes[3])),
		(*T5)(unsafe.Add(q.curBases[4], uintptr(q.curIdx)*q.compSizes[4]))
}

// Filter6 provides a fast, cache-friendly iterator over all entities that
// have the 6 components: T1, T2, T3, T4, T5, T6.
type Filter6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any] struct {
	queryCache
	curBases     [6]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [6]uintptr
	curArchSize  int
	ids          [6]uint8
}

// NewFilter6 creates a new `Filter` that iterates over all entities
// possessing at least the 6 components: T1, T2, T3, T4, T5, T6.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter6`.
func NewFilter6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any](w *World) *Filter6[T1, T2, T3, T4, T5, T6] {
	w.mu.RLock()
	defer w.mu.RUnlock()
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())
	id5 := w.getCompTypeID(reflect.TypeFor[T5]())
	id6 := w.getCompTypeID(reflect.TypeFor[T6]())

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 {
		panic("ecs: duplicate component types in Filter6")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)
	m.set(id4)
	m.set(id5)
	m.set(id6)

	f := &Filter6[T1, T2, T3, T4, T5, T6]{
		queryCache:  newQueryCache(w, m),
		ids:         [6]uint8{id1, id2, id3, id4, id5, id6},
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSizes[0] = w.components.compIDToSize[id1]
	f.compSizes[1] = w.components.compIDToSize[id2]
	f.compSizes[2] = w.components.compIDToSize[id3]
	f.compSizes[3] = w.components.compIDToSize[id4]
	f.compSizes[4] = w.components.compIDToSize[id5]
	f.compSizes[5] = w.components.compIDToSize[id6]

	f.updateMatching()
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter6`.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) New(w *World) *Filter6[T1, T2, T3, T4, T5, T6] {
	return NewFilter6[T1, T2, T3, T4, T5, T6](w)
}

// Reset rewinds the filter's iterator to the beginning. It must be called
// before re-iterating over a filter (e.g., in a loop).
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter6[T1, T2, T3, T4, T5, T6]) doReset() {
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		f.curBases[4] = a.compPointers[f.ids[4]]
		f.curBases[5] = a.compPointers[f.ids[5]]
		f.curEntityIDs = a.entityIDs
		f.curArchSize = a.size
	} else {
		f.curArchSize = 0
	}
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete. This method must
// be called before accessing the entity or its components.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Next() bool {
	f.curIdx++
	if f.curIdx < f.curArchSize {
		return true
	}
	f.curMatchIdx++
	if f.curMatchIdx >= len(f.matchingArches) {
		return false
	}
	a := f.matchingArches[f.curMatchIdx]
	f.curBases[0] = a.compPointers[f.ids[0]]
	f.curBases[1] = a.compPointers[f.ids[1]]
	f.curBases[2] = a.compPointers[f.ids[2]]
	f.curBases[3] = a.compPointers[f.ids[3]]
	f.curBases[4] = a.compPointers[f.ids[4]]
	f.curBases[5] = a.compPointers[f.ids[5]]
	f.curEntityIDs = a.entityIDs
	f.curArchSize = a.size
	f.curIdx = 0
	return true
}

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
//
// Returns:
//   - The current Entity.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the 6 components (T1, T2, T3, T4, T5, T6) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6).
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Get() (*T1, *T2, *T3, *T4, *T5, *T6) {
	return (*T1)(unsafe.Add(f.curBases[0], uintptr(f.curIdx)*f.compSizes[0])),
		(*T2)(unsafe.Add(f.curBases[1], uintptr(f.curIdx)*f.compSizes[1])),
		(*T3)(unsafe.Add(f.curBases[2], uintptr(f.curIdx)*f.compSizes[2])),
		(*T4)(unsafe.Add(f.curBases[3], uintptr(f.curIdx)*f.compSizes[3])),
		(*T5)(unsafe.Add(f.curBases[4], uintptr(f.curIdx)*f.compSizes[4])),
		(*T6)(unsafe.Add(f.curBases[5], uintptr(f.curIdx)*f.compSizes[5]))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) RemoveEntities() {
	f.world.mu.Lock()
	defer f.world.mu.Unlock()
	if f.IsStale() {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
			ent := a.entityIDs[i]
			meta := &f.world.entities.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.entities.freeIDs = append(f.world.entities.freeIDs, ent.ID)
		}
		a.size = 0
	}
	f.world.mutationVersion.Add(1)
	f.doReset()
}

// Entities returns all entities that match the filter.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Query6 is an allocation-free iterator snapshot for Filter6.
type Query6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any] struct {
	matchingArches []*archetype
	curBases       [6]unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSizes      [6]uintptr
	curArchSize    int
	ids            [6]uint8
}

// Query returns a new Query6 iterator from the Filter6.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Query() Query6[T1, T2, T3, T4, T5, T6] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	q := Query6[T1, T2, T3, T4, T5, T6]{
		matchingArches: f.matchingArches,
		ids:            f.ids,
		compSizes:      f.compSizes,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		q.curBases[0] = a.compPointers[q.ids[0]]
		q.curBases[1] = a.compPointers[q.ids[1]]
		q.curBases[2] = a.compPointers[q.ids[2]]
		q.curBases[3] = a.compPointers[q.ids[3]]
		q.curBases[4] = a.compPointers[q.ids[4]]
		q.curBases[5] = a.compPointers[q.ids[5]]
		q.curEntityIDs = a.entityIDs
		q.curArchSize = a.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query6[T1, T2, T3, T4, T5, T6]) Next() bool {
	q.curIdx++
	if q.curIdx < q.curArchSize {
		return true
	}
	q.curMatchIdx++
	if q.curMatchIdx >= len(q.matchingArches) {
		return false
	}
	a := q.matchingArches[q.curMatchIdx]
	q.curBases[0] = a.compPointers[q.ids[0]]
	q.curBases[1] = a.compPointers[q.ids[1]]
	q.curBases[2] = a.compPointers[q.ids[2]]
	q.curBases[3] = a.compPointers[q.ids[3]]
	q.curBases[4] = a.compPointers[q.ids[4]]
	q.curBases[5] = a.compPointers[q.ids[5]]
	q.curEntityIDs = a.entityIDs
	q.curArchSize = a.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query6[T1, T2, T3, T4, T5, T6]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns pointers to T1, T2, T3, T4, T5, T6 for the current entity.
func (q *Query6[T1, T2, T3, T4, T5, T6]) Get() (*T1, *T2, *T3, *T4, *T5, *T6) {
	return (*T1)(unsafe.Add(q.curBases[0], uintptr(q.curIdx)*q.compSizes[0])),
		(*T2)(unsafe.Add(q.curBases[1], uintptr(q.curIdx)*q.compSizes[1])),
		(*T3)(unsafe.Add(q.curBases[2], uintptr(q.curIdx)*q.compSizes[2])),
		(*T4)(unsafe.Add(q.curBases[3], uintptr(q.curIdx)*q.compSizes[3])),
		(*T5)(unsafe.Add(q.curBases[4], uintptr(q.curIdx)*q.compSizes[4])),
		(*T6)(unsafe.Add(q.curBases[5], uintptr(q.curIdx)*q.compSizes[5]))
}