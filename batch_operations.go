// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

import (
	"sort"
	"unsafe"
)

// AddComponentBatch adds a component to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch[T any](w *World, entities []Entity) []*T {
	id, ok := TryGetID[T]()
	if !ok {
		return nil
	}
	addMask := makeMask1(id)
	size := int(componentSizes[id])

	// Sort to group by archetype without map
	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res := make([]*T, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot := oldArch.getSlot(id)
			if slot == -1 {
				continue
			}
			base := unsafe.Pointer(&oldArch.componentData[slot][0])
			stride := uintptr(size)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p := unsafe.Pointer(uintptr(base) + uintptr(meta.Index)*stride)
				res[gi] = (*T)(p)
			}
			continue
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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		// Pre-extend all component data
		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot := newArch.getSlot(id)
		base := unsafe.Pointer(&newArch.componentData[slot][0])
		stride := uintptr(size)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num) // pre-allocate with cap

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			// Copy existing components
			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p := unsafe.Pointer(uintptr(base) + uintptr(newIndex)*stride)
			res[gi] = (*T)(p)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res
}

// AddComponentBatch2 adds two components to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch2[T1 any, T2 any](w *World, entities []Entity) ([]*T1, []*T2) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	if !ok1 || !ok2 {
		return nil, nil
	}
	addMask := makeMask2(id1, id2)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])

	// Sort to group by archetype without map
	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res1 := make([]*T1, len(entities))
	res2 := make([]*T2, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			if slot1 == -1 || slot2 == -1 {
				continue
			}
			base1 := unsafe.Pointer(&oldArch.componentData[slot1][0])
			base2 := unsafe.Pointer(&oldArch.componentData[slot2][0])
			stride1 := uintptr(size1)
			stride2 := uintptr(size2)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p1 := unsafe.Pointer(uintptr(base1) + uintptr(meta.Index)*stride1)
				p2 := unsafe.Pointer(uintptr(base2) + uintptr(meta.Index)*stride2)
				res1[gi] = (*T1)(p1)
				res2[gi] = (*T2)(p2)
			}
			continue
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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		base1 := unsafe.Pointer(&newArch.componentData[slot1][0])
		base2 := unsafe.Pointer(&newArch.componentData[slot2][0])
		stride1 := uintptr(size1)
		stride2 := uintptr(size2)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p1 := unsafe.Pointer(uintptr(base1) + uintptr(newIndex)*stride1)
			p2 := unsafe.Pointer(uintptr(base2) + uintptr(newIndex)*stride2)
			res1[gi] = (*T1)(p1)
			res2[gi] = (*T2)(p2)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res1, res2
}

// AddComponentBatch3 adds three components to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch3[T1 any, T2 any, T3 any](w *World, entities []Entity) ([]*T1, []*T2, []*T3) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	if !ok1 || !ok2 || !ok3 {
		return nil, nil, nil
	}
	addMask := makeMask3(id1, id2, id3)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res1 := make([]*T1, len(entities))
	res2 := make([]*T2, len(entities))
	res3 := make([]*T3, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 {
				continue
			}
			base1 := unsafe.Pointer(&oldArch.componentData[slot1][0])
			base2 := unsafe.Pointer(&oldArch.componentData[slot2][0])
			base3 := unsafe.Pointer(&oldArch.componentData[slot3][0])
			stride1 := uintptr(size1)
			stride2 := uintptr(size2)
			stride3 := uintptr(size3)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p1 := unsafe.Pointer(uintptr(base1) + uintptr(meta.Index)*stride1)
				p2 := unsafe.Pointer(uintptr(base2) + uintptr(meta.Index)*stride2)
				p3 := unsafe.Pointer(uintptr(base3) + uintptr(meta.Index)*stride3)
				res1[gi] = (*T1)(p1)
				res2[gi] = (*T2)(p2)
				res3[gi] = (*T3)(p3)
			}
			continue
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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		base1 := unsafe.Pointer(&newArch.componentData[slot1][0])
		base2 := unsafe.Pointer(&newArch.componentData[slot2][0])
		base3 := unsafe.Pointer(&newArch.componentData[slot3][0])
		stride1 := uintptr(size1)
		stride2 := uintptr(size2)
		stride3 := uintptr(size3)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p1 := unsafe.Pointer(uintptr(base1) + uintptr(newIndex)*stride1)
			p2 := unsafe.Pointer(uintptr(base2) + uintptr(newIndex)*stride2)
			p3 := unsafe.Pointer(uintptr(base3) + uintptr(newIndex)*stride3)
			res1[gi] = (*T1)(p1)
			res2[gi] = (*T2)(p2)
			res3[gi] = (*T3)(p3)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res1, res2, res3
}

// AddComponentBatch4 adds four components to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch4[T1 any, T2 any, T3 any, T4 any](w *World, entities []Entity) ([]*T1, []*T2, []*T3, []*T4) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return nil, nil, nil, nil
	}
	addMask := makeMask4(id1, id2, id3, id4)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res1 := make([]*T1, len(entities))
	res2 := make([]*T2, len(entities))
	res3 := make([]*T3, len(entities))
	res4 := make([]*T4, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			slot4 := oldArch.getSlot(id4)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 || slot4 == -1 {
				continue
			}
			base1 := unsafe.Pointer(&oldArch.componentData[slot1][0])
			base2 := unsafe.Pointer(&oldArch.componentData[slot2][0])
			base3 := unsafe.Pointer(&oldArch.componentData[slot3][0])
			base4 := unsafe.Pointer(&oldArch.componentData[slot4][0])
			stride1 := uintptr(size1)
			stride2 := uintptr(size2)
			stride3 := uintptr(size3)
			stride4 := uintptr(size4)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p1 := unsafe.Pointer(uintptr(base1) + uintptr(meta.Index)*stride1)
				p2 := unsafe.Pointer(uintptr(base2) + uintptr(meta.Index)*stride2)
				p3 := unsafe.Pointer(uintptr(base3) + uintptr(meta.Index)*stride3)
				p4 := unsafe.Pointer(uintptr(base4) + uintptr(meta.Index)*stride4)
				res1[gi] = (*T1)(p1)
				res2[gi] = (*T2)(p2)
				res3[gi] = (*T3)(p3)
				res4[gi] = (*T4)(p4)
			}
			continue
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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		slot4 := newArch.getSlot(id4)
		base1 := unsafe.Pointer(&newArch.componentData[slot1][0])
		base2 := unsafe.Pointer(&newArch.componentData[slot2][0])
		base3 := unsafe.Pointer(&newArch.componentData[slot3][0])
		base4 := unsafe.Pointer(&newArch.componentData[slot4][0])
		stride1 := uintptr(size1)
		stride2 := uintptr(size2)
		stride3 := uintptr(size3)
		stride4 := uintptr(size4)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p1 := unsafe.Pointer(uintptr(base1) + uintptr(newIndex)*stride1)
			p2 := unsafe.Pointer(uintptr(base2) + uintptr(newIndex)*stride2)
			p3 := unsafe.Pointer(uintptr(base3) + uintptr(newIndex)*stride3)
			p4 := unsafe.Pointer(uintptr(base4) + uintptr(newIndex)*stride4)
			res1[gi] = (*T1)(p1)
			res2[gi] = (*T2)(p2)
			res3[gi] = (*T3)(p3)
			res4[gi] = (*T4)(p4)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res1, res2, res3, res4
}

// AddComponentBatch5 adds five components to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, entities []Entity) ([]*T1, []*T2, []*T3, []*T4, []*T5) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	id5, ok5 := TryGetID[T5]()
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return nil, nil, nil, nil, nil
	}
	addMask := makeMask5(id1, id2, id3, id4, id5)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])
	size5 := int(componentSizes[id5])

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	res1 := make([]*T1, len(entities))
	res2 := make([]*T2, len(entities))
	res3 := make([]*T3, len(entities))
	res4 := make([]*T4, len(entities))
	res5 := make([]*T5, len(entities))

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			slot4 := oldArch.getSlot(id4)
			slot5 := oldArch.getSlot(id5)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 || slot4 == -1 || slot5 == -1 {
				continue
			}
			base1 := unsafe.Pointer(&oldArch.componentData[slot1][0])
			base2 := unsafe.Pointer(&oldArch.componentData[slot2][0])
			base3 := unsafe.Pointer(&oldArch.componentData[slot3][0])
			base4 := unsafe.Pointer(&oldArch.componentData[slot4][0])
			base5 := unsafe.Pointer(&oldArch.componentData[slot5][0])
			stride1 := uintptr(size1)
			stride2 := uintptr(size2)
			stride3 := uintptr(size3)
			stride4 := uintptr(size4)
			stride5 := uintptr(size5)
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				p1 := unsafe.Pointer(uintptr(base1) + uintptr(meta.Index)*stride1)
				p2 := unsafe.Pointer(uintptr(base2) + uintptr(meta.Index)*stride2)
				p3 := unsafe.Pointer(uintptr(base3) + uintptr(meta.Index)*stride3)
				p4 := unsafe.Pointer(uintptr(base4) + uintptr(meta.Index)*stride4)
				p5 := unsafe.Pointer(uintptr(base5) + uintptr(meta.Index)*stride5)
				res1[gi] = (*T1)(p1)
				res2[gi] = (*T2)(p2)
				res3[gi] = (*T3)(p3)
				res4[gi] = (*T4)(p4)
				res5[gi] = (*T5)(p5)
			}
			continue
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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		slot4 := newArch.getSlot(id4)
		slot5 := newArch.getSlot(id5)
		base1 := unsafe.Pointer(&newArch.componentData[slot1][0])
		base2 := unsafe.Pointer(&newArch.componentData[slot2][0])
		base3 := unsafe.Pointer(&newArch.componentData[slot3][0])
		base4 := unsafe.Pointer(&newArch.componentData[slot4][0])
		base5 := unsafe.Pointer(&newArch.componentData[slot5][0])
		stride1 := uintptr(size1)
		stride2 := uintptr(size2)
		stride3 := uintptr(size3)
		stride4 := uintptr(size4)
		stride5 := uintptr(size5)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			p1 := unsafe.Pointer(uintptr(base1) + uintptr(newIndex)*stride1)
			p2 := unsafe.Pointer(uintptr(base2) + uintptr(newIndex)*stride2)
			p3 := unsafe.Pointer(uintptr(base3) + uintptr(newIndex)*stride3)
			p4 := unsafe.Pointer(uintptr(base4) + uintptr(newIndex)*stride4)
			p5 := unsafe.Pointer(uintptr(base5) + uintptr(newIndex)*stride5)
			res1[gi] = (*T1)(p1)
			res2[gi] = (*T2)(p2)
			res3[gi] = (*T3)(p3)
			res4[gi] = (*T4)(p4)
			res5[gi] = (*T5)(p5)

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return res1, res2, res3, res4, res5
}

// SetComponentBatch sets a component to the same value for multiple entities.
// If the component is missing in some entities, it adds it.
// It does not return anything.
func SetComponentBatch[T any](w *World, entities []Entity, comp T) {
	id, ok := TryGetID[T]()
	if !ok {
		return
	}
	setMask := makeMask1(id)
	size := int(componentSizes[id])
	src := unsafe.Slice((*byte)(unsafe.Pointer(&comp)), size)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot := oldArch.getSlot(id)
			if slot == -1 {
				continue
			}
			bytes := oldArch.componentData[slot]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes[meta.Index*size:(meta.Index+1)*size], src)
			}
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot := newArch.getSlot(id)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			// Set the component
			bytes := newArch.componentData[slot]
			dstStart := len(bytes) - num*size + j*size
			copy(bytes[dstStart:dstStart+size], src)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// SetComponentBatch2 sets two components to the same values for multiple entities.
// If any component is missing in some entities, it adds them.
// It does not return anything.
func SetComponentBatch2[T1 any, T2 any](w *World, entities []Entity, comp1 T1, comp2 T2) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	if !ok1 || !ok2 {
		return
	}
	setMask := makeMask2(id1, id2)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			if slot1 == -1 || slot2 == -1 {
				continue
			}
			bytes1 := oldArch.componentData[slot1]
			bytes2 := oldArch.componentData[slot2]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
				copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
			}
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			// Set the components
			bytes1 := newArch.componentData[slot1]
			dstStart1 := len(bytes1) - num*size1 + j*size1
			copy(bytes1[dstStart1:dstStart1+size1], src1)
			bytes2 := newArch.componentData[slot2]
			dstStart2 := len(bytes2) - num*size2 + j*size2
			copy(bytes2[dstStart2:dstStart2+size2], src2)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// SetComponentBatch3 sets three components to the same values for multiple entities.
// If any component is missing in some entities, it adds them.
// It does not return anything.
func SetComponentBatch3[T1 any, T2 any, T3 any](w *World, entities []Entity, comp1 T1, comp2 T2, comp3 T3) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	if !ok1 || !ok2 || !ok3 {
		return
	}
	setMask := makeMask3(id1, id2, id3)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&comp3)), size3)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 {
				continue
			}
			bytes1 := oldArch.componentData[slot1]
			bytes2 := oldArch.componentData[slot2]
			bytes3 := oldArch.componentData[slot3]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
				copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
				copy(bytes3[meta.Index*size3:(meta.Index+1)*size3], src3)
			}
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			bytes1 := newArch.componentData[slot1]
			dstStart1 := len(bytes1) - num*size1 + j*size1
			copy(bytes1[dstStart1:dstStart1+size1], src1)
			bytes2 := newArch.componentData[slot2]
			dstStart2 := len(bytes2) - num*size2 + j*size2
			copy(bytes2[dstStart2:dstStart2+size2], src2)
			bytes3 := newArch.componentData[slot3]
			dstStart3 := len(bytes3) - num*size3 + j*size3
			copy(bytes3[dstStart3:dstStart3+size3], src3)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// SetComponentBatch4 sets four components to the same values for multiple entities.
// If any component is missing in some entities, it adds them.
// It does not return anything.
func SetComponentBatch4[T1 any, T2 any, T3 any, T4 any](w *World, entities []Entity, comp1 T1, comp2 T2, comp3 T3, comp4 T4) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return
	}
	setMask := makeMask4(id1, id2, id3, id4)
	size1 := int(componentSizes[id1])
	size2 := int(componentSizes[id2])
	size3 := int(componentSizes[id3])
	size4 := int(componentSizes[id4])
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&comp1)), size1)
	src2 := unsafe.Slice((*byte)(unsafe.Pointer(&comp2)), size2)
	src3 := unsafe.Slice((*byte)(unsafe.Pointer(&comp3)), size3)
	src4 := unsafe.Slice((*byte)(unsafe.Pointer(&comp4)), size4)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			slot4 := oldArch.getSlot(id4)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 || slot4 == -1 {
				continue
			}
			bytes1 := oldArch.componentData[slot1]
			bytes2 := oldArch.componentData[slot2]
			bytes3 := oldArch.componentData[slot3]
			bytes4 := oldArch.componentData[slot4]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
				copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
				copy(bytes3[meta.Index*size3:(meta.Index+1)*size3], src3)
				copy(bytes4[meta.Index*size4:(meta.Index+1)*size4], src4)
			}
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		slot4 := newArch.getSlot(id4)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			bytes1 := newArch.componentData[slot1]
			dstStart1 := len(bytes1) - num*size1 + j*size1
			copy(bytes1[dstStart1:dstStart1+size1], src1)
			bytes2 := newArch.componentData[slot2]
			dstStart2 := len(bytes2) - num*size2 + j*size2
			copy(bytes2[dstStart2:dstStart2+size2], src2)
			bytes3 := newArch.componentData[slot3]
			dstStart3 := len(bytes3) - num*size3 + j*size3
			copy(bytes3[dstStart3:dstStart3+size3], src3)
			bytes4 := newArch.componentData[slot4]
			dstStart4 := len(bytes4) - num*size4 + j*size4
			copy(bytes4[dstStart4:dstStart4+size4], src4)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// SetComponentBatch5 sets five components to the same values for multiple entities.
// If any component is missing in some entities, it adds them.
// It does not return anything.
func SetComponentBatch5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, entities []Entity, comp1 T1, comp2 T2, comp3 T3, comp4 T4, comp5 T5) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	id5, ok5 := TryGetID[T5]()
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return
	}
	setMask := makeMask5(id1, id2, id3, id4, id5)
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

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			slot1 := oldArch.getSlot(id1)
			slot2 := oldArch.getSlot(id2)
			slot3 := oldArch.getSlot(id3)
			slot4 := oldArch.getSlot(id4)
			slot5 := oldArch.getSlot(id5)
			if slot1 == -1 || slot2 == -1 || slot3 == -1 || slot4 == -1 || slot5 == -1 {
				continue
			}
			bytes1 := oldArch.componentData[slot1]
			bytes2 := oldArch.componentData[slot2]
			bytes3 := oldArch.componentData[slot3]
			bytes4 := oldArch.componentData[slot4]
			bytes5 := oldArch.componentData[slot5]
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				copy(bytes1[meta.Index*size1:(meta.Index+1)*size1], src1)
				copy(bytes2[meta.Index*size2:(meta.Index+1)*size2], src2)
				copy(bytes3[meta.Index*size3:(meta.Index+1)*size3], src3)
				copy(bytes4[meta.Index*size4:(meta.Index+1)*size4], src4)
				copy(bytes5[meta.Index*size5:(meta.Index+1)*size5], src5)
			}
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot1 := newArch.getSlot(id1)
		slot2 := newArch.getSlot(id2)
		slot3 := newArch.getSlot(id3)
		slot4 := newArch.getSlot(id4)
		slot5 := newArch.getSlot(id5)

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			bytes1 := newArch.componentData[slot1]
			dstStart1 := len(bytes1) - num*size1 + j*size1
			copy(bytes1[dstStart1:dstStart1+size1], src1)
			bytes2 := newArch.componentData[slot2]
			dstStart2 := len(bytes2) - num*size2 + j*size2
			copy(bytes2[dstStart2:dstStart2+size2], src2)
			bytes3 := newArch.componentData[slot3]
			dstStart3 := len(bytes3) - num*size3 + j*size3
			copy(bytes3[dstStart3:dstStart3+size3], src3)
			bytes4 := newArch.componentData[slot4]
			dstStart4 := len(bytes4) - num*size4 + j*size4
			copy(bytes4[dstStart4:dstStart4+size4], src4)
			bytes5 := newArch.componentData[slot5]
			dstStart5 := len(bytes5) - num*size5 + j*size5
			copy(bytes5[dstStart5:dstStart5+size5], src5)

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// removePair is used for batch removals.
type removePair struct {
	index int
	e     Entity
}

// RemoveComponentBatch removes a component from multiple entities if present.
func RemoveComponentBatch[T any](w *World, entities []Entity) {
	id, ok := TryGetID[T]()
	if !ok {
		return
	}
	removeMask := makeMask1(id)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// RemoveComponentBatch2 removes two components from multiple entities if present.
func RemoveComponentBatch2[T1 any, T2 any](w *World, entities []Entity) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	if !ok1 || !ok2 {
		return
	}
	removeMask := makeMask2(id1, id2)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// RemoveComponentBatch3 removes three components from multiple entities if present.
func RemoveComponentBatch3[T1 any, T2 any, T3 any](w *World, entities []Entity) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	if !ok1 || !ok2 || !ok3 {
		return
	}
	removeMask := makeMask3(id1, id2, id3)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// RemoveComponentBatch4 removes four components from multiple entities if present.
func RemoveComponentBatch4[T1 any, T2 any, T3 any, T4 any](w *World, entities []Entity) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return
	}
	removeMask := makeMask4(id1, id2, id3, id4)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// RemoveComponentBatch5 removes five components from multiple entities if present.
func RemoveComponentBatch5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, entities []Entity) {
	id1, ok1 := TryGetID[T1]()
	id2, ok2 := TryGetID[T2]()
	id3, ok3 := TryGetID[T3]()
	id4, ok4 := TryGetID[T4]()
	id5, ok5 := TryGetID[T5]()
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		return
	}
	removeMask := makeMask5(id1, id2, id3, id4, id5)

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}
