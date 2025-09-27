// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

import (
	"unsafe"
)

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
