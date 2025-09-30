// Package lazyecs implements a high-performance, zero-allocation,
// archetype-based Entity Component System for Go.
//
// Features:
// - Archetype-based storage with max 256 component types.
// - Bitmask for fast archetype lookup.
// - Unsafe pointers for zero-GC overhead on transactions.
// - Preallocated pools for entities and component arrays.
// - Simple, generic Builder and Filter APIs for 1 or 2 components.
// - Zero allocations (B/op and allocs/op = 0) during Create, Get, Query.
//
//go:generate go run ./cmd/generate
package lazyecs

import (
	"reflect"
	"sync"
	"unsafe"
)

// ----------------------------------------
// Constants and Types
// ----------------------------------------
const MaxComponentTypes = 256

// Entity is a unique ID + version tag.
type Entity struct {
	ID      uint32
	Version uint32
}

// entityMeta holds where an entity lives.
type entityMeta struct {
	archetypeIndex int    // index in World.archetypes
	index          int    // position inside archetype
	version        uint32 // current version
}

// compSpec bundles a component type’s ID and reflect.Type.
type compSpec struct {
	typ  reflect.Type
	size uintptr
	id   uint8
}

// archetype holds storage for one unique component-set mask.
type archetype struct {
	compPointers [MaxComponentTypes]unsafe.Pointer
	entityIDs    []Entity // prealloc len=cap
	compOrder    []uint8  // list of component IDs in this arch
	compSizes    [MaxComponentTypes]uintptr
	mask         bitmask256 // which component bits this arch uses
	index        int        // position in world.archetypes
	size         int        // current entity count
}

// ----------------------------------------
// World
// ----------------------------------------
type World struct {
	compIDToType     [MaxComponentTypes]reflect.Type
	maskToArcIndex   map[bitmask256]int // lookup mask→archetype index
	compTypeMap      map[reflect.Type]uint8
	Resources        sync.Map
	freeIDs          []uint32     // stack of free entity IDs
	metas            []entityMeta // len = capacity
	archetypes       []*archetype // all archetypes
	capacity         int
	initialCapacity  int
	nextEntityVer    uint32
	archetypeVersion uint32
	nextCompTypeID   uint16
}

// NewWorld preallocates pools for up to cap entities.
func NewWorld(cap int) *World {
	w := &World{
		capacity:         cap,
		initialCapacity:  cap,
		freeIDs:          make([]uint32, cap),
		metas:            make([]entityMeta, cap),
		archetypes:       make([]*archetype, 0),
		maskToArcIndex:   make(map[bitmask256]int),
		compTypeMap:      make(map[reflect.Type]uint8, 16),
		nextCompTypeID:   0,
		nextEntityVer:    1,
		archetypeVersion: 0,
	}
	for i := 0; i < cap; i++ {
		// fill freeIDs with [cap-1 .. 0]
		w.freeIDs[i] = uint32(cap - 1 - i)
	}
	for i := range w.metas {
		w.metas[i].archetypeIndex = -1
		w.metas[i].version = 0
	}
	return w
}

// register or fetch a component type ID for T.
func (w *World) getCompTypeID(t reflect.Type) uint8 {
	if id, ok := w.compTypeMap[t]; ok {
		return id
	}
	if w.nextCompTypeID >= MaxComponentTypes {
		panic("ecs: too many component types")
	}
	id := uint8(w.nextCompTypeID)
	w.compTypeMap[t] = id
	w.compIDToType[id] = t
	w.nextCompTypeID++
	return id
}

// getOrCreateArchetype returns an archetype for the given mask;
// if missing, allocates component storage arrays of length cap.
func (w *World) getOrCreateArchetype(mask bitmask256, specs []compSpec) *archetype {
	if idx, ok := w.maskToArcIndex[mask]; ok {
		return w.archetypes[idx]
	}
	// build new archetype
	a := &archetype{
		index:     len(w.archetypes),
		mask:      mask,
		size:      0,
		entityIDs: make([]Entity, w.capacity),
	}
	for _, sp := range specs {
		// allocate []T of length=cap
		slice := reflect.MakeSlice(reflect.SliceOf(sp.typ), w.capacity, w.capacity)
		a.compPointers[sp.id] = slice.UnsafePointer()
		a.compSizes[sp.id] = sp.size
		a.compOrder = append(a.compOrder, sp.id)
	}
	w.archetypes = append(w.archetypes, a)
	w.maskToArcIndex[mask] = a.index
	w.archetypeVersion++
	return a
}

// expand automatically increases capacity by initialCapacity when full.
func (w *World) expand() {
	oldCap := w.capacity
	newCap := oldCap * 2
	if newCap == 0 {
		newCap = 1
	}
	delta := newCap - oldCap
	// extend metas
	newMetas := make([]entityMeta, delta)
	for i := range newMetas {
		newMetas[i].archetypeIndex = -1
		newMetas[i].version = 0
	}
	w.metas = append(w.metas, newMetas...)
	// extend freeIDs with new IDs in reverse order
	newFree := make([]uint32, delta)
	for i := 0; i < delta; i++ {
		newFree[i] = uint32(newCap - 1 - i)
	}
	w.freeIDs = append(w.freeIDs, newFree...)
	w.capacity = newCap
	// resize all archetypes
	for _, a := range w.archetypes {
		a.resizeTo(newCap, w)
	}
}

// resizeTo resizes the archetype's storage to newCap, copying existing data.
func (a *archetype) resizeTo(newCap int, w *World) {
	if cap(a.entityIDs) >= newCap {
		return
	}
	// resize entityIDs
	newEnts := make([]Entity, newCap)
	copy(newEnts[:a.size], a.entityIDs[:a.size])
	a.entityIDs = newEnts
	// resize comps
	for _, cid := range a.compOrder {
		typ := w.compIDToType[cid]
		newSlice := reflect.MakeSlice(reflect.SliceOf(typ), newCap, newCap)
		newPtr := newSlice.UnsafePointer()
		oldPtr := a.compPointers[cid]
		bytes := uintptr(a.size) * a.compSizes[cid]
		if bytes > 0 {
			memCopy(newPtr, oldPtr, bytes)
		}
		a.compPointers[cid] = newPtr
	}
}

// createEntity bumps an entity into the given archetype.
// Zero allocations on hot path.
func (w *World) createEntity(a *archetype) Entity {
	if len(w.freeIDs) == 0 {
		w.expand()
	}
	// pop an ID
	last := len(w.freeIDs) - 1
	id := w.freeIDs[last]
	w.freeIDs = w.freeIDs[:last]
	meta := &w.metas[id]
	meta.archetypeIndex = a.index
	meta.index = a.size
	meta.version = w.nextEntityVer
	ent := Entity{ID: id, Version: meta.version}
	// place into archetype
	a.entityIDs[a.size] = ent
	a.size++
	w.nextEntityVer++
	return ent
}

// RemoveEntity deletes e from its archetype, swaps last element in.
// Zero allocations on hot path.
func (w *World) RemoveEntity(e Entity) {
	meta := &w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return // already deleted or stale
	}
	a := w.archetypes[meta.archetypeIndex]
	idx := meta.index
	lastIdx := a.size - 1
	// swap last into idx
	if idx < lastIdx {
		lastEnt := a.entityIDs[lastIdx]
		a.entityIDs[idx] = lastEnt
		for _, id := range a.compOrder {
			src := unsafe.Pointer(uintptr(a.compPointers[id]) + uintptr(lastIdx)*a.compSizes[id])
			dst := unsafe.Pointer(uintptr(a.compPointers[id]) + uintptr(idx)*a.compSizes[id])
			memCopy(dst, src, a.compSizes[id])
		}
		w.metas[lastEnt.ID].index = idx
	}
	a.size--
	w.freeIDs = append(w.freeIDs, e.ID)
	meta.archetypeIndex = -1
	meta.index = -1
	meta.version = 0
}

// IsValid checks if the entity is still valid.
func (w *World) IsValid(e Entity) bool {
	if int(e.ID) >= len(w.metas) {
		return false
	}
	meta := w.metas[e.ID]
	return meta.version != 0 && meta.version == e.Version
}

// removeFromArchetype removes the entity from the archetype without freeing the ID or invalidating version.
func (w *World) removeFromArchetype(a *archetype, meta *entityMeta) {
	idx := meta.index
	lastIdx := a.size - 1
	if idx < lastIdx {
		lastEnt := a.entityIDs[lastIdx]
		a.entityIDs[idx] = lastEnt
		for _, id := range a.compOrder {
			src := unsafe.Pointer(uintptr(a.compPointers[id]) + uintptr(lastIdx)*a.compSizes[id])
			dst := unsafe.Pointer(uintptr(a.compPointers[id]) + uintptr(idx)*a.compSizes[id])
			memCopy(dst, src, a.compSizes[id])
		}
		w.metas[lastEnt.ID].index = idx
	}
	a.size--
}

// memCopy copies size bytes from src to dst using word-by-word copy for performance.
func memCopy(dst, src unsafe.Pointer, size uintptr) {
	wordSize := unsafe.Sizeof(uintptr(0))
	words := size / wordSize
	d := dst
	s := src
	for i := uintptr(0); i < words; i++ {
		*(*uintptr)(d) = *(*uintptr)(s)
		d = unsafe.Add(d, wordSize)
		s = unsafe.Add(s, wordSize)
	}
	rem := size % wordSize
	for i := uintptr(0); i < rem; i++ {
		*(*byte)(d) = *(*byte)(s)
		d = unsafe.Add(d, 1)
		s = unsafe.Add(s, 1)
	}
}
