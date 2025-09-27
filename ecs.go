// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

import (
	"fmt"
	"math/bits"
	"reflect"
	"sync"
	"unsafe"
)

// ComponentID is a unique identifier for a component type.
type ComponentID uint32

const (
	bitsPerWord            = 64
	maskWords              = 4
	maxComponentTypes      = maskWords * bitsPerWord
	defaultInitialCapacity = 65536
)

// maskType is a bitmask used to represent a set of component types.
type maskType [maskWords]uint64

// has checks if the mask has a specific component ID.
func (self maskType) has(id ComponentID) bool {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		return false
	}
	bit := id % bitsPerWord
	return (self[word] & (1 << bit)) != 0
}

// setMask adds a component ID to the mask.
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

// unsetMask removes a component ID from the mask.
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

// makeMask creates a mask from a slice of component IDs.
func makeMask(ids []ComponentID) maskType {
	var m maskType
	for _, id := range ids {
		word := int(id / bitsPerWord)
		bit := id % bitsPerWord
		m[word] |= (1 << bit)
	}
	return m
}

// makeMask1 creates a mask for a single component ID.
func makeMask1(id1 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	return m
}

// makeMask2 creates a mask for two component IDs.
func makeMask2(id1, id2 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	return m
}

// makeMask3 creates a mask for three component IDs.
func makeMask3(id1, id2, id3 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	word3 := int(id3 / bitsPerWord)
	bit3 := id3 % bitsPerWord
	m[word3] |= (1 << bit3)
	return m
}

// makeMask4 creates a mask for four component IDs.
func makeMask4(id1, id2, id3, id4 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	word3 := int(id3 / bitsPerWord)
	bit3 := id3 % bitsPerWord
	m[word3] |= (1 << bit3)
	word4 := int(id4 / bitsPerWord)
	bit4 := id4 % bitsPerWord
	m[word4] |= (1 << bit4)
	return m
}

// makeMask5 creates a mask for five component IDs.
func makeMask5(id1, id2, id3, id4, id5 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	word3 := int(id3 / bitsPerWord)
	bit3 := id3 % bitsPerWord
	m[word3] |= (1 << bit3)
	word4 := int(id4 / bitsPerWord)
	bit4 := id4 % bitsPerWord
	m[word4] |= (1 << bit4)
	word5 := int(id5 / bitsPerWord)
	bit5 := id5 % bitsPerWord
	m[word5] |= (1 << bit5)
	return m
}

// includesAll checks if a mask contains all the bits of another mask.
func includesAll(m, include maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & include[i]) != include[i] {
			return false
		}
	}
	return true
}

// intersects checks if a mask has any bits in common with another mask.
func intersects(m, exclude maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & exclude[i]) != 0 {
			return true
		}
	}
	return false
}

var (
	nextComponentID ComponentID
	typeToID        = make(map[reflect.Type]ComponentID, maxComponentTypes)
	idToType        = make(map[ComponentID]reflect.Type, maxComponentTypes)
	componentSizes  [maxComponentTypes]uintptr
)

// ResetGlobalRegistry resets the global component registry.
// This is useful for tests or applications that need to re-initialize the ECS state.
func ResetGlobalRegistry() {
	nextComponentID = 0
	typeToID = make(map[reflect.Type]ComponentID, maxComponentTypes)
	idToType = make(map[ComponentID]reflect.Type, maxComponentTypes)
	componentSizes = [maxComponentTypes]uintptr{}
}

// RegisterComponent registers a component type and returns its unique ID.
// If the component type is already registered, it returns the existing ID.
// It panics if the maximum number of component types is exceeded.
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

// GetID returns the ComponentID for a given component type.
// It panics if the component type has not been registered.
func GetID[T any]() ComponentID {
	var zero T
	typ := reflect.TypeOf(zero)
	id, ok := typeToID[typ]
	if !ok {
		panic(fmt.Sprintf("component type %s not registered", typ))
	}
	return id
}

// TryGetID returns the ComponentID for a given component type and a boolean indicating if it was found.
// It does not panic if the component type is not registered.
func TryGetID[T any]() (ComponentID, bool) {
	var zero T
	typ := reflect.TypeOf(zero)
	id, ok := typeToID[typ]
	return id, ok
}

// Entity represents a unique entity in the ECS world.
type Entity struct {
	ID      uint32 // The unique ID of the entity.
	Version uint32 // The version of the entity, used to check for validity.
}

// entityMeta stores metadata about an entity.
type entityMeta struct {
	Archetype *Archetype // A pointer to the entity's archetype.
	Index     int        // The entity's index within the archetype.
	Version   uint32     // The current version of the entity.
}

// WorldOptions provides configuration options for creating a new World.
type WorldOptions struct {
	InitialCapacity int // The initial capacity for entities and components.
}

// Transition caches the target archetype and precomputed copy operations for a transition.
type Transition struct {
	target *Archetype
	copies []CopyOp
}

// CopyOp defines a single component copy operation from old to new archetype.
type CopyOp struct {
	from int // Slot in old archetype's componentData.
	to   int // Slot in new archetype's componentData.
	size int // Size of the component in bytes.
}

// World manages all entities, components, and systems.
type World struct {
	nextEntityID      uint32                                    // The next available entity ID.
	freeEntityIDs     []uint32                                  // A list of freed entity IDs to be recycled.
	entitiesSlice     []entityMeta                              // A slice mapping entity IDs to their metadata.
	archetypes        map[maskType]*Archetype                   // A map of component masks to archetypes.
	archetypesList    []*Archetype                              // A list of all archetypes.
	toRemove          []Entity                                  // A list of entities to be removed.
	removeSet         []Entity                                  // A set of entities to be removed in the current frame.
	Resources         sync.Map                                  // A map for storing global resources.
	initialCapacity   int                                       // The initial capacity for new archetypes.
	addTransitions    map[*Archetype]map[ComponentID]Transition // Cache for add component transitions with precomputed copies.
	removeTransitions map[*Archetype]map[ComponentID]Transition // Cache for remove component transitions with precomputed copies.
}

// NewWorld creates a new World with default options.
func NewWorld() *World {
	return NewWorldWithOptions(WorldOptions{})
}

// NewWorldWithOptions creates a new World with the specified options.
func NewWorldWithOptions(opts WorldOptions) *World {
	cap := defaultInitialCapacity
	if opts.InitialCapacity > 0 {
		cap = opts.InitialCapacity
	}
	w := &World{
		entitiesSlice:     make([]entityMeta, 0, cap),
		archetypes:        make(map[maskType]*Archetype, 32),
		archetypesList:    make([]*Archetype, 0, 64),
		toRemove:          make([]Entity, 0, cap),
		removeSet:         make([]Entity, 0, cap),
		freeEntityIDs:     make([]uint32, 0, cap),
		initialCapacity:   cap,
		addTransitions:    make(map[*Archetype]map[ComponentID]Transition),
		removeTransitions: make(map[*Archetype]map[ComponentID]Transition),
	}
	w.getOrCreateArchetype(maskType{})
	return w
}

// getOrCreateArchetype gets an existing archetype or creates a new one for the given component mask.
func (self *World) getOrCreateArchetype(mask maskType) *Archetype {
	if arch, ok := self.archetypes[mask]; ok {
		return arch
	}

	var count int
	for _, w := range mask {
		count += bits.OnesCount64(w)
	}
	compIDs := make([]ComponentID, 0, count)
	for word := 0; word < maskWords; word++ {
		w := mask[word]
		baseID := ComponentID(word * bitsPerWord)
		for bit := uint(0); bit < bitsPerWord; bit++ {
			if (w & (1 << bit)) != 0 {
				compIDs = append(compIDs, baseID+ComponentID(bit))
			}
		}
	}
	// No need to sort; IDs are appended in ascending order.

	newArch := &Archetype{
		mask:          mask,
		entities:      make([]Entity, 0, self.initialCapacity),
		componentIDs:  compIDs,
		componentData: make([][]byte, len(compIDs)),
	}
	var slots [maxComponentTypes]int
	for i := range slots {
		slots[i] = -1
	}
	for i, id := range compIDs {
		slots[id] = i
	}
	newArch.slots = slots

	for i, id := range compIDs {
		size := int(componentSizes[id])
		newArch.componentData[i] = make([]byte, 0, self.initialCapacity*size)
	}

	self.archetypes[mask] = newArch
	self.archetypesList = append(self.archetypesList, newArch)
	return newArch
}

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// extendSlice extends a slice by n elements, reallocating if necessary.
func extendSlice[T any](s []T, n int) []T {
	newLen := len(s) + n
	if cap(s) >= newLen {
		return s[:newLen]
	}
	newCap := max(2*cap(s), newLen)
	ns := make([]T, newLen, newCap)
	copy(ns, s)
	return ns
}

// extendByteSlice extends a byte slice by n bytes, reallocating if necessary.
func extendByteSlice(s []byte, n int) []byte {
	newLen := len(s) + n
	if cap(s) >= newLen {
		return s[:newLen]
	}
	newCap := max(2*cap(s), newLen)
	ns := make([]byte, newLen, newCap)
	copy(ns, s)
	return ns
}

// CreateEntity creates a new entity with no components.
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

	version := uint32(1)
	if int(id) < len(self.entitiesSlice) {
		meta := self.entitiesSlice[id]
		version = meta.Version + 1
		if version == 0 {
			version = 1
		}
	}

	e := Entity{ID: id, Version: version}
	arch := self.getOrCreateArchetype(maskType{})
	index := len(arch.entities)
	arch.entities = extendSlice(arch.entities, 1)
	arch.entities[index] = e

	if int(id) >= len(self.entitiesSlice) {
		self.entitiesSlice = extendSlice(self.entitiesSlice, int(id)-len(self.entitiesSlice)+1)
	}
	self.entitiesSlice[id] = entityMeta{Archetype: arch, Index: index, Version: e.Version}
	return e
}

// CreateEntities creates a batch of new entities with no components.
func (self *World) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}

	entities := make([]Entity, count)
	arch := self.getOrCreateArchetype(maskType{})
	startIndex := len(arch.entities)
	arch.entities = extendSlice(arch.entities, count)

	maxID := uint32(0)
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

		if id > maxID {
			maxID = id
		}

		version := uint32(1)
		if int(id) < len(self.entitiesSlice) {
			meta := self.entitiesSlice[id]
			version = meta.Version + 1
			if version == 0 {
				version = 1
			}
		}

		e := Entity{ID: id, Version: version}
		entities[i] = e
		arch.entities[startIndex+i] = e
	}

	if int(maxID) >= len(self.entitiesSlice) {
		self.entitiesSlice = extendSlice(self.entitiesSlice, int(maxID)-len(self.entitiesSlice)+1)
	}

	for i := 0; i < count; i++ {
		id := entities[i].ID
		idx := startIndex + i
		self.entitiesSlice[id] = entityMeta{Archetype: arch, Index: idx, Version: entities[i].Version}
	}

	return entities
}

// RemoveEntity marks an entity for removal. The actual removal is processed by ProcessRemovals.
func (self *World) RemoveEntity(e Entity) {
	self.toRemove = extendSlice(self.toRemove, 1)
	self.toRemove[len(self.toRemove)-1] = e
}

// ProcessRemovals processes the entities marked for removal.
// This should be called once per frame, e.g., at the end of the game loop.
func (self *World) ProcessRemovals() {
	if len(self.toRemove) == 0 {
		return
	}

	self.removeSet = self.removeSet[:0]
	for _, e := range self.toRemove {
		if int(e.ID) < len(self.entitiesSlice) {
			meta := self.entitiesSlice[e.ID]
			if meta.Version == e.Version {
				self.removeSet = append(self.removeSet, e)
			}
		}
	}

	oldFreeLen := len(self.freeEntityIDs)
	self.freeEntityIDs = extendSlice(self.freeEntityIDs, len(self.removeSet))

	for i, e := range self.removeSet {
		id := e.ID
		meta := self.entitiesSlice[id]
		self.removeEntityFromArchetype(e, meta.Archetype, meta.Index)
		self.freeEntityIDs[oldFreeLen+i] = id
		self.entitiesSlice[id] = entityMeta{}
	}
	self.toRemove = self.toRemove[:0]
}

// removeEntityFromArchetype removes an entity from an archetype using the swap-and-pop method.
func (self *World) removeEntityFromArchetype(e Entity, arch *Archetype, index int) {
	lastIndex := len(arch.entities) - 1
	if lastIndex < 0 || index > lastIndex {
		return
	}
	lastEntity := arch.entities[lastIndex]

	arch.entities[index] = lastEntity
	arch.entities = arch.entities[:lastIndex]

	if e.ID != lastEntity.ID {
		meta := self.entitiesSlice[lastEntity.ID]
		meta.Index = index
		self.entitiesSlice[lastEntity.ID] = meta
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
// It returns a pointer to the newly added component and a boolean indicating success.
// If the entity already has the component, it returns a pointer to the existing component.
func AddComponent[T any](w *World, e Entity) (*T, bool) {
	if int(e.ID) >= len(w.entitiesSlice) {
		return nil, false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
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

	var transition Transition
	if addMap, ok := w.addTransitions[oldArch]; ok {
		if tr, ok := addMap[compID]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newMask := setMask(oldArch.mask, compID)
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		addMap, ok := w.addTransitions[oldArch]
		if !ok {
			addMap = make(map[ComponentID]Transition)
			w.addTransitions[oldArch] = addMap
		}
		addMap[compID] = transition
	} else {
		newArch = transition.target
	}

	oldIndex := meta.Index
	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

	newIdx := newArch.getSlot(compID)
	if newIdx == -1 {
		return nil, false
	}
	newBytes := newArch.componentData[newIdx]
	newBytes = extendByteSlice(newBytes, size)
	newArch.componentData[newIdx] = newBytes

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entitiesSlice[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	finalIdx := newArch.getSlot(compID)
	finalBytes := newArch.componentData[finalIdx]
	return (*T)(unsafe.Pointer(&finalBytes[newIndex*size])), true
}

// SetComponent sets the component data for an entity.
// If the entity does not have the component, it will be added.
// It returns a boolean indicating success.
func SetComponent[T any](w *World, e Entity, comp T) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	compID, ok := TryGetID[T]()
	if !ok {
		return false
	}
	size := int(componentSizes[compID])
	src := unsafe.Slice((*byte)(unsafe.Pointer(&comp)), size)

	oldArch := meta.Archetype
	if oldArch.mask.has(compID) {
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
		var transition Transition
		if addMap, ok := w.addTransitions[oldArch]; ok {
			if tr, ok := addMap[compID]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newMask := setMask(oldArch.mask, compID)
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			addMap, ok := w.addTransitions[oldArch]
			if !ok {
				addMap = make(map[ComponentID]Transition)
				w.addTransitions[oldArch] = addMap
			}
			addMap[compID] = transition
		} else {
			newArch = transition.target
		}

		oldIndex := meta.Index
		newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

		newCompIdx := newArch.getSlot(compID)
		if newCompIdx == -1 {
			return false
		}
		newBytes := newArch.componentData[newCompIdx]
		newBytes = extendByteSlice(newBytes, size)
		copy(newBytes[len(newBytes)-size:], src)
		newArch.componentData[newCompIdx] = newBytes

		meta.Archetype = newArch
		meta.Index = newIndex
		w.entitiesSlice[e.ID] = meta

		w.removeEntityFromArchetype(e, oldArch, oldIndex)
		return true
	}
}

// RemoveComponent removes a component of type T from an entity.
// It returns a boolean indicating whether the component was successfully removed.
// If the entity does not have the component, it returns true.
func RemoveComponent[T any](w *World, e Entity) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	compID, ok := TryGetID[T]()
	if !ok {
		return false
	}

	oldArch := meta.Archetype
	if !oldArch.mask.has(compID) {
		return true
	}

	oldIndex := meta.Index

	var transition Transition
	if removeMap, ok := w.removeTransitions[oldArch]; ok {
		if tr, ok := removeMap[compID]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newMask := unsetMask(oldArch.mask, compID)
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs)-1)
		for from, id := range oldArch.componentIDs {
			if id == compID {
				continue
			}
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		removeMap, ok := w.removeTransitions[oldArch]
		if !ok {
			removeMap = make(map[ComponentID]Transition)
			w.removeTransitions[oldArch] = removeMap
		}
		removeMap[compID] = transition
	} else {
		newArch = transition.target
	}

	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entitiesSlice[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	return true
}

// GetComponent retrieves a pointer to the component of type T for the given entity.
// It returns a pointer to the component and a boolean indicating whether the component was found.
func GetComponent[T any](w *World, e Entity) (*T, bool) {
	if int(e.ID) >= len(w.entitiesSlice) {
		return nil, false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
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

// moveEntityBetweenArchetypes moves an entity from an old archetype to a new one.
// It copies component data using the precomputed list of copy operations.
// It returns the new index of the entity in the new archetype.
func moveEntityBetweenArchetypes(e Entity, oldIndex int, oldArch, newArch *Archetype, copies []CopyOp) int {
	newIndex := len(newArch.entities)
	newArch.entities = extendSlice(newArch.entities, 1)
	newArch.entities[newIndex] = e

	for _, op := range copies {
		oldBytes := oldArch.componentData[op.from]
		size := op.size
		src := oldBytes[oldIndex*size : (oldIndex+1)*size]
		newBytes := newArch.componentData[op.to]
		newBytes = extendByteSlice(newBytes, size)
		copy(newBytes[len(newBytes)-size:], src)
		newArch.componentData[op.to] = newBytes
	}
	return newIndex
}

// Archetype represents a unique combination of component types.
// Entities with the same set of components are stored in the same archetype.
type Archetype struct {
	mask          maskType               // The component mask for this archetype.
	componentData [][]byte               // Byte slices of component data.
	componentIDs  []ComponentID          // A sorted list of component IDs in this archetype.
	entities      []Entity               // The list of entities in this archetype.
	slots         [maxComponentTypes]int // Slot lookup for component IDs; -1 if not present.
}

// getSlot finds the index of a component ID in the archetype's componentID list.
// It uses a lookup array for constant time access.
func (self *Archetype) getSlot(id ComponentID) int {
	return self.slots[id]
}

// Query is an iterator over entities that have a specific set of components.
// This query is for entities with one component type.
type Query[T1 any] struct {
	world         *World         // The world to query.
	includeMask   maskType       // A mask of components to include.
	excludeMask   maskType       // A mask of components to exclude.
	id1           ComponentID    // The ID of the first component.
	archIdx       int            // The current archetype index.
	index         int            // The current entity index within the archetype.
	currentArch   *Archetype     // The current archetype being iterated.
	base1         unsafe.Pointer // A pointer to the base of the first component's storage.
	stride1       uintptr        // The size of the first component type.
	currentEntity Entity         // The current entity being iterated.
}

// Reset resets the query for reuse.
func (self *Query[T1]) Reset() {
	self.archIdx = 0
	self.index = -1
	self.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (self *Query[T1]) Next() bool {
	self.index++
	if self.currentArch != nil && self.index < len(self.currentArch.entities) {
		self.currentEntity = self.currentArch.entities[self.index]
		return true
	}

	for self.archIdx < len(self.world.archetypesList) {
		arch := self.world.archetypesList[self.archIdx]
		self.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, self.includeMask) || intersects(arch.mask, self.excludeMask) {
			continue
		}
		self.currentArch = arch
		slot1 := arch.getSlot(self.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			self.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			self.base1 = nil
		}
		self.stride1 = componentSizes[self.id1]
		self.index = 0
		self.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns a pointer to the component for the current entity.
func (self *Query[T1]) Get() *T1 {
	p1 := unsafe.Pointer(uintptr(self.base1) + uintptr(self.index)*self.stride1)
	return (*T1)(p1)
}

// Entity returns the current entity.
func (self *Query[T1]) Entity() Entity {
	return self.currentEntity
}

// Query2 is an iterator over entities that have a specific set of components.
// This query is for entities with two component types.
type Query2[T1 any, T2 any] struct {
	world         *World         // The world to query.
	includeMask   maskType       // A mask of components to include.
	excludeMask   maskType       // A mask of components to exclude.
	id1           ComponentID    // The ID of the first component.
	id2           ComponentID    // The ID of the second component.
	archIdx       int            // The current archetype index.
	index         int            // The current entity index within the archetype.
	currentArch   *Archetype     // The current archetype being iterated.
	base1         unsafe.Pointer // A pointer to the base of the first component's storage.
	stride1       uintptr        // The size of the first component type.
	base2         unsafe.Pointer // A pointer to the base of the second component's storage.
	stride2       uintptr        // The size of the second component type.
	currentEntity Entity         // The current entity being iterated.
}

// Reset resets the query for reuse.
func (self *Query2[T1, T2]) Reset() {
	self.archIdx = 0
	self.index = -1
	self.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (self *Query2[T1, T2]) Next() bool {
	self.index++
	if self.currentArch != nil && self.index < len(self.currentArch.entities) {
		self.currentEntity = self.currentArch.entities[self.index]
		return true
	}

	for self.archIdx < len(self.world.archetypesList) {
		arch := self.world.archetypesList[self.archIdx]
		self.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, self.includeMask) || intersects(arch.mask, self.excludeMask) {
			continue
		}
		self.currentArch = arch
		slot1 := arch.getSlot(self.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			self.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			self.base1 = nil
		}
		self.stride1 = componentSizes[self.id1]
		slot2 := arch.getSlot(self.id2)
		if slot2 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot2]) > 0 {
			self.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
		} else {
			self.base2 = nil
		}
		self.stride2 = componentSizes[self.id2]
		self.index = 0
		self.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (self *Query2[T1, T2]) Get() (*T1, *T2) {
	p1 := unsafe.Pointer(uintptr(self.base1) + uintptr(self.index)*self.stride1)
	p2 := unsafe.Pointer(uintptr(self.base2) + uintptr(self.index)*self.stride2)
	return (*T1)(p1), (*T2)(p2)
}

// Entity returns the current entity.
func (self *Query2[T1, T2]) Entity() Entity {
	return self.currentEntity
}

// Query3 is an iterator over entities that have a specific set of components.
// This query is for entities with three component types.
type Query3[T1 any, T2 any, T3 any] struct {
	world         *World         // The world to query.
	includeMask   maskType       // A mask of components to include.
	excludeMask   maskType       // A mask of components to exclude.
	id1           ComponentID    // The ID of the first component.
	id2           ComponentID    // The ID of the second component.
	id3           ComponentID    // The ID of the third component.
	archIdx       int            // The current archetype index.
	index         int            // The current entity index within the archetype.
	currentArch   *Archetype     // The current archetype being iterated.
	base1         unsafe.Pointer // A pointer to the base of the first component's storage.
	stride1       uintptr        // The size of the first component type.
	base2         unsafe.Pointer // A pointer to the base of the second component's storage.
	stride2       uintptr        // The size of the second component type.
	base3         unsafe.Pointer // A pointer to the base of the third component's storage.
	stride3       uintptr        // The size of the third component type.
	currentEntity Entity         // The current entity being iterated.
}

// Reset resets the query for reuse.
func (self *Query3[T1, T2, T3]) Reset() {
	self.archIdx = 0
	self.index = -1
	self.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (self *Query3[T1, T2, T3]) Next() bool {
	self.index++
	if self.currentArch != nil && self.index < len(self.currentArch.entities) {
		self.currentEntity = self.currentArch.entities[self.index]
		return true
	}

	for self.archIdx < len(self.world.archetypesList) {
		arch := self.world.archetypesList[self.archIdx]
		self.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, self.includeMask) || intersects(arch.mask, self.excludeMask) {
			continue
		}
		self.currentArch = arch
		slot1 := arch.getSlot(self.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			self.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			self.base1 = nil
		}
		self.stride1 = componentSizes[self.id1]
		slot2 := arch.getSlot(self.id2)
		if slot2 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot2]) > 0 {
			self.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
		} else {
			self.base2 = nil
		}
		self.stride2 = componentSizes[self.id2]
		slot3 := arch.getSlot(self.id3)
		if slot3 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot3]) > 0 {
			self.base3 = unsafe.Pointer(&arch.componentData[slot3][0])
		} else {
			self.base3 = nil
		}
		self.stride3 = componentSizes[self.id3]
		self.index = 0
		self.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (self *Query3[T1, T2, T3]) Get() (*T1, *T2, *T3) {
	p1 := unsafe.Pointer(uintptr(self.base1) + uintptr(self.index)*self.stride1)
	p2 := unsafe.Pointer(uintptr(self.base2) + uintptr(self.index)*self.stride2)
	p3 := unsafe.Pointer(uintptr(self.base3) + uintptr(self.index)*self.stride3)
	return (*T1)(p1), (*T2)(p2), (*T3)(p3)
}

// Entity returns the current entity.
func (self *Query3[T1, T2, T3]) Entity() Entity {
	return self.currentEntity
}

// Query4 is an iterator over entities that have a specific set of components.
// This query is for entities with four component types.
type Query4[T1 any, T2 any, T3 any, T4 any] struct {
	world         *World         // The world to query.
	includeMask   maskType       // A mask of components to include.
	excludeMask   maskType       // A mask of components to exclude.
	id1           ComponentID    // The ID of the first component.
	id2           ComponentID    // The ID of the second component.
	id3           ComponentID    // The ID of the third component.
	id4           ComponentID    // The ID of the fourth component.
	archIdx       int            // The current archetype index.
	index         int            // The current entity index within the archetype.
	currentArch   *Archetype     // The current archetype being iterated.
	base1         unsafe.Pointer // A pointer to the base of the first component's storage.
	stride1       uintptr        // The size of the first component type.
	base2         unsafe.Pointer // A pointer to the base of the second component's storage.
	stride2       uintptr        // The size of the second component type.
	base3         unsafe.Pointer // A pointer to the base of the third component's storage.
	stride3       uintptr        // The size of the third component type.
	base4         unsafe.Pointer // A pointer to the base of the fourth component's storage.
	stride4       uintptr        // The size of the fourth component type.
	currentEntity Entity         // The current entity being iterated.
}

// Reset resets the query for reuse.
func (self *Query4[T1, T2, T3, T4]) Reset() {
	self.archIdx = 0
	self.index = -1
	self.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (self *Query4[T1, T2, T3, T4]) Next() bool {
	self.index++
	if self.currentArch != nil && self.index < len(self.currentArch.entities) {
		self.currentEntity = self.currentArch.entities[self.index]
		return true
	}

	for self.archIdx < len(self.world.archetypesList) {
		arch := self.world.archetypesList[self.archIdx]
		self.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, self.includeMask) || intersects(arch.mask, self.excludeMask) {
			continue
		}
		self.currentArch = arch
		slot1 := arch.getSlot(self.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			self.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			self.base1 = nil
		}
		self.stride1 = componentSizes[self.id1]
		slot2 := arch.getSlot(self.id2)
		if slot2 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot2]) > 0 {
			self.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
		} else {
			self.base2 = nil
		}
		self.stride2 = componentSizes[self.id2]
		slot3 := arch.getSlot(self.id3)
		if slot3 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot3]) > 0 {
			self.base3 = unsafe.Pointer(&arch.componentData[slot3][0])
		} else {
			self.base3 = nil
		}
		self.stride3 = componentSizes[self.id3]
		slot4 := arch.getSlot(self.id4)
		if slot4 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot4]) > 0 {
			self.base4 = unsafe.Pointer(&arch.componentData[slot4][0])
		} else {
			self.base4 = nil
		}
		self.stride4 = componentSizes[self.id4]
		self.index = 0
		self.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (self *Query4[T1, T2, T3, T4]) Get() (*T1, *T2, *T3, *T4) {
	p1 := unsafe.Pointer(uintptr(self.base1) + uintptr(self.index)*self.stride1)
	p2 := unsafe.Pointer(uintptr(self.base2) + uintptr(self.index)*self.stride2)
	p3 := unsafe.Pointer(uintptr(self.base3) + uintptr(self.index)*self.stride3)
	p4 := unsafe.Pointer(uintptr(self.base4) + uintptr(self.index)*self.stride4)
	return (*T1)(p1), (*T2)(p2), (*T3)(p3), (*T4)(p4)
}

// Entity returns the current entity.
func (self *Query4[T1, T2, T3, T4]) Entity() Entity {
	return self.currentEntity
}

// Query5 is an iterator over entities that have a specific set of components.
// This query is for entities with five component types.
type Query5[T1 any, T2 any, T3 any, T4 any, T5 any] struct {
	world         *World         // The world to query.
	includeMask   maskType       // A mask of components to include.
	excludeMask   maskType       // A mask of components to exclude.
	id1           ComponentID    // The ID of the first component.
	id2           ComponentID    // The ID of the second component.
	id3           ComponentID    // The ID of the third component.
	id4           ComponentID    // The ID of the fourth component.
	id5           ComponentID    // The ID of the fifth component.
	archIdx       int            // The current archetype index.
	index         int            // The current entity index within the archetype.
	currentArch   *Archetype     // The current archetype being iterated.
	base1         unsafe.Pointer // A pointer to the base of the first component's storage.
	stride1       uintptr        // The size of the first component type.
	base2         unsafe.Pointer // A pointer to the base of the second component's storage.
	stride2       uintptr        // The size of the second component type.
	base3         unsafe.Pointer // A pointer to the base of the third component's storage.
	stride3       uintptr        // The size of the third component type.
	base4         unsafe.Pointer // A pointer to the base of the fourth component's storage.
	stride4       uintptr        // The size of the fourth component type.
	base5         unsafe.Pointer // A pointer to the base of the fifth component's storage.
	stride5       uintptr        // The size of the fifth component type.
	currentEntity Entity         // The current entity being iterated.
}

// Reset resets the query for reuse.
func (self *Query5[T1, T2, T3, T4, T5]) Reset() {
	self.archIdx = 0
	self.index = -1
	self.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (self *Query5[T1, T2, T3, T4, T5]) Next() bool {
	self.index++
	if self.currentArch != nil && self.index < len(self.currentArch.entities) {
		self.currentEntity = self.currentArch.entities[self.index]
		return true
	}

	for self.archIdx < len(self.world.archetypesList) {
		arch := self.world.archetypesList[self.archIdx]
		self.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, self.includeMask) || intersects(arch.mask, self.excludeMask) {
			continue
		}
		self.currentArch = arch
		slot1 := arch.getSlot(self.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			self.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			self.base1 = nil
		}
		self.stride1 = componentSizes[self.id1]
		slot2 := arch.getSlot(self.id2)
		if slot2 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot2]) > 0 {
			self.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
		} else {
			self.base2 = nil
		}
		self.stride2 = componentSizes[self.id2]
		slot3 := arch.getSlot(self.id3)
		if slot3 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot3]) > 0 {
			self.base3 = unsafe.Pointer(&arch.componentData[slot3][0])
		} else {
			self.base3 = nil
		}
		self.stride3 = componentSizes[self.id3]
		slot4 := arch.getSlot(self.id4)
		if slot4 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot4]) > 0 {
			self.base4 = unsafe.Pointer(&arch.componentData[slot4][0])
		} else {
			self.base4 = nil
		}
		self.stride4 = componentSizes[self.id4]
		slot5 := arch.getSlot(self.id5)
		if slot5 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot5]) > 0 {
			self.base5 = unsafe.Pointer(&arch.componentData[slot5][0])
		} else {
			self.base5 = nil
		}
		self.stride5 = componentSizes[self.id5]
		self.index = 0
		self.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (self *Query5[T1, T2, T3, T4, T5]) Get() (*T1, *T2, *T3, *T4, *T5) {
	p1 := unsafe.Pointer(uintptr(self.base1) + uintptr(self.index)*self.stride1)
	p2 := unsafe.Pointer(uintptr(self.base2) + uintptr(self.index)*self.stride2)
	p3 := unsafe.Pointer(uintptr(self.base3) + uintptr(self.index)*self.stride3)
	p4 := unsafe.Pointer(uintptr(self.base4) + uintptr(self.index)*self.stride4)
	p5 := unsafe.Pointer(uintptr(self.base5) + uintptr(self.index)*self.stride5)
	return (*T1)(p1), (*T2)(p2), (*T3)(p3), (*T4)(p4), (*T5)(p5)
}

// Entity returns the current entity.
func (self *Query5[T1, T2, T3, T4, T5]) Entity() Entity {
	return self.currentEntity
}

// CreateQuery creates a new query for entities with one specific component type.
// It allows specifying component types to exclude from the query results.
//
// Parameters:
//
//	w: The World to query.
//	excludes: A variadic list of ComponentIDs to exclude from the query.
//
// Returns:
//
//	A pointer to a new Query[T1].
func CreateQuery[T1 any](w *World, excludes ...ComponentID) *Query[T1] {
	id1 := GetID[T1]()
	return &Query[T1]{
		world:       w,
		includeMask: makeMask1(id1),
		excludeMask: makeMask(excludes),
		id1:         id1,
		archIdx:     0,
		index:       -1,
	}
}

// CreateQuery2 creates a new query for entities with two specific component types.
// It allows specifying component types to exclude from the query results.
//
// Parameters:
//
//	w: The World to query.
//	excludes: A variadic list of ComponentIDs to exclude from the query.
//
// Returns:
//
//	A pointer to a new Query2[T1, T2].
func CreateQuery2[T1 any, T2 any](w *World, excludes ...ComponentID) *Query2[T1, T2] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	return &Query2[T1, T2]{
		world:       w,
		includeMask: makeMask2(id1, id2),
		excludeMask: makeMask(excludes),
		id1:         id1,
		id2:         id2,
		archIdx:     0,
		index:       -1,
	}
}

// CreateQuery3 creates a new query for entities with three specific component types.
// It allows specifying component types to exclude from the query results.
//
// Parameters:
//
//	w: The World to query.
//	excludes: A variadic list of ComponentIDs to exclude from the query.
//
// Returns:
//
//	A pointer to a new Query3[T1, T2, T3].
func CreateQuery3[T1 any, T2 any, T3 any](w *World, excludes ...ComponentID) *Query3[T1, T2, T3] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	id3 := GetID[T3]()
	return &Query3[T1, T2, T3]{
		world:       w,
		includeMask: makeMask3(id1, id2, id3),
		excludeMask: makeMask(excludes),
		id1:         id1,
		id2:         id2,
		id3:         id3,
		archIdx:     0,
		index:       -1,
	}
}

// CreateQuery4 creates a new query for entities with four specific component types.
// It allows specifying component types to exclude from the query results.
//
// Parameters:
//
//	w: The World to query.
//	excludes: A variadic list of ComponentIDs to exclude from the query.
//
// Returns:
//
//	A pointer to a new Query4[T1, T2, T3, T4].
func CreateQuery4[T1 any, T2 any, T3 any, T4 any](w *World, excludes ...ComponentID) *Query4[T1, T2, T3, T4] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	id3 := GetID[T3]()
	id4 := GetID[T4]()
	return &Query4[T1, T2, T3, T4]{
		world:       w,
		includeMask: makeMask4(id1, id2, id3, id4),
		excludeMask: makeMask(excludes),
		id1:         id1,
		id2:         id2,
		id3:         id3,
		id4:         id4,
		archIdx:     0,
		index:       -1,
	}
}

// CreateQuery5 creates a new query for entities with five specific component types.
// It allows specifying component types to exclude from the query results.
//
// Parameters:
//
//	w: The World to query.
//	excludes: A variadic list of ComponentIDs to exclude from the query.
//
// Returns:
//
//	A pointer to a new Query5[T1, T2, T3, T4, T5].
func CreateQuery5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, excludes ...ComponentID) *Query5[T1, T2, T3, T4, T5] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	id3 := GetID[T3]()
	id4 := GetID[T4]()
	id5 := GetID[T5]()
	return &Query5[T1, T2, T3, T4, T5]{
		world:       w,
		includeMask: makeMask5(id1, id2, id3, id4, id5),
		excludeMask: makeMask(excludes),
		id1:         id1,
		id2:         id2,
		id3:         id3,
		id4:         id4,
		id5:         id5,
		archIdx:     0,
		index:       -1,
	}
}
