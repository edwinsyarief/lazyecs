package teishoku

import (
	"reflect"
	"unsafe"
)

// MaxComponentTypes defines the maximum number of unique component types that can be
// registered in a World. This value is fixed at 256.
const MaxComponentTypes = 256

const ChunkSize = 1024

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
	chunkIndex     int    // index in archetype.chunks
	index          int    // position inside the chunk's component array
	version        uint32 // current version, 0 if the entity is dead
}

// compSpec bundles a component type’s ID and reflect.Type.
type compSpec struct {
	typ  reflect.Type
	size uintptr
	id   uint8
}

// chunk holds fixed-size storage for ChunkSize entities.
type chunk struct {
	entityIDs    [ChunkSize]Entity
	compPointers [MaxComponentTypes]unsafe.Pointer
	size         int // number of entities in this chunk, 0 to ChunkSize
}

// archetype holds storage for one unique component-set mask.
type archetype struct {
	chunks    []*chunk
	compOrder []uint8 // list of component IDs in this arch
	compSizes [MaxComponentTypes]uintptr
	mask      bitmask256 // which component bits this arch uses
	index     int        // position in world.archetypes
	size      int        // total entity count across chunks
}

// componentRegistry ...
type componentRegistry struct {
	compIDToType   [MaxComponentTypes]reflect.Type
	compTypeMap    map[reflect.Type]uint8
	compIDToSize   [MaxComponentTypes]uintptr
	nextCompTypeID uint16 // counter for assigning new component type IDs
}

// entityRegistry ...
type entityRegistry struct {
	freeIDs         []uint32     // stack of recycled entity IDs
	metas           []entityMeta // stores metadata for each entity, indexed by entity ID
	capacity        int          // current maximum number of entities
	initialCapacity int          // initial capacity, used for expansion
	nextEntityVer   uint32       // version for the next created entity
}

// archetypeRegistry ...
type archetypeRegistry struct {
	maskToArcIndex   map[bitmask256]int // lookup mask→archetype index
	archetypes       []*archetype       // list of all archetypes in the world
	archetypeVersion uint32             // incremented when a new archetype is created
}

// World ...
type World struct {
	resources       *Resources
	archetypes      archetypeRegistry
	entities        entityRegistry
	components      componentRegistry
	mutationVersion uint32 // incremented on entity mutations
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
//   - The newly created World.
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
			maskToArcIndex: make(map[bitmask256]int),
			archetypes:     make([]*archetype, 0, 16),
		},
	}
	for i := range w.entities.freeIDs {
		w.entities.freeIDs[i] = uint32(initialCapacity - 1 - i)
	}
	for i := range w.entities.metas {
		w.entities.metas[i].archetypeIndex = -1
		w.entities.metas[i].chunkIndex = -1
		w.entities.metas[i].index = -1
		w.entities.metas[i].version = 0
	}
	// Pre-create the empty archetype
	var emptyMask bitmask256
	w.getOrCreateArchetype(emptyMask, []compSpec{})
	return w
}

// ClearEntities removes all entities from the world, recycling their IDs and
// resetting archetypes. This is an efficient way to reset the world state
// without deallocating memory.
func (w *World) ClearEntities() {
	for i := range w.entities.metas {
		w.entities.metas[i].archetypeIndex = -1
		w.entities.metas[i].chunkIndex = -1
		w.entities.metas[i].index = -1
		w.entities.metas[i].version = 0
	}
	w.entities.freeIDs = w.entities.freeIDs[:0]
	for i := uint32(0); i < uint32(w.entities.capacity); i++ {
		w.entities.freeIDs = append(w.entities.freeIDs, i)
	}
	for _, a := range w.archetypes.archetypes {
		a.chunks = a.chunks[:0]
		a.size = 0
	}
	w.mutationVersion++
}

// IsValid checks if the entity is currently alive in the world. An entity is
// valid if its ID is within bounds and its version matches the world's current
// version for that ID. This prevents "stale" entity references from accessing
// incorrect data after an entity has been deleted and its ID recycled.
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

// getCompTypeID register or fetch a component type ID for T.
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
		chunks:    make([]*chunk, 0, 4),
		compOrder: make([]uint8, len(specs)),
	}
	for i, sp := range specs {
		a.compOrder[i] = sp.id
		a.compSizes[sp.id] = sp.size
	}
	w.archetypes.archetypes = append(w.archetypes.archetypes, a)
	w.archetypes.maskToArcIndex[mask] = a.index
	w.archetypes.archetypeVersion++
	return a
}

// newChunk creates a new chunk for the archetype.
func (w *World) newChunk(a *archetype) *chunk {
	c := &chunk{}
	for _, cid := range a.compOrder {
		typ := w.components.compIDToType[cid]
		slice := reflect.MakeSlice(reflect.SliceOf(typ), ChunkSize, ChunkSize)
		c.compPointers[cid] = slice.UnsafePointer()
	}
	return c
}

// expand automatically increases capacity when full.
func (w *World) expand(additional int) {
	oldCap := w.entities.capacity
	newCap := oldCap * 2
	if newCap == 0 {
		newCap = 1
	}
	if newCap < oldCap+additional {
		newCap = oldCap + additional
	}
	delta := newCap - oldCap
	newMetas := make([]entityMeta, delta)
	for i := range newMetas {
		newMetas[i].archetypeIndex = -1
		newMetas[i].chunkIndex = -1
		newMetas[i].index = -1
		newMetas[i].version = 0
	}
	w.entities.metas = append(w.entities.metas, newMetas...)
	newFree := make([]uint32, delta)
	for i := range delta {
		newFree[i] = uint32(newCap - 1 - i)
	}
	w.entities.freeIDs = append(w.entities.freeIDs, newFree...)
	w.entities.capacity = newCap
}

// createEntity bumps an entity into the given archetype.
// Zero allocations on hot path.
func (w *World) createEntity(a *archetype) Entity {
	if len(w.entities.freeIDs) == 0 {
		w.expand(1)
	}
	// pop an ID
	last := len(w.entities.freeIDs) - 1
	id := w.entities.freeIDs[last]
	w.entities.freeIDs = w.entities.freeIDs[:last]
	if len(a.chunks) == 0 || a.chunks[len(a.chunks)-1].size == ChunkSize {
		a.chunks = append(a.chunks, w.newChunk(a))
	}
	lastC := a.chunks[len(a.chunks)-1]
	idx := lastC.size
	meta := &w.entities.metas[id]
	meta.archetypeIndex = a.index
	meta.chunkIndex = len(a.chunks) - 1
	meta.index = idx
	meta.version = w.entities.nextEntityVer
	ent := Entity{ID: id, Version: meta.version}
	// place into archetype
	lastC.entityIDs[idx] = ent
	lastC.size++
	a.size++
	w.entities.nextEntityVer++
	w.mutationVersion++
	return ent
}

// CreateEntity creates a new entity with no components.
func (w *World) CreateEntity() Entity {
	emptyMask := bitmask256{}
	idx, ok := w.archetypes.maskToArcIndex[emptyMask]
	if !ok {
		panic("ecs: empty archetype not found")
	}
	a := w.archetypes.archetypes[idx]
	return w.createEntity(a)
}

// CreateEntities creates a batch of entities with no components and returns their IDs.
func (w *World) CreateEntities(count int) []Entity {
	if count == 0 {
		return nil
	}
	emptyMask := bitmask256{}
	idx, ok := w.archetypes.maskToArcIndex[emptyMask]
	if !ok {
		panic("ecs: empty archetype not found")
	}
	a := w.archetypes.archetypes[idx]
	ents := make([]Entity, count)
	remaining := count
	for remaining > 0 {
		if len(a.chunks) == 0 || a.chunks[len(a.chunks)-1].size == ChunkSize {
			a.chunks = append(a.chunks, w.newChunk(a))
		}
		lastC := a.chunks[len(a.chunks)-1]
		avail := ChunkSize - lastC.size
		batch := min(avail, remaining)
		if len(w.entities.freeIDs) < batch {
			w.expand(batch - len(w.entities.freeIDs) + 1)
		}
		startIdx := lastC.size
		popped := w.entities.freeIDs[len(w.entities.freeIDs)-batch:]
		w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-batch]
		for k := 0; k < batch; k++ {
			id := popped[k]
			meta := &w.entities.metas[id]
			meta.archetypeIndex = a.index
			meta.chunkIndex = len(a.chunks) - 1
			meta.index = startIdx + k
			meta.version = w.entities.nextEntityVer
			ent := Entity{ID: id, Version: meta.version}
			lastC.entityIDs[startIdx+k] = ent
			ents[count-remaining+k] = ent
			w.entities.nextEntityVer++
		}
		lastC.size += batch
		a.size += batch
		remaining -= batch
	}
	w.mutationVersion++
	return ents
}

// RemoveEntity removes a single entity.
func (w *World) RemoveEntity(e Entity) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = -1
	meta.chunkIndex = -1
	meta.index = -1
	meta.version = 0
	w.entities.freeIDs = append(w.entities.freeIDs, e.ID)
	w.mutationVersion++
}

// RemoveEntities removes a batch of entities.
func (w *World) RemoveEntities(ents []Entity) {
	for _, e := range ents {
		w.RemoveEntity(e)
	}
}

// removeFromArchetype removes the entity from the archetype without freeing the ID or invalidating version.
func (w *World) removeFromArchetype(a *archetype, meta *entityMeta) {
	chunkIdx := meta.chunkIndex
	chunk := a.chunks[chunkIdx]
	idx := meta.index
	lastIdx := chunk.size - 1
	if idx < lastIdx {
		lastEnt := chunk.entityIDs[lastIdx]
		chunk.entityIDs[idx] = lastEnt
		for _, cid := range a.compOrder {
			size := a.compSizes[cid]
			src := unsafe.Pointer(uintptr(chunk.compPointers[cid]) + uintptr(lastIdx)*size)
			dst := unsafe.Pointer(uintptr(chunk.compPointers[cid]) + uintptr(idx)*size)
			memCopy(dst, src, size)
		}
		w.entities.metas[lastEnt.ID].index = idx
	}
	chunk.size--
	a.size--
	if chunk.size == 0 {
		lastChunkIdx := len(a.chunks) - 1
		if chunkIdx < lastChunkIdx {
			a.chunks[chunkIdx] = a.chunks[lastChunkIdx]
			swappedChunk := a.chunks[chunkIdx]
			for j := 0; j < swappedChunk.size; j++ {
				ent := swappedChunk.entityIDs[j]
				w.entities.metas[ent.ID].chunkIndex = chunkIdx
			}
		}
		a.chunks = a.chunks[:lastChunkIdx]
	}
	w.mutationVersion++
}

// memCopy copies size bytes from src to dst using built-in copy for performance.
func memCopy(dst, src unsafe.Pointer, size uintptr) {
	if size == 0 {
		return
	}
	dstBytes := unsafe.Slice((*byte)(dst), size)
	srcBytes := unsafe.Slice((*byte)(src), size)
	copy(dstBytes, srcBytes)
}
