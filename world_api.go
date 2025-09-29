package lazyecs

import (
	"math/bits"
	"reflect"
	"unsafe"
)

// GetComponent returns a pointer to the component of type T for the entity, or nil if not present or invalid.
func GetComponent[T any](w *World, e Entity) *T {
	if !w.IsValid(e) {
		return nil
	}
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	meta := w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	var m bitmask256
	m.set(id)
	if !a.mask.contains(m) {
		return nil
	}
	ptr := unsafe.Pointer(uintptr(a.compPointers[id]) + uintptr(meta.index)*a.compSizes[id])
	return (*T)(ptr)
}

// SetComponent sets the component of type T on the entity, adding it if not present.
func SetComponent[T any](w *World, e Entity, val T) {
	if !w.IsValid(e) {
		return
	}
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	meta := &w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	var singleMask bitmask256
	singleMask.set(id)
	if a.mask.contains(singleMask) {
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
		for wi := 0; wi < 4; wi++ {
			word := newMask[wi]
			for word != 0 {
				bit := bits.TrailingZeros64(word)
				cid := uint8(wi*64 + bit)
				typ := w.compIDToType[cid]
				tempSpecs[count] = compSpec{cid, typ, typ.Size()}
				count++
				word &= word - 1 // clear lowest set bit
			}
		}
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
	if !w.IsValid(e) {
		return
	}
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	meta := &w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	var singleMask bitmask256
	singleMask.set(id)
	if !a.mask.contains(singleMask) {
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
		for wi := 0; wi < 4; wi++ {
			word := newMask[wi]
			for word != 0 {
				bit := bits.TrailingZeros64(word)
				cid := uint8(wi*64 + bit)
				typ := w.compIDToType[cid]
				tempSpecs[count] = compSpec{cid, typ, typ.Size()}
				count++
				word &= word - 1 // clear lowest set bit
			}
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
