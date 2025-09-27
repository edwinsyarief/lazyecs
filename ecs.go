// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

import (
	"fmt"
	"math/bits"
	"reflect"
	"sort"
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

// orMask performs a bitwise OR between two masks.
func orMask(m1, m2 maskType) maskType {
	var nm maskType
	for i := 0; i < maskWords; i++ {
		nm[i] = m1[i] | m2[i]
	}
	return nm
}

// andNotMask performs a bitwise AND NOT (m1 &^ m2) between two masks.
func andNotMask(m1, m2 maskType) maskType {
	var nm maskType
	for i := 0; i < maskWords; i++ {
		nm[i] = m1[i] &^ m2[i]
	}
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
	nextEntityID      uint32                                 // The next available entity ID.
	freeEntityIDs     []uint32                               // A list of freed entity IDs to be recycled.
	entitiesSlice     []entityMeta                           // A slice mapping entity IDs to their metadata.
	archetypes        map[maskType]*Archetype                // A map of component masks to archetypes.
	archetypesList    []*Archetype                           // A list of all archetypes.
	toRemove          []Entity                               // A list of entities to be removed.
	removeSet         []Entity                               // A set of entities to be removed in the current frame.
	Resources         sync.Map                               // A map for storing global resources.
	initialCapacity   int                                    // The initial capacity for new archetypes.
	addTransitions    map[*Archetype]map[maskType]Transition // Cache for add component transitions with precomputed copies.
	removeTransitions map[*Archetype]map[maskType]Transition // Cache for remove component transitions with precomputed copies.
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
		addTransitions:    make(map[*Archetype]map[maskType]Transition),
		removeTransitions: make(map[*Archetype]map[maskType]Transition),
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
	addMask := makeMask1(compID)
	if intersects(oldArch.mask, addMask) {
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

	newMask := orMask(oldArch.mask, addMask)

	var transition Transition
	addMap, ok := w.addTransitions[oldArch]
	if ok {
		if tr, ok := addMap[addMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.addTransitions[oldArch]; !ok {
			w.addTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.addTransitions[oldArch][addMask] = transition
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

// AddComponent2 adds two components to an entity if not already present.
// It returns pointers to the components (existing or new) and a boolean indicating success.
func AddComponent2[T1 any, T2 any](w *World, e Entity) (*T1, *T2, bool) {
	if int(e.ID) >= len(w.entitiesSlice) {
		return nil, nil, false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return nil, nil, false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	if !ok1 || !ok2 {
		return nil, nil, false
	}
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])

	oldArch := meta.Archetype
	addMask := makeMask2(id1, id2)
	if includesAll(oldArch.mask, addMask) {
		idx1 := oldArch.getSlot(id1)
		idx2 := oldArch.getSlot(id2)
		if idx1 == -1 || idx2 == -1 {
			return nil, nil, false
		}
		bytes1 := oldArch.componentData[idx1]
		bytes2 := oldArch.componentData[idx2]
		if meta.Index*size1 >= len(bytes1) || meta.Index*size2 >= len(bytes2) {
			return nil, nil, false
		}
		return (*T1)(unsafe.Pointer(&bytes1[meta.Index*size1])), (*T2)(unsafe.Pointer(&bytes2[meta.Index*size2])), true
	}

	newMask := orMask(oldArch.mask, addMask)

	var transition Transition
	addMap, ok := w.addTransitions[oldArch]
	if ok {
		if tr, ok := addMap[addMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.addTransitions[oldArch]; !ok {
			w.addTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.addTransitions[oldArch][addMask] = transition
	} else {
		newArch = transition.target
	}

	oldIndex := meta.Index
	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

	// Extend new components (zero-initialized)
	ids := []ComponentID{id1, id2}
	for _, id := range ids {
		if !oldArch.mask.has(id) {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return nil, nil, false
			}
			bytes := newArch.componentData[idx]
			bytes = extendByteSlice(bytes, int(componentSizes[id]))
			newArch.componentData[idx] = bytes
		}
	}

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entitiesSlice[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	idx1 := newArch.getSlot(id1)
	idx2 := newArch.getSlot(id2)
	if idx1 == -1 || idx2 == -1 {
		return nil, nil, false
	}
	bytes1 := newArch.componentData[idx1]
	bytes2 := newArch.componentData[idx2]
	return (*T1)(unsafe.Pointer(&bytes1[newIndex*size1])), (*T2)(unsafe.Pointer(&bytes2[newIndex*size2])), true
}

// AddComponent3 adds three components to an entity if not already present.
// It returns pointers to the components (existing or new) and a boolean indicating success.
func AddComponent3[T1 any, T2 any, T3 any](w *World, e Entity) (*T1, *T2, *T3, bool) {
	if int(e.ID) >= len(w.entitiesSlice) {
		return nil, nil, nil, false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return nil, nil, nil, false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	if !ok1 || !ok2 || !ok3 {
		return nil, nil, nil, false
	}
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])

	oldArch := meta.Archetype
	addMask := makeMask3(id1, id2, id3)
	if includesAll(oldArch.mask, addMask) {
		idx1 := oldArch.getSlot(id1)
		idx2 := oldArch.getSlot(id2)
		idx3 := oldArch.getSlot(id3)
		if idx1 == -1 || idx2 == -1 || idx3 == -1 {
			return nil, nil, nil, false
		}
		bytes1 := oldArch.componentData[idx1]
		bytes2 := oldArch.componentData[idx2]
		bytes3 := oldArch.componentData[idx3]
		if meta.Index*size1 >= len(bytes1) || meta.Index*size2 >= len(bytes2) || meta.Index*size3 >= len(bytes3) {
			return nil, nil, nil, false
		}
		return (*T1)(unsafe.Pointer(&bytes1[meta.Index*size1])), (*T2)(unsafe.Pointer(&bytes2[meta.Index*size2])), (*T3)(unsafe.Pointer(&bytes3[meta.Index*size3])), true
	}

	newMask := orMask(oldArch.mask, addMask)

	var transition Transition
	addMap, ok := w.addTransitions[oldArch]
	if ok {
		if tr, ok := addMap[addMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.addTransitions[oldArch]; !ok {
			w.addTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.addTransitions[oldArch][addMask] = transition
	} else {
		newArch = transition.target
	}

	oldIndex := meta.Index
	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

	// Extend new components (zero-initialized)
	ids := []ComponentID{id1, id2, id3}
	for _, id := range ids {
		if !oldArch.mask.has(id) {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return nil, nil, nil, false
			}
			bytes := newArch.componentData[idx]
			bytes = extendByteSlice(bytes, int(componentSizes[id]))
			newArch.componentData[idx] = bytes
		}
	}

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entitiesSlice[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	idx1 := newArch.getSlot(id1)
	idx2 := newArch.getSlot(id2)
	idx3 := newArch.getSlot(id3)
	if idx1 == -1 || idx2 == -1 || idx3 == -1 {
		return nil, nil, nil, false
	}
	bytes1 := newArch.componentData[idx1]
	bytes2 := newArch.componentData[idx2]
	bytes3 := newArch.componentData[idx3]
	return (*T1)(unsafe.Pointer(&bytes1[newIndex*size1])), (*T2)(unsafe.Pointer(&bytes2[newIndex*size2])), (*T3)(unsafe.Pointer(&bytes3[newIndex*size3])), true
}

// AddComponent4 adds four components to an entity if not already present.
// It returns pointers to the components (existing or new) and a boolean indicating success.
func AddComponent4[T1 any, T2 any, T3 any, T4 any](w *World, e Entity) (*T1, *T2, *T3, *T4, bool) {
	if int(e.ID) >= len(w.entitiesSlice) {
		return nil, nil, nil, nil, false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return nil, nil, nil, nil, false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return nil, nil, nil, nil, false
	}
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])

	oldArch := meta.Archetype
	addMask := makeMask4(id1, id2, id3, id4)
	if includesAll(oldArch.mask, addMask) {
		idx1 := oldArch.getSlot(id1)
		idx2 := oldArch.getSlot(id2)
		idx3 := oldArch.getSlot(id3)
		idx4 := oldArch.getSlot(id4)
		if idx1 == -1 || idx2 == -1 || idx3 == -1 || idx4 == -1 {
			return nil, nil, nil, nil, false
		}
		bytes1 := oldArch.componentData[idx1]
		bytes2 := oldArch.componentData[idx2]
		bytes3 := oldArch.componentData[idx3]
		bytes4 := oldArch.componentData[idx4]
		if meta.Index*size1 >= len(bytes1) || meta.Index*size2 >= len(bytes2) || meta.Index*size3 >= len(bytes3) || meta.Index*size4 >= len(bytes4) {
			return nil, nil, nil, nil, false
		}
		return (*T1)(unsafe.Pointer(&bytes1[meta.Index*size1])), (*T2)(unsafe.Pointer(&bytes2[meta.Index*size2])), (*T3)(unsafe.Pointer(&bytes3[meta.Index*size3])), (*T4)(unsafe.Pointer(&bytes4[meta.Index*size4])), true
	}

	newMask := orMask(oldArch.mask, addMask)

	var transition Transition
	addMap, ok := w.addTransitions[oldArch]
	if ok {
		if tr, ok := addMap[addMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.addTransitions[oldArch]; !ok {
			w.addTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.addTransitions[oldArch][addMask] = transition
	} else {
		newArch = transition.target
	}

	oldIndex := meta.Index
	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

	// Extend new components (zero-initialized)
	ids := []ComponentID{id1, id2, id3, id4}
	for _, id := range ids {
		if !oldArch.mask.has(id) {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return nil, nil, nil, nil, false
			}
			bytes := newArch.componentData[idx]
			bytes = extendByteSlice(bytes, int(componentSizes[id]))
			newArch.componentData[idx] = bytes
		}
	}

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entitiesSlice[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	idx1 := newArch.getSlot(id1)
	idx2 := newArch.getSlot(id2)
	idx3 := newArch.getSlot(id3)
	idx4 := newArch.getSlot(id4)
	if idx1 == -1 || idx2 == -1 || idx3 == -1 || idx4 == -1 {
		return nil, nil, nil, nil, false
	}
	bytes1 := newArch.componentData[idx1]
	bytes2 := newArch.componentData[idx2]
	bytes3 := newArch.componentData[idx3]
	bytes4 := newArch.componentData[idx4]
	return (*T1)(unsafe.Pointer(&bytes1[newIndex*size1])), (*T2)(unsafe.Pointer(&bytes2[newIndex*size2])), (*T3)(unsafe.Pointer(&bytes3[newIndex*size3])), (*T4)(unsafe.Pointer(&bytes4[newIndex*size4])), true
}

// AddComponent5 adds five components to an entity if not already present.
// It returns pointers to the components (existing or new) and a boolean indicating success.
func AddComponent5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, e Entity) (*T1, *T2, *T3, *T4, *T5, bool) {
	if int(e.ID) >= len(w.entitiesSlice) {
		return nil, nil, nil, nil, nil, false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return nil, nil, nil, nil, nil, false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	id5, ok5 := TryGetID[T5]()
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return nil, nil, nil, nil, nil, false
	}
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])
	size5 := int(componentSizes[id5])

	oldArch := meta.Archetype
	addMask := makeMask5(id1, id2, id3, id4, id5)
	if includesAll(oldArch.mask, addMask) {
		idx1 := oldArch.getSlot(id1)
		idx2 := oldArch.getSlot(id2)
		idx3 := oldArch.getSlot(id3)
		idx4 := oldArch.getSlot(id4)
		idx5 := oldArch.getSlot(id5)
		if idx1 == -1 || idx2 == -1 || idx3 == -1 || idx4 == -1 || idx5 == -1 {
			return nil, nil, nil, nil, nil, false
		}
		bytes1 := oldArch.componentData[idx1]
		bytes2 := oldArch.componentData[idx2]
		bytes3 := oldArch.componentData[idx3]
		bytes4 := oldArch.componentData[idx4]
		bytes5 := oldArch.componentData[idx5]
		if meta.Index*size1 >= len(bytes1) || meta.Index*size2 >= len(bytes2) || meta.Index*size3 >= len(bytes3) || meta.Index*size4 >= len(bytes4) || meta.Index*size5 >= len(bytes5) {
			return nil, nil, nil, nil, nil, false
		}
		return (*T1)(unsafe.Pointer(&bytes1[meta.Index*size1])), (*T2)(unsafe.Pointer(&bytes2[meta.Index*size2])), (*T3)(unsafe.Pointer(&bytes3[meta.Index*size3])), (*T4)(unsafe.Pointer(&bytes4[meta.Index*size4])), (*T5)(unsafe.Pointer(&bytes5[meta.Index*size5])), true
	}

	newMask := orMask(oldArch.mask, addMask)

	var transition Transition
	addMap, ok := w.addTransitions[oldArch]
	if ok {
		if tr, ok := addMap[addMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.addTransitions[oldArch]; !ok {
			w.addTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.addTransitions[oldArch][addMask] = transition
	} else {
		newArch = transition.target
	}

	oldIndex := meta.Index
	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

	// Extend new components (zero-initialized)
	ids := []ComponentID{id1, id2, id3, id4, id5}
	for _, id := range ids {
		if !oldArch.mask.has(id) {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return nil, nil, nil, nil, nil, false
			}
			bytes := newArch.componentData[idx]
			bytes = extendByteSlice(bytes, int(componentSizes[id]))
			newArch.componentData[idx] = bytes
		}
	}

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entitiesSlice[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	idx1 := newArch.getSlot(id1)
	idx2 := newArch.getSlot(id2)
	idx3 := newArch.getSlot(id3)
	idx4 := newArch.getSlot(id4)
	idx5 := newArch.getSlot(id5)
	if idx1 == -1 || idx2 == -1 || idx3 == -1 || idx4 == -1 || idx5 == -1 {
		return nil, nil, nil, nil, nil, false
	}
	bytes1 := newArch.componentData[idx1]
	bytes2 := newArch.componentData[idx2]
	bytes3 := newArch.componentData[idx3]
	bytes4 := newArch.componentData[idx4]
	bytes5 := newArch.componentData[idx5]
	return (*T1)(unsafe.Pointer(&bytes1[newIndex*size1])), (*T2)(unsafe.Pointer(&bytes2[newIndex*size2])), (*T3)(unsafe.Pointer(&bytes3[newIndex*size3])), (*T4)(unsafe.Pointer(&bytes4[newIndex*size4])), (*T5)(unsafe.Pointer(&bytes5[newIndex*size5])), true
}

// AddComponentBatch adds a component to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch[T any](w *World, entities []Entity) []*T {
	id, ok := TryGetID[T]()
	if !ok {
		return nil
	}
	addMask := makeMask1(id)
	size := int(componentSizes[id])

	// Sort to group by archetype without map
	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res := make([]*T, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot := oldArch.getSlot(id)
			if slot == -1 {
				continue
			}
			base := unsafe.Pointer(&oldArch.componentData[slot][0])
			stride := uintptr(size)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p := unsafe.Pointer(uintptr(base) + uintptr(meta.Index)*stride)
				res[gi] = (*T)(p)
			}
			continue
		}

		newMask := orMask(oldArch.mask, addMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[addMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][addMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		// Pre-extend all component data
		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot := newArch.getSlot(id)
		base := unsafe.Pointer(&newArch.componentData[slot][0])
		stride := uintptr(size)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num) // pre-allocate with cap

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			// Copy existing components
			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p := unsafe.Pointer(uintptr(base) + uintptr(newIndex)*stride)
			res[gi] = (*T)(p)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res
}

// AddComponentBatch2 adds two components to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch2[T1 any, T2 any](w *World, entities []Entity) ([]*T1, []*T2) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	if !ok1 || !ok2 {
		return nil, nil
	}
	addMask := makeMask2(id1, id2)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])

	// Sort to group by archetype without map
	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res1 := make([]*T1, len(entities))
	res2 := make([]*T2, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			if slot1 == -1 || slot2 == -1 {
				continue
			}
			base1 := unsafe.Pointer(&oldArch.componentData[slot1][0])
			base2 := unsafe.Pointer(&oldArch.componentData[slot2][0])
			stride1 := uintptr(size1)
			stride2 := uintptr(size2)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p1 := unsafe.Pointer(uintptr(base1) + uintptr(meta.Index)*stride1)
				p2 := unsafe.Pointer(uintptr(base2) + uintptr(meta.Index)*stride2)
				res1[gi] = (*T1)(p1)
				res2[gi] = (*T2)(p2)
			}
			continue
		}

		newMask := orMask(oldArch.mask, addMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[addMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][addMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		base1 := unsafe.Pointer(&newArch.componentData[slot1][0])
		base2 := unsafe.Pointer(&newArch.componentData[slot2][0])
		stride1 := uintptr(size1)
		stride2 := uintptr(size2)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p1 := unsafe.Pointer(uintptr(base1) + uintptr(newIndex)*stride1)
			p2 := unsafe.Pointer(uintptr(base2) + uintptr(newIndex)*stride2)
			res1[gi] = (*T1)(p1)
			res2[gi] = (*T2)(p2)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res1, res2
}

// AddComponentBatch3 adds three components to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch3[T1 any, T2 any, T3 any](w *World, entities []Entity) ([]*T1, []*T2, []*T3) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	if !ok1 || !ok2 || !ok3 {
		return nil, nil, nil
	}
	addMask := makeMask3(id1, id2, id3)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res1 := make([]*T1, len(entities))
	res2 := make([]*T2, len(entities))
	res3 := make([]*T3, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 {
				continue
			}
			base1 := unsafe.Pointer(&oldArch.componentData[slot1][0])
			base2 := unsafe.Pointer(&oldArch.componentData[slot2][0])
			base3 := unsafe.Pointer(&oldArch.componentData[slot3][0])
			stride1 := uintptr(size1)
			stride2 := uintptr(size2)
			stride3 := uintptr(size3)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p1 := unsafe.Pointer(uintptr(base1) + uintptr(meta.Index)*stride1)
				p2 := unsafe.Pointer(uintptr(base2) + uintptr(meta.Index)*stride2)
				p3 := unsafe.Pointer(uintptr(base3) + uintptr(meta.Index)*stride3)
				res1[gi] = (*T1)(p1)
				res2[gi] = (*T2)(p2)
				res3[gi] = (*T3)(p3)
			}
			continue
		}

		newMask := orMask(oldArch.mask, addMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[addMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][addMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		base1 := unsafe.Pointer(&newArch.componentData[slot1][0])
		base2 := unsafe.Pointer(&newArch.componentData[slot2][0])
		base3 := unsafe.Pointer(&newArch.componentData[slot3][0])
		stride1 := uintptr(size1)
		stride2 := uintptr(size2)
		stride3 := uintptr(size3)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p1 := unsafe.Pointer(uintptr(base1) + uintptr(newIndex)*stride1)
			p2 := unsafe.Pointer(uintptr(base2) + uintptr(newIndex)*stride2)
			p3 := unsafe.Pointer(uintptr(base3) + uintptr(newIndex)*stride3)
			res1[gi] = (*T1)(p1)
			res2[gi] = (*T2)(p2)
			res3[gi] = (*T3)(p3)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res1, res2, res3
}

// AddComponentBatch4 adds four components to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch4[T1 any, T2 any, T3 any, T4 any](w *World, entities []Entity) ([]*T1, []*T2, []*T3, []*T4) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return nil, nil, nil, nil
	}
	addMask := makeMask4(id1, id2, id3, id4)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res1 := make([]*T1, len(entities))
	res2 := make([]*T2, len(entities))
	res3 := make([]*T3, len(entities))
	res4 := make([]*T4, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			slot4 := oldArch.getSlot(id4)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 || slot4 == -1 {
				continue
			}
			base1 := unsafe.Pointer(&oldArch.componentData[slot1][0])
			base2 := unsafe.Pointer(&oldArch.componentData[slot2][0])
			base3 := unsafe.Pointer(&oldArch.componentData[slot3][0])
			base4 := unsafe.Pointer(&oldArch.componentData[slot4][0])
			stride1 := uintptr(size1)
			stride2 := uintptr(size2)
			stride3 := uintptr(size3)
			stride4 := uintptr(size4)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p1 := unsafe.Pointer(uintptr(base1) + uintptr(meta.Index)*stride1)
				p2 := unsafe.Pointer(uintptr(base2) + uintptr(meta.Index)*stride2)
				p3 := unsafe.Pointer(uintptr(base3) + uintptr(meta.Index)*stride3)
				p4 := unsafe.Pointer(uintptr(base4) + uintptr(meta.Index)*stride4)
				res1[gi] = (*T1)(p1)
				res2[gi] = (*T2)(p2)
				res3[gi] = (*T3)(p3)
				res4[gi] = (*T4)(p4)
			}
			continue
		}

		newMask := orMask(oldArch.mask, addMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[addMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][addMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		slot4 := newArch.getSlot(id4)
		base1 := unsafe.Pointer(&newArch.componentData[slot1][0])
		base2 := unsafe.Pointer(&newArch.componentData[slot2][0])
		base3 := unsafe.Pointer(&newArch.componentData[slot3][0])
		base4 := unsafe.Pointer(&newArch.componentData[slot4][0])
		stride1 := uintptr(size1)
		stride2 := uintptr(size2)
		stride3 := uintptr(size3)
		stride4 := uintptr(size4)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p1 := unsafe.Pointer(uintptr(base1) + uintptr(newIndex)*stride1)
			p2 := unsafe.Pointer(uintptr(base2) + uintptr(newIndex)*stride2)
			p3 := unsafe.Pointer(uintptr(base3) + uintptr(newIndex)*stride3)
			p4 := unsafe.Pointer(uintptr(base4) + uintptr(newIndex)*stride4)
			res1[gi] = (*T1)(p1)
			res2[gi] = (*T2)(p2)
			res3[gi] = (*T3)(p3)
			res4[gi] = (*T4)(p4)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res1, res2, res3, res4
}

// AddComponentBatch5 adds five components to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, entities []Entity) ([]*T1, []*T2, []*T3, []*T4, []*T5) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	id5, ok5 := TryGetID[T5]()
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return nil, nil, nil, nil, nil
	}
	addMask := makeMask5(id1, id2, id3, id4, id5)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])
	size5 := int(componentSizes[id5])

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res1 := make([]*T1, len(entities))
	res2 := make([]*T2, len(entities))
	res3 := make([]*T3, len(entities))
	res4 := make([]*T4, len(entities))
	res5 := make([]*T5, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			slot4 := oldArch.getSlot(id4)
			slot5 := oldArch.getSlot(id5)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 || slot4 == -1 || slot5 == -1 {
				continue
			}
			base1 := unsafe.Pointer(&oldArch.componentData[slot1][0])
			base2 := unsafe.Pointer(&oldArch.componentData[slot2][0])
			base3 := unsafe.Pointer(&oldArch.componentData[slot3][0])
			base4 := unsafe.Pointer(&oldArch.componentData[slot4][0])
			base5 := unsafe.Pointer(&oldArch.componentData[slot5][0])
			stride1 := uintptr(size1)
			stride2 := uintptr(size2)
			stride3 := uintptr(size3)
			stride4 := uintptr(size4)
			stride5 := uintptr(size5)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p1 := unsafe.Pointer(uintptr(base1) + uintptr(meta.Index)*stride1)
				p2 := unsafe.Pointer(uintptr(base2) + uintptr(meta.Index)*stride2)
				p3 := unsafe.Pointer(uintptr(base3) + uintptr(meta.Index)*stride3)
				p4 := unsafe.Pointer(uintptr(base4) + uintptr(meta.Index)*stride4)
				p5 := unsafe.Pointer(uintptr(base5) + uintptr(meta.Index)*stride5)
				res1[gi] = (*T1)(p1)
				res2[gi] = (*T2)(p2)
				res3[gi] = (*T3)(p3)
				res4[gi] = (*T4)(p4)
				res5[gi] = (*T5)(p5)
			}
			continue
		}

		newMask := orMask(oldArch.mask, addMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[addMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][addMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		slot4 := newArch.getSlot(id4)
		slot5 := newArch.getSlot(id5)
		base1 := unsafe.Pointer(&newArch.componentData[slot1][0])
		base2 := unsafe.Pointer(&newArch.componentData[slot2][0])
		base3 := unsafe.Pointer(&newArch.componentData[slot3][0])
		base4 := unsafe.Pointer(&newArch.componentData[slot4][0])
		base5 := unsafe.Pointer(&newArch.componentData[slot5][0])
		stride1 := uintptr(size1)
		stride2 := uintptr(size2)
		stride3 := uintptr(size3)
		stride4 := uintptr(size4)
		stride5 := uintptr(size5)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p1 := unsafe.Pointer(uintptr(base1) + uintptr(newIndex)*stride1)
			p2 := unsafe.Pointer(uintptr(base2) + uintptr(newIndex)*stride2)
			p3 := unsafe.Pointer(uintptr(base3) + uintptr(newIndex)*stride3)
			p4 := unsafe.Pointer(uintptr(base4) + uintptr(newIndex)*stride4)
			p5 := unsafe.Pointer(uintptr(base5) + uintptr(newIndex)*stride5)
			res1[gi] = (*T1)(p1)
			res2[gi] = (*T2)(p2)
			res3[gi] = (*T3)(p3)
			res4[gi] = (*T4)(p4)
			res5[gi] = (*T5)(p5)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res1, res2, res3, res4, res5
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
	addMask := makeMask1(compID)
	if intersects(oldArch.mask, addMask) {
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
		newMask := orMask(oldArch.mask, addMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[addMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][addMask] = transition
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

// SetComponent2 sets two components for an entity.
// If any component is missing, it adds them; otherwise, updates existing ones.
// It returns a boolean indicating success.
func SetComponent2[T1 any, T2 any](w *World, e Entity, comp1 T1, comp2 T2) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	if !ok1 || !ok2 {
		return false
	}
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)

	oldArch := meta.Archetype
	setMask := makeMask2(id1, id2)
	if includesAll(oldArch.mask, setMask) {
		idx1 := oldArch.getSlot(id1)
		idx2 := oldArch.getSlot(id2)
		if idx1 == -1 || idx2 == -1 {
			return false
		}
		bytes1 := oldArch.componentData[idx1]
		bytes2 := oldArch.componentData[idx2]
		if meta.Index*size1 >= len(bytes1) || meta.Index*size2 >= len(bytes2) {
			return false
		}
		copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
		copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
		return true
	} else {
		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		oldIndex := meta.Index
		newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

		ids := []ComponentID{id1, id2}
		srcs := [][]byte{src1, src2}
		sizes := []int{size1, size2}
		for i, id := range ids {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return false
			}
			bytes := newArch.componentData[idx]
			if oldArch.mask.has(id) {
				copy(bytes[newIndex*sizes[i]:(newIndex+1)*sizes[i]], srcs[i])
			} else {
				bytes = extendByteSlice(bytes, sizes[i])
				copy(bytes[len(bytes)-sizes[i]:], srcs[i])
				newArch.componentData[idx] = bytes
			}
		}

		meta.Archetype = newArch
		meta.Index = newIndex
		w.entitiesSlice[e.ID] = meta

		w.removeEntityFromArchetype(e, oldArch, oldIndex)
		return true
	}
}

// SetComponent3 sets three components for an entity.
// If any component is missing, it adds them; otherwise, updates existing ones.
// It returns a boolean indicating success.
func SetComponent3[T1 any, T2 any, T3 any](w *World, e Entity, comp1 T1, comp2 T2, comp3 T3) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	if !ok1 || !ok2 || !ok3 {
		return false
	}
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&comp3)), size3)

	oldArch := meta.Archetype
	setMask := makeMask3(id1, id2, id3)
	if includesAll(oldArch.mask, setMask) {
		idx1 := oldArch.getSlot(id1)
		idx2 := oldArch.getSlot(id2)
		idx3 := oldArch.getSlot(id3)
		if idx1 == -1 || idx2 == -1 || idx3 == -1 {
			return false
		}
		bytes1 := oldArch.componentData[idx1]
		bytes2 := oldArch.componentData[idx2]
		bytes3 := oldArch.componentData[idx3]
		if meta.Index*size1 >= len(bytes1) || meta.Index*size2 >= len(bytes2) || meta.Index*size3 >= len(bytes3) {
			return false
		}
		copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
		copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
		copy(bytes3[meta.Index*size3:(meta.Index+1)*size3], src3)
		return true
	} else {
		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		oldIndex := meta.Index
		newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

		ids := []ComponentID{id1, id2, id3}
		srcs := [][]byte{src1, src2, src3}
		sizes := []int{size1, size2, size3}
		for i, id := range ids {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return false
			}
			bytes := newArch.componentData[idx]
			if oldArch.mask.has(id) {
				copy(bytes[newIndex*sizes[i]:(newIndex+1)*sizes[i]], srcs[i])
			} else {
				bytes = extendByteSlice(bytes, sizes[i])
				copy(bytes[len(bytes)-sizes[i]:], srcs[i])
				newArch.componentData[idx] = bytes
			}
		}

		meta.Archetype = newArch
		meta.Index = newIndex
		w.entitiesSlice[e.ID] = meta

		w.removeEntityFromArchetype(e, oldArch, oldIndex)
		return true
	}
}

// SetComponent4 sets four components for an entity.
// If any component is missing, it adds them; otherwise, updates existing ones.
// It returns a boolean indicating success.
func SetComponent4[T1 any, T2 any, T3 any, T4 any](w *World, e Entity, comp1 T1, comp2 T2, comp3 T3, comp4 T4) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return false
	}
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&comp3)), size3)
	src4 := unsafe.Slice((*byte)(unsafe.Pointer(&comp4)), size4)

	oldArch := meta.Archetype
	setMask := makeMask4(id1, id2, id3, id4)
	if includesAll(oldArch.mask, setMask) {
		idx1 := oldArch.getSlot(id1)
		idx2 := oldArch.getSlot(id2)
		idx3 := oldArch.getSlot(id3)
		idx4 := oldArch.getSlot(id4)
		if idx1 == -1 || idx2 == -1 || idx3 == -1 || idx4 == -1 {
			return false
		}
		bytes1 := oldArch.componentData[idx1]
		bytes2 := oldArch.componentData[idx2]
		bytes3 := oldArch.componentData[idx3]
		bytes4 := oldArch.componentData[idx4]
		if meta.Index*size1 >= len(bytes1) || meta.Index*size2 >= len(bytes2) || meta.Index*size3 >= len(bytes3) || meta.Index*size4 >= len(bytes4) {
			return false
		}
		copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
		copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
		copy(bytes3[meta.Index*size3:(meta.Index+1)*size3], src3)
		copy(bytes4[meta.Index*size4:(meta.Index+1)*size4], src4)
		return true
	} else {
		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		oldIndex := meta.Index
		newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

		ids := []ComponentID{id1, id2, id3, id4}
		srcs := [][]byte{src1, src2, src3, src4}
		sizes := []int{size1, size2, size3, size4}
		for i, id := range ids {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return false
			}
			bytes := newArch.componentData[idx]
			if oldArch.mask.has(id) {
				copy(bytes[newIndex*sizes[i]:(newIndex+1)*sizes[i]], srcs[i])
			} else {
				bytes = extendByteSlice(bytes, sizes[i])
				copy(bytes[len(bytes)-sizes[i]:], srcs[i])
				newArch.componentData[idx] = bytes
			}
		}

		meta.Archetype = newArch
		meta.Index = newIndex
		w.entitiesSlice[e.ID] = meta

		w.removeEntityFromArchetype(e, oldArch, oldIndex)
		return true
	}
}

// SetComponent5 sets five components for an entity.
// If any component is missing, it adds them; otherwise, updates existing ones.
// It returns a boolean indicating success.
func SetComponent5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, e Entity, comp1 T1, comp2 T2, comp3 T3, comp4 T4, comp5 T5) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	id5, ok5 := TryGetID[T5]()
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return false
	}
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])
	size5 := int(componentSizes[id5])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&comp3)), size3)
	src4 := unsafe.Slice((*byte)(unsafe.Pointer(&comp4)), size4)
	src5 := unsafe.Slice((*byte)(unsafe.Pointer(&comp5)), size5)

	oldArch := meta.Archetype
	setMask := makeMask5(id1, id2, id3, id4, id5)
	if includesAll(oldArch.mask, setMask) {
		idx1 := oldArch.getSlot(id1)
		idx2 := oldArch.getSlot(id2)
		idx3 := oldArch.getSlot(id3)
		idx4 := oldArch.getSlot(id4)
		idx5 := oldArch.getSlot(id5)
		if idx1 == -1 || idx2 == -1 || idx3 == -1 || idx4 == -1 || idx5 == -1 {
			return false
		}
		bytes1 := oldArch.componentData[idx1]
		bytes2 := oldArch.componentData[idx2]
		bytes3 := oldArch.componentData[idx3]
		bytes4 := oldArch.componentData[idx4]
		bytes5 := oldArch.componentData[idx5]
		if meta.Index*size1 >= len(bytes1) || meta.Index*size2 >= len(bytes2) || meta.Index*size3 >= len(bytes3) || meta.Index*size4 >= len(bytes4) || meta.Index*size5 >= len(bytes5) {
			return false
		}
		copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
		copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
		copy(bytes3[meta.Index*size3:(meta.Index+1)*size3], src3)
		copy(bytes4[meta.Index*size4:(meta.Index+1)*size4], src4)
		copy(bytes5[meta.Index*size5:(meta.Index+1)*size5], src5)
		return true
	} else {
		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		oldIndex := meta.Index
		newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

		ids := []ComponentID{id1, id2, id3, id4, id5}
		srcs := [][]byte{src1, src2, src3, src4, src5}
		sizes := []int{size1, size2, size3, size4, size5}
		for i, id := range ids {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return false
			}
			bytes := newArch.componentData[idx]
			if oldArch.mask.has(id) {
				copy(bytes[newIndex*sizes[i]:(newIndex+1)*sizes[i]], srcs[i])
			} else {
				bytes = extendByteSlice(bytes, sizes[i])
				copy(bytes[len(bytes)-sizes[i]:], srcs[i])
				newArch.componentData[idx] = bytes
			}
		}

		meta.Archetype = newArch
		meta.Index = newIndex
		w.entitiesSlice[e.ID] = meta

		w.removeEntityFromArchetype(e, oldArch, oldIndex)
		return true
	}
}

// SetComponentBatch sets a component to the same value for multiple entities.
// If the component is missing in some entities, it adds it.
// It does not return anything.
func SetComponentBatch[T any](w *World, entities []Entity, comp T) {
	id, ok := TryGetID[T]()
	if !ok {
		return
	}
	setMask := makeMask1(id)
	size := int(componentSizes[id])
	src := unsafe.Slice((*byte)(unsafe.Pointer(&comp)), size)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot := oldArch.getSlot(id)
			if slot == -1 {
				continue
			}
			bytes := oldArch.componentData[slot]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes[meta.Index*size:(meta.Index+1)*size], src)
			}
			continue
		}

		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot := newArch.getSlot(id)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			// Set the component
			bytes := newArch.componentData[slot]
			dstStart := len(bytes) - num*size + j*size
			copy(bytes[dstStart:dstStart+size], src)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// SetComponentBatch2 sets two components to the same values for multiple entities.
// If any component is missing in some entities, it adds them.
// It does not return anything.
func SetComponentBatch2[T1 any, T2 any](w *World, entities []Entity, comp1 T1, comp2 T2) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	if !ok1 || !ok2 {
		return
	}
	setMask := makeMask2(id1, id2)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			if slot1 == -1 || slot2 == -1 {
				continue
			}
			bytes1 := oldArch.componentData[slot1]
			bytes2 := oldArch.componentData[slot2]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
				copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
			}
			continue
		}

		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			// Set the components
			bytes1 := newArch.componentData[slot1]
			dstStart1 := len(bytes1) - num*size1 + j*size1
			copy(bytes1[dstStart1:dstStart1+size1], src1)
			bytes2 := newArch.componentData[slot2]
			dstStart2 := len(bytes2) - num*size2 + j*size2
			copy(bytes2[dstStart2:dstStart2+size2], src2)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// SetComponentBatch3 sets three components to the same values for multiple entities.
// If any component is missing in some entities, it adds them.
// It does not return anything.
func SetComponentBatch3[T1 any, T2 any, T3 any](w *World, entities []Entity, comp1 T1, comp2 T2, comp3 T3) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	if !ok1 || !ok2 || !ok3 {
		return
	}
	setMask := makeMask3(id1, id2, id3)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&comp3)), size3)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 {
				continue
			}
			bytes1 := oldArch.componentData[slot1]
			bytes2 := oldArch.componentData[slot2]
			bytes3 := oldArch.componentData[slot3]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
				copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
				copy(bytes3[meta.Index*size3:(meta.Index+1)*size3], src3)
			}
			continue
		}

		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			bytes1 := newArch.componentData[slot1]
			dstStart1 := len(bytes1) - num*size1 + j*size1
			copy(bytes1[dstStart1:dstStart1+size1], src1)
			bytes2 := newArch.componentData[slot2]
			dstStart2 := len(bytes2) - num*size2 + j*size2
			copy(bytes2[dstStart2:dstStart2+size2], src2)
			bytes3 := newArch.componentData[slot3]
			dstStart3 := len(bytes3) - num*size3 + j*size3
			copy(bytes3[dstStart3:dstStart3+size3], src3)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// SetComponentBatch4 sets four components to the same values for multiple entities.
// If any component is missing in some entities, it adds them.
// It does not return anything.
func SetComponentBatch4[T1 any, T2 any, T3 any, T4 any](w *World, entities []Entity, comp1 T1, comp2 T2, comp3 T3, comp4 T4) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return
	}
	setMask := makeMask4(id1, id2, id3, id4)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&comp3)), size3)
	src4 := unsafe.Slice((*byte)(unsafe.Pointer(&comp4)), size4)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			slot4 := oldArch.getSlot(id4)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 || slot4 == -1 {
				continue
			}
			bytes1 := oldArch.componentData[slot1]
			bytes2 := oldArch.componentData[slot2]
			bytes3 := oldArch.componentData[slot3]
			bytes4 := oldArch.componentData[slot4]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
				copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
				copy(bytes3[meta.Index*size3:(meta.Index+1)*size3], src3)
				copy(bytes4[meta.Index*size4:(meta.Index+1)*size4], src4)
			}
			continue
		}

		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		slot4 := newArch.getSlot(id4)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			bytes1 := newArch.componentData[slot1]
			dstStart1 := len(bytes1) - num*size1 + j*size1
			copy(bytes1[dstStart1:dstStart1+size1], src1)
			bytes2 := newArch.componentData[slot2]
			dstStart2 := len(bytes2) - num*size2 + j*size2
			copy(bytes2[dstStart2:dstStart2+size2], src2)
			bytes3 := newArch.componentData[slot3]
			dstStart3 := len(bytes3) - num*size3 + j*size3
			copy(bytes3[dstStart3:dstStart3+size3], src3)
			bytes4 := newArch.componentData[slot4]
			dstStart4 := len(bytes4) - num*size4 + j*size4
			copy(bytes4[dstStart4:dstStart4+size4], src4)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// SetComponentBatch5 sets five components to the same values for multiple entities.
// If any component is missing in some entities, it adds them.
// It does not return anything.
func SetComponentBatch5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, entities []Entity, comp1 T1, comp2 T2, comp3 T3, comp4 T4, comp5 T5) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	id5, ok5 := TryGetID[T5]()
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return
	}
	setMask := makeMask5(id1, id2, id3, id4, id5)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])
	size5 := int(componentSizes[id5])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&comp3)), size3)
	src4 := unsafe.Slice((*byte)(unsafe.Pointer(&comp4)), size4)
	src5 := unsafe.Slice((*byte)(unsafe.Pointer(&comp5)), size5)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			slot4 := oldArch.getSlot(id4)
			slot5 := oldArch.getSlot(id5)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 || slot4 == -1 || slot5 == -1 {
				continue
			}
			bytes1 := oldArch.componentData[slot1]
			bytes2 := oldArch.componentData[slot2]
			bytes3 := oldArch.componentData[slot3]
			bytes4 := oldArch.componentData[slot4]
			bytes5 := oldArch.componentData[slot5]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
				copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
				copy(bytes3[meta.Index*size3:(meta.Index+1)*size3], src3)
				copy(bytes4[meta.Index*size4:(meta.Index+1)*size4], src4)
				copy(bytes5[meta.Index*size5:(meta.Index+1)*size5], src5)
			}
			continue
		}

		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		slot4 := newArch.getSlot(id4)
		slot5 := newArch.getSlot(id5)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			bytes1 := newArch.componentData[slot1]
			dstStart1 := len(bytes1) - num*size1 + j*size1
			copy(bytes1[dstStart1:dstStart1+size1], src1)
			bytes2 := newArch.componentData[slot2]
			dstStart2 := len(bytes2) - num*size2 + j*size2
			copy(bytes2[dstStart2:dstStart2+size2], src2)
			bytes3 := newArch.componentData[slot3]
			dstStart3 := len(bytes3) - num*size3 + j*size3
			copy(bytes3[dstStart3:dstStart3+size3], src3)
			bytes4 := newArch.componentData[slot4]
			dstStart4 := len(bytes4) - num*size4 + j*size4
			copy(bytes4[dstStart4:dstStart4+size4], src4)
			bytes5 := newArch.componentData[slot5]
			dstStart5 := len(bytes5) - num*size5 + j*size5
			copy(bytes5[dstStart5:dstStart5+size5], src5)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
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
	removeMask := makeMask1(compID)
	if !intersects(oldArch.mask, removeMask) {
		return true
	}

	oldIndex := meta.Index

	newMask := andNotMask(oldArch.mask, removeMask)

	var transition Transition
	removeMap, ok := w.removeTransitions[oldArch]
	if ok {
		if tr, ok := removeMap[removeMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs)-1)
		for from, id := range oldArch.componentIDs {
			if removeMask.has(id) {
				continue
			}
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.removeTransitions[oldArch]; !ok {
			w.removeTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.removeTransitions[oldArch][removeMask] = transition
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

// RemoveComponent2 removes two components from an entity if present.
// It returns a boolean indicating success.
func RemoveComponent2[T1 any, T2 any](w *World, e Entity) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	if !ok1 || !ok2 {
		return false
	}

	oldArch := meta.Archetype
	removeMask := makeMask2(id1, id2)
	if !intersects(oldArch.mask, removeMask) {
		return true
	}

	oldIndex := meta.Index

	newMask := andNotMask(oldArch.mask, removeMask)

	var transition Transition
	removeMap, ok := w.removeTransitions[oldArch]
	if ok {
		if tr, ok := removeMap[removeMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			if removeMask.has(id) {
				continue
			}
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.removeTransitions[oldArch]; !ok {
			w.removeTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.removeTransitions[oldArch][removeMask] = transition
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

// RemoveComponent3 removes three components from an entity if present.
// It returns a boolean indicating success.
func RemoveComponent3[T1 any, T2 any, T3 any](w *World, e Entity) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	if !ok1 || !ok2 || !ok3 {
		return false
	}

	oldArch := meta.Archetype
	removeMask := makeMask3(id1, id2, id3)
	if !intersects(oldArch.mask, removeMask) {
		return true
	}

	oldIndex := meta.Index

	newMask := andNotMask(oldArch.mask, removeMask)

	var transition Transition
	removeMap, ok := w.removeTransitions[oldArch]
	if ok {
		if tr, ok := removeMap[removeMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			if removeMask.has(id) {
				continue
			}
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.removeTransitions[oldArch]; !ok {
			w.removeTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.removeTransitions[oldArch][removeMask] = transition
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

// RemoveComponent4 removes four components from an entity if present.
// It returns a boolean indicating success.
func RemoveComponent4[T1 any, T2 any, T3 any, T4 any](w *World, e Entity) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return false
	}

	oldArch := meta.Archetype
	removeMask := makeMask4(id1, id2, id3, id4)
	if !intersects(oldArch.mask, removeMask) {
		return true
	}

	oldIndex := meta.Index

	newMask := andNotMask(oldArch.mask, removeMask)

	var transition Transition
	removeMap, ok := w.removeTransitions[oldArch]
	if ok {
		if tr, ok := removeMap[removeMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			if removeMask.has(id) {
				continue
			}
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.removeTransitions[oldArch]; !ok {
			w.removeTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.removeTransitions[oldArch][removeMask] = transition
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

// RemoveComponent5 removes five components from an entity if present.
// It returns a boolean indicating success.
func RemoveComponent5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, e Entity) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	id5, ok5 := TryGetID[T5]()
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return false
	}

	oldArch := meta.Archetype
	removeMask := makeMask5(id1, id2, id3, id4, id5)
	if !intersects(oldArch.mask, removeMask) {
		return true
	}

	oldIndex := meta.Index

	newMask := andNotMask(oldArch.mask, removeMask)

	var transition Transition
	removeMap, ok := w.removeTransitions[oldArch]
	if ok {
		if tr, ok := removeMap[removeMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			if removeMask.has(id) {
				continue
			}
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.removeTransitions[oldArch]; !ok {
			w.removeTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.removeTransitions[oldArch][removeMask] = transition
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

// removePair is used for batch removals.
type removePair struct {
	index int
	e     Entity
}

// RemoveComponentBatch removes a component from multiple entities if present.
func RemoveComponentBatch[T any](w *World, entities []Entity) {
	id, ok := TryGetID[T]()
	if !ok {
		return
	}
	removeMask := makeMask1(id)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

		newMask := andNotMask(oldArch.mask, removeMask)
		var transition Transition
		removeMap, ok := w.removeTransitions[oldArch]
		if ok {
			if tr, ok := removeMap[removeMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				if removeMask.has(id) {
					continue
				}
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.removeTransitions[oldArch]; !ok {
				w.removeTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.removeTransitions[oldArch][removeMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// RemoveComponentBatch2 removes two components from multiple entities if present.
func RemoveComponentBatch2[T1 any, T2 any](w *World, entities []Entity) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	if !ok1 || !ok2 {
		return
	}
	removeMask := makeMask2(id1, id2)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

		newMask := andNotMask(oldArch.mask, removeMask)
		var transition Transition
		removeMap, ok := w.removeTransitions[oldArch]
		if ok {
			if tr, ok := removeMap[removeMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				if removeMask.has(id) {
					continue
				}
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.removeTransitions[oldArch]; !ok {
				w.removeTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.removeTransitions[oldArch][removeMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// RemoveComponentBatch3 removes three components from multiple entities if present.
func RemoveComponentBatch3[T1 any, T2 any, T3 any](w *World, entities []Entity) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	if !ok1 || !ok2 || !ok3 {
		return
	}
	removeMask := makeMask3(id1, id2, id3)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

		newMask := andNotMask(oldArch.mask, removeMask)
		var transition Transition
		removeMap, ok := w.removeTransitions[oldArch]
		if ok {
			if tr, ok := removeMap[removeMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				if removeMask.has(id) {
					continue
				}
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.removeTransitions[oldArch]; !ok {
				w.removeTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.removeTransitions[oldArch][removeMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// RemoveComponentBatch4 removes four components from multiple entities if present.
func RemoveComponentBatch4[T1 any, T2 any, T3 any, T4 any](w *World, entities []Entity) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return
	}
	removeMask := makeMask4(id1, id2, id3, id4)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

		newMask := andNotMask(oldArch.mask, removeMask)
		var transition Transition
		removeMap, ok := w.removeTransitions[oldArch]
		if ok {
			if tr, ok := removeMap[removeMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				if removeMask.has(id) {
					continue
				}
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.removeTransitions[oldArch]; !ok {
				w.removeTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.removeTransitions[oldArch][removeMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// RemoveComponentBatch5 removes five components from multiple entities if present.
func RemoveComponentBatch5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, entities []Entity) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	id5, ok5 := TryGetID[T5]()
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return
	}
	removeMask := makeMask5(id1, id2, id3, id4, id5)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

		newMask := andNotMask(oldArch.mask, removeMask)
		var transition Transition
		removeMap, ok := w.removeTransitions[oldArch]
		if ok {
			if tr, ok := removeMap[removeMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				if removeMask.has(id) {
					continue
				}
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.removeTransitions[oldArch]; !ok {
				w.removeTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.removeTransitions[oldArch][removeMask] = transition
		} else {
			newArch = transition.target
		}

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
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
