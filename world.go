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
		typ := w.components.compIDToType[cid]
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
	resources       *Resources
	archetypes      archetypeRegistry
	entities        entityRegistry
	components      componentRegistry
	mutationVersion uint32 // incremented on entity mutations
}
type componentRegistry struct {
	compIDToType   [MaxComponentTypes]reflect.Type
	compTypeMap    map[reflect.Type]uint8
	compIDToSize   [MaxComponentTypes]uintptr
	nextCompTypeID uint16 // counter for assigning new component type IDs
}
type entityRegistry struct {
	freeIDs         []uint32     // stack of recycled entity IDs
	metas           []entityMeta // stores metadata for each entity, indexed by entity ID
	capacity        int          // current maximum number of entities
	initialCapacity int          // initial capacity, used for expansion
	nextEntityVer   uint32       // version for the next created entity
}
type archetypeRegistry struct {
	maskToArcIndex   map[bitmask256]int // lookup mask→archetype index
	archetypes       []*archetype       // list of all archetypes in the world
	archetypeVersion uint32             // incremented when a new archetype is created
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
		resources: &Resources{},
		components: componentRegistry{
			compTypeMap: make(map[reflect.Type]uint8, 16),
		},
		entities: entityRegistry{
			capacity:        initialCapacity,
			initialCapacity: initialCapacity,
			freeIDs:         make([]uint32, initialCapacity),
			metas:           make([]entityMeta, initialCapacity),
			nextEntityVer:   1,
		},
		archetypes: archetypeRegistry{
			archetypes:     make([]*archetype, 0),
			maskToArcIndex: make(map[bitmask256]int),
		},
	}
	for i := range initialCapacity {
		w.entities.freeIDs[i] = uint32(initialCapacity - 1 - i)
		w.entities.metas[i].archetypeIndex = -1
		w.entities.metas[i].index = -1
		w.entities.metas[i].version = 0
	}
	// Create the empty archetype
	var emptyMask bitmask256
	w.getOrCreateArchetype(emptyMask, []compSpec{})
	w.archetypes.archetypeVersion = 1
	w.entities.nextEntityVer = 1
	return w
}

// CreateEntity creates a new entity with no components.
func (w *World) CreateEntity() Entity {
	a := w.archetypes.archetypes[0] // empty archetype
	w.mutationVersion++
	return w.createEntity(a)
}

// CreateEntities creates a batch of entities with no components and returns their IDs.
func (w *World) CreateEntities(count int) []Entity {
	if count == 0 {
		return nil
	}
	a := w.archetypes.archetypes[0]
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion++
	return a.entityIDs[startSize : startSize+count]
}

// RemoveEntity removes an entity, recycling its ID.
func (w *World) RemoveEntity(e Entity) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = -1
	meta.index = -1
	meta.version = 0
	w.entities.freeIDs = append(w.entities.freeIDs, e.ID)
	w.mutationVersion++
}

// RemoveEntities removes multiple entities.
func (w *World) RemoveEntities(ents []Entity) {
	for _, e := range ents {
		w.RemoveEntity(e)
	}
}

// ClearEntities removes all entities from the world.
func (w *World) ClearEntities() {
	for i := range w.entities.metas {
		w.entities.metas[i].archetypeIndex = -1
		w.entities.metas[i].index = -1
		w.entities.metas[i].version = 0
	}
	w.entities.freeIDs = w.entities.freeIDs[:0]
	for i := 0; i < w.entities.capacity; i++ {
		w.entities.freeIDs = append(w.entities.freeIDs, uint32(w.entities.capacity-1-i))
	}
	for _, a := range w.archetypes.archetypes {
		a.size = 0
	}
	w.entities.nextEntityVer = 1
	w.mutationVersion++
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
	if int(e.ID) >= len(w.entities.metas) {
		return false
	}
	meta := w.entities.metas[e.ID]
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
	if id, ok := w.components.compTypeMap[t]; ok {
		return id
	}
	if w.components.nextCompTypeID >= MaxComponentTypes {
		panic("ecs: too many component types")
	}
	id := uint8(w.components.nextCompTypeID)
	w.components.compTypeMap[t] = id
	w.components.compIDToType[id] = t
	w.components.compIDToSize[id] = t.Size()
	w.components.nextCompTypeID++
	return id
}

// getOrCreateArchetype returns an archetype for the given mask;
// if missing, allocates component storage arrays of length cap.
func (w *World) getOrCreateArchetype(mask bitmask256, specs []compSpec) *archetype {
	if idx, ok := w.archetypes.maskToArcIndex[mask]; ok {
		return w.archetypes.archetypes[idx]
	}
	// build new archetype
	a := &archetype{
		index:     len(w.archetypes.archetypes),
		mask:      mask,
		size:      0,
		entityIDs: make([]Entity, w.entities.capacity),
		compOrder: make([]uint8, 0, len(specs)),
	}
	for _, sp := range specs {
		// allocate []T of length=cap
		slice := reflect.MakeSlice(reflect.SliceOf(sp.typ), w.entities.capacity, w.entities.capacity)
		a.compPointers[sp.id] = slice.UnsafePointer()
		a.compSizes[sp.id] = sp.size
		a.compOrder = append(a.compOrder, sp.id)
	}
	w.archetypes.archetypes = append(w.archetypes.archetypes, a)
	w.archetypes.maskToArcIndex[mask] = a.index
	w.archetypes.archetypeVersion++
	return a
}

// expand automatically increases capacity by initialCapacity when full.
func (w *World) expand() {
	oldCap := w.entities.capacity
	newCap := oldCap * 2
	if newCap == 0 {
		newCap = 1
	}
	delta := newCap - oldCap
	// extend metas
	newMetas := make([]entityMeta, delta)
	for i := range newMetas {
		newMetas[i].archetypeIndex = -1
		newMetas[i].index = -1
		newMetas[i].version = 0
	}
	w.entities.metas = append(w.entities.metas, newMetas...)
	// extend freeIDs with new IDs in reverse order
	newFree := make([]uint32, delta)
	for i := range delta {
		newFree[i] = uint32(newCap - 1 - i)
	}
	w.entities.freeIDs = append(w.entities.freeIDs, newFree...)
	w.entities.capacity = newCap
	// resize all archetypes
	for _, a := range w.archetypes.archetypes {
		a.resizeTo(newCap, w)
	}
}

// createEntity bumps an entity into the given archetype.
// Zero allocations on hot path.
func (w *World) createEntity(a *archetype) Entity {
	if len(w.entities.freeIDs) == 0 {
		w.expand()
	}
	// pop an ID
	last := len(w.entities.freeIDs) - 1
	id := w.entities.freeIDs[last]
	w.entities.freeIDs = w.entities.freeIDs[:last]
	meta := &w.entities.metas[id]
	meta.archetypeIndex = a.index
	meta.index = a.size
	meta.version = w.entities.nextEntityVer
	ent := Entity{ID: id, Version: meta.version}
	// place into archetype
	a.entityIDs[a.size] = ent
	a.size++
	w.entities.nextEntityVer++
	w.mutationVersion++
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
		w.entities.metas[lastEnt.ID].index = idx
	}
	a.size--
	w.mutationVersion++
}

// memCopy copies size bytes from src to dst using word-by-word copy for performance.
func memCopy(dst, src unsafe.Pointer, size uintptr) {
	wordSize := unsafe.Sizeof(uintptr(0))
	words := size / wordSize
	d := dst
	s := src
	for i := range words {
		_ = i
		*(*uintptr)(d) = *(*uintptr)(s)
		d = unsafe.Add(d, wordSize)
		s = unsafe.Add(s, wordSize)
	}
	rem := size % wordSize
	for i := range rem {
		_ = i
		*(*byte)(d) = *(*byte)(s)
		d = unsafe.Add(d, 1)
		s = unsafe.Add(s, 1)
	}
}
