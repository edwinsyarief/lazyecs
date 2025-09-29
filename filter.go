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
