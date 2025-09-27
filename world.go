// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

import (
	"math/bits"
	"sync"
)

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
