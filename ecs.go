package lazyecs

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"unsafe"
)

// ComponentID represents a unique identifier for a component type.
// We use uint32 to allow for a reasonable number of component types without wasting space.
type ComponentID uint32

const (
	bitsPerWord            = 64                      // Number of bits in a uint64 word.
	maskWords              = 4                       // Number of words in the bitmask; supports up to 256 components.
	maxComponentTypes      = maskWords * bitsPerWord // Maximum supported component types.
	defaultInitialCapacity = 4096                    // Default initial capacity for slices; can be overridden.
)

// maskType is a bitmask for component sets.
// This fixed-size array allows fast bit operations for checking component presence.
type maskType [maskWords]uint64

// has checks if the mask has the bit set for the given ID.
// It returns false if the ID is out of range to prevent panics.
func (m maskType) has(id ComponentID) bool {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		return false
	}
	bit := id % bitsPerWord
	return (m[word] & (1 << bit)) != 0
}

// set returns a new mask with the bit set for the given ID.
// Panics if the ID exceeds the maximum allowed.
func setMask(m maskType, id ComponentID) maskType {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		panic(fmt.Sprintf("component ID %d exceeds maximum (%d)", id, maxComponentTypes))
	}
	bit := id % bitsPerWord
	nm := m
	nm[word] |= (1 << bit)
	return nm
}

// unset returns a new mask with the bit unset for the given ID.
// If ID is out of range, it returns the original mask unchanged.
func unsetMask(m maskType, id ComponentID) maskType {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		return m
	}
	bit := id % bitsPerWord
	nm := m
	nm[word] &^= (1 << bit)
	return nm
}

// makeMask creates a mask from a list of IDs.
// It iterates over the IDs and sets the corresponding bits.
func makeMask(ids []ComponentID) maskType {
	var m maskType
	for _, id := range ids {
		word := int(id / bitsPerWord)
		bit := id % bitsPerWord
		m[word] |= (1 << bit)
	}
	return m
}

// includesAll checks if m includes all bits set in include.
// This is used to verify if an archetype matches a query's required components.
func includesAll(m, include maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & include[i]) != include[i] {
			return false
		}
	}
	return true
}

// intersects checks if m and exclude have any overlapping bits.
// Used for exclusion filters in queries.
func intersects(m, exclude maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & exclude[i]) != 0 {
			return true
		}
	}
	return false
}

var (
	nextComponentID ComponentID                          // Next available component ID, starts at 0.
	typeToID        = make(map[reflect.Type]ComponentID) // Maps component types to their IDs.
	idToType        = make(map[ComponentID]reflect.Type) // Maps IDs back to component types.
	componentSizes  [maxComponentTypes]uintptr
)

// ResetGlobalRegistry resets the component registry for testing purposes.
// This clears all mappings and resets the next ID counter.
func ResetGlobalRegistry() {
	nextComponentID = 0
	typeToID = make(map[reflect.Type]ComponentID)
	idToType = make(map[ComponentID]reflect.Type)
	componentSizes = [maxComponentTypes]uintptr{}
}

// RegisterComponent registers a component type and returns its unique ID.
// It panics if the type is already registered or if we've hit the max component limit.
func RegisterComponent[T any]() ComponentID {
	var t T
	compType := reflect.TypeOf(t)

	if id, ok := typeToID[compType]; ok {
		return id
	}

	if int(nextComponentID) >= maxComponentTypes {
		panic(fmt.Sprintf("cannot register component %s: maximum number of component types (%d) reached", compType.Name(), maxComponentTypes))
	}

	id := nextComponentID
	typeToID[compType] = id
	idToType[id] = compType
	componentSizes[id] = unsafe.Sizeof(t)
	nextComponentID++
	return id
}

// GetID returns the ComponentID for a given type T.
// Panics if the type is not registered.
func GetID[T any]() ComponentID {
	var zero T
	typ := reflect.TypeOf(zero)
	id, ok := typeToID[typ]
	if !ok {
		panic(fmt.Sprintf("component type %s not registered", typ))
	}
	return id
}

// TryGetID returns the ComponentID for a given type T and whether it was found.
func TryGetID[T any]() (ComponentID, bool) {
	var zero T
	typ := reflect.TypeOf(zero)
	id, ok := typeToID[typ]
	return id, ok
}

// Entity is a unique identifier for an entity, including a version for safety.
// The version helps detect use-after-free errors.
type Entity struct {
	ID      uint32
	Version uint32
}

// entityMeta stores internal metadata for each entity.
// This includes its current archetype, index within it, and version.
type entityMeta struct {
	Archetype *Archetype
	Index     int
	Version   uint32
}

// WorldOptions allows configuring the World.
// Currently, it supports setting the initial capacity for archetypes.
type WorldOptions struct {
	InitialCapacity int // Initial capacity for slices; defaults to 4096 if <=0.
}

// World manages all entities, components, and systems.
// It uses archetypes for efficient storage and querying.
type World struct {
	nextEntityID    uint32                  // Next available entity ID.
	freeEntityIDs   []uint32                // Recycled entity IDs for reuse.
	entities        map[uint32]entityMeta   // Maps entity IDs to their metadata.
	archetypes      map[maskType]*Archetype // Maps component masks to archetypes.
	toRemove        []Entity                // Entities queued for removal.
	Resources       sync.Map                // General-purpose resource storage.
	initialCapacity int                     // Configurable initial capacity.
}

// NewWorld creates a new ECS world with default options.
// It initializes with an empty archetype.
func NewWorld() *World {
	return NewWorldWithOptions(WorldOptions{})
}

// NewWorldWithOptions creates a new ECS world with custom options.
// This allows setting initial capacities without changing the API.
func NewWorldWithOptions(opts WorldOptions) *World {
	cap := defaultInitialCapacity
	if opts.InitialCapacity > 0 {
		cap = opts.InitialCapacity
	}
	w := &World{
		entities:        make(map[uint32]entityMeta),
		archetypes:      make(map[maskType]*Archetype),
		toRemove:        make([]Entity, 0, cap),
		freeEntityIDs:   make([]uint32, 0, cap),
		initialCapacity: cap,
	}
	// Create the initial empty archetype with configurable capacity.
	w.getOrCreateArchetype(maskType{})
	return w
}

// getOrCreateArchetype finds or creates an archetype for a given mask.
func (self *World) getOrCreateArchetype(mask maskType) *Archetype {
	if arch, ok := self.archetypes[mask]; ok {
		return arch
	}

	newArch := &Archetype{
		mask:     mask,
		entities: make([]Entity, 0, self.initialCapacity),
	}

	compIDs := make([]ComponentID, 0, len(idToType))
	for id := range idToType {
		if mask.has(id) {
			compIDs = append(compIDs, id)
		}
	}

	sort.Slice(compIDs, func(i, j int) bool { return compIDs[i] < compIDs[j] })
	newArch.componentIDs = compIDs
	newArch.componentData = make([][]byte, len(compIDs))

	for i, id := range compIDs {
		size := int(componentSizes[id])
		newArch.componentData[i] = make([]byte, 0, self.initialCapacity*size)
	}

	self.archetypes[mask] = newArch
	return newArch
}

// CreateEntity creates a new entity with no components.
// It reuses free IDs if available to avoid fragmentation.
func (self *World) CreateEntity() Entity {
	var id uint32
	if len(self.freeEntityIDs) > 0 {
		id = self.freeEntityIDs[len(self.freeEntityIDs)-1]
		self.freeEntityIDs = self.freeEntityIDs[:len(self.freeEntityIDs)-1]
	} else {
		if self.nextEntityID == ^uint32(0) {
			panic("entity ID overflow")
		}
		id = self.nextEntityID
		self.nextEntityID++
	}

	meta, exists := self.entities[id]
	version := uint32(1)
	if exists {
		version = meta.Version + 1
		if version == 0 {
			version = 1 // Wrap-around safety.
		}
	}

	e := Entity{ID: id, Version: version}
	arch := self.archetypes[maskType{}]
	index := len(arch.entities)

	self.entities[id] = entityMeta{Archetype: arch, Index: index, Version: e.Version}
	arch.entities = append(arch.entities, e)

	return e
}

// CreateEntities batch-creates multiple entities with no components.
// This is more efficient than calling CreateEntity in a loop.
func (self *World) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}

	entities := make([]Entity, count)
	arch := self.archetypes[maskType{}]
	startIndex := len(arch.entities)

	for i := 0; i < count; i++ {
		var id uint32
		if len(self.freeEntityIDs) > 0 {
			id = self.freeEntityIDs[len(self.freeEntityIDs)-1]
			self.freeEntityIDs = self.freeEntityIDs[:len(self.freeEntityIDs)-1]
		} else {
			if self.nextEntityID == ^uint32(0) {
				panic("entity ID overflow")
			}
			id = self.nextEntityID
			self.nextEntityID++
		}

		meta, exists := self.entities[id]
		version := uint32(1)
		if exists {
			version = meta.Version + 1
			if version == 0 {
				version = 1
			}
		}

		e := Entity{ID: id, Version: version}
		entities[i] = e
		index := startIndex + i
		self.entities[id] = entityMeta{Archetype: arch, Index: index, Version: e.Version}
		arch.entities = append(arch.entities, e)
	}

	return entities
}

// RemoveEntity marks an entity for removal at the end of the frame.
// Actual removal is deferred to ProcessRemovals for batch processing.
func (self *World) RemoveEntity(e Entity) {
	self.toRemove = append(self.toRemove, e)
}

// ProcessRemovals cleans up entities marked for removal.
// It processes them in batch to improve efficiency.
func (self *World) ProcessRemovals() {
	if len(self.toRemove) == 0 {
		return
	}

	removeSet := make(map[uint32]Entity, len(self.toRemove))
	for _, e := range self.toRemove {
		meta, ok := self.entities[e.ID]
		if ok && e.Version == meta.Version {
			removeSet[e.ID] = e
		}
	}

	for id, e := range removeSet {
		if meta, ok := self.entities[id]; ok {
			self.removeEntityFromArchetype(e, meta.Archetype, meta.Index)
			self.freeEntityIDs = append(self.freeEntityIDs, id)
			delete(self.entities, id)
		}
	}

	self.toRemove = self.toRemove[:0]
}

// removeEntityFromArchetype performs an efficient "swap and pop" removal from an archetype.
// This avoids shifting elements by swapping with the last one and truncating.
func (self *World) removeEntityFromArchetype(e Entity, arch *Archetype, index int) {
	lastIndex := len(arch.entities) - 1
	if lastIndex < 0 || index > lastIndex {
		return
	}
	lastEntity := arch.entities[lastIndex]

	arch.entities[index] = lastEntity
	arch.entities = arch.entities[:lastIndex]

	if e.ID != lastEntity.ID {
		meta := self.entities[lastEntity.ID]
		meta.Index = index
		self.entities[lastEntity.ID] = meta
	}

	for i := range arch.componentData {
		id := arch.componentIDs[i]
		size := int(componentSizes[id])
		bytes := arch.componentData[i]
		copy(bytes[index*size:(index+1)*size], bytes[lastIndex*size:(lastIndex+1)*size])
		arch.componentData[i] = bytes[:lastIndex*size]
	}
}

// AddComponent adds a component of type T to an entity.
// Returns pointer to the component and success flag.
// If the component already exists, it returns the existing one.
func AddComponent[T any](w *World, e Entity) (*T, bool) {
	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return nil, false
	}

	compID, ok := TryGetID[T]()
	if !ok {
		return nil, false
	}

	size := int(componentSizes[compID])

	oldArch := meta.Archetype
	if oldArch.mask.has(compID) {
		idx := oldArch.getSlot(compID)
		if idx == -1 {
			return nil, false
		}
		bytes := oldArch.componentData[idx]
		if meta.Index*size >= len(bytes) {
			return nil, false
		}
		return (*T)(unsafe.Pointer(&bytes[meta.Index*size])), true
	}

	newMask := setMask(oldArch.mask, compID)
	newArch := w.getOrCreateArchetype(newMask)

	oldIndex := meta.Index
	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch)

	newIdx := newArch.getSlot(compID)
	if newIdx == -1 {
		return nil, false
	}
	newBytes := newArch.componentData[newIdx]
	newBytes = append(newBytes, make([]byte, size)...)
	newArch.componentData[newIdx] = newBytes

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entities[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	finalIdx := newArch.getSlot(compID)
	finalBytes := newArch.componentData[finalIdx]
	return (*T)(unsafe.Pointer(&finalBytes[newIndex*size])), true
}

// SetComponent adds a component with specific data to an entity, or updates it if it already exists.
// This is more convenient than AddComponent + manual set for initialization.
func SetComponent[T any](w *World, e Entity, comp T) bool {
	// 1. Validate the entity and get its metadata.
	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return false
	}

	// 2. Get the Component ID for type T.
	compID, ok := TryGetID[T]()
	if !ok {
		return false
	}

	size := int(componentSizes[compID])
	src := unsafe.Slice((*byte)(unsafe.Pointer(&comp)), size)

	oldArch := meta.Archetype
	// 3. Check if the entity already has this component.
	if oldArch.mask.has(compID) {
		// --- SCENARIO A: UPDATE EXISTING COMPONENT ---
		componentIndexInArchetype := oldArch.getSlot(compID)
		if componentIndexInArchetype == -1 {
			return false
		}
		bytes := oldArch.componentData[componentIndexInArchetype]
		if meta.Index*size >= len(bytes) {
			return false
		}
		copy(bytes[meta.Index*size:(meta.Index+1)*size], src)
		return true
	} else {
		// --- SCENARIO B: ADD NEW COMPONENT ---
		// a. Determine the new archetype's mask and get/create it.
		newMask := setMask(oldArch.mask, compID)
		newArch := w.getOrCreateArchetype(newMask)

		// b. Move the entity and all its existing component data to the new archetype.
		oldIndex := meta.Index
		newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch)

		// c. Append the new component data (passed in as `comp`) to the correct slice in the new archetype.
		newCompIdx := newArch.getSlot(compID)
		if newCompIdx == -1 {
			return false
		}
		newBytes := newArch.componentData[newCompIdx]
		newBytes = append(newBytes, src...)
		newArch.componentData[newCompIdx] = newBytes

		// d. Update the entity's metadata to point to its new location.
		meta.Archetype = newArch
		meta.Index = newIndex
		w.entities[e.ID] = meta

		// e. Remove the entity from its old archetype using swap-and-pop.
		w.removeEntityFromArchetype(e, oldArch, oldIndex)
		return true
	}
}

// RemoveComponent removes a component of type T from an entity.
// If the component doesn't exist, it returns true anyway.
func RemoveComponent[T any](w *World, e Entity) bool {
	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return false
	}

	compID, ok := TryGetID[T]()
	if !ok {
		return false
	}

	oldArch := meta.Archetype
	if !oldArch.mask.has(compID) {
		return true // Already removed.
	}

	oldIndex := meta.Index
	newMask := unsetMask(oldArch.mask, compID)
	newArch := w.getOrCreateArchetype(newMask)

	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, compID)

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entities[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	return true
}

// GetComponent retrieves a pointer to a component of type T for an entity.
// Returns nil and false if not found or entity invalid.
func GetComponent[T any](w *World, e Entity) (*T, bool) {
	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return nil, false
	}

	compID, ok := TryGetID[T]()
	if !ok {
		return nil, false
	}

	size := int(componentSizes[compID])

	arch := meta.Archetype
	idx := arch.getSlot(compID)
	if idx == -1 {
		return nil, false
	}
	bytes := arch.componentData[idx]
	if meta.Index*size >= len(bytes) {
		return nil, false
	}
	return (*T)(unsafe.Pointer(&bytes[meta.Index*size])), true
}

// moveEntityBetweenArchetypes copies an entity's data from one archetype to another, optionally excluding IDs.
// This is used when adding or removing components.
func moveEntityBetweenArchetypes(e Entity, oldIndex int, oldArch, newArch *Archetype, excludeIDs ...ComponentID) int {
	newIndex := len(newArch.entities)
	newArch.entities = append(newArch.entities, e)

	excludeSet := make(map[ComponentID]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		excludeSet[id] = struct{}{}
	}

	for i := range oldArch.componentIDs {
		id := oldArch.componentIDs[i]
		if _, excluded := excludeSet[id]; excluded {
			continue
		}
		oldBytes := oldArch.componentData[i]
		size := int(componentSizes[id])
		src := oldBytes[oldIndex*size : (oldIndex+1)*size]
		newIdx := newArch.getSlot(id)
		if newIdx == -1 {
			continue
		}
		newBytes := newArch.componentData[newIdx]
		newBytes = append(newBytes, src...)
		newArch.componentData[newIdx] = newBytes
	}
	return newIndex
}

// Archetype represents a unique combination of components.
// It stores entities and their component data in parallel slices for cache efficiency.
type Archetype struct {
	mask          maskType      // Bitmask of components in this archetype.
	componentData [][]byte      // Byte slices of component data.
	componentIDs  []ComponentID // Sorted list of component IDs.
	entities      []Entity      // List of entities in this archetype.
}

// getSlot returns the index of the component data slice for the given ID.
// Returns -1 if not found.
func (a *Archetype) getSlot(id ComponentID) int {
	i := sort.Search(len(a.componentIDs), func(j int) bool {
		return a.componentIDs[j] >= id
	})
	if i < len(a.componentIDs) && a.componentIDs[i] == id {
		return i
	}
	return -1
}

// Query is an iterator over entities matching 1 component type.
type Query[T1 any] struct {
	archetypes    []*Archetype
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	currentEntity Entity
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query[T1]) Next() bool {
	q.index++
	for q.archIdx < len(q.archetypes) {
		arch := q.archetypes[q.archIdx]
		if q.index < len(arch.entities) {
			if q.currentArch != arch {
				q.currentArch = arch
				id1 := GetID[T1]()
				slot1 := arch.getSlot(id1)
				if slot1 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot1]) > 0 {
					q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
				} else {
					q.base1 = nil
				}
				q.stride1 = componentSizes[id1]
			}
			q.currentEntity = arch.entities[q.index]
			return true
		}
		q.archIdx++
		q.index = 0
	}
	return false
}

// Get returns a pointer to the component for the current entity.
func (q *Query[T1]) Get() *T1 {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	return (*T1)(p1)
}

// Entity returns the current entity.
func (q *Query[T1]) Entity() Entity {
	return q.currentEntity
}

// Query2 is an iterator over entities matching 2 component types.
type Query2[T1 any, T2 any] struct {
	archetypes    []*Archetype
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	base2         unsafe.Pointer
	stride2       uintptr
	currentEntity Entity
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query2[T1, T2]) Next() bool {
	q.index++
	for q.archIdx < len(q.archetypes) {
		arch := q.archetypes[q.archIdx]
		if q.index < len(arch.entities) {
			if q.currentArch != arch {
				q.currentArch = arch
				id1 := GetID[T1]()
				slot1 := arch.getSlot(id1)
				if slot1 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot1]) > 0 {
					q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
				} else {
					q.base1 = nil
				}
				q.stride1 = componentSizes[id1]
				id2 := GetID[T2]()
				slot2 := arch.getSlot(id2)
				if slot2 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot2]) > 0 {
					q.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
				} else {
					q.base2 = nil
				}
				q.stride2 = componentSizes[id2]
			}
			q.currentEntity = arch.entities[q.index]
			return true
		}
		q.archIdx++
		q.index = 0
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (q *Query2[T1, T2]) Get() (*T1, *T2) {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	p2 := unsafe.Pointer(uintptr(q.base2) + uintptr(q.index)*q.stride2)
	return (*T1)(p1), (*T2)(p2)
}

// Entity returns the current entity.
func (q *Query2[T1, T2]) Entity() Entity {
	return q.currentEntity
}

// Query3 is an iterator over entities matching 3 component types.
type Query3[T1 any, T2 any, T3 any] struct {
	archetypes    []*Archetype
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	base2         unsafe.Pointer
	stride2       uintptr
	base3         unsafe.Pointer
	stride3       uintptr
	currentEntity Entity
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query3[T1, T2, T3]) Next() bool {
	q.index++
	for q.archIdx < len(q.archetypes) {
		arch := q.archetypes[q.archIdx]
		if q.index < len(arch.entities) {
			if q.currentArch != arch {
				q.currentArch = arch
				id1 := GetID[T1]()
				slot1 := arch.getSlot(id1)
				if slot1 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot1]) > 0 {
					q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
				} else {
					q.base1 = nil
				}
				q.stride1 = componentSizes[id1]
				id2 := GetID[T2]()
				slot2 := arch.getSlot(id2)
				if slot2 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot2]) > 0 {
					q.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
				} else {
					q.base2 = nil
				}
				q.stride2 = componentSizes[id2]
				id3 := GetID[T3]()
				slot3 := arch.getSlot(id3)
				if slot3 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot3]) > 0 {
					q.base3 = unsafe.Pointer(&arch.componentData[slot3][0])
				} else {
					q.base3 = nil
				}
				q.stride3 = componentSizes[id3]
			}
			q.currentEntity = arch.entities[q.index]
			return true
		}
		q.archIdx++
		q.index = 0
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (q *Query3[T1, T2, T3]) Get() (*T1, *T2, *T3) {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	p2 := unsafe.Pointer(uintptr(q.base2) + uintptr(q.index)*q.stride2)
	p3 := unsafe.Pointer(uintptr(q.base3) + uintptr(q.index)*q.stride3)
	return (*T1)(p1), (*T2)(p2), (*T3)(p3)
}

// Entity returns the current entity.
func (q *Query3[T1, T2, T3]) Entity() Entity {
	return q.currentEntity
}

// Query4 is an iterator over entities matching 4 component types.
type Query4[T1 any, T2 any, T3 any, T4 any] struct {
	archetypes    []*Archetype
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	base2         unsafe.Pointer
	stride2       uintptr
	base3         unsafe.Pointer
	stride3       uintptr
	base4         unsafe.Pointer
	stride4       uintptr
	currentEntity Entity
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query4[T1, T2, T3, T4]) Next() bool {
	q.index++
	for q.archIdx < len(q.archetypes) {
		arch := q.archetypes[q.archIdx]
		if q.index < len(arch.entities) {
			if q.currentArch != arch {
				q.currentArch = arch
				id1 := GetID[T1]()
				slot1 := arch.getSlot(id1)
				if slot1 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot1]) > 0 {
					q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
				} else {
					q.base1 = nil
				}
				q.stride1 = componentSizes[id1]
				id2 := GetID[T2]()
				slot2 := arch.getSlot(id2)
				if slot2 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot2]) > 0 {
					q.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
				} else {
					q.base2 = nil
				}
				q.stride2 = componentSizes[id2]
				id3 := GetID[T3]()
				slot3 := arch.getSlot(id3)
				if slot3 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot3]) > 0 {
					q.base3 = unsafe.Pointer(&arch.componentData[slot3][0])
				} else {
					q.base3 = nil
				}
				q.stride3 = componentSizes[id3]
				id4 := GetID[T4]()
				slot4 := arch.getSlot(id4)
				if slot4 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot4]) > 0 {
					q.base4 = unsafe.Pointer(&arch.componentData[slot4][0])
				} else {
					q.base4 = nil
				}
				q.stride4 = componentSizes[id4]
			}
			q.currentEntity = arch.entities[q.index]
			return true
		}
		q.archIdx++
		q.index = 0
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (q *Query4[T1, T2, T3, T4]) Get() (*T1, *T2, *T3, *T4) {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	p2 := unsafe.Pointer(uintptr(q.base2) + uintptr(q.index)*q.stride2)
	p3 := unsafe.Pointer(uintptr(q.base3) + uintptr(q.index)*q.stride3)
	p4 := unsafe.Pointer(uintptr(q.base4) + uintptr(q.index)*q.stride4)
	return (*T1)(p1), (*T2)(p2), (*T3)(p3), (*T4)(p4)
}

// Entity returns the current entity.
func (q *Query4[T1, T2, T3, T4]) Entity() Entity {
	return q.currentEntity
}

// Query5 is an iterator over entities matching 5 component types.
type Query5[T1 any, T2 any, T3 any, T4 any, T5 any] struct {
	archetypes    []*Archetype
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	base2         unsafe.Pointer
	stride2       uintptr
	base3         unsafe.Pointer
	stride3       uintptr
	base4         unsafe.Pointer
	stride4       uintptr
	base5         unsafe.Pointer
	stride5       uintptr
	currentEntity Entity
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query5[T1, T2, T3, T4, T5]) Next() bool {
	q.index++
	for q.archIdx < len(q.archetypes) {
		arch := q.archetypes[q.archIdx]
		if q.index < len(arch.entities) {
			if q.currentArch != arch {
				q.currentArch = arch
				id1 := GetID[T1]()
				slot1 := arch.getSlot(id1)
				if slot1 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot1]) > 0 {
					q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
				} else {
					q.base1 = nil
				}
				q.stride1 = componentSizes[id1]
				id2 := GetID[T2]()
				slot2 := arch.getSlot(id2)
				if slot2 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot2]) > 0 {
					q.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
				} else {
					q.base2 = nil
				}
				q.stride2 = componentSizes[id2]
				id3 := GetID[T3]()
				slot3 := arch.getSlot(id3)
				if slot3 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot3]) > 0 {
					q.base3 = unsafe.Pointer(&arch.componentData[slot3][0])
				} else {
					q.base3 = nil
				}
				q.stride3 = componentSizes[id3]
				id4 := GetID[T4]()
				slot4 := arch.getSlot(id4)
				if slot4 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot4]) > 0 {
					q.base4 = unsafe.Pointer(&arch.componentData[slot4][0])
				} else {
					q.base4 = nil
				}
				q.stride4 = componentSizes[id4]
				id5 := GetID[T5]()
				slot5 := arch.getSlot(id5)
				if slot5 < 0 {
					panic("missing component in matching archetype")
				}
				if len(arch.componentData[slot5]) > 0 {
					q.base5 = unsafe.Pointer(&arch.componentData[slot5][0])
				} else {
					q.base5 = nil
				}
				q.stride5 = componentSizes[id5]
			}
			q.currentEntity = arch.entities[q.index]
			return true
		}
		q.archIdx++
		q.index = 0
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (q *Query5[T1, T2, T3, T4, T5]) Get() (*T1, *T2, *T3, *T4, *T5) {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	p2 := unsafe.Pointer(uintptr(q.base2) + uintptr(q.index)*q.stride2)
	p3 := unsafe.Pointer(uintptr(q.base3) + uintptr(q.index)*q.stride3)
	p4 := unsafe.Pointer(uintptr(q.base4) + uintptr(q.index)*q.stride4)
	p5 := unsafe.Pointer(uintptr(q.base5) + uintptr(q.index)*q.stride5)
	return (*T1)(p1), (*T2)(p2), (*T3)(p3), (*T4)(p4), (*T5)(p5)
}

// Entity returns the current entity.
func (q *Query5[T1, T2, T3, T4, T5]) Entity() Entity {
	return q.currentEntity
}

// Filter creates a query for entities with the specified component.
func Filter[T1 any](w *World, excludes ...ComponentID) *Query[T1] {
	id1 := GetID[T1]()
	includeMask := makeMask([]ComponentID{id1})
	excludeMask := makeMask(excludes)

	matchingArchetypes := make([]*Archetype, 0, len(w.archetypes)/2)
	for m, arch := range w.archetypes {
		if len(arch.entities) > 0 && includesAll(m, includeMask) && !intersects(m, excludeMask) {
			matchingArchetypes = append(matchingArchetypes, arch)
		}
	}

	return &Query[T1]{
		archetypes: matchingArchetypes,
		archIdx:    0,
		index:      -1,
	}
}

// Filter2 creates a query for entities with the specified components.
func Filter2[T1 any, T2 any](w *World, excludes ...ComponentID) *Query2[T1, T2] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	includeMask := makeMask([]ComponentID{id1, id2})
	excludeMask := makeMask(excludes)

	matchingArchetypes := make([]*Archetype, 0, len(w.archetypes)/2)
	for m, arch := range w.archetypes {
		if len(arch.entities) > 0 && includesAll(m, includeMask) && !intersects(m, excludeMask) {
			matchingArchetypes = append(matchingArchetypes, arch)
		}
	}

	return &Query2[T1, T2]{
		archetypes: matchingArchetypes,
		archIdx:    0,
		index:      -1,
	}
}

// Filter3 creates a query for entities with the specified components.
func Filter3[T1 any, T2 any, T3 any](w *World, excludes ...ComponentID) *Query3[T1, T2, T3] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	id3 := GetID[T3]()
	includeMask := makeMask([]ComponentID{id1, id2, id3})
	excludeMask := makeMask(excludes)

	matchingArchetypes := make([]*Archetype, 0, len(w.archetypes)/2)
	for m, arch := range w.archetypes {
		if len(arch.entities) > 0 && includesAll(m, includeMask) && !intersects(m, excludeMask) {
			matchingArchetypes = append(matchingArchetypes, arch)
		}
	}

	return &Query3[T1, T2, T3]{
		archetypes: matchingArchetypes,
		archIdx:    0,
		index:      -1,
	}
}

// Filter4 creates a query for entities with the specified components.
func Filter4[T1 any, T2 any, T3 any, T4 any](w *World, excludes ...ComponentID) *Query4[T1, T2, T3, T4] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	id3 := GetID[T3]()
	id4 := GetID[T4]()
	includeMask := makeMask([]ComponentID{id1, id2, id3, id4})
	excludeMask := makeMask(excludes)

	matchingArchetypes := make([]*Archetype, 0, len(w.archetypes)/2)
	for m, arch := range w.archetypes {
		if len(arch.entities) > 0 && includesAll(m, includeMask) && !intersects(m, excludeMask) {
			matchingArchetypes = append(matchingArchetypes, arch)
		}
	}

	return &Query4[T1, T2, T3, T4]{
		archetypes: matchingArchetypes,
		archIdx:    0,
		index:      -1,
	}
}

// Filter5 creates a query for entities with the specified components.
func Filter5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, excludes ...ComponentID) *Query5[T1, T2, T3, T4, T5] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	id3 := GetID[T3]()
	id4 := GetID[T4]()
	id5 := GetID[T5]()
	includeMask := makeMask([]ComponentID{id1, id2, id3, id4, id5})
	excludeMask := makeMask(excludes)

	matchingArchetypes := make([]*Archetype, 0, len(w.archetypes)/2)
	for m, arch := range w.archetypes {
		if len(arch.entities) > 0 && includesAll(m, includeMask) && !intersects(m, excludeMask) {
			matchingArchetypes = append(matchingArchetypes, arch)
		}
	}

	return &Query5[T1, T2, T3, T4, T5]{
		archetypes: matchingArchetypes,
		archIdx:    0,
		index:      -1,
	}
}
