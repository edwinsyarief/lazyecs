package lazyecs

import (
	"reflect"
	"unsafe"
)

//----------------------------------------
// Filter for 1 component
//----------------------------------------

// Filter provides a fast iterator over entities with component T.
type Filter[T any] struct {
	world          *World
	mask           bitmask256
	compID         uint8
	matchingArches []*archetype
	lastVersion    uint32
	curMatchIdx    int
	curIdx         int
	curEnt         Entity
}

// NewFilter creates a filter for entities with component T.
func NewFilter[T any](w *World) *Filter[T] {
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	var m bitmask256
	m.set(id)
	f := &Filter[T]{world: w, mask: m, compID: id, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
	return f
}

// updateMatching updates the list of matching archetypes.
func (f *Filter[T]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// Reset resets the filter iterator.
func (f *Filter[T]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
}

// Next advances to the next entity with the component, returning true if found.
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

// Entity returns the current entity.
func (f *Filter[T]) Entity() Entity {
	return f.curEnt
}

// Get returns a pointer to the current component T.
func (f *Filter[T]) Get() *T {
	a := f.matchingArches[f.curMatchIdx]
	ptr := unsafe.Pointer(uintptr(a.compPointers[f.compID]) + uintptr(f.curIdx)*a.compSizes[f.compID])
	return (*T)(ptr)
}

//----------------------------------------
// Filter for 2 components
//----------------------------------------

// Filter2 provides a fast iterator over entities with components T1 and T2.
type Filter2[T1 any, T2 any] struct {
	world          *World
	mask           bitmask256
	id1            uint8
	id2            uint8
	matchingArches []*archetype
	lastVersion    uint32
	curMatchIdx    int
	curIdx         int
	curEnt         Entity
}

// NewFilter2 creates a filter for entities with components T1 and T2.
func NewFilter2[T1 any, T2 any](w *World) *Filter2[T1, T2] {
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	if id1 == id2 {
		panic("ecs: duplicate component types in Filter2")
	}
	var m bitmask256
	m.set(id1)
	m.set(id2)
	f := &Filter2[T1, T2]{world: w, mask: m, id1: id1, id2: id2, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
	return f
}

// updateMatching updates the list of matching archetypes.
func (f *Filter2[T1, T2]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// Reset resets the filter iterator.
func (f *Filter2[T1, T2]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
}

// Next advances to the next entity with the components, returning true if found.
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

// Entity returns the current entity.
func (f *Filter2[T1, T2]) Entity() Entity {
	return f.curEnt
}

// Get returns pointers to the current components T1 and T2.
func (f *Filter2[T1, T2]) Get() (*T1, *T2) {
	a := f.matchingArches[f.curMatchIdx]
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[f.id1]) + uintptr(f.curIdx)*a.compSizes[f.id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[f.id2]) + uintptr(f.curIdx)*a.compSizes[f.id2])
	return (*T1)(ptr1), (*T2)(ptr2)
}
