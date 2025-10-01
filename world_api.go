package lazyecs

import (
	"reflect"
	"unsafe"
)

// GetComponent returns a pointer to the component of type T for the entity, or nil if not present or invalid.
func GetComponent[T any](w *World, e Entity) *T {
	if int(e.ID) >= len(w.metas) {
		return nil
	}
	meta := w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return nil
	}
	id := w.getCompTypeID(reflect.TypeFor[T]())
	a := w.archetypes[meta.archetypeIndex]
	i := id >> 6
	o := id & 63
	if (a.mask[i] & (uint64(1) << uint64(o))) == 0 {
		return nil
	}
	ptr := unsafe.Pointer(uintptr(a.compPointers[id]) + uintptr(meta.index)*a.compSizes[id])
	return (*T)(ptr)
}

// SetComponent sets the component of type T on the entity, adding it if not present.
func SetComponent[T any](w *World, e Entity, val T) {
	if int(e.ID) >= len(w.metas) {
		return
	}
	meta := &w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return
	}
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	a := w.archetypes[meta.archetypeIndex]
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
	if idx, ok := w.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes[idx]
	} else {
		// build specs only when creating new archetype
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			tempSpecs[count] = compSpec{
				id:   cid,
				typ:  w.compIDToType[cid],
				size: w.compIDToSize[cid],
			}
			count++
		}
		tempSpecs[count] = compSpec{
			id:   id,
			typ:  w.compIDToType[id],
			size: w.compIDToSize[id],
		}
		count++
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
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
	w.removeFromArchetype(a, meta)
	// update meta
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
}

// RemoveComponent removes the component of type T from the entity if present.
func RemoveComponent[T any](w *World, e Entity) {
	if int(e.ID) >= len(w.metas) {
		return
	}
	meta := &w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return
	}
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	a := w.archetypes[meta.archetypeIndex]
	i := id >> 6
	o := id & 63
	if (a.mask[i] & (uint64(1) << uint64(o))) == 0 {
		return
	}
	// remove
	newMask := a.mask
	newMask.unset(id)
	var targetA *archetype
	if idx, ok := w.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes[idx]
	} else {
		// build specs only when creating new archetype
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			if cid == id {
				continue
			}
			tempSpecs[count] = compSpec{
				id:   cid,
				typ:  w.compIDToType[cid],
				size: w.compIDToSize[cid],
			}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
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
	w.removeFromArchetype(a, meta)
	// update meta
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
}
