package teishoku

import (
	"reflect"
	"unsafe"
)

// GetComponent2 retrieves pointers to the 2 components of type
// (T1, T2) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data (*T1, *T2), or nils if not found.
func GetComponent2[T1 any, T2 any](w *World, e Entity) (*T1, *T2) {
	if !w.IsValid(e) {
		return nil, nil
	}
	meta := w.entities.metas[e.ID]
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())

	if id2 == id1 {
		panic("ecs: duplicate component types in GetComponent2")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	if !has1 || !has2 {
		return nil, nil
	}
	chunk := a.chunks[meta.chunkIndex]
	ptr1 := unsafe.Pointer(uintptr(chunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(chunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
	return (*T1)(ptr1), (*T2)(ptr2)
}

// SetComponent2 adds or updates the 2 components (T1, T2) on the
// specified entity.
//
// If the entity does not already have all the components, this operation will
// cause the entity to move to a different archetype. If the entity is invalid,
// this function does nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
//   - v1: The component data of type T1 to set.
//   - v2: The component data of type T2 to set.
func SetComponent2[T1 any, T2 any](w *World, e Entity, v1 T1, v2 T2) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)

	if id2 == id1 {
		panic("ecs: duplicate component types in SetComponent2")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	oldChunk := a.chunks[meta.chunkIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	if has1 && has2 {
		ptr1 := unsafe.Pointer(uintptr(oldChunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(oldChunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(id1)
	}
	if !has2 {
		newMask.set(id2)
	}
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		if !has1 {
			tempSpecs[count] = compSpec{id: id1, typ: w.components.compIDToType[id1], size: w.components.compIDToSize[id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: id2, typ: w.components.compIDToType[id2], size: w.components.compIDToSize[id2]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(newChunk.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(newChunk.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// RemoveComponent2 removes the 2 components (T1, T2) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent2[T1 any, T2 any](w *World, e Entity) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)

	if id2 == id1 {
		panic("ecs: duplicate component types in RemoveComponent2")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	if !has1 && !has2 {
		return
	}
	newMask := a.mask
	newMask.unset(id1)
	newMask.unset(id2)
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	oldChunk := a.chunks[meta.chunkIndex]
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 {
			continue
		}
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// GetComponent3 retrieves pointers to the 3 components of type
// (T1, T2, T3) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3), or nils if not found.
func GetComponent3[T1 any, T2 any, T3 any](w *World, e Entity) (*T1, *T2, *T3) {
	if !w.IsValid(e) {
		return nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in GetComponent3")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	if !has1 || !has2 || !has3 {
		return nil, nil, nil
	}
	chunk := a.chunks[meta.chunkIndex]
	ptr1 := unsafe.Pointer(uintptr(chunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(chunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
	ptr3 := unsafe.Pointer(uintptr(chunk.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3)
}

// SetComponent3 adds or updates the 3 components (T1, T2, T3) on the
// specified entity.
//
// If the entity does not already have all the components, this operation will
// cause the entity to move to a different archetype. If the entity is invalid,
// this function does nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
//   - v1: The component data of type T1 to set.
//   - v2: The component data of type T2 to set.
//   - v3: The component data of type T3 to set.
func SetComponent3[T1 any, T2 any, T3 any](w *World, e Entity, v1 T1, v2 T2, v3 T3) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in SetComponent3")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	oldChunk := a.chunks[meta.chunkIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	if has1 && has2 && has3 {
		ptr1 := unsafe.Pointer(uintptr(oldChunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(oldChunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(oldChunk.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(id1)
	}
	if !has2 {
		newMask.set(id2)
	}
	if !has3 {
		newMask.set(id3)
	}
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		if !has1 {
			tempSpecs[count] = compSpec{id: id1, typ: w.components.compIDToType[id1], size: w.components.compIDToSize[id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: id2, typ: w.components.compIDToType[id2], size: w.components.compIDToSize[id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: id3, typ: w.components.compIDToType[id3], size: w.components.compIDToSize[id3]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(newChunk.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(newChunk.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(newChunk.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// RemoveComponent3 removes the 3 components (T1, T2, T3) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent3[T1 any, T2 any, T3 any](w *World, e Entity) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in RemoveComponent3")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	if !has1 && !has2 && !has3 {
		return
	}
	newMask := a.mask
	newMask.unset(id1)
	newMask.unset(id2)
	newMask.unset(id3)
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	oldChunk := a.chunks[meta.chunkIndex]
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 {
			continue
		}
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// GetComponent4 retrieves pointers to the 4 components of type
// (T1, T2, T3, T4) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4), or nils if not found.
func GetComponent4[T1 any, T2 any, T3 any, T4 any](w *World, e Entity) (*T1, *T2, *T3, *T4) {
	if !w.IsValid(e) {
		return nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 {
		panic("ecs: duplicate component types in GetComponent4")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	has4 := a.mask.containsBit(id4)
	if !has1 || !has2 || !has3 || !has4 {
		return nil, nil, nil, nil
	}
	chunk := a.chunks[meta.chunkIndex]
	ptr1 := unsafe.Pointer(uintptr(chunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(chunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
	ptr3 := unsafe.Pointer(uintptr(chunk.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
	ptr4 := unsafe.Pointer(uintptr(chunk.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4)
}

// SetComponent4 adds or updates the 4 components (T1, T2, T3, T4) on the
// specified entity.
//
// If the entity does not already have all the components, this operation will
// cause the entity to move to a different archetype. If the entity is invalid,
// this function does nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
//   - v1: The component data of type T1 to set.
//   - v2: The component data of type T2 to set.
//   - v3: The component data of type T3 to set.
//   - v4: The component data of type T4 to set.
func SetComponent4[T1 any, T2 any, T3 any, T4 any](w *World, e Entity, v1 T1, v2 T2, v3 T3, v4 T4) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 {
		panic("ecs: duplicate component types in SetComponent4")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	oldChunk := a.chunks[meta.chunkIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	has4 := a.mask.containsBit(id4)
	if has1 && has2 && has3 && has4 {
		ptr1 := unsafe.Pointer(uintptr(oldChunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(oldChunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(oldChunk.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(oldChunk.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
		*(*T4)(ptr4) = v4
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(id1)
	}
	if !has2 {
		newMask.set(id2)
	}
	if !has3 {
		newMask.set(id3)
	}
	if !has4 {
		newMask.set(id4)
	}
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		if !has1 {
			tempSpecs[count] = compSpec{id: id1, typ: w.components.compIDToType[id1], size: w.components.compIDToSize[id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: id2, typ: w.components.compIDToType[id2], size: w.components.compIDToSize[id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: id3, typ: w.components.compIDToType[id3], size: w.components.compIDToSize[id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: id4, typ: w.components.compIDToType[id4], size: w.components.compIDToSize[id4]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(newChunk.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(newChunk.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(newChunk.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(newChunk.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// RemoveComponent4 removes the 4 components (T1, T2, T3, T4) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent4[T1 any, T2 any, T3 any, T4 any](w *World, e Entity) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 {
		panic("ecs: duplicate component types in RemoveComponent4")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	has4 := a.mask.containsBit(id4)
	if !has1 && !has2 && !has3 && !has4 {
		return
	}
	newMask := a.mask
	newMask.unset(id1)
	newMask.unset(id2)
	newMask.unset(id3)
	newMask.unset(id4)
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	oldChunk := a.chunks[meta.chunkIndex]
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 {
			continue
		}
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// GetComponent5 retrieves pointers to the 5 components of type
// (T1, T2, T3, T4, T5) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5), or nils if not found.
func GetComponent5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, e Entity) (*T1, *T2, *T3, *T4, *T5) {
	if !w.IsValid(e) {
		return nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())
	id5 := w.getCompTypeID(reflect.TypeFor[T5]())

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 {
		panic("ecs: duplicate component types in GetComponent5")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	has4 := a.mask.containsBit(id4)
	has5 := a.mask.containsBit(id5)
	if !has1 || !has2 || !has3 || !has4 || !has5 {
		return nil, nil, nil, nil, nil
	}
	chunk := a.chunks[meta.chunkIndex]
	ptr1 := unsafe.Pointer(uintptr(chunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(chunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
	ptr3 := unsafe.Pointer(uintptr(chunk.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
	ptr4 := unsafe.Pointer(uintptr(chunk.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
	ptr5 := unsafe.Pointer(uintptr(chunk.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4), (*T5)(ptr5)
}

// SetComponent5 adds or updates the 5 components (T1, T2, T3, T4, T5) on the
// specified entity.
//
// If the entity does not already have all the components, this operation will
// cause the entity to move to a different archetype. If the entity is invalid,
// this function does nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
//   - v1: The component data of type T1 to set.
//   - v2: The component data of type T2 to set.
//   - v3: The component data of type T3 to set.
//   - v4: The component data of type T4 to set.
//   - v5: The component data of type T5 to set.
func SetComponent5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)
	id5 := w.getCompTypeID(t5)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 {
		panic("ecs: duplicate component types in SetComponent5")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	oldChunk := a.chunks[meta.chunkIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	has4 := a.mask.containsBit(id4)
	has5 := a.mask.containsBit(id5)
	if has1 && has2 && has3 && has4 && has5 {
		ptr1 := unsafe.Pointer(uintptr(oldChunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(oldChunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(oldChunk.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(oldChunk.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
		*(*T4)(ptr4) = v4
		ptr5 := unsafe.Pointer(uintptr(oldChunk.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
		*(*T5)(ptr5) = v5
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(id1)
	}
	if !has2 {
		newMask.set(id2)
	}
	if !has3 {
		newMask.set(id3)
	}
	if !has4 {
		newMask.set(id4)
	}
	if !has5 {
		newMask.set(id5)
	}
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		if !has1 {
			tempSpecs[count] = compSpec{id: id1, typ: w.components.compIDToType[id1], size: w.components.compIDToSize[id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: id2, typ: w.components.compIDToType[id2], size: w.components.compIDToSize[id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: id3, typ: w.components.compIDToType[id3], size: w.components.compIDToSize[id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: id4, typ: w.components.compIDToType[id4], size: w.components.compIDToSize[id4]}
			count++
		}
		if !has5 {
			tempSpecs[count] = compSpec{id: id5, typ: w.components.compIDToType[id5], size: w.components.compIDToSize[id5]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(newChunk.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(newChunk.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(newChunk.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(newChunk.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	ptr5 := unsafe.Pointer(uintptr(newChunk.compPointers[id5]) + uintptr(newIdx)*targetA.compSizes[id5])
	*(*T5)(ptr5) = v5
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// RemoveComponent5 removes the 5 components (T1, T2, T3, T4, T5) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, e Entity) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)
	id5 := w.getCompTypeID(t5)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 {
		panic("ecs: duplicate component types in RemoveComponent5")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	has4 := a.mask.containsBit(id4)
	has5 := a.mask.containsBit(id5)
	if !has1 && !has2 && !has3 && !has4 && !has5 {
		return
	}
	newMask := a.mask
	newMask.unset(id1)
	newMask.unset(id2)
	newMask.unset(id3)
	newMask.unset(id4)
	newMask.unset(id5)
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	oldChunk := a.chunks[meta.chunkIndex]
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 {
			continue
		}
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// GetComponent6 retrieves pointers to the 6 components of type
// (T1, T2, T3, T4, T5, T6) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6), or nils if not found.
func GetComponent6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any](w *World, e Entity) (*T1, *T2, *T3, *T4, *T5, *T6) {
	if !w.IsValid(e) {
		return nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	id1 := w.getCompTypeID(reflect.TypeFor[T1]())
	id2 := w.getCompTypeID(reflect.TypeFor[T2]())
	id3 := w.getCompTypeID(reflect.TypeFor[T3]())
	id4 := w.getCompTypeID(reflect.TypeFor[T4]())
	id5 := w.getCompTypeID(reflect.TypeFor[T5]())
	id6 := w.getCompTypeID(reflect.TypeFor[T6]())

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 {
		panic("ecs: duplicate component types in GetComponent6")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	has4 := a.mask.containsBit(id4)
	has5 := a.mask.containsBit(id5)
	has6 := a.mask.containsBit(id6)
	if !has1 || !has2 || !has3 || !has4 || !has5 || !has6 {
		return nil, nil, nil, nil, nil, nil
	}
	chunk := a.chunks[meta.chunkIndex]
	ptr1 := unsafe.Pointer(uintptr(chunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(chunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
	ptr3 := unsafe.Pointer(uintptr(chunk.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
	ptr4 := unsafe.Pointer(uintptr(chunk.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
	ptr5 := unsafe.Pointer(uintptr(chunk.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
	ptr6 := unsafe.Pointer(uintptr(chunk.compPointers[id6]) + uintptr(meta.index)*a.compSizes[id6])
	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4), (*T5)(ptr5), (*T6)(ptr6)
}

// SetComponent6 adds or updates the 6 components (T1, T2, T3, T4, T5, T6) on the
// specified entity.
//
// If the entity does not already have all the components, this operation will
// cause the entity to move to a different archetype. If the entity is invalid,
// this function does nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
//   - v1: The component data of type T1 to set.
//   - v2: The component data of type T2 to set.
//   - v3: The component data of type T3 to set.
//   - v4: The component data of type T4 to set.
//   - v5: The component data of type T5 to set.
//   - v6: The component data of type T6 to set.
func SetComponent6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any](w *World, e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)
	id5 := w.getCompTypeID(t5)
	id6 := w.getCompTypeID(t6)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 {
		panic("ecs: duplicate component types in SetComponent6")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	oldChunk := a.chunks[meta.chunkIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	has4 := a.mask.containsBit(id4)
	has5 := a.mask.containsBit(id5)
	has6 := a.mask.containsBit(id6)
	if has1 && has2 && has3 && has4 && has5 && has6 {
		ptr1 := unsafe.Pointer(uintptr(oldChunk.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(oldChunk.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(oldChunk.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(oldChunk.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
		*(*T4)(ptr4) = v4
		ptr5 := unsafe.Pointer(uintptr(oldChunk.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
		*(*T5)(ptr5) = v5
		ptr6 := unsafe.Pointer(uintptr(oldChunk.compPointers[id6]) + uintptr(meta.index)*a.compSizes[id6])
		*(*T6)(ptr6) = v6
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(id1)
	}
	if !has2 {
		newMask.set(id2)
	}
	if !has3 {
		newMask.set(id3)
	}
	if !has4 {
		newMask.set(id4)
	}
	if !has5 {
		newMask.set(id5)
	}
	if !has6 {
		newMask.set(id6)
	}
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		if !has1 {
			tempSpecs[count] = compSpec{id: id1, typ: w.components.compIDToType[id1], size: w.components.compIDToSize[id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: id2, typ: w.components.compIDToType[id2], size: w.components.compIDToSize[id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: id3, typ: w.components.compIDToType[id3], size: w.components.compIDToSize[id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: id4, typ: w.components.compIDToType[id4], size: w.components.compIDToSize[id4]}
			count++
		}
		if !has5 {
			tempSpecs[count] = compSpec{id: id5, typ: w.components.compIDToType[id5], size: w.components.compIDToSize[id5]}
			count++
		}
		if !has6 {
			tempSpecs[count] = compSpec{id: id6, typ: w.components.compIDToType[id6], size: w.components.compIDToSize[id6]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(newChunk.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(newChunk.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(newChunk.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(newChunk.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	ptr5 := unsafe.Pointer(uintptr(newChunk.compPointers[id5]) + uintptr(newIdx)*targetA.compSizes[id5])
	*(*T5)(ptr5) = v5
	ptr6 := unsafe.Pointer(uintptr(newChunk.compPointers[id6]) + uintptr(newIdx)*targetA.compSizes[id6])
	*(*T6)(ptr6) = v6
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// RemoveComponent6 removes the 6 components (T1, T2, T3, T4, T5, T6) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any](w *World, e Entity) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)
	id5 := w.getCompTypeID(t5)
	id6 := w.getCompTypeID(t6)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 {
		panic("ecs: duplicate component types in RemoveComponent6")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := a.mask.containsBit(id1)
	has2 := a.mask.containsBit(id2)
	has3 := a.mask.containsBit(id3)
	has4 := a.mask.containsBit(id4)
	has5 := a.mask.containsBit(id5)
	has6 := a.mask.containsBit(id6)
	if !has1 && !has2 && !has3 && !has4 && !has5 && !has6 {
		return
	}
	newMask := a.mask
	newMask.unset(id1)
	newMask.unset(id2)
	newMask.unset(id3)
	newMask.unset(id4)
	newMask.unset(id5)
	newMask.unset(id6)
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	if len(targetA.chunks) == 0 || targetA.chunks[len(targetA.chunks)-1].size == ChunkSize {
		targetA.chunks = append(targetA.chunks, w.newChunk(targetA))
	}
	newChunk := targetA.chunks[len(targetA.chunks)-1]
	newIdx := newChunk.size
	newChunk.entityIDs[newIdx] = e
	newChunk.size++
	targetA.size++
	oldChunk := a.chunks[meta.chunkIndex]
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 {
			continue
		}
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}
