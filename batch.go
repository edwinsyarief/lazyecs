// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

import (
	"unsafe"
)

// Batch provides a way to create entities with a pre-defined set of components.
// This is much faster than creating an entity and adding components one by one.
type Batch[T1 any] struct {
	world *World
	arch  *Archetype
	id1   ComponentID
	size1 int
}

// CreateBatch creates a new Batch for creating entities with one component type.
// It pre-computes the archetype, making entity creation very fast.
func CreateBatch[T1 any](w *World) *Batch[T1] {
	id1 := GetID[T1]()
	mask := makeMask1(id1)
	arch := w.getOrCreateArchetype(mask)
	return &Batch[T1]{
		world: w,
		arch:  arch,
		id1:   id1,
		size1: int(componentSizes[id1]),
	}
}

// CreateEntities creates a specified number of entities with the batch's components.
// The components are zero-initialized.
// This function is designed to be allocation-free if the world has enough capacity.
func (self *Batch[T1]) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}
	w := self.world
	arch := self.arch

	entities := make([]Entity, count)
	startIndex := len(arch.entities)
	arch.entities = extendSlice(arch.entities, count)

	// Extend component slices
	arch.componentData[arch.getSlot(self.id1)] = extendByteSlice(arch.componentData[arch.getSlot(self.id1)], count*self.size1)

	maxID := uint32(0)
	for i := 0; i < count; i++ {
		var id uint32
		if len(w.freeEntityIDs) > 0 {
			id = w.freeEntityIDs[len(w.freeEntityIDs)-1]
			w.freeEntityIDs = w.freeEntityIDs[:len(w.freeEntityIDs)-1]
		} else {
			id = w.nextEntityID
			w.nextEntityID++
		}
		if id > maxID {
			maxID = id
		}
		version := uint32(1)
		if int(id) < len(w.entitiesSlice) {
			version = w.entitiesSlice[id].Version + 1
			if version == 0 { // Handle overflow
				version = 1
			}
		}
		e := Entity{ID: id, Version: version}
		entities[i] = e
		arch.entities[startIndex+i] = e
	}

	if int(maxID) >= len(w.entitiesSlice) {
		w.entitiesSlice = extendSlice(w.entitiesSlice, int(maxID)-len(w.entitiesSlice)+1)
	}

	for i := 0; i < count; i++ {
		e := entities[i]
		idx := startIndex + i
		w.entitiesSlice[e.ID] = entityMeta{Archetype: arch, Index: idx, Version: e.Version}
	}
	return entities
}

// CreateEntitiesWithComponent creates entities with the specified component value.
func (self *Batch[T1]) CreateEntitiesWithComponent(count int, c1 T1) []Entity {
	entities := self.CreateEntities(count)
	if len(entities) == 0 {
		return nil
	}
	arch := self.arch
	slot1 := arch.getSlot(self.id1)
	data1 := arch.componentData[slot1]
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&c1)), self.size1)

	startIndex := len(data1) - count*self.size1
	for i := 0; i < count; i++ {
		copy(data1[startIndex+i*self.size1:], src1)
	}
	return entities
}

// Batch2 provides a way to create entities with a pre-defined set of components.
type Batch2[T1 any, T2 any] struct {
	world *World
	arch  *Archetype
	id1   ComponentID
	id2   ComponentID
	size1 int
	size2 int
}

// CreateBatch2 creates a new Batch for creating entities with two component types.
func CreateBatch2[T1 any, T2 any](w *World) *Batch2[T1, T2] {
	id1, id2 := GetID[T1](), GetID[T2]()
	mask := makeMask2(id1, id2)
	arch := w.getOrCreateArchetype(mask)
	return &Batch2[T1, T2]{
		world: w,
		arch:  arch,
		id1:   id1,
		id2:   id2,
		size1: int(componentSizes[id1]),
		size2: int(componentSizes[id2]),
	}
}

// CreateEntities creates a specified number of entities with the batch's components.
func (self *Batch2[T1, T2]) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}
	w := self.world
	arch := self.arch

	entities := make([]Entity, count)
	startIndex := len(arch.entities)
	arch.entities = extendSlice(arch.entities, count)

	arch.componentData[arch.getSlot(self.id1)] = extendByteSlice(arch.componentData[arch.getSlot(self.id1)], count*self.size1)
	arch.componentData[arch.getSlot(self.id2)] = extendByteSlice(arch.componentData[arch.getSlot(self.id2)], count*self.size2)

	maxID := uint32(0)
	for i := 0; i < count; i++ {
		var id uint32
		if len(w.freeEntityIDs) > 0 {
			id = w.freeEntityIDs[len(w.freeEntityIDs)-1]
			w.freeEntityIDs = w.freeEntityIDs[:len(w.freeEntityIDs)-1]
		} else {
			id = w.nextEntityID
			w.nextEntityID++
		}
		if id > maxID {
			maxID = id
		}
		version := uint32(1)
		if int(id) < len(w.entitiesSlice) {
			version = w.entitiesSlice[id].Version + 1
			if version == 0 {
				version = 1
			}
		}
		e := Entity{ID: id, Version: version}
		entities[i] = e
		arch.entities[startIndex+i] = e
	}

	if int(maxID) >= len(w.entitiesSlice) {
		w.entitiesSlice = extendSlice(w.entitiesSlice, int(maxID)-len(w.entitiesSlice)+1)
	}

	for i := 0; i < count; i++ {
		e := entities[i]
		idx := startIndex + i
		w.entitiesSlice[e.ID] = entityMeta{Archetype: arch, Index: idx, Version: e.Version}
	}
	return entities
}

// CreateEntitiesWithComponents creates entities with the specified component values.
func (self *Batch2[T1, T2]) CreateEntitiesWithComponents(count int, c1 T1, c2 T2) []Entity {
	entities := self.CreateEntities(count)
	if len(entities) == 0 {
		return nil
	}
	arch := self.arch
	slot1, slot2 := arch.getSlot(self.id1), arch.getSlot(self.id2)
	data1, data2 := arch.componentData[slot1], arch.componentData[slot2]
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&c1)), self.size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&c2)), self.size2)

	startIndex1 := len(data1) - count*self.size1
	startIndex2 := len(data2) - count*self.size2
	for i := 0; i < count; i++ {
		copy(data1[startIndex1+i*self.size1:], src1)
		copy(data2[startIndex2+i*self.size2:], src2)
	}
	return entities
}

// Batch3 provides a way to create entities with a pre-defined set of components.
type Batch3[T1 any, T2 any, T3 any] struct {
	world               *World
	arch                *Archetype
	id1, id2, id3       ComponentID
	size1, size2, size3 int
}

// CreateBatch3 creates a new Batch for creating entities with three component types.
func CreateBatch3[T1 any, T2 any, T3 any](w *World) *Batch3[T1, T2, T3] {
	id1, id2, id3 := GetID[T1](), GetID[T2](), GetID[T3]()
	mask := makeMask3(id1, id2, id3)
	arch := w.getOrCreateArchetype(mask)
	return &Batch3[T1, T2, T3]{
		world: w,
		arch:  arch,
		id1:   id1, id2: id2, id3: id3,
		size1: int(componentSizes[id1]),
		size2: int(componentSizes[id2]),
		size3: int(componentSizes[id3]),
	}
}

// CreateEntities creates a specified number of entities with the batch's components.
func (self *Batch3[T1, T2, T3]) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}
	w := self.world
	arch := self.arch

	entities := make([]Entity, count)
	startIndex := len(arch.entities)
	arch.entities = extendSlice(arch.entities, count)

	arch.componentData[arch.getSlot(self.id1)] = extendByteSlice(arch.componentData[arch.getSlot(self.id1)], count*self.size1)
	arch.componentData[arch.getSlot(self.id2)] = extendByteSlice(arch.componentData[arch.getSlot(self.id2)], count*self.size2)
	arch.componentData[arch.getSlot(self.id3)] = extendByteSlice(arch.componentData[arch.getSlot(self.id3)], count*self.size3)

	maxID := uint32(0)
	for i := 0; i < count; i++ {
		var id uint32
		if len(w.freeEntityIDs) > 0 {
			id = w.freeEntityIDs[len(w.freeEntityIDs)-1]
			w.freeEntityIDs = w.freeEntityIDs[:len(w.freeEntityIDs)-1]
		} else {
			id = w.nextEntityID
			w.nextEntityID++
		}
		if id > maxID {
			maxID = id
		}
		version := uint32(1)
		if int(id) < len(w.entitiesSlice) {
			version = w.entitiesSlice[id].Version + 1
			if version == 0 {
				version = 1
			}
		}
		e := Entity{ID: id, Version: version}
		entities[i] = e
		arch.entities[startIndex+i] = e
	}

	if int(maxID) >= len(w.entitiesSlice) {
		w.entitiesSlice = extendSlice(w.entitiesSlice, int(maxID)-len(w.entitiesSlice)+1)
	}

	for i := 0; i < count; i++ {
		e := entities[i]
		idx := startIndex + i
		w.entitiesSlice[e.ID] = entityMeta{Archetype: arch, Index: idx, Version: e.Version}
	}
	return entities
}

// CreateEntitiesWithComponents creates entities with the specified component values.
func (self *Batch3[T1, T2, T3]) CreateEntitiesWithComponents(count int, c1 T1, c2 T2, c3 T3) []Entity {
	entities := self.CreateEntities(count)
	if len(entities) == 0 {
		return nil
	}
	arch := self.arch
	slot1, slot2, slot3 := arch.getSlot(self.id1), arch.getSlot(self.id2), arch.getSlot(self.id3)
	data1, data2, data3 := arch.componentData[slot1], arch.componentData[slot2], arch.componentData[slot3]
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&c1)), self.size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&c2)), self.size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&c3)), self.size3)

	startIndex1, startIndex2, startIndex3 := len(data1)-count*self.size1, len(data2)-count*self.size2, len(data3)-count*self.size3
	for i := 0; i < count; i++ {
		copy(data1[startIndex1+i*self.size1:], src1)
		copy(data2[startIndex2+i*self.size2:], src2)
		copy(data3[startIndex3+i*self.size3:], src3)
	}
	return entities
}

// Batch4 provides a way to create entities with a pre-defined set of components.
type Batch4[T1 any, T2 any, T3 any, T4 any] struct {
	world                      *World
	arch                       *Archetype
	id1, id2, id3, id4         ComponentID
	size1, size2, size3, size4 int
}

// CreateBatch4 creates a new Batch for creating entities with four component types.
func CreateBatch4[T1 any, T2 any, T3 any, T4 any](w *World) *Batch4[T1, T2, T3, T4] {
	id1, id2, id3, id4 := GetID[T1](), GetID[T2](), GetID[T3](), GetID[T4]()
	mask := makeMask4(id1, id2, id3, id4)
	arch := w.getOrCreateArchetype(mask)
	return &Batch4[T1, T2, T3, T4]{
		world: w,
		arch:  arch,
		id1:   id1, id2: id2, id3: id3, id4: id4,
		size1: int(componentSizes[id1]),
		size2: int(componentSizes[id2]),
		size3: int(componentSizes[id3]),
		size4: int(componentSizes[id4]),
	}
}

// CreateEntities creates a specified number of entities with the batch's components.
func (self *Batch4[T1, T2, T3, T4]) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}
	w := self.world
	arch := self.arch

	entities := make([]Entity, count)
	startIndex := len(arch.entities)
	arch.entities = extendSlice(arch.entities, count)

	arch.componentData[arch.getSlot(self.id1)] = extendByteSlice(arch.componentData[arch.getSlot(self.id1)], count*self.size1)
	arch.componentData[arch.getSlot(self.id2)] = extendByteSlice(arch.componentData[arch.getSlot(self.id2)], count*self.size2)
	arch.componentData[arch.getSlot(self.id3)] = extendByteSlice(arch.componentData[arch.getSlot(self.id3)], count*self.size3)
	arch.componentData[arch.getSlot(self.id4)] = extendByteSlice(arch.componentData[arch.getSlot(self.id4)], count*self.size4)

	maxID := uint32(0)
	for i := 0; i < count; i++ {
		var id uint32
		if len(w.freeEntityIDs) > 0 {
			id = w.freeEntityIDs[len(w.freeEntityIDs)-1]
			w.freeEntityIDs = w.freeEntityIDs[:len(w.freeEntityIDs)-1]
		} else {
			id = w.nextEntityID
			w.nextEntityID++
		}
		if id > maxID {
			maxID = id
		}
		version := uint32(1)
		if int(id) < len(w.entitiesSlice) {
			version = w.entitiesSlice[id].Version + 1
			if version == 0 {
				version = 1
			}
		}
		e := Entity{ID: id, Version: version}
		entities[i] = e
		arch.entities[startIndex+i] = e
	}

	if int(maxID) >= len(w.entitiesSlice) {
		w.entitiesSlice = extendSlice(w.entitiesSlice, int(maxID)-len(w.entitiesSlice)+1)
	}

	for i := 0; i < count; i++ {
		e := entities[i]
		idx := startIndex + i
		w.entitiesSlice[e.ID] = entityMeta{Archetype: arch, Index: idx, Version: e.Version}
	}
	return entities
}

// CreateEntitiesWithComponents creates entities with the specified component values.
func (self *Batch4[T1, T2, T3, T4]) CreateEntitiesWithComponents(count int, c1 T1, c2 T2, c3 T3, c4 T4) []Entity {
	entities := self.CreateEntities(count)
	if len(entities) == 0 {
		return nil
	}
	arch := self.arch
	slot1, slot2, slot3, slot4 := arch.getSlot(self.id1), arch.getSlot(self.id2), arch.getSlot(self.id3), arch.getSlot(self.id4)
	data1, data2, data3, data4 := arch.componentData[slot1], arch.componentData[slot2], arch.componentData[slot3], arch.componentData[slot4]
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&c1)), self.size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&c2)), self.size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&c3)), self.size3)
	src4 := unsafe.Slice((*byte)(unsafe.Pointer(&c4)), self.size4)

	startIndex1, startIndex2, startIndex3, startIndex4 := len(data1)-count*self.size1, len(data2)-count*self.size2, len(data3)-count*self.size3, len(data4)-count*self.size4
	for i := 0; i < count; i++ {
		copy(data1[startIndex1+i*self.size1:], src1)
		copy(data2[startIndex2+i*self.size2:], src2)
		copy(data3[startIndex3+i*self.size3:], src3)
		copy(data4[startIndex4+i*self.size4:], src4)
	}
	return entities
}

// Batch5 provides a way to create entities with a pre-defined set of components.
type Batch5[T1 any, T2 any, T3 any, T4 any, T5 any] struct {
	world                             *World
	arch                              *Archetype
	id1, id2, id3, id4, id5           ComponentID
	size1, size2, size3, size4, size5 int
}

// CreateBatch5 creates a new Batch for creating entities with five component types.
func CreateBatch5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World) *Batch5[T1, T2, T3, T4, T5] {
	id1, id2, id3, id4, id5 := GetID[T1](), GetID[T2](), GetID[T3](), GetID[T4](), GetID[T5]()
	mask := makeMask5(id1, id2, id3, id4, id5)
	arch := w.getOrCreateArchetype(mask)
	return &Batch5[T1, T2, T3, T4, T5]{
		world: w,
		arch:  arch,
		id1:   id1, id2: id2, id3: id3, id4: id4, id5: id5,
		size1: int(componentSizes[id1]),
		size2: int(componentSizes[id2]),
		size3: int(componentSizes[id3]),
		size4: int(componentSizes[id4]),
		size5: int(componentSizes[id5]),
	}
}

// CreateEntities creates a specified number of entities with the batch's components.
func (self *Batch5[T1, T2, T3, T4, T5]) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}
	w := self.world
	arch := self.arch

	entities := make([]Entity, count)
	startIndex := len(arch.entities)
	arch.entities = extendSlice(arch.entities, count)

	arch.componentData[arch.getSlot(self.id1)] = extendByteSlice(arch.componentData[arch.getSlot(self.id1)], count*self.size1)
	arch.componentData[arch.getSlot(self.id2)] = extendByteSlice(arch.componentData[arch.getSlot(self.id2)], count*self.size2)
	arch.componentData[arch.getSlot(self.id3)] = extendByteSlice(arch.componentData[arch.getSlot(self.id3)], count*self.size3)
	arch.componentData[arch.getSlot(self.id4)] = extendByteSlice(arch.componentData[arch.getSlot(self.id4)], count*self.size4)
	arch.componentData[arch.getSlot(self.id5)] = extendByteSlice(arch.componentData[arch.getSlot(self.id5)], count*self.size5)

	maxID := uint32(0)
	for i := 0; i < count; i++ {
		var id uint32
		if len(w.freeEntityIDs) > 0 {
			id = w.freeEntityIDs[len(w.freeEntityIDs)-1]
			w.freeEntityIDs = w.freeEntityIDs[:len(w.freeEntityIDs)-1]
		} else {
			id = w.nextEntityID
			w.nextEntityID++
		}
		if id > maxID {
			maxID = id
		}
		version := uint32(1)
		if int(id) < len(w.entitiesSlice) {
			version = w.entitiesSlice[id].Version + 1
			if version == 0 {
				version = 1
			}
		}
		e := Entity{ID: id, Version: version}
		entities[i] = e
		arch.entities[startIndex+i] = e
	}

	if int(maxID) >= len(w.entitiesSlice) {
		w.entitiesSlice = extendSlice(w.entitiesSlice, int(maxID)-len(w.entitiesSlice)+1)
	}

	for i := 0; i < count; i++ {
		e := entities[i]
		idx := startIndex + i
		w.entitiesSlice[e.ID] = entityMeta{Archetype: arch, Index: idx, Version: e.Version}
	}
	return entities
}

// CreateEntitiesWithComponents creates entities with the specified component values.
func (self *Batch5[T1, T2, T3, T4, T5]) CreateEntitiesWithComponents(count int, c1 T1, c2 T2, c3 T3, c4 T4, c5 T5) []Entity {
	entities := self.CreateEntities(count)
	if len(entities) == 0 {
		return nil
	}
	arch := self.arch
	slot1, slot2, slot3, slot4, slot5 := arch.getSlot(self.id1), arch.getSlot(self.id2), arch.getSlot(self.id3), arch.getSlot(self.id4), arch.getSlot(self.id5)
	data1, data2, data3, data4, data5 := arch.componentData[slot1], arch.componentData[slot2], arch.componentData[slot3], arch.componentData[slot4], arch.componentData[slot5]
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&c1)), self.size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&c2)), self.size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&c3)), self.size3)
	src4 := unsafe.Slice((*byte)(unsafe.Pointer(&c4)), self.size4)
	src5 := unsafe.Slice((*byte)(unsafe.Pointer(&c5)), self.size5)

	startIndex1, startIndex2, startIndex3, startIndex4, startIndex5 := len(data1)-count*self.size1, len(data2)-count*self.size2, len(data3)-count*self.size3, len(data4)-count*self.size4, len(data5)-count*self.size5
	for i := 0; i < count; i++ {
		copy(data1[startIndex1+i*self.size1:], src1)
		copy(data2[startIndex2+i*self.size2:], src2)
		copy(data3[startIndex3+i*self.size3:], src3)
		copy(data4[startIndex4+i*self.size4:], src4)
		copy(data5[startIndex5+i*self.size5:], src5)
	}
	return entities
}
