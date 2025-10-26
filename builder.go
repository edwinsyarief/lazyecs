package teishoku

import (
	"reflect"
	"unsafe"
)

// Builder ...
type Builder[T any] struct {
	world  *World
	arch   *archetype
	compID uint8
}

// NewBuilder ...
func NewBuilder[T any](w *World) *Builder[T] {
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	var mask bitmask256
	mask.set(id)
	sp := compSpec{id: id, typ: t, size: w.components.compIDToSize[id]}
	arch := w.getOrCreateArchetype(mask, []compSpec{sp})
	return &Builder[T]{world: w, arch: arch, compID: id}
}

// New ...
func (b *Builder[T]) New(w *World) *Builder[T] {
	return NewBuilder[T](w)
}

// NewEntity ...
func (b *Builder[T]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities ...
func (b *Builder[T]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	a := b.arch
	remaining := count
	for remaining > 0 {
		if len(a.chunks) == 0 || a.chunks[len(a.chunks)-1].size == ChunkSize {
			a.chunks = append(a.chunks, w.newChunk(a))
		}
		lastC := a.chunks[len(a.chunks)-1]
		avail := ChunkSize - lastC.size
		batch := min(avail, remaining)
		if len(w.entities.freeIDs) < batch {
			w.expand(batch - len(w.entities.freeIDs) + 1)
		}
		startIdx := lastC.size
		popped := w.entities.freeIDs[len(w.entities.freeIDs)-batch:]
		w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-batch]
		for k := 0; k < batch; k++ {
			id := popped[k]
			meta := &w.entities.metas[id]
			meta.archetypeIndex = a.index
			meta.chunkIndex = len(a.chunks) - 1
			meta.index = startIdx + k
			meta.version = w.entities.nextEntityVer
			ent := Entity{ID: id, Version: meta.version}
			lastC.entityIDs[startIdx+k] = ent
			w.entities.nextEntityVer++
		}
		lastC.size += batch
		a.size += batch
		remaining -= batch
	}
	w.mutationVersion++
}

// NewEntitiesWithValueSet ...
func (b *Builder[T]) NewEntitiesWithValueSet(count int, comp T) {
	if count == 0 {
		return
	}
	w := b.world
	a := b.arch
	remaining := count
	for remaining > 0 {
		if len(a.chunks) == 0 || a.chunks[len(a.chunks)-1].size == ChunkSize {
			a.chunks = append(a.chunks, w.newChunk(a))
		}
		lastC := a.chunks[len(a.chunks)-1]
		avail := ChunkSize - lastC.size
		batch := min(avail, remaining)
		if len(w.entities.freeIDs) < batch {
			w.expand(batch - len(w.entities.freeIDs) + 1)
		}
		startIdx := lastC.size
		popped := w.entities.freeIDs[len(w.entities.freeIDs)-batch:]
		w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-batch]
		for k := 0; k < batch; k++ {
			id := popped[k]
			meta := &w.entities.metas[id]
			meta.archetypeIndex = a.index
			meta.chunkIndex = len(a.chunks) - 1
			meta.index = startIdx + k
			meta.version = w.entities.nextEntityVer
			ent := Entity{ID: id, Version: meta.version}
			lastC.entityIDs[startIdx+k] = ent
			ptr := unsafe.Pointer(uintptr(lastC.compPointers[b.compID]) + uintptr(startIdx+k)*a.compSizes[b.compID])
			*(*T)(ptr) = comp
			w.entities.nextEntityVer++
		}
		lastC.size += batch
		a.size += batch
		remaining -= batch
	}
	w.mutationVersion++
}

// Get ...
func (b *Builder[T]) Get(e Entity) *T {
	w := b.world
	if !w.IsValid(e) {
		return nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	id := b.compID
	i := id >> 6
	o := id & 63
	if (a.mask[i] & (uint64(1) << uint64(o))) == 0 {
		return nil
	}
	chunk := a.chunks[meta.chunkIndex]
	ptr := unsafe.Pointer(uintptr(chunk.compPointers[id]) + uintptr(meta.index)*a.compSizes[id])
	return (*T)(ptr)
}

// Set ...
func (b *Builder[T]) Set(e Entity, comp T) {
	w := b.world
	if !w.IsValid(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	id := b.compID
	i := id >> 6
	o := id & 63
	if (a.mask[i] & (uint64(1) << uint64(o))) != 0 {
		chunk := a.chunks[meta.chunkIndex]
		ptr := unsafe.Pointer(uintptr(chunk.compPointers[id]) + uintptr(meta.index)*a.compSizes[id])
		*(*T)(ptr) = comp
		return
	}
	newMask := a.mask
	newMask.set(id)
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
		tempSpecs[count] = compSpec{id: id, typ: w.components.compIDToType[id], size: w.components.compIDToSize[id]}
		count++
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
		src := unsafe.Pointer(uintptr(oldChunk.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(newChunk.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	dst := unsafe.Pointer(uintptr(newChunk.compPointers[id]) + uintptr(newIdx)*targetA.compSizes[id])
	*(*T)(dst) = comp
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.chunkIndex = len(targetA.chunks) - 1
	meta.index = newIdx
}

// SetBatch ...
func (b *Builder[T]) SetBatch(entities []Entity, comp T) {
	for _, e := range entities {
		b.Set(e, comp)
	}
}
