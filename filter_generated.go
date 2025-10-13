package lazyecs

import (
	"reflect"
	"unsafe"
)

// This template generates the code for N-ary Filters (Filter2, Filter3, etc.).
// A Filter is a high-performance iterator (or query) that finds all entities
// possessing a specific set of components. It works by iterating directly over
// the contiguous memory blocks of matching archetypes, which is extremely fast.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types.
// - .Components: A slice of ComponentInfo structs for looping.
// - .ReturnTypes: The list of pointer types for the Get() method, e.g., "*T1, *T2".
// - .ReturnPtrs: The expression for returning the pointers, e.g., "(*T1)(p1), (*T2)(p2)".
// Filter2 provides a fast, cache-friendly iterator over all entities that
// have the 2 components: T1, T2.
type Filter2[T1 any, T2 any] struct {
	world *World
	mask  bitmask256
	id1   uint8
	id2   uint8

	matchingArches []*archetype
	lastVersion    uint32 // world.archetypeVersion when matchingArches was last updated
	curMatchIdx    int    // index into matchingArches
	curIdx         int    // index into the current archetype's entity/component array
	curEnt         Entity
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
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)

	if id2 == id1 {
		panic("ecs: duplicate component types in Filter2")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)

	f := &Filter2[T1, T2]{world: w, mask: m, id1: id1, id2: id2, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
	return f
}

// New is a convenience function that creates a new filter instance.
func (f *Filter2[T1, T2]) New(w *World) *Filter2[T1, T2] {
	return NewFilter2[T1, T2](w)
}

// updateMatching rebuilds the filter's list of archetypes that match its
// component mask.
func (f *Filter2[T1, T2]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter2[T1, T2]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter2[T1, T2]) Next() bool {
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
func (f *Filter2[T1, T2]) Entity() Entity {
	return f.curEnt
}

// Get returns pointers to the 2 components (T1, T2) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2).
func (f *Filter2[T1, T2]) Get() (*T1, *T2) {
	a := f.matchingArches[f.curMatchIdx]
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[f.id1]) + uintptr(f.curIdx)*a.compSizes[f.id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[f.id2]) + uintptr(f.curIdx)*a.compSizes[f.id2])

	return (*T1)(ptr1), (*T2)(ptr2)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter2[T1, T2]) RemoveEntities() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
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

// This template generates the code for N-ary Filters (Filter2, Filter3, etc.).
// A Filter is a high-performance iterator (or query) that finds all entities
// possessing a specific set of components. It works by iterating directly over
// the contiguous memory blocks of matching archetypes, which is extremely fast.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types.
// - .Components: A slice of ComponentInfo structs for looping.
// - .ReturnTypes: The list of pointer types for the Get() method, e.g., "*T1, *T2".
// - .ReturnPtrs: The expression for returning the pointers, e.g., "(*T1)(p1), (*T2)(p2)".
// Filter3 provides a fast, cache-friendly iterator over all entities that
// have the 3 components: T1, T2, T3.
type Filter3[T1 any, T2 any, T3 any] struct {
	world *World
	mask  bitmask256
	id1   uint8
	id2   uint8
	id3   uint8

	matchingArches []*archetype
	lastVersion    uint32 // world.archetypeVersion when matchingArches was last updated
	curMatchIdx    int    // index into matchingArches
	curIdx         int    // index into the current archetype's entity/component array
	curEnt         Entity
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
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in Filter3")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)

	f := &Filter3[T1, T2, T3]{world: w, mask: m, id1: id1, id2: id2, id3: id3, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
	return f
}

// New is a convenience function that creates a new filter instance.
func (f *Filter3[T1, T2, T3]) New(w *World) *Filter3[T1, T2, T3] {
	return NewFilter3[T1, T2, T3](w)
}

// updateMatching rebuilds the filter's list of archetypes that match its
// component mask.
func (f *Filter3[T1, T2, T3]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter3[T1, T2, T3]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter3[T1, T2, T3]) Next() bool {
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
func (f *Filter3[T1, T2, T3]) Entity() Entity {
	return f.curEnt
}

// Get returns pointers to the 3 components (T1, T2, T3) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3).
func (f *Filter3[T1, T2, T3]) Get() (*T1, *T2, *T3) {
	a := f.matchingArches[f.curMatchIdx]
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[f.id1]) + uintptr(f.curIdx)*a.compSizes[f.id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[f.id2]) + uintptr(f.curIdx)*a.compSizes[f.id2])
	ptr3 := unsafe.Pointer(uintptr(a.compPointers[f.id3]) + uintptr(f.curIdx)*a.compSizes[f.id3])

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter3[T1, T2, T3]) RemoveEntities() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
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

// This template generates the code for N-ary Filters (Filter2, Filter3, etc.).
// A Filter is a high-performance iterator (or query) that finds all entities
// possessing a specific set of components. It works by iterating directly over
// the contiguous memory blocks of matching archetypes, which is extremely fast.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types.
// - .Components: A slice of ComponentInfo structs for looping.
// - .ReturnTypes: The list of pointer types for the Get() method, e.g., "*T1, *T2".
// - .ReturnPtrs: The expression for returning the pointers, e.g., "(*T1)(p1), (*T2)(p2)".
// Filter4 provides a fast, cache-friendly iterator over all entities that
// have the 4 components: T1, T2, T3, T4.
type Filter4[T1 any, T2 any, T3 any, T4 any] struct {
	world *World
	mask  bitmask256
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8

	matchingArches []*archetype
	lastVersion    uint32 // world.archetypeVersion when matchingArches was last updated
	curMatchIdx    int    // index into matchingArches
	curIdx         int    // index into the current archetype's entity/component array
	curEnt         Entity
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
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 {
		panic("ecs: duplicate component types in Filter4")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)
	m.set(id4)

	f := &Filter4[T1, T2, T3, T4]{world: w, mask: m, id1: id1, id2: id2, id3: id3, id4: id4, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
	return f
}

// New is a convenience function that creates a new filter instance.
func (f *Filter4[T1, T2, T3, T4]) New(w *World) *Filter4[T1, T2, T3, T4] {
	return NewFilter4[T1, T2, T3, T4](w)
}

// updateMatching rebuilds the filter's list of archetypes that match its
// component mask.
func (f *Filter4[T1, T2, T3, T4]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter4[T1, T2, T3, T4]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter4[T1, T2, T3, T4]) Next() bool {
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
func (f *Filter4[T1, T2, T3, T4]) Entity() Entity {
	return f.curEnt
}

// Get returns pointers to the 4 components (T1, T2, T3, T4) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4).
func (f *Filter4[T1, T2, T3, T4]) Get() (*T1, *T2, *T3, *T4) {
	a := f.matchingArches[f.curMatchIdx]
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[f.id1]) + uintptr(f.curIdx)*a.compSizes[f.id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[f.id2]) + uintptr(f.curIdx)*a.compSizes[f.id2])
	ptr3 := unsafe.Pointer(uintptr(a.compPointers[f.id3]) + uintptr(f.curIdx)*a.compSizes[f.id3])
	ptr4 := unsafe.Pointer(uintptr(a.compPointers[f.id4]) + uintptr(f.curIdx)*a.compSizes[f.id4])

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter4[T1, T2, T3, T4]) RemoveEntities() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
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

// This template generates the code for N-ary Filters (Filter2, Filter3, etc.).
// A Filter is a high-performance iterator (or query) that finds all entities
// possessing a specific set of components. It works by iterating directly over
// the contiguous memory blocks of matching archetypes, which is extremely fast.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types.
// - .Components: A slice of ComponentInfo structs for looping.
// - .ReturnTypes: The list of pointer types for the Get() method, e.g., "*T1, *T2".
// - .ReturnPtrs: The expression for returning the pointers, e.g., "(*T1)(p1), (*T2)(p2)".
// Filter5 provides a fast, cache-friendly iterator over all entities that
// have the 5 components: T1, T2, T3, T4, T5.
type Filter5[T1 any, T2 any, T3 any, T4 any, T5 any] struct {
	world *World
	mask  bitmask256
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8
	id5   uint8

	matchingArches []*archetype
	lastVersion    uint32 // world.archetypeVersion when matchingArches was last updated
	curMatchIdx    int    // index into matchingArches
	curIdx         int    // index into the current archetype's entity/component array
	curEnt         Entity
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
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)
	id5 := w.getCompTypeID(t5)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 {
		panic("ecs: duplicate component types in Filter5")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	m.set(id3)
	m.set(id4)
	m.set(id5)

	f := &Filter5[T1, T2, T3, T4, T5]{world: w, mask: m, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
	return f
}

// New is a convenience function that creates a new filter instance.
func (f *Filter5[T1, T2, T3, T4, T5]) New(w *World) *Filter5[T1, T2, T3, T4, T5] {
	return NewFilter5[T1, T2, T3, T4, T5](w)
}

// updateMatching rebuilds the filter's list of archetypes that match its
// component mask.
func (f *Filter5[T1, T2, T3, T4, T5]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter5[T1, T2, T3, T4, T5]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter5[T1, T2, T3, T4, T5]) Next() bool {
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
func (f *Filter5[T1, T2, T3, T4, T5]) Entity() Entity {
	return f.curEnt
}

// Get returns pointers to the 5 components (T1, T2, T3, T4, T5) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5).
func (f *Filter5[T1, T2, T3, T4, T5]) Get() (*T1, *T2, *T3, *T4, *T5) {
	a := f.matchingArches[f.curMatchIdx]
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[f.id1]) + uintptr(f.curIdx)*a.compSizes[f.id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[f.id2]) + uintptr(f.curIdx)*a.compSizes[f.id2])
	ptr3 := unsafe.Pointer(uintptr(a.compPointers[f.id3]) + uintptr(f.curIdx)*a.compSizes[f.id3])
	ptr4 := unsafe.Pointer(uintptr(a.compPointers[f.id4]) + uintptr(f.curIdx)*a.compSizes[f.id4])
	ptr5 := unsafe.Pointer(uintptr(a.compPointers[f.id5]) + uintptr(f.curIdx)*a.compSizes[f.id5])

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4), (*T5)(ptr5)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter5[T1, T2, T3, T4, T5]) RemoveEntities() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
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

// This template generates the code for N-ary Filters (Filter2, Filter3, etc.).
// A Filter is a high-performance iterator (or query) that finds all entities
// possessing a specific set of components. It works by iterating directly over
// the contiguous memory blocks of matching archetypes, which is extremely fast.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types.
// - .Components: A slice of ComponentInfo structs for looping.
// - .ReturnTypes: The list of pointer types for the Get() method, e.g., "*T1, *T2".
// - .ReturnPtrs: The expression for returning the pointers, e.g., "(*T1)(p1), (*T2)(p2)".
// Filter6 provides a fast, cache-friendly iterator over all entities that
// have the 6 components: T1, T2, T3, T4, T5, T6.
type Filter6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any] struct {
	world *World
	mask  bitmask256
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8
	id5   uint8
	id6   uint8

	matchingArches []*archetype
	lastVersion    uint32 // world.archetypeVersion when matchingArches was last updated
	curMatchIdx    int    // index into matchingArches
	curIdx         int    // index into the current archetype's entity/component array
	curEnt         Entity
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
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)
	id5 := w.getCompTypeID(t5)
	id6 := w.getCompTypeID(t6)

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

	f := &Filter6[T1, T2, T3, T4, T5, T6]{world: w, mask: m, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5, id6: id6, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
	return f
}

// New is a convenience function that creates a new filter instance.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) New(w *World) *Filter6[T1, T2, T3, T4, T5, T6] {
	return NewFilter6[T1, T2, T3, T4, T5, T6](w)
}

// updateMatching rebuilds the filter's list of archetypes that match its
// component mask.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Next() bool {
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
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Entity() Entity {
	return f.curEnt
}

// Get returns pointers to the 6 components (T1, T2, T3, T4, T5, T6) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6).
func (f *Filter6[T1, T2, T3, T4, T5, T6]) Get() (*T1, *T2, *T3, *T4, *T5, *T6) {
	a := f.matchingArches[f.curMatchIdx]
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[f.id1]) + uintptr(f.curIdx)*a.compSizes[f.id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[f.id2]) + uintptr(f.curIdx)*a.compSizes[f.id2])
	ptr3 := unsafe.Pointer(uintptr(a.compPointers[f.id3]) + uintptr(f.curIdx)*a.compSizes[f.id3])
	ptr4 := unsafe.Pointer(uintptr(a.compPointers[f.id4]) + uintptr(f.curIdx)*a.compSizes[f.id4])
	ptr5 := unsafe.Pointer(uintptr(a.compPointers[f.id5]) + uintptr(f.curIdx)*a.compSizes[f.id5])
	ptr6 := unsafe.Pointer(uintptr(a.compPointers[f.id6]) + uintptr(f.curIdx)*a.compSizes[f.id6])

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4), (*T5)(ptr5), (*T6)(ptr6)
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter6[T1, T2, T3, T4, T5, T6]) RemoveEntities() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
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
