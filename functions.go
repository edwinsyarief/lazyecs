package teishoku

import (
	"reflect"
	"unsafe"
)

// GetComponent retrieves a pointer to the component of type `T` for the given
// entity. It provides a direct, type-safe way to access component data.
//
// If the entity is invalid, does not have the component, or if the entity ID is
// out of bounds, this function returns nil.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the component.
//
// Returns:
//   - A pointer to the component data (*T), or nil if not found.
func GetComponent[T any](w *World, e Entity) *T {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil
	}
	meta := w.entities.metas[e.ID]
	w.components.mu.RLock()
	id := w.getCompTypeIDNoLock(reflect.TypeFor[T]())
	w.components.mu.RUnlock()
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i := id >> 6
	o := id & 63
	if (a.mask[i] & (uint64(1) << uint64(o))) == 0 {
		return nil
	}
	return (*T)(unsafe.Add(a.compPointers[id], uintptr(meta.index)*a.compSizes[id]))
}

// SetComponent adds a component of type `T` with the given value to an entity,
// or updates it if the component already exists.
//
// If the entity does not already have the component, adding it will cause the
// entity to move to a different archetype. This is a relatively expensive
// operation compared to updating an existing component. If the entity is
// invalid, this function does nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
//   - val: The component data of type `T` to set.
func SetComponent[T any](w *World, e Entity, val T) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t := reflect.TypeFor[T]()
	w.components.mu.RLock()
	id := w.getCompTypeIDNoLock(t)
	w.components.mu.RUnlock()
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i := id >> 6
	o := id & 63
	if (a.mask[i] & (uint64(1) << uint64(o))) != 0 {
		// already has, just set
		ptr := unsafe.Pointer(uintptr(a.compPointers[id]) + uintptr(meta.index)*a.compSizes[id])
		*(*T)(ptr) = val
		return
	}
	// add new
	newMask := a.mask
	newMask.set(id)
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		// build specs only when creating new archetype
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			tempSpecs[count] = compSpec{
				id:   cid,
				typ:  w.components.compIDToType[cid],
				size: w.components.compIDToSize[cid],
			}
			count++
		}
		tempSpecs[count] = compSpec{
			id:   id,
			typ:  w.components.compIDToType[id],
			size: w.components.compIDToSize[id],
		}
		count++
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	// move to target
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	// copy existing components
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	// set new component
	dst := unsafe.Pointer(uintptr(targetA.compPointers[id]) + uintptr(newIdx)*targetA.compSizes[id])
	*(*T)(dst) = val
	// remove from old
	w.removeFromArchetypeNoLock(a, meta)
	// update meta
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// RemoveComponent removes the component of type `T` from the specified entity.
//
// This operation will cause the entity to move to a new archetype that does not
// include the removed component. This can be an expensive operation. If the
// entity is invalid or does not have the component, this function does nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent[T any](w *World, e Entity) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	t := reflect.TypeFor[T]()
	w.components.mu.RLock()
	id := w.getCompTypeIDNoLock(t)
	w.components.mu.RUnlock()
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i := id >> 6
	o := id & 63
	if (a.mask[i] & (uint64(1) << uint64(o))) == 0 {
		return
	}
	// remove
	newMask := a.mask
	newMask.unset(id)
	var targetA *archetype
	if idx, ok := w.archetypes.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes.archetypes[idx]
	} else {
		// build specs only when creating new archetype
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		w.components.mu.RLock()
		for _, cid := range a.compOrder {
			if cid == id {
				continue
			}
			tempSpecs[count] = compSpec{
				id:   cid,
				typ:  w.components.compIDToType[cid],
				size: w.components.compIDToSize[cid],
			}
			count++
		}
		w.components.mu.RUnlock()
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetypeNoLock(newMask, specs)
	}
	// move to target
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	// copy existing components except removed
	for _, cid := range a.compOrder {
		if cid == id {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	// remove from old
	w.removeFromArchetypeNoLock(a, meta)
	// update meta
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}
