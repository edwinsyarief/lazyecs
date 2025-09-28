package lazyecs

import "unsafe"

// Query is an iterator over entities that have a specific set of components.
type Query[T any] struct {
	world         *World
	currentArch   *Archetype
	base1         unsafe.Pointer
	includeMask   maskType
	excludeMask   maskType
	archIdx       int
	index         int
	stride1       uintptr
	currentEntity Entity
	id1           ComponentID
}

// CreateQuery creates a new query for entities with one specific component type.
func CreateQuery[T any](w *World, excludes ...ComponentID) *Query[T] {
	id1 := GetID[T]()
	return &Query[T]{
		world:       w,
		includeMask: makeMask1(id1),
		excludeMask: makeMask(excludes),
		id1:         id1,
		archIdx:     0,
		index:       -1,
	}
}

// New creates a new query for entities with one specific component type.
func (self *Query[T]) New(w *World, excludes ...ComponentID) *Query[T] {
	return CreateQuery[T](w, excludes...)
}

// Reset resets the query for reuse.
func (self *Query[T]) Reset() {
	self.archIdx = 0
	self.index = -1
	self.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (self *Query[T]) Next() bool {
	self.index++
	if self.currentArch != nil && self.index < len(self.currentArch.entities) {
		self.currentEntity = self.currentArch.entities[self.index]
		return true
	}
	if self.archIdx == -1 {
		return false // End of special query
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
func (self *Query[T]) Get() *T {
	p1 := unsafe.Pointer(uintptr(self.base1) + uintptr(self.index)*self.stride1)
	return (*T)(p1)
}

// Entity returns the current entity.
func (self *Query[T]) Entity() Entity {
	return self.currentEntity
}
