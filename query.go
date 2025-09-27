// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

import "unsafe"

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
