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
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(reflect.TypeFor[T1]())
	id2 := w.getCompTypeIDNoLock(reflect.TypeFor[T2]())
	
	w.components.mu.RUnlock()

	if id2 == id1 {
		panic("ecs: duplicate component types in GetComponent2")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 {
		return nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[id1], uintptr(meta.index)*a.compSizes[id1])),
		(*T2)(unsafe.Add(a.compPointers[id2], uintptr(meta.index)*a.compSizes[id2]))
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	
	w.components.mu.RUnlock()

	if id2 == id1 {
		panic("ecs: duplicate component types in SetComponent2")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	
	if has1 && has2 {
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
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
		w.components.mu.RLock()
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
		
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(targetA.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(targetA.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	
	w.components.mu.RUnlock()

	if id2 == id1 {
		panic("ecs: duplicate component types in RemoveComponent2")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	
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
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
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
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(reflect.TypeFor[T1]())
	id2 := w.getCompTypeIDNoLock(reflect.TypeFor[T2]())
	id3 := w.getCompTypeIDNoLock(reflect.TypeFor[T3]())
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in GetComponent3")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 {
		return nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[id1], uintptr(meta.index)*a.compSizes[id1])),
		(*T2)(unsafe.Add(a.compPointers[id2], uintptr(meta.index)*a.compSizes[id2])),
		(*T3)(unsafe.Add(a.compPointers[id3], uintptr(meta.index)*a.compSizes[id3]))
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in SetComponent3")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	
	if has1 && has2 && has3 {
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
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
		w.components.mu.RLock()
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
		
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(targetA.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(targetA.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(targetA.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in RemoveComponent3")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	
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
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
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
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(reflect.TypeFor[T1]())
	id2 := w.getCompTypeIDNoLock(reflect.TypeFor[T2]())
	id3 := w.getCompTypeIDNoLock(reflect.TypeFor[T3]())
	id4 := w.getCompTypeIDNoLock(reflect.TypeFor[T4]())
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 {
		panic("ecs: duplicate component types in GetComponent4")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 {
		return nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[id1], uintptr(meta.index)*a.compSizes[id1])),
		(*T2)(unsafe.Add(a.compPointers[id2], uintptr(meta.index)*a.compSizes[id2])),
		(*T3)(unsafe.Add(a.compPointers[id3], uintptr(meta.index)*a.compSizes[id3])),
		(*T4)(unsafe.Add(a.compPointers[id4], uintptr(meta.index)*a.compSizes[id4]))
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 {
		panic("ecs: duplicate component types in SetComponent4")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	
	if has1 && has2 && has3 && has4 {
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
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
		w.components.mu.RLock()
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
		
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(targetA.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(targetA.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(targetA.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(targetA.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 {
		panic("ecs: duplicate component types in RemoveComponent4")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	
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
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
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
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(reflect.TypeFor[T1]())
	id2 := w.getCompTypeIDNoLock(reflect.TypeFor[T2]())
	id3 := w.getCompTypeIDNoLock(reflect.TypeFor[T3]())
	id4 := w.getCompTypeIDNoLock(reflect.TypeFor[T4]())
	id5 := w.getCompTypeIDNoLock(reflect.TypeFor[T5]())
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 {
		panic("ecs: duplicate component types in GetComponent5")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 {
		return nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[id1], uintptr(meta.index)*a.compSizes[id1])),
		(*T2)(unsafe.Add(a.compPointers[id2], uintptr(meta.index)*a.compSizes[id2])),
		(*T3)(unsafe.Add(a.compPointers[id3], uintptr(meta.index)*a.compSizes[id3])),
		(*T4)(unsafe.Add(a.compPointers[id4], uintptr(meta.index)*a.compSizes[id4])),
		(*T5)(unsafe.Add(a.compPointers[id5], uintptr(meta.index)*a.compSizes[id5]))
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 {
		panic("ecs: duplicate component types in SetComponent5")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	
	if has1 && has2 && has3 && has4 && has5 {
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
		*(*T4)(ptr4) = v4
		ptr5 := unsafe.Pointer(uintptr(a.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
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
		w.components.mu.RLock()
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
		
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(targetA.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(targetA.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(targetA.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(targetA.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	ptr5 := unsafe.Pointer(uintptr(targetA.compPointers[id5]) + uintptr(newIdx)*targetA.compSizes[id5])
	*(*T5)(ptr5) = v5
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 {
		panic("ecs: duplicate component types in RemoveComponent5")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	
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
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
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
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(reflect.TypeFor[T1]())
	id2 := w.getCompTypeIDNoLock(reflect.TypeFor[T2]())
	id3 := w.getCompTypeIDNoLock(reflect.TypeFor[T3]())
	id4 := w.getCompTypeIDNoLock(reflect.TypeFor[T4]())
	id5 := w.getCompTypeIDNoLock(reflect.TypeFor[T5]())
	id6 := w.getCompTypeIDNoLock(reflect.TypeFor[T6]())
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 {
		panic("ecs: duplicate component types in GetComponent6")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 {
		return nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[id1], uintptr(meta.index)*a.compSizes[id1])),
		(*T2)(unsafe.Add(a.compPointers[id2], uintptr(meta.index)*a.compSizes[id2])),
		(*T3)(unsafe.Add(a.compPointers[id3], uintptr(meta.index)*a.compSizes[id3])),
		(*T4)(unsafe.Add(a.compPointers[id4], uintptr(meta.index)*a.compSizes[id4])),
		(*T5)(unsafe.Add(a.compPointers[id5], uintptr(meta.index)*a.compSizes[id5])),
		(*T6)(unsafe.Add(a.compPointers[id6], uintptr(meta.index)*a.compSizes[id6]))
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 {
		panic("ecs: duplicate component types in SetComponent6")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 {
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
		*(*T4)(ptr4) = v4
		ptr5 := unsafe.Pointer(uintptr(a.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
		*(*T5)(ptr5) = v5
		ptr6 := unsafe.Pointer(uintptr(a.compPointers[id6]) + uintptr(meta.index)*a.compSizes[id6])
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
		w.components.mu.RLock()
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
		
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(targetA.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(targetA.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(targetA.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(targetA.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	ptr5 := unsafe.Pointer(uintptr(targetA.compPointers[id5]) + uintptr(newIdx)*targetA.compSizes[id5])
	*(*T5)(ptr5) = v5
	ptr6 := unsafe.Pointer(uintptr(targetA.compPointers[id6]) + uintptr(newIdx)*targetA.compSizes[id6])
	*(*T6)(ptr6) = v6
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
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
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 {
		panic("ecs: duplicate component types in RemoveComponent6")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	
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
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// GetComponent7 retrieves pointers to the 7 components of type
// (T1, T2, T3, T4, T5, T6, T7) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6, *T7), or nils if not found.
func GetComponent7[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any](w *World, e Entity) (*T1, *T2, *T3, *T4, *T5, *T6, *T7) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(reflect.TypeFor[T1]())
	id2 := w.getCompTypeIDNoLock(reflect.TypeFor[T2]())
	id3 := w.getCompTypeIDNoLock(reflect.TypeFor[T3]())
	id4 := w.getCompTypeIDNoLock(reflect.TypeFor[T4]())
	id5 := w.getCompTypeIDNoLock(reflect.TypeFor[T5]())
	id6 := w.getCompTypeIDNoLock(reflect.TypeFor[T6]())
	id7 := w.getCompTypeIDNoLock(reflect.TypeFor[T7]())
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 {
		panic("ecs: duplicate component types in GetComponent7")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 || (a.mask[i7]&(uint64(1)<<uint64(o7))) == 0 {
		return nil, nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[id1], uintptr(meta.index)*a.compSizes[id1])),
		(*T2)(unsafe.Add(a.compPointers[id2], uintptr(meta.index)*a.compSizes[id2])),
		(*T3)(unsafe.Add(a.compPointers[id3], uintptr(meta.index)*a.compSizes[id3])),
		(*T4)(unsafe.Add(a.compPointers[id4], uintptr(meta.index)*a.compSizes[id4])),
		(*T5)(unsafe.Add(a.compPointers[id5], uintptr(meta.index)*a.compSizes[id5])),
		(*T6)(unsafe.Add(a.compPointers[id6], uintptr(meta.index)*a.compSizes[id6])),
		(*T7)(unsafe.Add(a.compPointers[id7], uintptr(meta.index)*a.compSizes[id7]))
}

// SetComponent7 adds or updates the 7 components (T1, T2, T3, T4, T5, T6, T7) on the
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
//   - v7: The component data of type T7 to set.
func SetComponent7[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any](w *World, e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	t7 := reflect.TypeFor[T7]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	id7 := w.getCompTypeIDNoLock(t7)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 {
		panic("ecs: duplicate component types in SetComponent7")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	has7 := (a.mask[i7] & (uint64(1) << uint64(o7))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 && has7 {
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
		*(*T4)(ptr4) = v4
		ptr5 := unsafe.Pointer(uintptr(a.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
		*(*T5)(ptr5) = v5
		ptr6 := unsafe.Pointer(uintptr(a.compPointers[id6]) + uintptr(meta.index)*a.compSizes[id6])
		*(*T6)(ptr6) = v6
		ptr7 := unsafe.Pointer(uintptr(a.compPointers[id7]) + uintptr(meta.index)*a.compSizes[id7])
		*(*T7)(ptr7) = v7
		
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
	if !has7 {
		newMask.set(id7)
	}
	
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
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
		if !has7 {
			tempSpecs[count] = compSpec{id: id7, typ: w.components.compIDToType[id7], size: w.components.compIDToSize[id7]}
			count++
		}
		
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(targetA.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(targetA.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(targetA.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(targetA.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	ptr5 := unsafe.Pointer(uintptr(targetA.compPointers[id5]) + uintptr(newIdx)*targetA.compSizes[id5])
	*(*T5)(ptr5) = v5
	ptr6 := unsafe.Pointer(uintptr(targetA.compPointers[id6]) + uintptr(newIdx)*targetA.compSizes[id6])
	*(*T6)(ptr6) = v6
	ptr7 := unsafe.Pointer(uintptr(targetA.compPointers[id7]) + uintptr(newIdx)*targetA.compSizes[id7])
	*(*T7)(ptr7) = v7
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// RemoveComponent7 removes the 7 components (T1, T2, T3, T4, T5, T6, T7) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent7[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any](w *World, e Entity) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	t7 := reflect.TypeFor[T7]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	id7 := w.getCompTypeIDNoLock(t7)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 {
		panic("ecs: duplicate component types in RemoveComponent7")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	has7 := (a.mask[i7] & (uint64(1) << uint64(o7))) != 0
	
	if !has1 && !has2 && !has3 && !has4 && !has5 && !has6 && !has7 {
		return
	}
	newMask := a.mask
	newMask.unset(id1)
	newMask.unset(id2)
	newMask.unset(id3)
	newMask.unset(id4)
	newMask.unset(id5)
	newMask.unset(id6)
	newMask.unset(id7)
	
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 || cid == id7 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 || cid == id7 {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// GetComponent8 retrieves pointers to the 8 components of type
// (T1, T2, T3, T4, T5, T6, T7, T8) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8), or nils if not found.
func GetComponent8[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any](w *World, e Entity) (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(reflect.TypeFor[T1]())
	id2 := w.getCompTypeIDNoLock(reflect.TypeFor[T2]())
	id3 := w.getCompTypeIDNoLock(reflect.TypeFor[T3]())
	id4 := w.getCompTypeIDNoLock(reflect.TypeFor[T4]())
	id5 := w.getCompTypeIDNoLock(reflect.TypeFor[T5]())
	id6 := w.getCompTypeIDNoLock(reflect.TypeFor[T6]())
	id7 := w.getCompTypeIDNoLock(reflect.TypeFor[T7]())
	id8 := w.getCompTypeIDNoLock(reflect.TypeFor[T8]())
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 {
		panic("ecs: duplicate component types in GetComponent8")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	i8 := id8 >> 6
	o8 := id8 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 || (a.mask[i7]&(uint64(1)<<uint64(o7))) == 0 || (a.mask[i8]&(uint64(1)<<uint64(o8))) == 0 {
		return nil, nil, nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[id1], uintptr(meta.index)*a.compSizes[id1])),
		(*T2)(unsafe.Add(a.compPointers[id2], uintptr(meta.index)*a.compSizes[id2])),
		(*T3)(unsafe.Add(a.compPointers[id3], uintptr(meta.index)*a.compSizes[id3])),
		(*T4)(unsafe.Add(a.compPointers[id4], uintptr(meta.index)*a.compSizes[id4])),
		(*T5)(unsafe.Add(a.compPointers[id5], uintptr(meta.index)*a.compSizes[id5])),
		(*T6)(unsafe.Add(a.compPointers[id6], uintptr(meta.index)*a.compSizes[id6])),
		(*T7)(unsafe.Add(a.compPointers[id7], uintptr(meta.index)*a.compSizes[id7])),
		(*T8)(unsafe.Add(a.compPointers[id8], uintptr(meta.index)*a.compSizes[id8]))
}

// SetComponent8 adds or updates the 8 components (T1, T2, T3, T4, T5, T6, T7, T8) on the
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
//   - v7: The component data of type T7 to set.
//   - v8: The component data of type T8 to set.
func SetComponent8[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any](w *World, e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	t7 := reflect.TypeFor[T7]()
	t8 := reflect.TypeFor[T8]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	id7 := w.getCompTypeIDNoLock(t7)
	id8 := w.getCompTypeIDNoLock(t8)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 {
		panic("ecs: duplicate component types in SetComponent8")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	i8 := id8 >> 6
	o8 := id8 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	has7 := (a.mask[i7] & (uint64(1) << uint64(o7))) != 0
	has8 := (a.mask[i8] & (uint64(1) << uint64(o8))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 && has7 && has8 {
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
		*(*T4)(ptr4) = v4
		ptr5 := unsafe.Pointer(uintptr(a.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
		*(*T5)(ptr5) = v5
		ptr6 := unsafe.Pointer(uintptr(a.compPointers[id6]) + uintptr(meta.index)*a.compSizes[id6])
		*(*T6)(ptr6) = v6
		ptr7 := unsafe.Pointer(uintptr(a.compPointers[id7]) + uintptr(meta.index)*a.compSizes[id7])
		*(*T7)(ptr7) = v7
		ptr8 := unsafe.Pointer(uintptr(a.compPointers[id8]) + uintptr(meta.index)*a.compSizes[id8])
		*(*T8)(ptr8) = v8
		
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
	if !has7 {
		newMask.set(id7)
	}
	if !has8 {
		newMask.set(id8)
	}
	
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
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
		if !has7 {
			tempSpecs[count] = compSpec{id: id7, typ: w.components.compIDToType[id7], size: w.components.compIDToSize[id7]}
			count++
		}
		if !has8 {
			tempSpecs[count] = compSpec{id: id8, typ: w.components.compIDToType[id8], size: w.components.compIDToSize[id8]}
			count++
		}
		
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(targetA.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(targetA.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(targetA.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(targetA.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	ptr5 := unsafe.Pointer(uintptr(targetA.compPointers[id5]) + uintptr(newIdx)*targetA.compSizes[id5])
	*(*T5)(ptr5) = v5
	ptr6 := unsafe.Pointer(uintptr(targetA.compPointers[id6]) + uintptr(newIdx)*targetA.compSizes[id6])
	*(*T6)(ptr6) = v6
	ptr7 := unsafe.Pointer(uintptr(targetA.compPointers[id7]) + uintptr(newIdx)*targetA.compSizes[id7])
	*(*T7)(ptr7) = v7
	ptr8 := unsafe.Pointer(uintptr(targetA.compPointers[id8]) + uintptr(newIdx)*targetA.compSizes[id8])
	*(*T8)(ptr8) = v8
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// RemoveComponent8 removes the 8 components (T1, T2, T3, T4, T5, T6, T7, T8) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent8[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any](w *World, e Entity) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	t7 := reflect.TypeFor[T7]()
	t8 := reflect.TypeFor[T8]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	id7 := w.getCompTypeIDNoLock(t7)
	id8 := w.getCompTypeIDNoLock(t8)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 {
		panic("ecs: duplicate component types in RemoveComponent8")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	i8 := id8 >> 6
	o8 := id8 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	has7 := (a.mask[i7] & (uint64(1) << uint64(o7))) != 0
	has8 := (a.mask[i8] & (uint64(1) << uint64(o8))) != 0
	
	if !has1 && !has2 && !has3 && !has4 && !has5 && !has6 && !has7 && !has8 {
		return
	}
	newMask := a.mask
	newMask.unset(id1)
	newMask.unset(id2)
	newMask.unset(id3)
	newMask.unset(id4)
	newMask.unset(id5)
	newMask.unset(id6)
	newMask.unset(id7)
	newMask.unset(id8)
	
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 || cid == id7 || cid == id8 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 || cid == id7 || cid == id8 {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// GetComponent9 retrieves pointers to the 9 components of type
// (T1, T2, T3, T4, T5, T6, T7, T8, T9) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9), or nils if not found.
func GetComponent9[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any](w *World, e Entity) (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(reflect.TypeFor[T1]())
	id2 := w.getCompTypeIDNoLock(reflect.TypeFor[T2]())
	id3 := w.getCompTypeIDNoLock(reflect.TypeFor[T3]())
	id4 := w.getCompTypeIDNoLock(reflect.TypeFor[T4]())
	id5 := w.getCompTypeIDNoLock(reflect.TypeFor[T5]())
	id6 := w.getCompTypeIDNoLock(reflect.TypeFor[T6]())
	id7 := w.getCompTypeIDNoLock(reflect.TypeFor[T7]())
	id8 := w.getCompTypeIDNoLock(reflect.TypeFor[T8]())
	id9 := w.getCompTypeIDNoLock(reflect.TypeFor[T9]())
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 || id9 == id1 || id9 == id2 || id9 == id3 || id9 == id4 || id9 == id5 || id9 == id6 || id9 == id7 || id9 == id8 {
		panic("ecs: duplicate component types in GetComponent9")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	i8 := id8 >> 6
	o8 := id8 & 63
	i9 := id9 >> 6
	o9 := id9 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 || (a.mask[i7]&(uint64(1)<<uint64(o7))) == 0 || (a.mask[i8]&(uint64(1)<<uint64(o8))) == 0 || (a.mask[i9]&(uint64(1)<<uint64(o9))) == 0 {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[id1], uintptr(meta.index)*a.compSizes[id1])),
		(*T2)(unsafe.Add(a.compPointers[id2], uintptr(meta.index)*a.compSizes[id2])),
		(*T3)(unsafe.Add(a.compPointers[id3], uintptr(meta.index)*a.compSizes[id3])),
		(*T4)(unsafe.Add(a.compPointers[id4], uintptr(meta.index)*a.compSizes[id4])),
		(*T5)(unsafe.Add(a.compPointers[id5], uintptr(meta.index)*a.compSizes[id5])),
		(*T6)(unsafe.Add(a.compPointers[id6], uintptr(meta.index)*a.compSizes[id6])),
		(*T7)(unsafe.Add(a.compPointers[id7], uintptr(meta.index)*a.compSizes[id7])),
		(*T8)(unsafe.Add(a.compPointers[id8], uintptr(meta.index)*a.compSizes[id8])),
		(*T9)(unsafe.Add(a.compPointers[id9], uintptr(meta.index)*a.compSizes[id9]))
}

// SetComponent9 adds or updates the 9 components (T1, T2, T3, T4, T5, T6, T7, T8, T9) on the
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
//   - v7: The component data of type T7 to set.
//   - v8: The component data of type T8 to set.
//   - v9: The component data of type T9 to set.
func SetComponent9[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any](w *World, e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8, v9 T9) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	t7 := reflect.TypeFor[T7]()
	t8 := reflect.TypeFor[T8]()
	t9 := reflect.TypeFor[T9]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	id7 := w.getCompTypeIDNoLock(t7)
	id8 := w.getCompTypeIDNoLock(t8)
	id9 := w.getCompTypeIDNoLock(t9)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 || id9 == id1 || id9 == id2 || id9 == id3 || id9 == id4 || id9 == id5 || id9 == id6 || id9 == id7 || id9 == id8 {
		panic("ecs: duplicate component types in SetComponent9")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	i8 := id8 >> 6
	o8 := id8 & 63
	i9 := id9 >> 6
	o9 := id9 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	has7 := (a.mask[i7] & (uint64(1) << uint64(o7))) != 0
	has8 := (a.mask[i8] & (uint64(1) << uint64(o8))) != 0
	has9 := (a.mask[i9] & (uint64(1) << uint64(o9))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 && has7 && has8 && has9 {
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
		*(*T4)(ptr4) = v4
		ptr5 := unsafe.Pointer(uintptr(a.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
		*(*T5)(ptr5) = v5
		ptr6 := unsafe.Pointer(uintptr(a.compPointers[id6]) + uintptr(meta.index)*a.compSizes[id6])
		*(*T6)(ptr6) = v6
		ptr7 := unsafe.Pointer(uintptr(a.compPointers[id7]) + uintptr(meta.index)*a.compSizes[id7])
		*(*T7)(ptr7) = v7
		ptr8 := unsafe.Pointer(uintptr(a.compPointers[id8]) + uintptr(meta.index)*a.compSizes[id8])
		*(*T8)(ptr8) = v8
		ptr9 := unsafe.Pointer(uintptr(a.compPointers[id9]) + uintptr(meta.index)*a.compSizes[id9])
		*(*T9)(ptr9) = v9
		
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
	if !has7 {
		newMask.set(id7)
	}
	if !has8 {
		newMask.set(id8)
	}
	if !has9 {
		newMask.set(id9)
	}
	
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
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
		if !has7 {
			tempSpecs[count] = compSpec{id: id7, typ: w.components.compIDToType[id7], size: w.components.compIDToSize[id7]}
			count++
		}
		if !has8 {
			tempSpecs[count] = compSpec{id: id8, typ: w.components.compIDToType[id8], size: w.components.compIDToSize[id8]}
			count++
		}
		if !has9 {
			tempSpecs[count] = compSpec{id: id9, typ: w.components.compIDToType[id9], size: w.components.compIDToSize[id9]}
			count++
		}
		
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(targetA.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(targetA.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(targetA.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(targetA.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	ptr5 := unsafe.Pointer(uintptr(targetA.compPointers[id5]) + uintptr(newIdx)*targetA.compSizes[id5])
	*(*T5)(ptr5) = v5
	ptr6 := unsafe.Pointer(uintptr(targetA.compPointers[id6]) + uintptr(newIdx)*targetA.compSizes[id6])
	*(*T6)(ptr6) = v6
	ptr7 := unsafe.Pointer(uintptr(targetA.compPointers[id7]) + uintptr(newIdx)*targetA.compSizes[id7])
	*(*T7)(ptr7) = v7
	ptr8 := unsafe.Pointer(uintptr(targetA.compPointers[id8]) + uintptr(newIdx)*targetA.compSizes[id8])
	*(*T8)(ptr8) = v8
	ptr9 := unsafe.Pointer(uintptr(targetA.compPointers[id9]) + uintptr(newIdx)*targetA.compSizes[id9])
	*(*T9)(ptr9) = v9
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// RemoveComponent9 removes the 9 components (T1, T2, T3, T4, T5, T6, T7, T8, T9) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent9[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any](w *World, e Entity) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	t7 := reflect.TypeFor[T7]()
	t8 := reflect.TypeFor[T8]()
	t9 := reflect.TypeFor[T9]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	id7 := w.getCompTypeIDNoLock(t7)
	id8 := w.getCompTypeIDNoLock(t8)
	id9 := w.getCompTypeIDNoLock(t9)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 || id9 == id1 || id9 == id2 || id9 == id3 || id9 == id4 || id9 == id5 || id9 == id6 || id9 == id7 || id9 == id8 {
		panic("ecs: duplicate component types in RemoveComponent9")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	i8 := id8 >> 6
	o8 := id8 & 63
	i9 := id9 >> 6
	o9 := id9 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	has7 := (a.mask[i7] & (uint64(1) << uint64(o7))) != 0
	has8 := (a.mask[i8] & (uint64(1) << uint64(o8))) != 0
	has9 := (a.mask[i9] & (uint64(1) << uint64(o9))) != 0
	
	if !has1 && !has2 && !has3 && !has4 && !has5 && !has6 && !has7 && !has8 && !has9 {
		return
	}
	newMask := a.mask
	newMask.unset(id1)
	newMask.unset(id2)
	newMask.unset(id3)
	newMask.unset(id4)
	newMask.unset(id5)
	newMask.unset(id6)
	newMask.unset(id7)
	newMask.unset(id8)
	newMask.unset(id9)
	
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 || cid == id7 || cid == id8 || cid == id9 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 || cid == id7 || cid == id8 || cid == id9 {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// GetComponent10 retrieves pointers to the 10 components of type
// (T1, T2, T3, T4, T5, T6, T7, T8, T9, T10) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9, *T10), or nils if not found.
func GetComponent10[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any, T10 any](w *World, e Entity) (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9, *T10) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(reflect.TypeFor[T1]())
	id2 := w.getCompTypeIDNoLock(reflect.TypeFor[T2]())
	id3 := w.getCompTypeIDNoLock(reflect.TypeFor[T3]())
	id4 := w.getCompTypeIDNoLock(reflect.TypeFor[T4]())
	id5 := w.getCompTypeIDNoLock(reflect.TypeFor[T5]())
	id6 := w.getCompTypeIDNoLock(reflect.TypeFor[T6]())
	id7 := w.getCompTypeIDNoLock(reflect.TypeFor[T7]())
	id8 := w.getCompTypeIDNoLock(reflect.TypeFor[T8]())
	id9 := w.getCompTypeIDNoLock(reflect.TypeFor[T9]())
	id10 := w.getCompTypeIDNoLock(reflect.TypeFor[T10]())
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 || id9 == id1 || id9 == id2 || id9 == id3 || id9 == id4 || id9 == id5 || id9 == id6 || id9 == id7 || id9 == id8 || id10 == id1 || id10 == id2 || id10 == id3 || id10 == id4 || id10 == id5 || id10 == id6 || id10 == id7 || id10 == id8 || id10 == id9 {
		panic("ecs: duplicate component types in GetComponent10")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	i8 := id8 >> 6
	o8 := id8 & 63
	i9 := id9 >> 6
	o9 := id9 & 63
	i10 := id10 >> 6
	o10 := id10 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 || (a.mask[i7]&(uint64(1)<<uint64(o7))) == 0 || (a.mask[i8]&(uint64(1)<<uint64(o8))) == 0 || (a.mask[i9]&(uint64(1)<<uint64(o9))) == 0 || (a.mask[i10]&(uint64(1)<<uint64(o10))) == 0 {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[id1], uintptr(meta.index)*a.compSizes[id1])),
		(*T2)(unsafe.Add(a.compPointers[id2], uintptr(meta.index)*a.compSizes[id2])),
		(*T3)(unsafe.Add(a.compPointers[id3], uintptr(meta.index)*a.compSizes[id3])),
		(*T4)(unsafe.Add(a.compPointers[id4], uintptr(meta.index)*a.compSizes[id4])),
		(*T5)(unsafe.Add(a.compPointers[id5], uintptr(meta.index)*a.compSizes[id5])),
		(*T6)(unsafe.Add(a.compPointers[id6], uintptr(meta.index)*a.compSizes[id6])),
		(*T7)(unsafe.Add(a.compPointers[id7], uintptr(meta.index)*a.compSizes[id7])),
		(*T8)(unsafe.Add(a.compPointers[id8], uintptr(meta.index)*a.compSizes[id8])),
		(*T9)(unsafe.Add(a.compPointers[id9], uintptr(meta.index)*a.compSizes[id9])),
		(*T10)(unsafe.Add(a.compPointers[id10], uintptr(meta.index)*a.compSizes[id10]))
}

// SetComponent10 adds or updates the 10 components (T1, T2, T3, T4, T5, T6, T7, T8, T9, T10) on the
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
//   - v7: The component data of type T7 to set.
//   - v8: The component data of type T8 to set.
//   - v9: The component data of type T9 to set.
//   - v10: The component data of type T10 to set.
func SetComponent10[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any, T10 any](w *World, e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8, v9 T9, v10 T10) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	t7 := reflect.TypeFor[T7]()
	t8 := reflect.TypeFor[T8]()
	t9 := reflect.TypeFor[T9]()
	t10 := reflect.TypeFor[T10]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	id7 := w.getCompTypeIDNoLock(t7)
	id8 := w.getCompTypeIDNoLock(t8)
	id9 := w.getCompTypeIDNoLock(t9)
	id10 := w.getCompTypeIDNoLock(t10)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 || id9 == id1 || id9 == id2 || id9 == id3 || id9 == id4 || id9 == id5 || id9 == id6 || id9 == id7 || id9 == id8 || id10 == id1 || id10 == id2 || id10 == id3 || id10 == id4 || id10 == id5 || id10 == id6 || id10 == id7 || id10 == id8 || id10 == id9 {
		panic("ecs: duplicate component types in SetComponent10")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	i8 := id8 >> 6
	o8 := id8 & 63
	i9 := id9 >> 6
	o9 := id9 & 63
	i10 := id10 >> 6
	o10 := id10 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	has7 := (a.mask[i7] & (uint64(1) << uint64(o7))) != 0
	has8 := (a.mask[i8] & (uint64(1) << uint64(o8))) != 0
	has9 := (a.mask[i9] & (uint64(1) << uint64(o9))) != 0
	has10 := (a.mask[i10] & (uint64(1) << uint64(o10))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 && has7 && has8 && has9 && has10 {
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
		*(*T1)(ptr1) = v1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
		*(*T2)(ptr2) = v2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
		*(*T3)(ptr3) = v3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
		*(*T4)(ptr4) = v4
		ptr5 := unsafe.Pointer(uintptr(a.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
		*(*T5)(ptr5) = v5
		ptr6 := unsafe.Pointer(uintptr(a.compPointers[id6]) + uintptr(meta.index)*a.compSizes[id6])
		*(*T6)(ptr6) = v6
		ptr7 := unsafe.Pointer(uintptr(a.compPointers[id7]) + uintptr(meta.index)*a.compSizes[id7])
		*(*T7)(ptr7) = v7
		ptr8 := unsafe.Pointer(uintptr(a.compPointers[id8]) + uintptr(meta.index)*a.compSizes[id8])
		*(*T8)(ptr8) = v8
		ptr9 := unsafe.Pointer(uintptr(a.compPointers[id9]) + uintptr(meta.index)*a.compSizes[id9])
		*(*T9)(ptr9) = v9
		ptr10 := unsafe.Pointer(uintptr(a.compPointers[id10]) + uintptr(meta.index)*a.compSizes[id10])
		*(*T10)(ptr10) = v10
		
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
	if !has7 {
		newMask.set(id7)
	}
	if !has8 {
		newMask.set(id8)
	}
	if !has9 {
		newMask.set(id9)
	}
	if !has10 {
		newMask.set(id10)
	}
	
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
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
		if !has7 {
			tempSpecs[count] = compSpec{id: id7, typ: w.components.compIDToType[id7], size: w.components.compIDToSize[id7]}
			count++
		}
		if !has8 {
			tempSpecs[count] = compSpec{id: id8, typ: w.components.compIDToType[id8], size: w.components.compIDToSize[id8]}
			count++
		}
		if !has9 {
			tempSpecs[count] = compSpec{id: id9, typ: w.components.compIDToType[id9], size: w.components.compIDToSize[id9]}
			count++
		}
		if !has10 {
			tempSpecs[count] = compSpec{id: id10, typ: w.components.compIDToType[id10], size: w.components.compIDToSize[id10]}
			count++
		}
		
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	ptr1 := unsafe.Pointer(uintptr(targetA.compPointers[id1]) + uintptr(newIdx)*targetA.compSizes[id1])
	*(*T1)(ptr1) = v1
	ptr2 := unsafe.Pointer(uintptr(targetA.compPointers[id2]) + uintptr(newIdx)*targetA.compSizes[id2])
	*(*T2)(ptr2) = v2
	ptr3 := unsafe.Pointer(uintptr(targetA.compPointers[id3]) + uintptr(newIdx)*targetA.compSizes[id3])
	*(*T3)(ptr3) = v3
	ptr4 := unsafe.Pointer(uintptr(targetA.compPointers[id4]) + uintptr(newIdx)*targetA.compSizes[id4])
	*(*T4)(ptr4) = v4
	ptr5 := unsafe.Pointer(uintptr(targetA.compPointers[id5]) + uintptr(newIdx)*targetA.compSizes[id5])
	*(*T5)(ptr5) = v5
	ptr6 := unsafe.Pointer(uintptr(targetA.compPointers[id6]) + uintptr(newIdx)*targetA.compSizes[id6])
	*(*T6)(ptr6) = v6
	ptr7 := unsafe.Pointer(uintptr(targetA.compPointers[id7]) + uintptr(newIdx)*targetA.compSizes[id7])
	*(*T7)(ptr7) = v7
	ptr8 := unsafe.Pointer(uintptr(targetA.compPointers[id8]) + uintptr(newIdx)*targetA.compSizes[id8])
	*(*T8)(ptr8) = v8
	ptr9 := unsafe.Pointer(uintptr(targetA.compPointers[id9]) + uintptr(newIdx)*targetA.compSizes[id9])
	*(*T9)(ptr9) = v9
	ptr10 := unsafe.Pointer(uintptr(targetA.compPointers[id10]) + uintptr(newIdx)*targetA.compSizes[id10])
	*(*T10)(ptr10) = v10
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// RemoveComponent10 removes the 10 components (T1, T2, T3, T4, T5, T6, T7, T8, T9, T10) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent10[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any, T10 any](w *World, e Entity) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	t4 := reflect.TypeFor[T4]()
	t5 := reflect.TypeFor[T5]()
	t6 := reflect.TypeFor[T6]()
	t7 := reflect.TypeFor[T7]()
	t8 := reflect.TypeFor[T8]()
	t9 := reflect.TypeFor[T9]()
	t10 := reflect.TypeFor[T10]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	id4 := w.getCompTypeIDNoLock(t4)
	id5 := w.getCompTypeIDNoLock(t5)
	id6 := w.getCompTypeIDNoLock(t6)
	id7 := w.getCompTypeIDNoLock(t7)
	id8 := w.getCompTypeIDNoLock(t8)
	id9 := w.getCompTypeIDNoLock(t9)
	id10 := w.getCompTypeIDNoLock(t10)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 || id6 == id1 || id6 == id2 || id6 == id3 || id6 == id4 || id6 == id5 || id7 == id1 || id7 == id2 || id7 == id3 || id7 == id4 || id7 == id5 || id7 == id6 || id8 == id1 || id8 == id2 || id8 == id3 || id8 == id4 || id8 == id5 || id8 == id6 || id8 == id7 || id9 == id1 || id9 == id2 || id9 == id3 || id9 == id4 || id9 == id5 || id9 == id6 || id9 == id7 || id9 == id8 || id10 == id1 || id10 == id2 || id10 == id3 || id10 == id4 || id10 == id5 || id10 == id6 || id10 == id7 || id10 == id8 || id10 == id9 {
		panic("ecs: duplicate component types in RemoveComponent10")
	}
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := id1 >> 6
	o1 := id1 & 63
	i2 := id2 >> 6
	o2 := id2 & 63
	i3 := id3 >> 6
	o3 := id3 & 63
	i4 := id4 >> 6
	o4 := id4 & 63
	i5 := id5 >> 6
	o5 := id5 & 63
	i6 := id6 >> 6
	o6 := id6 & 63
	i7 := id7 >> 6
	o7 := id7 & 63
	i8 := id8 >> 6
	o8 := id8 & 63
	i9 := id9 >> 6
	o9 := id9 & 63
	i10 := id10 >> 6
	o10 := id10 & 63
	
	has1 := (a.mask[i1] & (uint64(1) << uint64(o1))) != 0
	has2 := (a.mask[i2] & (uint64(1) << uint64(o2))) != 0
	has3 := (a.mask[i3] & (uint64(1) << uint64(o3))) != 0
	has4 := (a.mask[i4] & (uint64(1) << uint64(o4))) != 0
	has5 := (a.mask[i5] & (uint64(1) << uint64(o5))) != 0
	has6 := (a.mask[i6] & (uint64(1) << uint64(o6))) != 0
	has7 := (a.mask[i7] & (uint64(1) << uint64(o7))) != 0
	has8 := (a.mask[i8] & (uint64(1) << uint64(o8))) != 0
	has9 := (a.mask[i9] & (uint64(1) << uint64(o9))) != 0
	has10 := (a.mask[i10] & (uint64(1) << uint64(o10))) != 0
	
	if !has1 && !has2 && !has3 && !has4 && !has5 && !has6 && !has7 && !has8 && !has9 && !has10 {
		return
	}
	newMask := a.mask
	newMask.unset(id1)
	newMask.unset(id2)
	newMask.unset(id3)
	newMask.unset(id4)
	newMask.unset(id5)
	newMask.unset(id6)
	newMask.unset(id7)
	newMask.unset(id8)
	newMask.unset(id9)
	newMask.unset(id10)
	
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 || cid == id7 || cid == id8 || cid == id9 || cid == id10 {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.components.compIDToType[cid], size: w.components.compIDToSize[cid]}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if cid == id1 || cid == id2 || cid == id3 || cid == id4 || cid == id5 || cid == id6 || cid == id7 || cid == id8 || cid == id9 || cid == id10 {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

