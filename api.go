package lazyecs

import (
	"sort"
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

// Query is an iterator over entities that have a specific set of components.
type Query[T any] struct {
	world         *World
	includeMask   maskType
	excludeMask   maskType
	id1           ComponentID
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	currentEntity Entity
}

// CreateQuery creates a new query for entities with one specific component type.
func CreateQuery[T any](w *World, excludes ...ComponentID) Query[T] {
	id1 := GetID[T]()
	return Query[T]{
		world:       w,
		includeMask: makeMask1(id1),
		excludeMask: makeMask(excludes),
		id1:         id1,
		archIdx:     0,
		index:       -1,
	}
}

// Reset resets the query for reuse.
func (self *Query[T]) Reset() {
	self.archIdx = 0
	self.index = -1
	self.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (self *Query[T]) Next() bool {
	self.index++
	if self.currentArch != nil && self.index < len(self.currentArch.entities) {
		self.currentEntity = self.currentArch.entities[self.index]
		return true
	}
	if self.archIdx == -1 {
		return false // End of special query
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
func (self *Query[T]) Get() *T {
	p1 := unsafe.Pointer(uintptr(self.base1) + uintptr(self.index)*self.stride1)
	return (*T)(p1)
}

// Entity returns the current entity.
func (self *Query[T]) Entity() Entity {
	return self.currentEntity
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

// SetComponentBatch sets a component to the same value for multiple entities.
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
