package lazyecs

import (
	"sort"
	"unsafe"
)

// Batch provides a way to create entities with a pre-defined set of components.
type Batch[T any] struct {
	world *World
	arch  *Archetype
	id1   ComponentID
	size1 int
}

// CreateBatch creates a new Batch for creating entities with one component type.
func CreateBatch[T any](w *World) *Batch[T] {
	id1, ok := TryGetID[T]()
	if !ok {
		panic("component in CreateBatch is not registered")
	}
	mask := makeMask1(id1)
	arch := w.getOrCreateArchetype(mask)
	return &Batch[T]{
		world: w,
		arch:  arch,
		id1:   id1,
		size1: int(componentSizes[id1]),
	}
}

// CreateEntities creates a specified number of entities with the batch's components.
func (self *Batch[T]) CreateEntities(count int) []Entity {
	return self.CreateEntitiesTo(count, nil)
}

// CreateEntitiesTo creates entities and appends them to the destination slice.
func (self *Batch[T]) CreateEntitiesTo(count int, dst []Entity) []Entity {
	if count <= 0 {
		return dst
	}
	w := self.world
	arch := self.arch

	startLen := len(dst)
	dst = extendSlice(dst, count)
	entities := dst[startLen:]

	startIndex := len(arch.entities)
	arch.entities = extendSlice(arch.entities, count)

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
	return dst
}

// CreateEntitiesWithComponents creates entities with the specified component value.
func (self *Batch[T]) CreateEntitiesWithComponents(count int, c1 T) []Entity {
	return self.CreateEntitiesWithComponentsTo(count, nil, c1)
}

// CreateEntitiesWithComponentsTo creates entities with components and appends them to the destination slice.
func (self *Batch[T]) CreateEntitiesWithComponentsTo(count int, dst []Entity, c1 T) []Entity {
	startLen := len(dst)
	dst = self.CreateEntitiesTo(count, dst)
	entities := dst[startLen:]

	if len(entities) == 0 {
		return dst
	}
	arch := self.arch
	slot1 := arch.getSlot(self.id1)
	data1 := arch.componentData[slot1]
	src1 := unsafe.Slice((*byte)(unsafe.Pointer(&c1)), self.size1)

	startIndex := len(data1) - count*self.size1
	for i := 0; i < count; i++ {
		copy(data1[startIndex+i*self.size1:], src1)
	}
	return dst
}

// AddComponentBatch adds a component to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch[T any](w *World, entities []Entity) []*T {
	id, ok := TryGetID[T]()
	if !ok {
		return nil
	}
	addMask := makeMask1(id)
	size := int(componentSizes[id])

	// Get temporary slices from the world's pools to avoid allocations.
	temp := w.getEntrySlice(len(entities))
	defer w.putEntrySlice(temp)

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

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		slot := newArch.getSlot(id)
		base := unsafe.Pointer(&newArch.componentData[slot][0])
		stride := uintptr(size)

		pairs := w.getRemovePairSlice(num)
		defer w.putRemovePairSlice(pairs)

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

// SetComponentBatch sets a component to the same value for multiple entities.
func SetComponentBatch[T any](w *World, entities []Entity, comp T) {
	id, ok := TryGetID[T]()
	if !ok {
		return
	}
	setMask := makeMask1(id)
	size := int(componentSizes[id])
	src := unsafe.Slice((*byte)(unsafe.Pointer(&comp)), size)

	temp := w.getEntrySlice(len(entities))
	defer w.putEntrySlice(temp)

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

		pairs := w.getRemovePairSlice(num)
		defer w.putRemovePairSlice(pairs)

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

// RemoveComponentBatch removes a component from multiple entities if present.
func RemoveComponentBatch[T any](w *World, entities []Entity) {
	id, ok := TryGetID[T]()
	if !ok {
		return
	}
	removeMask := makeMask1(id)

	temp := w.getEntrySlice(len(entities))
	defer w.putEntrySlice(temp)

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

		pairs := w.getRemovePairSlice(num)
		defer w.putRemovePairSlice(pairs)

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
