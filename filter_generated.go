package teishoku

import (
	"reflect"
	"unsafe"
)

// Filter2 provides a fast, cache-friendly iterator over all entities that
// have the 2 components: T1, T2.
type Filter2[T1 any, T2 any] struct {
	queryCache
	curBase1 unsafe.Pointer
	curBase2 unsafe.Pointer

	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSize1    uintptr
	compSize2    uintptr

	curArchSize int
	id1         uint8
	id2         uint8
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
		id1:         id1,
		id2:         id2,
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSize1 = w.components.compIDToSize[id1]
	f.compSize2 = w.components.compIDToSize[id2]

	f.updateMatching()
	f.updateCachedEntities()
	f.Reset()
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
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBase1 = a.compPointers[f.id1]
		f.curBase2 = a.compPointers[f.id2]

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
	f.curBase1 = a.compPointers[f.id1]
	f.curBase2 = a.compPointers[f.id2]

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
	ptr1 := unsafe.Pointer(uintptr(f.curBase1) + uintptr(f.curIdx)*f.compSize1)
	ptr2 := unsafe.Pointer(uintptr(f.curBase2) + uintptr(f.curIdx)*f.compSize2)

	return (*T1)(ptr1), (*T2)(ptr2)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter2[T1, T2]) RemoveEntities() {
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
	f.world.mutationVersion++
	f.Reset()
}

// Entities returns all entities that match the filter.
func (f *Filter2[T1, T2]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Filter3 provides a fast, cache-friendly iterator over all entities that
// have the 3 components: T1, T2, T3.
type Filter3[T1 any, T2 any, T3 any] struct {
	queryCache
	curBase1 unsafe.Pointer
	curBase2 unsafe.Pointer
	curBase3 unsafe.Pointer

	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSize1    uintptr
	compSize2    uintptr
	compSize3    uintptr

	curArchSize int
	id1         uint8
	id2         uint8
	id3         uint8
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
		id1:         id1,
		id2:         id2,
		id3:         id3,
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSize1 = w.components.compIDToSize[id1]
	f.compSize2 = w.components.compIDToSize[id2]
	f.compSize3 = w.components.compIDToSize[id3]

	f.updateMatching()
	f.updateCachedEntities()
	f.Reset()
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
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBase1 = a.compPointers[f.id1]
		f.curBase2 = a.compPointers[f.id2]
		f.curBase3 = a.compPointers[f.id3]

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
	f.curBase1 = a.compPointers[f.id1]
	f.curBase2 = a.compPointers[f.id2]
	f.curBase3 = a.compPointers[f.id3]

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
	ptr1 := unsafe.Pointer(uintptr(f.curBase1) + uintptr(f.curIdx)*f.compSize1)
	ptr2 := unsafe.Pointer(uintptr(f.curBase2) + uintptr(f.curIdx)*f.compSize2)
	ptr3 := unsafe.Pointer(uintptr(f.curBase3) + uintptr(f.curIdx)*f.compSize3)

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter3[T1, T2, T3]) RemoveEntities() {
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
	f.world.mutationVersion++
	f.Reset()
}

// Entities returns all entities that match the filter.
func (f *Filter3[T1, T2, T3]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Filter4 provides a fast, cache-friendly iterator over all entities that
// have the 4 components: T1, T2, T3, T4.
type Filter4[T1 any, T2 any, T3 any, T4 any] struct {
	queryCache
	curBase1 unsafe.Pointer
	curBase2 unsafe.Pointer
	curBase3 unsafe.Pointer
	curBase4 unsafe.Pointer

	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSize1    uintptr
	compSize2    uintptr
	compSize3    uintptr
	compSize4    uintptr

	curArchSize int
	id1         uint8
	id2         uint8
	id3         uint8
	id4         uint8
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
		id1:         id1,
		id2:         id2,
		id3:         id3,
		id4:         id4,
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSize1 = w.components.compIDToSize[id1]
	f.compSize2 = w.components.compIDToSize[id2]
	f.compSize3 = w.components.compIDToSize[id3]
	f.compSize4 = w.components.compIDToSize[id4]

	f.updateMatching()
	f.updateCachedEntities()
	f.Reset()
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
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBase1 = a.compPointers[f.id1]
		f.curBase2 = a.compPointers[f.id2]
		f.curBase3 = a.compPointers[f.id3]
		f.curBase4 = a.compPointers[f.id4]

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
	f.curBase1 = a.compPointers[f.id1]
	f.curBase2 = a.compPointers[f.id2]
	f.curBase3 = a.compPointers[f.id3]
	f.curBase4 = a.compPointers[f.id4]

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
	ptr1 := unsafe.Pointer(uintptr(f.curBase1) + uintptr(f.curIdx)*f.compSize1)
	ptr2 := unsafe.Pointer(uintptr(f.curBase2) + uintptr(f.curIdx)*f.compSize2)
	ptr3 := unsafe.Pointer(uintptr(f.curBase3) + uintptr(f.curIdx)*f.compSize3)
	ptr4 := unsafe.Pointer(uintptr(f.curBase4) + uintptr(f.curIdx)*f.compSize4)

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter4[T1, T2, T3, T4]) RemoveEntities() {
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
	f.world.mutationVersion++
	f.Reset()
}

// Entities returns all entities that match the filter.
func (f *Filter4[T1, T2, T3, T4]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Filter5 provides a fast, cache-friendly iterator over all entities that
// have the 5 components: T1, T2, T3, T4, T5.
type Filter5[T1 any, T2 any, T3 any, T4 any, T5 any] struct {
	queryCache
	curBase1 unsafe.Pointer
	curBase2 unsafe.Pointer
	curBase3 unsafe.Pointer
	curBase4 unsafe.Pointer
	curBase5 unsafe.Pointer

	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSize1    uintptr
	compSize2    uintptr
	compSize3    uintptr
	compSize4    uintptr
	compSize5    uintptr

	curArchSize int
	id1         uint8
	id2         uint8
	id3         uint8
	id4         uint8
	id5         uint8
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
		id1:         id1,
		id2:         id2,
		id3:         id3,
		id4:         id4,
		id5:         id5,
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSize1 = w.components.compIDToSize[id1]
	f.compSize2 = w.components.compIDToSize[id2]
	f.compSize3 = w.components.compIDToSize[id3]
	f.compSize4 = w.components.compIDToSize[id4]
	f.compSize5 = w.components.compIDToSize[id5]

	f.updateMatching()
	f.updateCachedEntities()
	f.Reset()
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
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBase1 = a.compPointers[f.id1]
		f.curBase2 = a.compPointers[f.id2]
		f.curBase3 = a.compPointers[f.id3]
		f.curBase4 = a.compPointers[f.id4]
		f.curBase5 = a.compPointers[f.id5]

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
	f.curBase1 = a.compPointers[f.id1]
	f.curBase2 = a.compPointers[f.id2]
	f.curBase3 = a.compPointers[f.id3]
	f.curBase4 = a.compPointers[f.id4]
	f.curBase5 = a.compPointers[f.id5]

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
	ptr1 := unsafe.Pointer(uintptr(f.curBase1) + uintptr(f.curIdx)*f.compSize1)
	ptr2 := unsafe.Pointer(uintptr(f.curBase2) + uintptr(f.curIdx)*f.compSize2)
	ptr3 := unsafe.Pointer(uintptr(f.curBase3) + uintptr(f.curIdx)*f.compSize3)
	ptr4 := unsafe.Pointer(uintptr(f.curBase4) + uintptr(f.curIdx)*f.compSize4)
	ptr5 := unsafe.Pointer(uintptr(f.curBase5) + uintptr(f.curIdx)*f.compSize5)

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4), (*T5)(ptr5)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter5[T1, T2, T3, T4, T5]) RemoveEntities() {
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
	f.world.mutationVersion++
	f.Reset()
}

// Entities returns all entities that match the filter.
func (f *Filter5[T1, T2, T3, T4, T5]) Entities() []Entity {
	return f.queryCache.Entities()
}

// Filter6 provides a fast, cache-friendly iterator over all entities that
// have the 6 components: T1, T2, T3, T4, T5, T6.
type Filter6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any] struct {
	queryCache
	curBase1 unsafe.Pointer
	curBase2 unsafe.Pointer
	curBase3 unsafe.Pointer
	curBase4 unsafe.Pointer
	curBase5 unsafe.Pointer
	curBase6 unsafe.Pointer

	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSize1    uintptr
	compSize2    uintptr
	compSize3    uintptr
	compSize4    uintptr
	compSize5    uintptr
	compSize6    uintptr

	curArchSize int
	id1         uint8
	id2         uint8
	id3         uint8
	id4         uint8
	id5         uint8
	id6         uint8
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
		id1:         id1,
		id2:         id2,
		id3:         id3,
		id4:         id4,
		id5:         id5,
		id6:         id6,
		curMatchIdx: 0,
		curIdx:      -1,
	}
	f.compSize1 = w.components.compIDToSize[id1]
	f.compSize2 = w.components.compIDToSize[id2]
	f.compSize3 = w.components.compIDToSize[id3]
	f.compSize4 = w.components.compIDToSize[id4]
	f.compSize5 = w.components.compIDToSize[id5]
	f.compSize6 = w.components.compIDToSize[id6]

	f.updateMatching()
	f.updateCachedEntities()
	f.Reset()
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
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		f.curBase1 = a.compPointers[f.id1]
		f.curBase2 = a.compPointers[f.id2]
		f.curBase3 = a.compPointers[f.id3]
		f.curBase4 = a.compPointers[f.id4]
		f.curBase5 = a.compPointers[f.id5]
		f.curBase6 = a.compPointers[f.id6]

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
	f.curBase1 = a.compPointers[f.id1]
	f.curBase2 = a.compPointers[f.id2]
	f.curBase3 = a.compPointers[f.id3]
	f.curBase4 = a.compPointers[f.id4]
	f.curBase5 = a.compPointers[f.id5]
	f.curBase6 = a.compPointers[f.id6]

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
	ptr1 := unsafe.Pointer(uintptr(f.curBase1) + uintptr(f.curIdx)*f.compSize1)
	ptr2 := unsafe.Pointer(uintptr(f.curBase2) + uintptr(f.curIdx)*f.compSize2)
	ptr3 := unsafe.Pointer(uintptr(f.curBase3) + uintptr(f.curIdx)*f.compSize3)
	ptr4 := unsafe.Pointer(uintptr(f.curBase4) + uintptr(f.curIdx)*f.compSize4)
	ptr5 := unsafe.Pointer(uintptr(f.curBase5) + uintptr(f.curIdx)*f.compSize5)
	ptr6 := unsafe.Pointer(uintptr(f.curBase6) + uintptr(f.curIdx)*f.compSize6)

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4), (*T5)(ptr5), (*T6)(ptr6)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) RemoveEntities() {
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
	f.world.mutationVersion++
	f.Reset()
}

// Entities returns all entities that match the filter.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Entities() []Entity {
	return f.queryCache.Entities()
}
