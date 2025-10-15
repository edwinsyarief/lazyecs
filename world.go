package teishoku

import (
	"reflect"
	"unsafe"
)

// MaxComponentTypes defines the maximum number of unique component types that can be
// registered in a World. This value is fixed at 256.
const MaxComponentTypes = 256

// Entity represents a unique identifier for an object in the World. It combines
// a 32-bit ID with a 32-bit version to ensure that recycled IDs are not confused
// with new entities.
type Entity struct {
	// ID is the unique, recyclable identifier for the entity.
	ID uint32
	// Version is a generation counter to protect against stale entity references.
	// It is incremented each time an entity ID is reused.
	Version uint32
}

// entityMeta holds the internal location and state of an entity.
type entityMeta struct {
	archetypeIndex int    // index in World.archetypes
	index          int    // position inside the archetype's component arrays
	version        uint32 // current version, 0 if the entity is dead
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

// World is the central container for all entities, components, and archetypes.
// It manages the entire state of the ECS, including entity creation, deletion,
// and component management. All operations are performed within the context of a
// World. The World is not thread-safe and should not be accessed from
// multiple goroutines concurrently.
type World struct {
	// Resources provides a thread-safe, generic key-value store for global data
	// that needs to be accessible from anywhere in the application, such as
	// configuration objects, resource managers, or event buses.
	resources *Resources

	compIDToType     [MaxComponentTypes]reflect.Type
	maskToArcIndex   map[bitmask256]int // lookup mask→archetype index
	compTypeMap      map[reflect.Type]uint8
	freeIDs          []uint32     // stack of recycled entity IDs
	metas            []entityMeta // stores metadata for each entity, indexed by entity ID
	archetypes       []*archetype // list of all archetypes in the world
	compIDToSize     [MaxComponentTypes]uintptr
	capacity         int    // current maximum number of entities
	initialCapacity  int    // initial capacity, used for expansion
	nextEntityVer    uint32 // version for the next created entity
	archetypeVersion uint32 // incremented when a new archetype is created
	nextCompTypeID   uint16 // counter for assigning new component type IDs
}

// NewWorld creates and initializes a new World with a specified initial
// capacity for entities. It pre-allocates memory for the entity metadata and
// free ID list to optimize performance.
//
// Parameters:
//   - initialCapacity: The number of entities to pre-allocate memory for.
//     Choosing a suitable capacity can prevent re-allocations during runtime.
//
// Returns:
//   - A pointer to the newly created World.
func NewWorld(initialCapacity int) World {
	w := World{
		resources:        &Resources{},
		capacity:         initialCapacity,
		initialCapacity:  initialCapacity,
		freeIDs:          make([]uint32, initialCapacity),
		metas:            make([]entityMeta, initialCapacity),
		archetypes:       make([]*archetype, 0),
		maskToArcIndex:   make(map[bitmask256]int),
		compTypeMap:      make(map[reflect.Type]uint8, 16),
		nextCompTypeID:   0,
		nextEntityVer:    1,
		archetypeVersion: 0,
	}
	for i := 0; i < initialCapacity; i++ {
		// fill freeIDs with [cap-1 .. 0]
		w.freeIDs[i] = uint32(initialCapacity - 1 - i)
	}
	for i := range w.metas {
		w.metas[i].archetypeIndex = -1
		w.metas[i].version = 0
	}
	// Pre-create empty archetype for zero-component entities
	w.getOrCreateArchetype(bitmask256{}, nil)
	return w
}

// CreateEntity creates a new entity with no components.
func (w *World) CreateEntity() Entity {
	var mask bitmask256
	var specs []compSpec
	a := w.getOrCreateArchetype(mask, specs)
	return w.createEntity(a)
}

// CreateEntities creates a batch of entities with no components and returns them.
// The returned slice is a view into internal storage; do not modify its contents.
func (w *World) CreateEntities(count int) []Entity {
	if count == 0 {
		return nil
	}
	var mask bitmask256
	var specs []compSpec
	a := w.getOrCreateArchetype(mask, specs)
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.nextEntityVer++
	}
	return a.entityIDs[startSize : startSize+count]
}

// RemoveEntity removes an entity from the World, freeing its ID for reuse and
// invalidating all references to it. If the entity is invalid or already removed,
// this operation does nothing.
//
// Parameters:
//   - e: The Entity to remove.
func (w *World) RemoveEntity(e Entity) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	idx := meta.index
	lastIdx := a.size - 1
	if idx < lastIdx {
		lastEnt := a.entityIDs[lastIdx]
		a.entityIDs[idx] = lastEnt
		for _, cid := range a.compOrder {
			src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(lastIdx)*a.compSizes[cid])
			dst := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(idx)*a.compSizes[cid])
			memCopy(dst, src, a.compSizes[cid])
		}
		w.metas[lastEnt.ID].index = idx
	}
	// Invalidate the removed entity's metadata and recycle its ID.
	a.size--
	w.freeIDs = append(w.freeIDs, e.ID)
	meta.archetypeIndex = -1
	meta.index = -1
	meta.version = 0 // Mark as dead
}

// RemoveEntities removes multiple entities in a batch. It processes each entity
// individually but is convenient for removing lists of entities. Invalid entities
// are skipped. This operation incurs no additional allocations beyond what
// RemoveEntity would for each.
//
// Parameters:
//   - ents: A slice of Entities to remove.
func (w *World) RemoveEntities(ents []Entity) {
	for _, e := range ents {
		w.RemoveEntity(e)
	}
}

// ClearEntities removes all entities from the World, resetting it to an empty state
// while preserving archetypes and component registrations. This is extremely efficient,
// as it batch-invalidates all metadata and resets free IDs without per-entity operations.
//
// After calling this, all previous Entity references become invalid.
func (w *World) ClearEntities() {
	w.freeIDs = w.freeIDs[:0]
	for i := 0; i < w.capacity; i++ {
		w.freeIDs = append(w.freeIDs, uint32(w.capacity-1-i))
	}
	for i := range w.metas {
		w.metas[i].archetypeIndex = -1
		w.metas[i].index = -1
		w.metas[i].version = 0
	}
	for _, a := range w.archetypes {
		a.size = 0
	}
	w.nextEntityVer = 1
}

// IsValid checks if an entity reference is still valid (i.e., it has not been
// removed). It verifies that the entity's ID is within bounds and that its
// version matches the current version stored in the world's metadata.
//
// Parameters:
//   - e: The Entity to validate.
//
// Returns:
//   - true if the entity is valid, false otherwise.
func (w *World) IsValid(e Entity) bool {
	if int(e.ID) >= len(w.metas) {
		return false
	}
	meta := w.metas[e.ID]
	return meta.version != 0 && meta.version == e.Version
}

// Resources returns the world's resource manager. It provides a thread-safe,
// generic key-value store for global data that needs to be accessible from
// anywhere in the application, such as configuration objects, resource managers,
// or event buses.
//
// Returns:
//   - A pointer to the Resources object.
func (w *World) Resources() *Resources {
	return w.resources
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
	w.compIDToSize[id] = t.Size()
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
		compOrder: make([]uint8, 0, len(specs)),
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

// removeFromArchetype removes the entity from the archetype without freeing the ID or invalidating version.
func (w *World) removeFromArchetype(a *archetype, meta *entityMeta) {
	idx := meta.index
	lastIdx := a.size - 1
	if idx < lastIdx {
		lastEnt := a.entityIDs[lastIdx]
		a.entityIDs[idx] = lastEnt
		for _, cid := range a.compOrder {
			src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(lastIdx)*a.compSizes[cid])
			dst := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(idx)*a.compSizes[cid])
			memCopy(dst, src, a.compSizes[cid])
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
