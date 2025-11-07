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
		ids:         [2]uint8{ id1, id2 },
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

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter2[T1, T2]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter2[T1, T2]) doReset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
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
// entities and recycling their IDs without moving any memory.
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

// Entities returns all entities that match the filter.
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
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		
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
		ids:         [3]uint8{ id1, id2, id3 },
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

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter3[T1, T2, T3]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter3[T1, T2, T3]) doReset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
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
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		
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
		ids:         [4]uint8{ id1, id2, id3, id4 },
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

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter4[T1, T2, T3, T4]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter4[T1, T2, T3, T4]) doReset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
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
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		
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
		ids:         [5]uint8{ id1, id2, id3, id4, id5 },
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

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter5[T1, T2, T3, T4, T5]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter5[T1, T2, T3, T4, T5]) doReset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
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
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		f.curBases[4] = a.compPointers[f.ids[4]]
		
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
		ids:         [6]uint8{ id1, id2, id3, id4, id5, id6 },
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

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter6[T1, T2, T3, T4, T5, T6]) doReset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
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
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		f.curBases[4] = a.compPointers[f.ids[4]]
		f.curBases[5] = a.compPointers[f.ids[5]]
		
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

// Filter7 provides a fast, cache-friendly iterator over all entities that
// have the 7 components: T1, T2, T3, T4, T5, T6, T7.
type Filter7[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any] struct {
	queryCache
	curBases     [7]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [7]uintptr
	curArchSize  int
	ids          [7]uint8
}

// NewFilter7 creates a new `Filter` that iterates over all entities
// possessing at least the 7 components: T1, T2, T3, T4, T5, T6, T7.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter7`.
func NewFilter7[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any](w *World) *Filter7[T1, T2, T3, T4, T5, T6, T7] {
	w.mu.RLock()
	defer w.mu.RUnlock()
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())
	id5 := w.getCompTypeID(reflect.TypeFor[T5]())
	id6 := w.getCompTypeID(reflect.TypeFor[T6]())
	id7 := w.getCompTypeID(reflect.TypeFor[T7]())
	
	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 {
		panic("ecs: duplicate component types in Filter7")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)
	m.set(id4)
	m.set(id5)
	m.set(id6)
	m.set(id7)
	
	f := &Filter7[T1, T2, T3, T4, T5, T6, T7]{
		queryCache:  newQueryCache(w, m),
		ids:         [7]uint8{ id1, id2, id3, id4, id5, id6, id7 },
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSizes[0] = w.components.compIDToSize[id1]
	f.compSizes[1] = w.components.compIDToSize[id2]
	f.compSizes[2] = w.components.compIDToSize[id3]
	f.compSizes[3] = w.components.compIDToSize[id4]
	f.compSizes[4] = w.components.compIDToSize[id5]
	f.compSizes[5] = w.components.compIDToSize[id6]
	f.compSizes[6] = w.components.compIDToSize[id7]
	
	f.updateMatching()
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter7`.
func (f *Filter7[T1, T2, T3, T4, T5, T6, T7]) New(w *World) *Filter7[T1, T2, T3, T4, T5, T6, T7] {
	return NewFilter7[T1, T2, T3, T4, T5, T6, T7](w)
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter7[T1, T2, T3, T4, T5, T6, T7]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter7[T1, T2, T3, T4, T5, T6, T7]) doReset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
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
		f.curBases[6] = a.compPointers[f.ids[6]]
		
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
func (f *Filter7[T1, T2, T3, T4, T5, T6, T7]) Next() bool {
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
	f.curBases[6] = a.compPointers[f.ids[6]]
	
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
func (f *Filter7[T1, T2, T3, T4, T5, T6, T7]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the 7 components (T1, T2, T3, T4, T5, T6, T7) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6, *T7).
func (f *Filter7[T1, T2, T3, T4, T5, T6, T7]) Get() (*T1, *T2, *T3, *T4, *T5, *T6, *T7) {
	return (*T1)(unsafe.Add(f.curBases[0], uintptr(f.curIdx)*f.compSizes[0])),
		(*T2)(unsafe.Add(f.curBases[1], uintptr(f.curIdx)*f.compSizes[1])),
		(*T3)(unsafe.Add(f.curBases[2], uintptr(f.curIdx)*f.compSizes[2])),
		(*T4)(unsafe.Add(f.curBases[3], uintptr(f.curIdx)*f.compSizes[3])),
		(*T5)(unsafe.Add(f.curBases[4], uintptr(f.curIdx)*f.compSizes[4])),
		(*T6)(unsafe.Add(f.curBases[5], uintptr(f.curIdx)*f.compSizes[5])),
		(*T7)(unsafe.Add(f.curBases[6], uintptr(f.curIdx)*f.compSizes[6]))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter7[T1, T2, T3, T4, T5, T6, T7]) RemoveEntities() {
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
func (f *Filter7[T1, T2, T3, T4, T5, T6, T7]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Query7 is an allocation-free iterator snapshot for Filter7.
type Query7[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any] struct {
	matchingArches []*archetype
	curBases       [7]unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSizes      [7]uintptr
	curArchSize    int
	ids            [7]uint8
}

// Query returns a new Query7 iterator from the Filter7.
func (f *Filter7[T1, T2, T3, T4, T5, T6, T7]) Query() Query7[T1, T2, T3, T4, T5, T6, T7] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	q := Query7[T1, T2, T3, T4, T5, T6, T7]{
		matchingArches: f.matchingArches,
		ids:            f.ids,
		compSizes:      f.compSizes,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		f.curBases[4] = a.compPointers[f.ids[4]]
		f.curBases[5] = a.compPointers[f.ids[5]]
		f.curBases[6] = a.compPointers[f.ids[6]]
		
		q.curEntityIDs = a.entityIDs
		q.curArchSize = a.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query7[T1, T2, T3, T4, T5, T6, T7]) Next() bool {
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
	q.curBases[6] = a.compPointers[q.ids[6]]
	
	q.curEntityIDs = a.entityIDs
	q.curArchSize = a.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query7[T1, T2, T3, T4, T5, T6, T7]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns pointers to T1, T2, T3, T4, T5, T6, T7 for the current entity.
func (q *Query7[T1, T2, T3, T4, T5, T6, T7]) Get() (*T1, *T2, *T3, *T4, *T5, *T6, *T7) {
	return (*T1)(unsafe.Add(q.curBases[0], uintptr(q.curIdx)*q.compSizes[0])),
		(*T2)(unsafe.Add(q.curBases[1], uintptr(q.curIdx)*q.compSizes[1])),
		(*T3)(unsafe.Add(q.curBases[2], uintptr(q.curIdx)*q.compSizes[2])),
		(*T4)(unsafe.Add(q.curBases[3], uintptr(q.curIdx)*q.compSizes[3])),
		(*T5)(unsafe.Add(q.curBases[4], uintptr(q.curIdx)*q.compSizes[4])),
		(*T6)(unsafe.Add(q.curBases[5], uintptr(q.curIdx)*q.compSizes[5])),
		(*T7)(unsafe.Add(q.curBases[6], uintptr(q.curIdx)*q.compSizes[6]))
}

// Filter8 provides a fast, cache-friendly iterator over all entities that
// have the 8 components: T1, T2, T3, T4, T5, T6, T7, T8.
type Filter8[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any] struct {
	queryCache
	curBases     [8]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [8]uintptr
	curArchSize  int
	ids          [8]uint8
}

// NewFilter8 creates a new `Filter` that iterates over all entities
// possessing at least the 8 components: T1, T2, T3, T4, T5, T6, T7, T8.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter8`.
func NewFilter8[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any](w *World) *Filter8[T1, T2, T3, T4, T5, T6, T7, T8] {
	w.mu.RLock()
	defer w.mu.RUnlock()
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())
	id5 := w.getCompTypeID(reflect.TypeFor[T5]())
	id6 := w.getCompTypeID(reflect.TypeFor[T6]())
	id7 := w.getCompTypeID(reflect.TypeFor[T7]())
	id8 := w.getCompTypeID(reflect.TypeFor[T8]())
	
	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 {
		panic("ecs: duplicate component types in Filter8")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)
	m.set(id4)
	m.set(id5)
	m.set(id6)
	m.set(id7)
	m.set(id8)
	
	f := &Filter8[T1, T2, T3, T4, T5, T6, T7, T8]{
		queryCache:  newQueryCache(w, m),
		ids:         [8]uint8{ id1, id2, id3, id4, id5, id6, id7, id8 },
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSizes[0] = w.components.compIDToSize[id1]
	f.compSizes[1] = w.components.compIDToSize[id2]
	f.compSizes[2] = w.components.compIDToSize[id3]
	f.compSizes[3] = w.components.compIDToSize[id4]
	f.compSizes[4] = w.components.compIDToSize[id5]
	f.compSizes[5] = w.components.compIDToSize[id6]
	f.compSizes[6] = w.components.compIDToSize[id7]
	f.compSizes[7] = w.components.compIDToSize[id8]
	
	f.updateMatching()
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter8`.
func (f *Filter8[T1, T2, T3, T4, T5, T6, T7, T8]) New(w *World) *Filter8[T1, T2, T3, T4, T5, T6, T7, T8] {
	return NewFilter8[T1, T2, T3, T4, T5, T6, T7, T8](w)
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter8[T1, T2, T3, T4, T5, T6, T7, T8]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter8[T1, T2, T3, T4, T5, T6, T7, T8]) doReset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
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
		f.curBases[6] = a.compPointers[f.ids[6]]
		f.curBases[7] = a.compPointers[f.ids[7]]
		
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
func (f *Filter8[T1, T2, T3, T4, T5, T6, T7, T8]) Next() bool {
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
	f.curBases[6] = a.compPointers[f.ids[6]]
	f.curBases[7] = a.compPointers[f.ids[7]]
	
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
func (f *Filter8[T1, T2, T3, T4, T5, T6, T7, T8]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the 8 components (T1, T2, T3, T4, T5, T6, T7, T8) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8).
func (f *Filter8[T1, T2, T3, T4, T5, T6, T7, T8]) Get() (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8) {
	return (*T1)(unsafe.Add(f.curBases[0], uintptr(f.curIdx)*f.compSizes[0])),
		(*T2)(unsafe.Add(f.curBases[1], uintptr(f.curIdx)*f.compSizes[1])),
		(*T3)(unsafe.Add(f.curBases[2], uintptr(f.curIdx)*f.compSizes[2])),
		(*T4)(unsafe.Add(f.curBases[3], uintptr(f.curIdx)*f.compSizes[3])),
		(*T5)(unsafe.Add(f.curBases[4], uintptr(f.curIdx)*f.compSizes[4])),
		(*T6)(unsafe.Add(f.curBases[5], uintptr(f.curIdx)*f.compSizes[5])),
		(*T7)(unsafe.Add(f.curBases[6], uintptr(f.curIdx)*f.compSizes[6])),
		(*T8)(unsafe.Add(f.curBases[7], uintptr(f.curIdx)*f.compSizes[7]))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter8[T1, T2, T3, T4, T5, T6, T7, T8]) RemoveEntities() {
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
func (f *Filter8[T1, T2, T3, T4, T5, T6, T7, T8]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Query8 is an allocation-free iterator snapshot for Filter8.
type Query8[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any] struct {
	matchingArches []*archetype
	curBases       [8]unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSizes      [8]uintptr
	curArchSize    int
	ids            [8]uint8
}

// Query returns a new Query8 iterator from the Filter8.
func (f *Filter8[T1, T2, T3, T4, T5, T6, T7, T8]) Query() Query8[T1, T2, T3, T4, T5, T6, T7, T8] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	q := Query8[T1, T2, T3, T4, T5, T6, T7, T8]{
		matchingArches: f.matchingArches,
		ids:            f.ids,
		compSizes:      f.compSizes,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		f.curBases[4] = a.compPointers[f.ids[4]]
		f.curBases[5] = a.compPointers[f.ids[5]]
		f.curBases[6] = a.compPointers[f.ids[6]]
		f.curBases[7] = a.compPointers[f.ids[7]]
		
		q.curEntityIDs = a.entityIDs
		q.curArchSize = a.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query8[T1, T2, T3, T4, T5, T6, T7, T8]) Next() bool {
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
	q.curBases[6] = a.compPointers[q.ids[6]]
	q.curBases[7] = a.compPointers[q.ids[7]]
	
	q.curEntityIDs = a.entityIDs
	q.curArchSize = a.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query8[T1, T2, T3, T4, T5, T6, T7, T8]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns pointers to T1, T2, T3, T4, T5, T6, T7, T8 for the current entity.
func (q *Query8[T1, T2, T3, T4, T5, T6, T7, T8]) Get() (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8) {
	return (*T1)(unsafe.Add(q.curBases[0], uintptr(q.curIdx)*q.compSizes[0])),
		(*T2)(unsafe.Add(q.curBases[1], uintptr(q.curIdx)*q.compSizes[1])),
		(*T3)(unsafe.Add(q.curBases[2], uintptr(q.curIdx)*q.compSizes[2])),
		(*T4)(unsafe.Add(q.curBases[3], uintptr(q.curIdx)*q.compSizes[3])),
		(*T5)(unsafe.Add(q.curBases[4], uintptr(q.curIdx)*q.compSizes[4])),
		(*T6)(unsafe.Add(q.curBases[5], uintptr(q.curIdx)*q.compSizes[5])),
		(*T7)(unsafe.Add(q.curBases[6], uintptr(q.curIdx)*q.compSizes[6])),
		(*T8)(unsafe.Add(q.curBases[7], uintptr(q.curIdx)*q.compSizes[7]))
}

// Filter9 provides a fast, cache-friendly iterator over all entities that
// have the 9 components: T1, T2, T3, T4, T5, T6, T7, T8, T9.
type Filter9[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any] struct {
	queryCache
	curBases     [9]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [9]uintptr
	curArchSize  int
	ids          [9]uint8
}

// NewFilter9 creates a new `Filter` that iterates over all entities
// possessing at least the 9 components: T1, T2, T3, T4, T5, T6, T7, T8, T9.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter9`.
func NewFilter9[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any](w *World) *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9] {
	w.mu.RLock()
	defer w.mu.RUnlock()
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())
	id5 := w.getCompTypeID(reflect.TypeFor[T5]())
	id6 := w.getCompTypeID(reflect.TypeFor[T6]())
	id7 := w.getCompTypeID(reflect.TypeFor[T7]())
	id8 := w.getCompTypeID(reflect.TypeFor[T8]())
	id9 := w.getCompTypeID(reflect.TypeFor[T9]())
	
	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 || id9 == id1 || id9 == id2 || id9 == id3 || id9 == id4 || id9 == id5 || id9 == id6 || id9 == id7 || id9 == id8 {
		panic("ecs: duplicate component types in Filter9")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)
	m.set(id4)
	m.set(id5)
	m.set(id6)
	m.set(id7)
	m.set(id8)
	m.set(id9)
	
	f := &Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]{
		queryCache:  newQueryCache(w, m),
		ids:         [9]uint8{ id1, id2, id3, id4, id5, id6, id7, id8, id9 },
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSizes[0] = w.components.compIDToSize[id1]
	f.compSizes[1] = w.components.compIDToSize[id2]
	f.compSizes[2] = w.components.compIDToSize[id3]
	f.compSizes[3] = w.components.compIDToSize[id4]
	f.compSizes[4] = w.components.compIDToSize[id5]
	f.compSizes[5] = w.components.compIDToSize[id6]
	f.compSizes[6] = w.components.compIDToSize[id7]
	f.compSizes[7] = w.components.compIDToSize[id8]
	f.compSizes[8] = w.components.compIDToSize[id9]
	
	f.updateMatching()
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter9`.
func (f *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) New(w *World) *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9] {
	return NewFilter9[T1, T2, T3, T4, T5, T6, T7, T8, T9](w)
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) doReset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
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
		f.curBases[6] = a.compPointers[f.ids[6]]
		f.curBases[7] = a.compPointers[f.ids[7]]
		f.curBases[8] = a.compPointers[f.ids[8]]
		
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
func (f *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Next() bool {
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
	f.curBases[6] = a.compPointers[f.ids[6]]
	f.curBases[7] = a.compPointers[f.ids[7]]
	f.curBases[8] = a.compPointers[f.ids[8]]
	
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
func (f *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the 9 components (T1, T2, T3, T4, T5, T6, T7, T8, T9) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9).
func (f *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Get() (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9) {
	return (*T1)(unsafe.Add(f.curBases[0], uintptr(f.curIdx)*f.compSizes[0])),
		(*T2)(unsafe.Add(f.curBases[1], uintptr(f.curIdx)*f.compSizes[1])),
		(*T3)(unsafe.Add(f.curBases[2], uintptr(f.curIdx)*f.compSizes[2])),
		(*T4)(unsafe.Add(f.curBases[3], uintptr(f.curIdx)*f.compSizes[3])),
		(*T5)(unsafe.Add(f.curBases[4], uintptr(f.curIdx)*f.compSizes[4])),
		(*T6)(unsafe.Add(f.curBases[5], uintptr(f.curIdx)*f.compSizes[5])),
		(*T7)(unsafe.Add(f.curBases[6], uintptr(f.curIdx)*f.compSizes[6])),
		(*T8)(unsafe.Add(f.curBases[7], uintptr(f.curIdx)*f.compSizes[7])),
		(*T9)(unsafe.Add(f.curBases[8], uintptr(f.curIdx)*f.compSizes[8]))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) RemoveEntities() {
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
func (f *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Query9 is an allocation-free iterator snapshot for Filter9.
type Query9[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any] struct {
	matchingArches []*archetype
	curBases       [9]unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSizes      [9]uintptr
	curArchSize    int
	ids            [9]uint8
}

// Query returns a new Query9 iterator from the Filter9.
func (f *Filter9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Query() Query9[T1, T2, T3, T4, T5, T6, T7, T8, T9] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	q := Query9[T1, T2, T3, T4, T5, T6, T7, T8, T9]{
		matchingArches: f.matchingArches,
		ids:            f.ids,
		compSizes:      f.compSizes,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		f.curBases[4] = a.compPointers[f.ids[4]]
		f.curBases[5] = a.compPointers[f.ids[5]]
		f.curBases[6] = a.compPointers[f.ids[6]]
		f.curBases[7] = a.compPointers[f.ids[7]]
		f.curBases[8] = a.compPointers[f.ids[8]]
		
		q.curEntityIDs = a.entityIDs
		q.curArchSize = a.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Next() bool {
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
	q.curBases[6] = a.compPointers[q.ids[6]]
	q.curBases[7] = a.compPointers[q.ids[7]]
	q.curBases[8] = a.compPointers[q.ids[8]]
	
	q.curEntityIDs = a.entityIDs
	q.curArchSize = a.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns pointers to T1, T2, T3, T4, T5, T6, T7, T8, T9 for the current entity.
func (q *Query9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Get() (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9) {
	return (*T1)(unsafe.Add(q.curBases[0], uintptr(q.curIdx)*q.compSizes[0])),
		(*T2)(unsafe.Add(q.curBases[1], uintptr(q.curIdx)*q.compSizes[1])),
		(*T3)(unsafe.Add(q.curBases[2], uintptr(q.curIdx)*q.compSizes[2])),
		(*T4)(unsafe.Add(q.curBases[3], uintptr(q.curIdx)*q.compSizes[3])),
		(*T5)(unsafe.Add(q.curBases[4], uintptr(q.curIdx)*q.compSizes[4])),
		(*T6)(unsafe.Add(q.curBases[5], uintptr(q.curIdx)*q.compSizes[5])),
		(*T7)(unsafe.Add(q.curBases[6], uintptr(q.curIdx)*q.compSizes[6])),
		(*T8)(unsafe.Add(q.curBases[7], uintptr(q.curIdx)*q.compSizes[7])),
		(*T9)(unsafe.Add(q.curBases[8], uintptr(q.curIdx)*q.compSizes[8]))
}

// Filter10 provides a fast, cache-friendly iterator over all entities that
// have the 10 components: T1, T2, T3, T4, T5, T6, T7, T8, T9, T10.
type Filter10[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any, T10 any] struct {
	queryCache
	curBases     [10]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [10]uintptr
	curArchSize  int
	ids          [10]uint8
}

// NewFilter10 creates a new `Filter` that iterates over all entities
// possessing at least the 10 components: T1, T2, T3, T4, T5, T6, T7, T8, T9, T10.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter10`.
func NewFilter10[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any, T10 any](w *World) *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10] {
	w.mu.RLock()
	defer w.mu.RUnlock()
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())
	id5 := w.getCompTypeID(reflect.TypeFor[T5]())
	id6 := w.getCompTypeID(reflect.TypeFor[T6]())
	id7 := w.getCompTypeID(reflect.TypeFor[T7]())
	id8 := w.getCompTypeID(reflect.TypeFor[T8]())
	id9 := w.getCompTypeID(reflect.TypeFor[T9]())
	id10 := w.getCompTypeID(reflect.TypeFor[T10]())
	
	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 || id9 == id1 || id9 == id2 || id9 == id3 || id9 == id4 || id9 == id5 || id9 == id6 || id9 == id7 || id9 == id8 || id10 == id1 || id10 == id2 || id10 == id3 || id10 == id4 || id10 == id5 || id10 == id6 || id10 == id7 || id10 == id8 || id10 == id9 {
		panic("ecs: duplicate component types in Filter10")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)
	m.set(id4)
	m.set(id5)
	m.set(id6)
	m.set(id7)
	m.set(id8)
	m.set(id9)
	m.set(id10)
	
	f := &Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]{
		queryCache:  newQueryCache(w, m),
		ids:         [10]uint8{ id1, id2, id3, id4, id5, id6, id7, id8, id9, id10 },
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSizes[0] = w.components.compIDToSize[id1]
	f.compSizes[1] = w.components.compIDToSize[id2]
	f.compSizes[2] = w.components.compIDToSize[id3]
	f.compSizes[3] = w.components.compIDToSize[id4]
	f.compSizes[4] = w.components.compIDToSize[id5]
	f.compSizes[5] = w.components.compIDToSize[id6]
	f.compSizes[6] = w.components.compIDToSize[id7]
	f.compSizes[7] = w.components.compIDToSize[id8]
	f.compSizes[8] = w.components.compIDToSize[id9]
	f.compSizes[9] = w.components.compIDToSize[id10]
	
	f.updateMatching()
	f.updateCachedEntities()
	f.doReset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter10`.
func (f *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) New(w *World) *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10] {
	return NewFilter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10](w)
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Reset() {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	f.doReset()
}

func (f *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) doReset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
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
		f.curBases[6] = a.compPointers[f.ids[6]]
		f.curBases[7] = a.compPointers[f.ids[7]]
		f.curBases[8] = a.compPointers[f.ids[8]]
		f.curBases[9] = a.compPointers[f.ids[9]]
		
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
func (f *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Next() bool {
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
	f.curBases[6] = a.compPointers[f.ids[6]]
	f.curBases[7] = a.compPointers[f.ids[7]]
	f.curBases[8] = a.compPointers[f.ids[8]]
	f.curBases[9] = a.compPointers[f.ids[9]]
	
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
func (f *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the 10 components (T1, T2, T3, T4, T5, T6, T7, T8, T9, T10) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9, *T10).
func (f *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Get() (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9, *T10) {
	return (*T1)(unsafe.Add(f.curBases[0], uintptr(f.curIdx)*f.compSizes[0])),
		(*T2)(unsafe.Add(f.curBases[1], uintptr(f.curIdx)*f.compSizes[1])),
		(*T3)(unsafe.Add(f.curBases[2], uintptr(f.curIdx)*f.compSizes[2])),
		(*T4)(unsafe.Add(f.curBases[3], uintptr(f.curIdx)*f.compSizes[3])),
		(*T5)(unsafe.Add(f.curBases[4], uintptr(f.curIdx)*f.compSizes[4])),
		(*T6)(unsafe.Add(f.curBases[5], uintptr(f.curIdx)*f.compSizes[5])),
		(*T7)(unsafe.Add(f.curBases[6], uintptr(f.curIdx)*f.compSizes[6])),
		(*T8)(unsafe.Add(f.curBases[7], uintptr(f.curIdx)*f.compSizes[7])),
		(*T9)(unsafe.Add(f.curBases[8], uintptr(f.curIdx)*f.compSizes[8])),
		(*T10)(unsafe.Add(f.curBases[9], uintptr(f.curIdx)*f.compSizes[9]))
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) RemoveEntities() {
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
func (f *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Query10 is an allocation-free iterator snapshot for Filter10.
type Query10[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any, T10 any] struct {
	matchingArches []*archetype
	curBases       [10]unsafe.Pointer
	curEntityIDs   []Entity
	curMatchIdx    int
	curIdx         int
	compSizes      [10]uintptr
	curArchSize    int
	ids            [10]uint8
}

// Query returns a new Query10 iterator from the Filter10.
func (f *Filter10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Query() Query10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10] {
	f.world.mu.RLock()
	defer f.world.mu.RUnlock()
	if f.isArchetypeStale() {
		f.updateMatching()
	}
	q := Query10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]{
		matchingArches: f.matchingArches,
		ids:            f.ids,
		compSizes:      f.compSizes,
		curMatchIdx:    0,
		curIdx:         -1,
	}
	if len(q.matchingArches) > 0 {
		a := q.matchingArches[0]
		f.curBases[0] = a.compPointers[f.ids[0]]
		f.curBases[1] = a.compPointers[f.ids[1]]
		f.curBases[2] = a.compPointers[f.ids[2]]
		f.curBases[3] = a.compPointers[f.ids[3]]
		f.curBases[4] = a.compPointers[f.ids[4]]
		f.curBases[5] = a.compPointers[f.ids[5]]
		f.curBases[6] = a.compPointers[f.ids[6]]
		f.curBases[7] = a.compPointers[f.ids[7]]
		f.curBases[8] = a.compPointers[f.ids[8]]
		f.curBases[9] = a.compPointers[f.ids[9]]
		
		q.curEntityIDs = a.entityIDs
		q.curArchSize = a.size
	} else {
		q.curArchSize = 0
	}
	return q
}

// Next advances the query to the next matching entity.
func (q *Query10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Next() bool {
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
	q.curBases[6] = a.compPointers[q.ids[6]]
	q.curBases[7] = a.compPointers[q.ids[7]]
	q.curBases[8] = a.compPointers[q.ids[8]]
	q.curBases[9] = a.compPointers[q.ids[9]]
	
	q.curEntityIDs = a.entityIDs
	q.curArchSize = a.size
	q.curIdx = 0
	return true
}

// Entity returns the current entity in the query.
func (q *Query10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Entity() Entity {
	return q.curEntityIDs[q.curIdx]
}

// Get returns pointers to T1, T2, T3, T4, T5, T6, T7, T8, T9, T10 for the current entity.
func (q *Query10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Get() (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9, *T10) {
	return (*T1)(unsafe.Add(q.curBases[0], uintptr(q.curIdx)*q.compSizes[0])),
		(*T2)(unsafe.Add(q.curBases[1], uintptr(q.curIdx)*q.compSizes[1])),
		(*T3)(unsafe.Add(q.curBases[2], uintptr(q.curIdx)*q.compSizes[2])),
		(*T4)(unsafe.Add(q.curBases[3], uintptr(q.curIdx)*q.compSizes[3])),
		(*T5)(unsafe.Add(q.curBases[4], uintptr(q.curIdx)*q.compSizes[4])),
		(*T6)(unsafe.Add(q.curBases[5], uintptr(q.curIdx)*q.compSizes[5])),
		(*T7)(unsafe.Add(q.curBases[6], uintptr(q.curIdx)*q.compSizes[6])),
		(*T8)(unsafe.Add(q.curBases[7], uintptr(q.curIdx)*q.compSizes[7])),
		(*T9)(unsafe.Add(q.curBases[8], uintptr(q.curIdx)*q.compSizes[8])),
		(*T10)(unsafe.Add(q.curBases[9], uintptr(q.curIdx)*q.compSizes[9]))
}

