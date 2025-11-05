package teishoku

import (
	"reflect"
	"unsafe"
)

// Builder2 provides a highly efficient, type-safe API for creating entities
// with a predefined set of 2 components: T1, T2.
type Builder2[T1 any, T2 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
	
}

// NewBuilder2 creates a new `Builder` for entities with the 2 components
// T1, T2. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder2`.
func NewBuilder2[T1 any, T2 any](w *World) *Builder2[T1, T2] {
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	
	w.components.mu.RUnlock()

	if id2 == id1 {
		panic("ecs: duplicate component types in Builder2")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	
	w.components.mu.RLock()
	specs := []compSpec{
		{id: id1, typ: t1, size: w.components.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.components.compIDToSize[id2]},
		
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder2[T1, T2]{world: w, arch: arch, id1: id1, id2: id2}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder2`.
func (b *Builder2[T1, T2]) New(w *World) *Builder2[T1, T2] {
	return NewBuilder2[T1, T2](w)
}

// NewEntity creates a single new entity with the 2 components defined by the
// builder: T1, T2. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder2[T1, T2]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 2 components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder2[T1, T2]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// NewEntitiesWithValueSet creates a batch of `count` entities and initializes
// their components to the provided values.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp1: The initial value for the component T1.
//   - comp2: The initial value for the component T2.
func (b *Builder2[T1, T2]) NewEntitiesWithValueSet(count int, comp1 T1, comp2 T2) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])) = comp1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])) = comp2
		
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// Get retrieves pointers to the components for the given entity.
//
// If the entity is invalid or does not have all the components, this returns nils.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data, or nils if not found.
func (b *Builder2[T1, T2]) Get(e Entity) (*T1, *T2) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := b.id1 >> 6
	o1 := b.id1 & 63
	i2 := b.id2 >> 6
	o2 := b.id2 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 {
		return nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[b.id1], uintptr(meta.index)*a.compSizes[b.id1])),
		(*T2)(unsafe.Add(a.compPointers[b.id2], uintptr(meta.index)*a.compSizes[b.id2]))
}

// Set adds or updates the components for a given entity with the specified
// values.
//
// If the entity already has all the components, their values are updated. If not,
// the missing components are added, which may trigger an archetype change.
//
// It is safe to call this on an invalid entity; the operation will be ignored.
//
// Parameters:
//   - e: The entity to modify.
//   - v1: The value for T1.
//   - v2: The value for T2.
func (b *Builder2[T1, T2]) Set(e Entity, v1 T1, v2 T2) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := (a.mask[b.id1>>6] & (uint64(1) << uint64(b.id1&63))) != 0
	has2 := (a.mask[b.id2>>6] & (uint64(1) << uint64(b.id2&63))) != 0
	
	if has1 && has2 {
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])) = v1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])) = v2
		
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(b.id1)
	}
	if !has2 {
		newMask.set(b.id2)
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
			tempSpecs[count] = compSpec{id: b.id1, typ: w.components.compIDToType[b.id1], size: w.components.compIDToSize[b.id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: b.id2, typ: w.components.compIDToType[b.id2], size: w.components.compIDToSize[b.id2]}
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
	*(*T1)(unsafe.Pointer(uintptr(targetA.compPointers[b.id1]) + uintptr(newIdx)*targetA.compSizes[b.id1])) = v1
	*(*T2)(unsafe.Pointer(uintptr(targetA.compPointers[b.id2]) + uintptr(newIdx)*targetA.compSizes[b.id2])) = v2
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// SetBatch efficiently sets the component values for a slice of entities.
// It iterates over the entities and calls `Set` for each one.
//
// Parameters:
//   - entities: A slice of entities to modify.
//   - v1: The component value to set for type T1.
//   - v2: The component value to set for type T2.
func (b *Builder2[T1, T2]) SetBatch(entities []Entity, v1 T1, v2 T2) {
	for _, e := range entities {
		b.Set(e, v1, v2)
	}
}

// Builder3 provides a highly efficient, type-safe API for creating entities
// with a predefined set of 3 components: T1, T2, T3.
type Builder3[T1 any, T2 any, T3 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
	id3   uint8
	
}

// NewBuilder3 creates a new `Builder` for entities with the 3 components
// T1, T2, T3. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder3`.
func NewBuilder3[T1 any, T2 any, T3 any](w *World) *Builder3[T1, T2, T3] {
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	t3 := reflect.TypeFor[T3]()
	
	w.components.mu.RLock()
	id1 := w.getCompTypeIDNoLock(t1)
	id2 := w.getCompTypeIDNoLock(t2)
	id3 := w.getCompTypeIDNoLock(t3)
	
	w.components.mu.RUnlock()

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in Builder3")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	
	w.components.mu.RLock()
	specs := []compSpec{
		{id: id1, typ: t1, size: w.components.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.components.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.components.compIDToSize[id3]},
		
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder3[T1, T2, T3]{world: w, arch: arch, id1: id1, id2: id2, id3: id3}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder3`.
func (b *Builder3[T1, T2, T3]) New(w *World) *Builder3[T1, T2, T3] {
	return NewBuilder3[T1, T2, T3](w)
}

// NewEntity creates a single new entity with the 3 components defined by the
// builder: T1, T2, T3. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder3[T1, T2, T3]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 3 components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder3[T1, T2, T3]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// NewEntitiesWithValueSet creates a batch of `count` entities and initializes
// their components to the provided values.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp1: The initial value for the component T1.
//   - comp2: The initial value for the component T2.
//   - comp3: The initial value for the component T3.
func (b *Builder3[T1, T2, T3]) NewEntitiesWithValueSet(count int, comp1 T1, comp2 T2, comp3 T3) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])) = comp1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])) = comp2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])) = comp3
		
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// Get retrieves pointers to the components for the given entity.
//
// If the entity is invalid or does not have all the components, this returns nils.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data, or nils if not found.
func (b *Builder3[T1, T2, T3]) Get(e Entity) (*T1, *T2, *T3) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := b.id1 >> 6
	o1 := b.id1 & 63
	i2 := b.id2 >> 6
	o2 := b.id2 & 63
	i3 := b.id3 >> 6
	o3 := b.id3 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 {
		return nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[b.id1], uintptr(meta.index)*a.compSizes[b.id1])),
		(*T2)(unsafe.Add(a.compPointers[b.id2], uintptr(meta.index)*a.compSizes[b.id2])),
		(*T3)(unsafe.Add(a.compPointers[b.id3], uintptr(meta.index)*a.compSizes[b.id3]))
}

// Set adds or updates the components for a given entity with the specified
// values.
//
// If the entity already has all the components, their values are updated. If not,
// the missing components are added, which may trigger an archetype change.
//
// It is safe to call this on an invalid entity; the operation will be ignored.
//
// Parameters:
//   - e: The entity to modify.
//   - v1: The value for T1.
//   - v2: The value for T2.
//   - v3: The value for T3.
func (b *Builder3[T1, T2, T3]) Set(e Entity, v1 T1, v2 T2, v3 T3) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := (a.mask[b.id1>>6] & (uint64(1) << uint64(b.id1&63))) != 0
	has2 := (a.mask[b.id2>>6] & (uint64(1) << uint64(b.id2&63))) != 0
	has3 := (a.mask[b.id3>>6] & (uint64(1) << uint64(b.id3&63))) != 0
	
	if has1 && has2 && has3 {
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])) = v1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])) = v2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(meta.index)*a.compSizes[b.id3])) = v3
		
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(b.id1)
	}
	if !has2 {
		newMask.set(b.id2)
	}
	if !has3 {
		newMask.set(b.id3)
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
			tempSpecs[count] = compSpec{id: b.id1, typ: w.components.compIDToType[b.id1], size: w.components.compIDToSize[b.id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: b.id2, typ: w.components.compIDToType[b.id2], size: w.components.compIDToSize[b.id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: b.id3, typ: w.components.compIDToType[b.id3], size: w.components.compIDToSize[b.id3]}
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
	*(*T1)(unsafe.Pointer(uintptr(targetA.compPointers[b.id1]) + uintptr(newIdx)*targetA.compSizes[b.id1])) = v1
	*(*T2)(unsafe.Pointer(uintptr(targetA.compPointers[b.id2]) + uintptr(newIdx)*targetA.compSizes[b.id2])) = v2
	*(*T3)(unsafe.Pointer(uintptr(targetA.compPointers[b.id3]) + uintptr(newIdx)*targetA.compSizes[b.id3])) = v3
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// SetBatch efficiently sets the component values for a slice of entities.
// It iterates over the entities and calls `Set` for each one.
//
// Parameters:
//   - entities: A slice of entities to modify.
//   - v1: The component value to set for type T1.
//   - v2: The component value to set for type T2.
//   - v3: The component value to set for type T3.
func (b *Builder3[T1, T2, T3]) SetBatch(entities []Entity, v1 T1, v2 T2, v3 T3) {
	for _, e := range entities {
		b.Set(e, v1, v2, v3)
	}
}

// Builder4 provides a highly efficient, type-safe API for creating entities
// with a predefined set of 4 components: T1, T2, T3, T4.
type Builder4[T1 any, T2 any, T3 any, T4 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8
	
}

// NewBuilder4 creates a new `Builder` for entities with the 4 components
// T1, T2, T3, T4. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder4`.
func NewBuilder4[T1 any, T2 any, T3 any, T4 any](w *World) *Builder4[T1, T2, T3, T4] {
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
		panic("ecs: duplicate component types in Builder4")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	mask.set(id4)
	
	w.components.mu.RLock()
	specs := []compSpec{
		{id: id1, typ: t1, size: w.components.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.components.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.components.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.components.compIDToSize[id4]},
		
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder4[T1, T2, T3, T4]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder4`.
func (b *Builder4[T1, T2, T3, T4]) New(w *World) *Builder4[T1, T2, T3, T4] {
	return NewBuilder4[T1, T2, T3, T4](w)
}

// NewEntity creates a single new entity with the 4 components defined by the
// builder: T1, T2, T3, T4. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder4[T1, T2, T3, T4]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 4 components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder4[T1, T2, T3, T4]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// NewEntitiesWithValueSet creates a batch of `count` entities and initializes
// their components to the provided values.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp1: The initial value for the component T1.
//   - comp2: The initial value for the component T2.
//   - comp3: The initial value for the component T3.
//   - comp4: The initial value for the component T4.
func (b *Builder4[T1, T2, T3, T4]) NewEntitiesWithValueSet(count int, comp1 T1, comp2 T2, comp3 T3, comp4 T4) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])) = comp1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])) = comp2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])) = comp3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])) = comp4
		
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// Get retrieves pointers to the components for the given entity.
//
// If the entity is invalid or does not have all the components, this returns nils.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data, or nils if not found.
func (b *Builder4[T1, T2, T3, T4]) Get(e Entity) (*T1, *T2, *T3, *T4) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := b.id1 >> 6
	o1 := b.id1 & 63
	i2 := b.id2 >> 6
	o2 := b.id2 & 63
	i3 := b.id3 >> 6
	o3 := b.id3 & 63
	i4 := b.id4 >> 6
	o4 := b.id4 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 {
		return nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[b.id1], uintptr(meta.index)*a.compSizes[b.id1])),
		(*T2)(unsafe.Add(a.compPointers[b.id2], uintptr(meta.index)*a.compSizes[b.id2])),
		(*T3)(unsafe.Add(a.compPointers[b.id3], uintptr(meta.index)*a.compSizes[b.id3])),
		(*T4)(unsafe.Add(a.compPointers[b.id4], uintptr(meta.index)*a.compSizes[b.id4]))
}

// Set adds or updates the components for a given entity with the specified
// values.
//
// If the entity already has all the components, their values are updated. If not,
// the missing components are added, which may trigger an archetype change.
//
// It is safe to call this on an invalid entity; the operation will be ignored.
//
// Parameters:
//   - e: The entity to modify.
//   - v1: The value for T1.
//   - v2: The value for T2.
//   - v3: The value for T3.
//   - v4: The value for T4.
func (b *Builder4[T1, T2, T3, T4]) Set(e Entity, v1 T1, v2 T2, v3 T3, v4 T4) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := (a.mask[b.id1>>6] & (uint64(1) << uint64(b.id1&63))) != 0
	has2 := (a.mask[b.id2>>6] & (uint64(1) << uint64(b.id2&63))) != 0
	has3 := (a.mask[b.id3>>6] & (uint64(1) << uint64(b.id3&63))) != 0
	has4 := (a.mask[b.id4>>6] & (uint64(1) << uint64(b.id4&63))) != 0
	
	if has1 && has2 && has3 && has4 {
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])) = v1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])) = v2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(meta.index)*a.compSizes[b.id3])) = v3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(meta.index)*a.compSizes[b.id4])) = v4
		
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(b.id1)
	}
	if !has2 {
		newMask.set(b.id2)
	}
	if !has3 {
		newMask.set(b.id3)
	}
	if !has4 {
		newMask.set(b.id4)
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
			tempSpecs[count] = compSpec{id: b.id1, typ: w.components.compIDToType[b.id1], size: w.components.compIDToSize[b.id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: b.id2, typ: w.components.compIDToType[b.id2], size: w.components.compIDToSize[b.id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: b.id3, typ: w.components.compIDToType[b.id3], size: w.components.compIDToSize[b.id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: b.id4, typ: w.components.compIDToType[b.id4], size: w.components.compIDToSize[b.id4]}
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
	*(*T1)(unsafe.Pointer(uintptr(targetA.compPointers[b.id1]) + uintptr(newIdx)*targetA.compSizes[b.id1])) = v1
	*(*T2)(unsafe.Pointer(uintptr(targetA.compPointers[b.id2]) + uintptr(newIdx)*targetA.compSizes[b.id2])) = v2
	*(*T3)(unsafe.Pointer(uintptr(targetA.compPointers[b.id3]) + uintptr(newIdx)*targetA.compSizes[b.id3])) = v3
	*(*T4)(unsafe.Pointer(uintptr(targetA.compPointers[b.id4]) + uintptr(newIdx)*targetA.compSizes[b.id4])) = v4
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// SetBatch efficiently sets the component values for a slice of entities.
// It iterates over the entities and calls `Set` for each one.
//
// Parameters:
//   - entities: A slice of entities to modify.
//   - v1: The component value to set for type T1.
//   - v2: The component value to set for type T2.
//   - v3: The component value to set for type T3.
//   - v4: The component value to set for type T4.
func (b *Builder4[T1, T2, T3, T4]) SetBatch(entities []Entity, v1 T1, v2 T2, v3 T3, v4 T4) {
	for _, e := range entities {
		b.Set(e, v1, v2, v3, v4)
	}
}

// Builder5 provides a highly efficient, type-safe API for creating entities
// with a predefined set of 5 components: T1, T2, T3, T4, T5.
type Builder5[T1 any, T2 any, T3 any, T4 any, T5 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8
	id5   uint8
	
}

// NewBuilder5 creates a new `Builder` for entities with the 5 components
// T1, T2, T3, T4, T5. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder5`.
func NewBuilder5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World) *Builder5[T1, T2, T3, T4, T5] {
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
		panic("ecs: duplicate component types in Builder5")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	mask.set(id4)
	mask.set(id5)
	
	w.components.mu.RLock()
	specs := []compSpec{
		{id: id1, typ: t1, size: w.components.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.components.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.components.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.components.compIDToSize[id4]},
		{id: id5, typ: t5, size: w.components.compIDToSize[id5]},
		
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder5[T1, T2, T3, T4, T5]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder5`.
func (b *Builder5[T1, T2, T3, T4, T5]) New(w *World) *Builder5[T1, T2, T3, T4, T5] {
	return NewBuilder5[T1, T2, T3, T4, T5](w)
}

// NewEntity creates a single new entity with the 5 components defined by the
// builder: T1, T2, T3, T4, T5. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder5[T1, T2, T3, T4, T5]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 5 components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder5[T1, T2, T3, T4, T5]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// NewEntitiesWithValueSet creates a batch of `count` entities and initializes
// their components to the provided values.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp1: The initial value for the component T1.
//   - comp2: The initial value for the component T2.
//   - comp3: The initial value for the component T3.
//   - comp4: The initial value for the component T4.
//   - comp5: The initial value for the component T5.
func (b *Builder5[T1, T2, T3, T4, T5]) NewEntitiesWithValueSet(count int, comp1 T1, comp2 T2, comp3 T3, comp4 T4, comp5 T5) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])) = comp1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])) = comp2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])) = comp3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])) = comp4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(startSize+k)*a.compSizes[b.id5])) = comp5
		
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// Get retrieves pointers to the components for the given entity.
//
// If the entity is invalid or does not have all the components, this returns nils.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data, or nils if not found.
func (b *Builder5[T1, T2, T3, T4, T5]) Get(e Entity) (*T1, *T2, *T3, *T4, *T5) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := b.id1 >> 6
	o1 := b.id1 & 63
	i2 := b.id2 >> 6
	o2 := b.id2 & 63
	i3 := b.id3 >> 6
	o3 := b.id3 & 63
	i4 := b.id4 >> 6
	o4 := b.id4 & 63
	i5 := b.id5 >> 6
	o5 := b.id5 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 {
		return nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[b.id1], uintptr(meta.index)*a.compSizes[b.id1])),
		(*T2)(unsafe.Add(a.compPointers[b.id2], uintptr(meta.index)*a.compSizes[b.id2])),
		(*T3)(unsafe.Add(a.compPointers[b.id3], uintptr(meta.index)*a.compSizes[b.id3])),
		(*T4)(unsafe.Add(a.compPointers[b.id4], uintptr(meta.index)*a.compSizes[b.id4])),
		(*T5)(unsafe.Add(a.compPointers[b.id5], uintptr(meta.index)*a.compSizes[b.id5]))
}

// Set adds or updates the components for a given entity with the specified
// values.
//
// If the entity already has all the components, their values are updated. If not,
// the missing components are added, which may trigger an archetype change.
//
// It is safe to call this on an invalid entity; the operation will be ignored.
//
// Parameters:
//   - e: The entity to modify.
//   - v1: The value for T1.
//   - v2: The value for T2.
//   - v3: The value for T3.
//   - v4: The value for T4.
//   - v5: The value for T5.
func (b *Builder5[T1, T2, T3, T4, T5]) Set(e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := (a.mask[b.id1>>6] & (uint64(1) << uint64(b.id1&63))) != 0
	has2 := (a.mask[b.id2>>6] & (uint64(1) << uint64(b.id2&63))) != 0
	has3 := (a.mask[b.id3>>6] & (uint64(1) << uint64(b.id3&63))) != 0
	has4 := (a.mask[b.id4>>6] & (uint64(1) << uint64(b.id4&63))) != 0
	has5 := (a.mask[b.id5>>6] & (uint64(1) << uint64(b.id5&63))) != 0
	
	if has1 && has2 && has3 && has4 && has5 {
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])) = v1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])) = v2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(meta.index)*a.compSizes[b.id3])) = v3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(meta.index)*a.compSizes[b.id4])) = v4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(meta.index)*a.compSizes[b.id5])) = v5
		
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(b.id1)
	}
	if !has2 {
		newMask.set(b.id2)
	}
	if !has3 {
		newMask.set(b.id3)
	}
	if !has4 {
		newMask.set(b.id4)
	}
	if !has5 {
		newMask.set(b.id5)
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
			tempSpecs[count] = compSpec{id: b.id1, typ: w.components.compIDToType[b.id1], size: w.components.compIDToSize[b.id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: b.id2, typ: w.components.compIDToType[b.id2], size: w.components.compIDToSize[b.id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: b.id3, typ: w.components.compIDToType[b.id3], size: w.components.compIDToSize[b.id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: b.id4, typ: w.components.compIDToType[b.id4], size: w.components.compIDToSize[b.id4]}
			count++
		}
		if !has5 {
			tempSpecs[count] = compSpec{id: b.id5, typ: w.components.compIDToType[b.id5], size: w.components.compIDToSize[b.id5]}
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
	*(*T1)(unsafe.Pointer(uintptr(targetA.compPointers[b.id1]) + uintptr(newIdx)*targetA.compSizes[b.id1])) = v1
	*(*T2)(unsafe.Pointer(uintptr(targetA.compPointers[b.id2]) + uintptr(newIdx)*targetA.compSizes[b.id2])) = v2
	*(*T3)(unsafe.Pointer(uintptr(targetA.compPointers[b.id3]) + uintptr(newIdx)*targetA.compSizes[b.id3])) = v3
	*(*T4)(unsafe.Pointer(uintptr(targetA.compPointers[b.id4]) + uintptr(newIdx)*targetA.compSizes[b.id4])) = v4
	*(*T5)(unsafe.Pointer(uintptr(targetA.compPointers[b.id5]) + uintptr(newIdx)*targetA.compSizes[b.id5])) = v5
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// SetBatch efficiently sets the component values for a slice of entities.
// It iterates over the entities and calls `Set` for each one.
//
// Parameters:
//   - entities: A slice of entities to modify.
//   - v1: The component value to set for type T1.
//   - v2: The component value to set for type T2.
//   - v3: The component value to set for type T3.
//   - v4: The component value to set for type T4.
//   - v5: The component value to set for type T5.
func (b *Builder5[T1, T2, T3, T4, T5]) SetBatch(entities []Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5) {
	for _, e := range entities {
		b.Set(e, v1, v2, v3, v4, v5)
	}
}

// Builder6 provides a highly efficient, type-safe API for creating entities
// with a predefined set of 6 components: T1, T2, T3, T4, T5, T6.
type Builder6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8
	id5   uint8
	id6   uint8
	
}

// NewBuilder6 creates a new `Builder` for entities with the 6 components
// T1, T2, T3, T4, T5, T6. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder6`.
func NewBuilder6[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any](w *World) *Builder6[T1, T2, T3, T4, T5, T6] {
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
		panic("ecs: duplicate component types in Builder6")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	mask.set(id4)
	mask.set(id5)
	mask.set(id6)
	
	w.components.mu.RLock()
	specs := []compSpec{
		{id: id1, typ: t1, size: w.components.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.components.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.components.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.components.compIDToSize[id4]},
		{id: id5, typ: t5, size: w.components.compIDToSize[id5]},
		{id: id6, typ: t6, size: w.components.compIDToSize[id6]},
		
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder6[T1, T2, T3, T4, T5, T6]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5, id6: id6}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder6`.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) New(w *World) *Builder6[T1, T2, T3, T4, T5, T6] {
	return NewBuilder6[T1, T2, T3, T4, T5, T6](w)
}

// NewEntity creates a single new entity with the 6 components defined by the
// builder: T1, T2, T3, T4, T5, T6. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 6 components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// NewEntitiesWithValueSet creates a batch of `count` entities and initializes
// their components to the provided values.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp1: The initial value for the component T1.
//   - comp2: The initial value for the component T2.
//   - comp3: The initial value for the component T3.
//   - comp4: The initial value for the component T4.
//   - comp5: The initial value for the component T5.
//   - comp6: The initial value for the component T6.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) NewEntitiesWithValueSet(count int, comp1 T1, comp2 T2, comp3 T3, comp4 T4, comp5 T5, comp6 T6) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])) = comp1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])) = comp2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])) = comp3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])) = comp4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(startSize+k)*a.compSizes[b.id5])) = comp5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(startSize+k)*a.compSizes[b.id6])) = comp6
		
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// Get retrieves pointers to the components for the given entity.
//
// If the entity is invalid or does not have all the components, this returns nils.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data, or nils if not found.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) Get(e Entity) (*T1, *T2, *T3, *T4, *T5, *T6) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := b.id1 >> 6
	o1 := b.id1 & 63
	i2 := b.id2 >> 6
	o2 := b.id2 & 63
	i3 := b.id3 >> 6
	o3 := b.id3 & 63
	i4 := b.id4 >> 6
	o4 := b.id4 & 63
	i5 := b.id5 >> 6
	o5 := b.id5 & 63
	i6 := b.id6 >> 6
	o6 := b.id6 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 {
		return nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[b.id1], uintptr(meta.index)*a.compSizes[b.id1])),
		(*T2)(unsafe.Add(a.compPointers[b.id2], uintptr(meta.index)*a.compSizes[b.id2])),
		(*T3)(unsafe.Add(a.compPointers[b.id3], uintptr(meta.index)*a.compSizes[b.id3])),
		(*T4)(unsafe.Add(a.compPointers[b.id4], uintptr(meta.index)*a.compSizes[b.id4])),
		(*T5)(unsafe.Add(a.compPointers[b.id5], uintptr(meta.index)*a.compSizes[b.id5])),
		(*T6)(unsafe.Add(a.compPointers[b.id6], uintptr(meta.index)*a.compSizes[b.id6]))
}

// Set adds or updates the components for a given entity with the specified
// values.
//
// If the entity already has all the components, their values are updated. If not,
// the missing components are added, which may trigger an archetype change.
//
// It is safe to call this on an invalid entity; the operation will be ignored.
//
// Parameters:
//   - e: The entity to modify.
//   - v1: The value for T1.
//   - v2: The value for T2.
//   - v3: The value for T3.
//   - v4: The value for T4.
//   - v5: The value for T5.
//   - v6: The value for T6.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) Set(e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := (a.mask[b.id1>>6] & (uint64(1) << uint64(b.id1&63))) != 0
	has2 := (a.mask[b.id2>>6] & (uint64(1) << uint64(b.id2&63))) != 0
	has3 := (a.mask[b.id3>>6] & (uint64(1) << uint64(b.id3&63))) != 0
	has4 := (a.mask[b.id4>>6] & (uint64(1) << uint64(b.id4&63))) != 0
	has5 := (a.mask[b.id5>>6] & (uint64(1) << uint64(b.id5&63))) != 0
	has6 := (a.mask[b.id6>>6] & (uint64(1) << uint64(b.id6&63))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 {
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])) = v1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])) = v2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(meta.index)*a.compSizes[b.id3])) = v3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(meta.index)*a.compSizes[b.id4])) = v4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(meta.index)*a.compSizes[b.id5])) = v5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(meta.index)*a.compSizes[b.id6])) = v6
		
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(b.id1)
	}
	if !has2 {
		newMask.set(b.id2)
	}
	if !has3 {
		newMask.set(b.id3)
	}
	if !has4 {
		newMask.set(b.id4)
	}
	if !has5 {
		newMask.set(b.id5)
	}
	if !has6 {
		newMask.set(b.id6)
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
			tempSpecs[count] = compSpec{id: b.id1, typ: w.components.compIDToType[b.id1], size: w.components.compIDToSize[b.id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: b.id2, typ: w.components.compIDToType[b.id2], size: w.components.compIDToSize[b.id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: b.id3, typ: w.components.compIDToType[b.id3], size: w.components.compIDToSize[b.id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: b.id4, typ: w.components.compIDToType[b.id4], size: w.components.compIDToSize[b.id4]}
			count++
		}
		if !has5 {
			tempSpecs[count] = compSpec{id: b.id5, typ: w.components.compIDToType[b.id5], size: w.components.compIDToSize[b.id5]}
			count++
		}
		if !has6 {
			tempSpecs[count] = compSpec{id: b.id6, typ: w.components.compIDToType[b.id6], size: w.components.compIDToSize[b.id6]}
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
	*(*T1)(unsafe.Pointer(uintptr(targetA.compPointers[b.id1]) + uintptr(newIdx)*targetA.compSizes[b.id1])) = v1
	*(*T2)(unsafe.Pointer(uintptr(targetA.compPointers[b.id2]) + uintptr(newIdx)*targetA.compSizes[b.id2])) = v2
	*(*T3)(unsafe.Pointer(uintptr(targetA.compPointers[b.id3]) + uintptr(newIdx)*targetA.compSizes[b.id3])) = v3
	*(*T4)(unsafe.Pointer(uintptr(targetA.compPointers[b.id4]) + uintptr(newIdx)*targetA.compSizes[b.id4])) = v4
	*(*T5)(unsafe.Pointer(uintptr(targetA.compPointers[b.id5]) + uintptr(newIdx)*targetA.compSizes[b.id5])) = v5
	*(*T6)(unsafe.Pointer(uintptr(targetA.compPointers[b.id6]) + uintptr(newIdx)*targetA.compSizes[b.id6])) = v6
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// SetBatch efficiently sets the component values for a slice of entities.
// It iterates over the entities and calls `Set` for each one.
//
// Parameters:
//   - entities: A slice of entities to modify.
//   - v1: The component value to set for type T1.
//   - v2: The component value to set for type T2.
//   - v3: The component value to set for type T3.
//   - v4: The component value to set for type T4.
//   - v5: The component value to set for type T5.
//   - v6: The component value to set for type T6.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) SetBatch(entities []Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6) {
	for _, e := range entities {
		b.Set(e, v1, v2, v3, v4, v5, v6)
	}
}

// Builder7 provides a highly efficient, type-safe API for creating entities
// with a predefined set of 7 components: T1, T2, T3, T4, T5, T6, T7.
type Builder7[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8
	id5   uint8
	id6   uint8
	id7   uint8
	
}

// NewBuilder7 creates a new `Builder` for entities with the 7 components
// T1, T2, T3, T4, T5, T6, T7. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder7`.
func NewBuilder7[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any](w *World) *Builder7[T1, T2, T3, T4, T5, T6, T7] {
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
		panic("ecs: duplicate component types in Builder7")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	mask.set(id4)
	mask.set(id5)
	mask.set(id6)
	mask.set(id7)
	
	w.components.mu.RLock()
	specs := []compSpec{
		{id: id1, typ: t1, size: w.components.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.components.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.components.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.components.compIDToSize[id4]},
		{id: id5, typ: t5, size: w.components.compIDToSize[id5]},
		{id: id6, typ: t6, size: w.components.compIDToSize[id6]},
		{id: id7, typ: t7, size: w.components.compIDToSize[id7]},
		
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder7[T1, T2, T3, T4, T5, T6, T7]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5, id6: id6, id7: id7}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder7`.
func (b *Builder7[T1, T2, T3, T4, T5, T6, T7]) New(w *World) *Builder7[T1, T2, T3, T4, T5, T6, T7] {
	return NewBuilder7[T1, T2, T3, T4, T5, T6, T7](w)
}

// NewEntity creates a single new entity with the 7 components defined by the
// builder: T1, T2, T3, T4, T5, T6, T7. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder7[T1, T2, T3, T4, T5, T6, T7]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 7 components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder7[T1, T2, T3, T4, T5, T6, T7]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// NewEntitiesWithValueSet creates a batch of `count` entities and initializes
// their components to the provided values.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp1: The initial value for the component T1.
//   - comp2: The initial value for the component T2.
//   - comp3: The initial value for the component T3.
//   - comp4: The initial value for the component T4.
//   - comp5: The initial value for the component T5.
//   - comp6: The initial value for the component T6.
//   - comp7: The initial value for the component T7.
func (b *Builder7[T1, T2, T3, T4, T5, T6, T7]) NewEntitiesWithValueSet(count int, comp1 T1, comp2 T2, comp3 T3, comp4 T4, comp5 T5, comp6 T6, comp7 T7) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])) = comp1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])) = comp2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])) = comp3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])) = comp4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(startSize+k)*a.compSizes[b.id5])) = comp5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(startSize+k)*a.compSizes[b.id6])) = comp6
		*(*T7)(unsafe.Pointer(uintptr(a.compPointers[b.id7]) + uintptr(startSize+k)*a.compSizes[b.id7])) = comp7
		
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// Get retrieves pointers to the components for the given entity.
//
// If the entity is invalid or does not have all the components, this returns nils.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data, or nils if not found.
func (b *Builder7[T1, T2, T3, T4, T5, T6, T7]) Get(e Entity) (*T1, *T2, *T3, *T4, *T5, *T6, *T7) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := b.id1 >> 6
	o1 := b.id1 & 63
	i2 := b.id2 >> 6
	o2 := b.id2 & 63
	i3 := b.id3 >> 6
	o3 := b.id3 & 63
	i4 := b.id4 >> 6
	o4 := b.id4 & 63
	i5 := b.id5 >> 6
	o5 := b.id5 & 63
	i6 := b.id6 >> 6
	o6 := b.id6 & 63
	i7 := b.id7 >> 6
	o7 := b.id7 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 || (a.mask[i7]&(uint64(1)<<uint64(o7))) == 0 {
		return nil, nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[b.id1], uintptr(meta.index)*a.compSizes[b.id1])),
		(*T2)(unsafe.Add(a.compPointers[b.id2], uintptr(meta.index)*a.compSizes[b.id2])),
		(*T3)(unsafe.Add(a.compPointers[b.id3], uintptr(meta.index)*a.compSizes[b.id3])),
		(*T4)(unsafe.Add(a.compPointers[b.id4], uintptr(meta.index)*a.compSizes[b.id4])),
		(*T5)(unsafe.Add(a.compPointers[b.id5], uintptr(meta.index)*a.compSizes[b.id5])),
		(*T6)(unsafe.Add(a.compPointers[b.id6], uintptr(meta.index)*a.compSizes[b.id6])),
		(*T7)(unsafe.Add(a.compPointers[b.id7], uintptr(meta.index)*a.compSizes[b.id7]))
}

// Set adds or updates the components for a given entity with the specified
// values.
//
// If the entity already has all the components, their values are updated. If not,
// the missing components are added, which may trigger an archetype change.
//
// It is safe to call this on an invalid entity; the operation will be ignored.
//
// Parameters:
//   - e: The entity to modify.
//   - v1: The value for T1.
//   - v2: The value for T2.
//   - v3: The value for T3.
//   - v4: The value for T4.
//   - v5: The value for T5.
//   - v6: The value for T6.
//   - v7: The value for T7.
func (b *Builder7[T1, T2, T3, T4, T5, T6, T7]) Set(e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := (a.mask[b.id1>>6] & (uint64(1) << uint64(b.id1&63))) != 0
	has2 := (a.mask[b.id2>>6] & (uint64(1) << uint64(b.id2&63))) != 0
	has3 := (a.mask[b.id3>>6] & (uint64(1) << uint64(b.id3&63))) != 0
	has4 := (a.mask[b.id4>>6] & (uint64(1) << uint64(b.id4&63))) != 0
	has5 := (a.mask[b.id5>>6] & (uint64(1) << uint64(b.id5&63))) != 0
	has6 := (a.mask[b.id6>>6] & (uint64(1) << uint64(b.id6&63))) != 0
	has7 := (a.mask[b.id7>>6] & (uint64(1) << uint64(b.id7&63))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 && has7 {
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])) = v1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])) = v2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(meta.index)*a.compSizes[b.id3])) = v3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(meta.index)*a.compSizes[b.id4])) = v4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(meta.index)*a.compSizes[b.id5])) = v5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(meta.index)*a.compSizes[b.id6])) = v6
		*(*T7)(unsafe.Pointer(uintptr(a.compPointers[b.id7]) + uintptr(meta.index)*a.compSizes[b.id7])) = v7
		
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(b.id1)
	}
	if !has2 {
		newMask.set(b.id2)
	}
	if !has3 {
		newMask.set(b.id3)
	}
	if !has4 {
		newMask.set(b.id4)
	}
	if !has5 {
		newMask.set(b.id5)
	}
	if !has6 {
		newMask.set(b.id6)
	}
	if !has7 {
		newMask.set(b.id7)
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
			tempSpecs[count] = compSpec{id: b.id1, typ: w.components.compIDToType[b.id1], size: w.components.compIDToSize[b.id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: b.id2, typ: w.components.compIDToType[b.id2], size: w.components.compIDToSize[b.id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: b.id3, typ: w.components.compIDToType[b.id3], size: w.components.compIDToSize[b.id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: b.id4, typ: w.components.compIDToType[b.id4], size: w.components.compIDToSize[b.id4]}
			count++
		}
		if !has5 {
			tempSpecs[count] = compSpec{id: b.id5, typ: w.components.compIDToType[b.id5], size: w.components.compIDToSize[b.id5]}
			count++
		}
		if !has6 {
			tempSpecs[count] = compSpec{id: b.id6, typ: w.components.compIDToType[b.id6], size: w.components.compIDToSize[b.id6]}
			count++
		}
		if !has7 {
			tempSpecs[count] = compSpec{id: b.id7, typ: w.components.compIDToType[b.id7], size: w.components.compIDToSize[b.id7]}
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
	*(*T1)(unsafe.Pointer(uintptr(targetA.compPointers[b.id1]) + uintptr(newIdx)*targetA.compSizes[b.id1])) = v1
	*(*T2)(unsafe.Pointer(uintptr(targetA.compPointers[b.id2]) + uintptr(newIdx)*targetA.compSizes[b.id2])) = v2
	*(*T3)(unsafe.Pointer(uintptr(targetA.compPointers[b.id3]) + uintptr(newIdx)*targetA.compSizes[b.id3])) = v3
	*(*T4)(unsafe.Pointer(uintptr(targetA.compPointers[b.id4]) + uintptr(newIdx)*targetA.compSizes[b.id4])) = v4
	*(*T5)(unsafe.Pointer(uintptr(targetA.compPointers[b.id5]) + uintptr(newIdx)*targetA.compSizes[b.id5])) = v5
	*(*T6)(unsafe.Pointer(uintptr(targetA.compPointers[b.id6]) + uintptr(newIdx)*targetA.compSizes[b.id6])) = v6
	*(*T7)(unsafe.Pointer(uintptr(targetA.compPointers[b.id7]) + uintptr(newIdx)*targetA.compSizes[b.id7])) = v7
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// SetBatch efficiently sets the component values for a slice of entities.
// It iterates over the entities and calls `Set` for each one.
//
// Parameters:
//   - entities: A slice of entities to modify.
//   - v1: The component value to set for type T1.
//   - v2: The component value to set for type T2.
//   - v3: The component value to set for type T3.
//   - v4: The component value to set for type T4.
//   - v5: The component value to set for type T5.
//   - v6: The component value to set for type T6.
//   - v7: The component value to set for type T7.
func (b *Builder7[T1, T2, T3, T4, T5, T6, T7]) SetBatch(entities []Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7) {
	for _, e := range entities {
		b.Set(e, v1, v2, v3, v4, v5, v6, v7)
	}
}

// Builder8 provides a highly efficient, type-safe API for creating entities
// with a predefined set of 8 components: T1, T2, T3, T4, T5, T6, T7, T8.
type Builder8[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8
	id5   uint8
	id6   uint8
	id7   uint8
	id8   uint8
	
}

// NewBuilder8 creates a new `Builder` for entities with the 8 components
// T1, T2, T3, T4, T5, T6, T7, T8. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder8`.
func NewBuilder8[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any](w *World) *Builder8[T1, T2, T3, T4, T5, T6, T7, T8] {
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
		panic("ecs: duplicate component types in Builder8")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	mask.set(id4)
	mask.set(id5)
	mask.set(id6)
	mask.set(id7)
	mask.set(id8)
	
	w.components.mu.RLock()
	specs := []compSpec{
		{id: id1, typ: t1, size: w.components.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.components.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.components.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.components.compIDToSize[id4]},
		{id: id5, typ: t5, size: w.components.compIDToSize[id5]},
		{id: id6, typ: t6, size: w.components.compIDToSize[id6]},
		{id: id7, typ: t7, size: w.components.compIDToSize[id7]},
		{id: id8, typ: t8, size: w.components.compIDToSize[id8]},
		
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder8[T1, T2, T3, T4, T5, T6, T7, T8]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5, id6: id6, id7: id7, id8: id8}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder8`.
func (b *Builder8[T1, T2, T3, T4, T5, T6, T7, T8]) New(w *World) *Builder8[T1, T2, T3, T4, T5, T6, T7, T8] {
	return NewBuilder8[T1, T2, T3, T4, T5, T6, T7, T8](w)
}

// NewEntity creates a single new entity with the 8 components defined by the
// builder: T1, T2, T3, T4, T5, T6, T7, T8. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder8[T1, T2, T3, T4, T5, T6, T7, T8]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 8 components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder8[T1, T2, T3, T4, T5, T6, T7, T8]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// NewEntitiesWithValueSet creates a batch of `count` entities and initializes
// their components to the provided values.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp1: The initial value for the component T1.
//   - comp2: The initial value for the component T2.
//   - comp3: The initial value for the component T3.
//   - comp4: The initial value for the component T4.
//   - comp5: The initial value for the component T5.
//   - comp6: The initial value for the component T6.
//   - comp7: The initial value for the component T7.
//   - comp8: The initial value for the component T8.
func (b *Builder8[T1, T2, T3, T4, T5, T6, T7, T8]) NewEntitiesWithValueSet(count int, comp1 T1, comp2 T2, comp3 T3, comp4 T4, comp5 T5, comp6 T6, comp7 T7, comp8 T8) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])) = comp1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])) = comp2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])) = comp3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])) = comp4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(startSize+k)*a.compSizes[b.id5])) = comp5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(startSize+k)*a.compSizes[b.id6])) = comp6
		*(*T7)(unsafe.Pointer(uintptr(a.compPointers[b.id7]) + uintptr(startSize+k)*a.compSizes[b.id7])) = comp7
		*(*T8)(unsafe.Pointer(uintptr(a.compPointers[b.id8]) + uintptr(startSize+k)*a.compSizes[b.id8])) = comp8
		
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// Get retrieves pointers to the components for the given entity.
//
// If the entity is invalid or does not have all the components, this returns nils.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data, or nils if not found.
func (b *Builder8[T1, T2, T3, T4, T5, T6, T7, T8]) Get(e Entity) (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := b.id1 >> 6
	o1 := b.id1 & 63
	i2 := b.id2 >> 6
	o2 := b.id2 & 63
	i3 := b.id3 >> 6
	o3 := b.id3 & 63
	i4 := b.id4 >> 6
	o4 := b.id4 & 63
	i5 := b.id5 >> 6
	o5 := b.id5 & 63
	i6 := b.id6 >> 6
	o6 := b.id6 & 63
	i7 := b.id7 >> 6
	o7 := b.id7 & 63
	i8 := b.id8 >> 6
	o8 := b.id8 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 || (a.mask[i7]&(uint64(1)<<uint64(o7))) == 0 || (a.mask[i8]&(uint64(1)<<uint64(o8))) == 0 {
		return nil, nil, nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[b.id1], uintptr(meta.index)*a.compSizes[b.id1])),
		(*T2)(unsafe.Add(a.compPointers[b.id2], uintptr(meta.index)*a.compSizes[b.id2])),
		(*T3)(unsafe.Add(a.compPointers[b.id3], uintptr(meta.index)*a.compSizes[b.id3])),
		(*T4)(unsafe.Add(a.compPointers[b.id4], uintptr(meta.index)*a.compSizes[b.id4])),
		(*T5)(unsafe.Add(a.compPointers[b.id5], uintptr(meta.index)*a.compSizes[b.id5])),
		(*T6)(unsafe.Add(a.compPointers[b.id6], uintptr(meta.index)*a.compSizes[b.id6])),
		(*T7)(unsafe.Add(a.compPointers[b.id7], uintptr(meta.index)*a.compSizes[b.id7])),
		(*T8)(unsafe.Add(a.compPointers[b.id8], uintptr(meta.index)*a.compSizes[b.id8]))
}

// Set adds or updates the components for a given entity with the specified
// values.
//
// If the entity already has all the components, their values are updated. If not,
// the missing components are added, which may trigger an archetype change.
//
// It is safe to call this on an invalid entity; the operation will be ignored.
//
// Parameters:
//   - e: The entity to modify.
//   - v1: The value for T1.
//   - v2: The value for T2.
//   - v3: The value for T3.
//   - v4: The value for T4.
//   - v5: The value for T5.
//   - v6: The value for T6.
//   - v7: The value for T7.
//   - v8: The value for T8.
func (b *Builder8[T1, T2, T3, T4, T5, T6, T7, T8]) Set(e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := (a.mask[b.id1>>6] & (uint64(1) << uint64(b.id1&63))) != 0
	has2 := (a.mask[b.id2>>6] & (uint64(1) << uint64(b.id2&63))) != 0
	has3 := (a.mask[b.id3>>6] & (uint64(1) << uint64(b.id3&63))) != 0
	has4 := (a.mask[b.id4>>6] & (uint64(1) << uint64(b.id4&63))) != 0
	has5 := (a.mask[b.id5>>6] & (uint64(1) << uint64(b.id5&63))) != 0
	has6 := (a.mask[b.id6>>6] & (uint64(1) << uint64(b.id6&63))) != 0
	has7 := (a.mask[b.id7>>6] & (uint64(1) << uint64(b.id7&63))) != 0
	has8 := (a.mask[b.id8>>6] & (uint64(1) << uint64(b.id8&63))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 && has7 && has8 {
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])) = v1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])) = v2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(meta.index)*a.compSizes[b.id3])) = v3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(meta.index)*a.compSizes[b.id4])) = v4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(meta.index)*a.compSizes[b.id5])) = v5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(meta.index)*a.compSizes[b.id6])) = v6
		*(*T7)(unsafe.Pointer(uintptr(a.compPointers[b.id7]) + uintptr(meta.index)*a.compSizes[b.id7])) = v7
		*(*T8)(unsafe.Pointer(uintptr(a.compPointers[b.id8]) + uintptr(meta.index)*a.compSizes[b.id8])) = v8
		
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(b.id1)
	}
	if !has2 {
		newMask.set(b.id2)
	}
	if !has3 {
		newMask.set(b.id3)
	}
	if !has4 {
		newMask.set(b.id4)
	}
	if !has5 {
		newMask.set(b.id5)
	}
	if !has6 {
		newMask.set(b.id6)
	}
	if !has7 {
		newMask.set(b.id7)
	}
	if !has8 {
		newMask.set(b.id8)
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
			tempSpecs[count] = compSpec{id: b.id1, typ: w.components.compIDToType[b.id1], size: w.components.compIDToSize[b.id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: b.id2, typ: w.components.compIDToType[b.id2], size: w.components.compIDToSize[b.id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: b.id3, typ: w.components.compIDToType[b.id3], size: w.components.compIDToSize[b.id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: b.id4, typ: w.components.compIDToType[b.id4], size: w.components.compIDToSize[b.id4]}
			count++
		}
		if !has5 {
			tempSpecs[count] = compSpec{id: b.id5, typ: w.components.compIDToType[b.id5], size: w.components.compIDToSize[b.id5]}
			count++
		}
		if !has6 {
			tempSpecs[count] = compSpec{id: b.id6, typ: w.components.compIDToType[b.id6], size: w.components.compIDToSize[b.id6]}
			count++
		}
		if !has7 {
			tempSpecs[count] = compSpec{id: b.id7, typ: w.components.compIDToType[b.id7], size: w.components.compIDToSize[b.id7]}
			count++
		}
		if !has8 {
			tempSpecs[count] = compSpec{id: b.id8, typ: w.components.compIDToType[b.id8], size: w.components.compIDToSize[b.id8]}
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
	*(*T1)(unsafe.Pointer(uintptr(targetA.compPointers[b.id1]) + uintptr(newIdx)*targetA.compSizes[b.id1])) = v1
	*(*T2)(unsafe.Pointer(uintptr(targetA.compPointers[b.id2]) + uintptr(newIdx)*targetA.compSizes[b.id2])) = v2
	*(*T3)(unsafe.Pointer(uintptr(targetA.compPointers[b.id3]) + uintptr(newIdx)*targetA.compSizes[b.id3])) = v3
	*(*T4)(unsafe.Pointer(uintptr(targetA.compPointers[b.id4]) + uintptr(newIdx)*targetA.compSizes[b.id4])) = v4
	*(*T5)(unsafe.Pointer(uintptr(targetA.compPointers[b.id5]) + uintptr(newIdx)*targetA.compSizes[b.id5])) = v5
	*(*T6)(unsafe.Pointer(uintptr(targetA.compPointers[b.id6]) + uintptr(newIdx)*targetA.compSizes[b.id6])) = v6
	*(*T7)(unsafe.Pointer(uintptr(targetA.compPointers[b.id7]) + uintptr(newIdx)*targetA.compSizes[b.id7])) = v7
	*(*T8)(unsafe.Pointer(uintptr(targetA.compPointers[b.id8]) + uintptr(newIdx)*targetA.compSizes[b.id8])) = v8
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// SetBatch efficiently sets the component values for a slice of entities.
// It iterates over the entities and calls `Set` for each one.
//
// Parameters:
//   - entities: A slice of entities to modify.
//   - v1: The component value to set for type T1.
//   - v2: The component value to set for type T2.
//   - v3: The component value to set for type T3.
//   - v4: The component value to set for type T4.
//   - v5: The component value to set for type T5.
//   - v6: The component value to set for type T6.
//   - v7: The component value to set for type T7.
//   - v8: The component value to set for type T8.
func (b *Builder8[T1, T2, T3, T4, T5, T6, T7, T8]) SetBatch(entities []Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8) {
	for _, e := range entities {
		b.Set(e, v1, v2, v3, v4, v5, v6, v7, v8)
	}
}

// Builder9 provides a highly efficient, type-safe API for creating entities
// with a predefined set of 9 components: T1, T2, T3, T4, T5, T6, T7, T8, T9.
type Builder9[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8
	id5   uint8
	id6   uint8
	id7   uint8
	id8   uint8
	id9   uint8
	
}

// NewBuilder9 creates a new `Builder` for entities with the 9 components
// T1, T2, T3, T4, T5, T6, T7, T8, T9. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder9`.
func NewBuilder9[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any](w *World) *Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9] {
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
		panic("ecs: duplicate component types in Builder9")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	mask.set(id4)
	mask.set(id5)
	mask.set(id6)
	mask.set(id7)
	mask.set(id8)
	mask.set(id9)
	
	w.components.mu.RLock()
	specs := []compSpec{
		{id: id1, typ: t1, size: w.components.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.components.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.components.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.components.compIDToSize[id4]},
		{id: id5, typ: t5, size: w.components.compIDToSize[id5]},
		{id: id6, typ: t6, size: w.components.compIDToSize[id6]},
		{id: id7, typ: t7, size: w.components.compIDToSize[id7]},
		{id: id8, typ: t8, size: w.components.compIDToSize[id8]},
		{id: id9, typ: t9, size: w.components.compIDToSize[id9]},
		
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5, id6: id6, id7: id7, id8: id8, id9: id9}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder9`.
func (b *Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) New(w *World) *Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9] {
	return NewBuilder9[T1, T2, T3, T4, T5, T6, T7, T8, T9](w)
}

// NewEntity creates a single new entity with the 9 components defined by the
// builder: T1, T2, T3, T4, T5, T6, T7, T8, T9. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 9 components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// NewEntitiesWithValueSet creates a batch of `count` entities and initializes
// their components to the provided values.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp1: The initial value for the component T1.
//   - comp2: The initial value for the component T2.
//   - comp3: The initial value for the component T3.
//   - comp4: The initial value for the component T4.
//   - comp5: The initial value for the component T5.
//   - comp6: The initial value for the component T6.
//   - comp7: The initial value for the component T7.
//   - comp8: The initial value for the component T8.
//   - comp9: The initial value for the component T9.
func (b *Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) NewEntitiesWithValueSet(count int, comp1 T1, comp2 T2, comp3 T3, comp4 T4, comp5 T5, comp6 T6, comp7 T7, comp8 T8, comp9 T9) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])) = comp1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])) = comp2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])) = comp3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])) = comp4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(startSize+k)*a.compSizes[b.id5])) = comp5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(startSize+k)*a.compSizes[b.id6])) = comp6
		*(*T7)(unsafe.Pointer(uintptr(a.compPointers[b.id7]) + uintptr(startSize+k)*a.compSizes[b.id7])) = comp7
		*(*T8)(unsafe.Pointer(uintptr(a.compPointers[b.id8]) + uintptr(startSize+k)*a.compSizes[b.id8])) = comp8
		*(*T9)(unsafe.Pointer(uintptr(a.compPointers[b.id9]) + uintptr(startSize+k)*a.compSizes[b.id9])) = comp9
		
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// Get retrieves pointers to the components for the given entity.
//
// If the entity is invalid or does not have all the components, this returns nils.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data, or nils if not found.
func (b *Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Get(e Entity) (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := b.id1 >> 6
	o1 := b.id1 & 63
	i2 := b.id2 >> 6
	o2 := b.id2 & 63
	i3 := b.id3 >> 6
	o3 := b.id3 & 63
	i4 := b.id4 >> 6
	o4 := b.id4 & 63
	i5 := b.id5 >> 6
	o5 := b.id5 & 63
	i6 := b.id6 >> 6
	o6 := b.id6 & 63
	i7 := b.id7 >> 6
	o7 := b.id7 & 63
	i8 := b.id8 >> 6
	o8 := b.id8 & 63
	i9 := b.id9 >> 6
	o9 := b.id9 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 || (a.mask[i7]&(uint64(1)<<uint64(o7))) == 0 || (a.mask[i8]&(uint64(1)<<uint64(o8))) == 0 || (a.mask[i9]&(uint64(1)<<uint64(o9))) == 0 {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[b.id1], uintptr(meta.index)*a.compSizes[b.id1])),
		(*T2)(unsafe.Add(a.compPointers[b.id2], uintptr(meta.index)*a.compSizes[b.id2])),
		(*T3)(unsafe.Add(a.compPointers[b.id3], uintptr(meta.index)*a.compSizes[b.id3])),
		(*T4)(unsafe.Add(a.compPointers[b.id4], uintptr(meta.index)*a.compSizes[b.id4])),
		(*T5)(unsafe.Add(a.compPointers[b.id5], uintptr(meta.index)*a.compSizes[b.id5])),
		(*T6)(unsafe.Add(a.compPointers[b.id6], uintptr(meta.index)*a.compSizes[b.id6])),
		(*T7)(unsafe.Add(a.compPointers[b.id7], uintptr(meta.index)*a.compSizes[b.id7])),
		(*T8)(unsafe.Add(a.compPointers[b.id8], uintptr(meta.index)*a.compSizes[b.id8])),
		(*T9)(unsafe.Add(a.compPointers[b.id9], uintptr(meta.index)*a.compSizes[b.id9]))
}

// Set adds or updates the components for a given entity with the specified
// values.
//
// If the entity already has all the components, their values are updated. If not,
// the missing components are added, which may trigger an archetype change.
//
// It is safe to call this on an invalid entity; the operation will be ignored.
//
// Parameters:
//   - e: The entity to modify.
//   - v1: The value for T1.
//   - v2: The value for T2.
//   - v3: The value for T3.
//   - v4: The value for T4.
//   - v5: The value for T5.
//   - v6: The value for T6.
//   - v7: The value for T7.
//   - v8: The value for T8.
//   - v9: The value for T9.
func (b *Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) Set(e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8, v9 T9) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := (a.mask[b.id1>>6] & (uint64(1) << uint64(b.id1&63))) != 0
	has2 := (a.mask[b.id2>>6] & (uint64(1) << uint64(b.id2&63))) != 0
	has3 := (a.mask[b.id3>>6] & (uint64(1) << uint64(b.id3&63))) != 0
	has4 := (a.mask[b.id4>>6] & (uint64(1) << uint64(b.id4&63))) != 0
	has5 := (a.mask[b.id5>>6] & (uint64(1) << uint64(b.id5&63))) != 0
	has6 := (a.mask[b.id6>>6] & (uint64(1) << uint64(b.id6&63))) != 0
	has7 := (a.mask[b.id7>>6] & (uint64(1) << uint64(b.id7&63))) != 0
	has8 := (a.mask[b.id8>>6] & (uint64(1) << uint64(b.id8&63))) != 0
	has9 := (a.mask[b.id9>>6] & (uint64(1) << uint64(b.id9&63))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 && has7 && has8 && has9 {
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])) = v1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])) = v2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(meta.index)*a.compSizes[b.id3])) = v3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(meta.index)*a.compSizes[b.id4])) = v4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(meta.index)*a.compSizes[b.id5])) = v5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(meta.index)*a.compSizes[b.id6])) = v6
		*(*T7)(unsafe.Pointer(uintptr(a.compPointers[b.id7]) + uintptr(meta.index)*a.compSizes[b.id7])) = v7
		*(*T8)(unsafe.Pointer(uintptr(a.compPointers[b.id8]) + uintptr(meta.index)*a.compSizes[b.id8])) = v8
		*(*T9)(unsafe.Pointer(uintptr(a.compPointers[b.id9]) + uintptr(meta.index)*a.compSizes[b.id9])) = v9
		
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(b.id1)
	}
	if !has2 {
		newMask.set(b.id2)
	}
	if !has3 {
		newMask.set(b.id3)
	}
	if !has4 {
		newMask.set(b.id4)
	}
	if !has5 {
		newMask.set(b.id5)
	}
	if !has6 {
		newMask.set(b.id6)
	}
	if !has7 {
		newMask.set(b.id7)
	}
	if !has8 {
		newMask.set(b.id8)
	}
	if !has9 {
		newMask.set(b.id9)
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
			tempSpecs[count] = compSpec{id: b.id1, typ: w.components.compIDToType[b.id1], size: w.components.compIDToSize[b.id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: b.id2, typ: w.components.compIDToType[b.id2], size: w.components.compIDToSize[b.id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: b.id3, typ: w.components.compIDToType[b.id3], size: w.components.compIDToSize[b.id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: b.id4, typ: w.components.compIDToType[b.id4], size: w.components.compIDToSize[b.id4]}
			count++
		}
		if !has5 {
			tempSpecs[count] = compSpec{id: b.id5, typ: w.components.compIDToType[b.id5], size: w.components.compIDToSize[b.id5]}
			count++
		}
		if !has6 {
			tempSpecs[count] = compSpec{id: b.id6, typ: w.components.compIDToType[b.id6], size: w.components.compIDToSize[b.id6]}
			count++
		}
		if !has7 {
			tempSpecs[count] = compSpec{id: b.id7, typ: w.components.compIDToType[b.id7], size: w.components.compIDToSize[b.id7]}
			count++
		}
		if !has8 {
			tempSpecs[count] = compSpec{id: b.id8, typ: w.components.compIDToType[b.id8], size: w.components.compIDToSize[b.id8]}
			count++
		}
		if !has9 {
			tempSpecs[count] = compSpec{id: b.id9, typ: w.components.compIDToType[b.id9], size: w.components.compIDToSize[b.id9]}
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
	*(*T1)(unsafe.Pointer(uintptr(targetA.compPointers[b.id1]) + uintptr(newIdx)*targetA.compSizes[b.id1])) = v1
	*(*T2)(unsafe.Pointer(uintptr(targetA.compPointers[b.id2]) + uintptr(newIdx)*targetA.compSizes[b.id2])) = v2
	*(*T3)(unsafe.Pointer(uintptr(targetA.compPointers[b.id3]) + uintptr(newIdx)*targetA.compSizes[b.id3])) = v3
	*(*T4)(unsafe.Pointer(uintptr(targetA.compPointers[b.id4]) + uintptr(newIdx)*targetA.compSizes[b.id4])) = v4
	*(*T5)(unsafe.Pointer(uintptr(targetA.compPointers[b.id5]) + uintptr(newIdx)*targetA.compSizes[b.id5])) = v5
	*(*T6)(unsafe.Pointer(uintptr(targetA.compPointers[b.id6]) + uintptr(newIdx)*targetA.compSizes[b.id6])) = v6
	*(*T7)(unsafe.Pointer(uintptr(targetA.compPointers[b.id7]) + uintptr(newIdx)*targetA.compSizes[b.id7])) = v7
	*(*T8)(unsafe.Pointer(uintptr(targetA.compPointers[b.id8]) + uintptr(newIdx)*targetA.compSizes[b.id8])) = v8
	*(*T9)(unsafe.Pointer(uintptr(targetA.compPointers[b.id9]) + uintptr(newIdx)*targetA.compSizes[b.id9])) = v9
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// SetBatch efficiently sets the component values for a slice of entities.
// It iterates over the entities and calls `Set` for each one.
//
// Parameters:
//   - entities: A slice of entities to modify.
//   - v1: The component value to set for type T1.
//   - v2: The component value to set for type T2.
//   - v3: The component value to set for type T3.
//   - v4: The component value to set for type T4.
//   - v5: The component value to set for type T5.
//   - v6: The component value to set for type T6.
//   - v7: The component value to set for type T7.
//   - v8: The component value to set for type T8.
//   - v9: The component value to set for type T9.
func (b *Builder9[T1, T2, T3, T4, T5, T6, T7, T8, T9]) SetBatch(entities []Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8, v9 T9) {
	for _, e := range entities {
		b.Set(e, v1, v2, v3, v4, v5, v6, v7, v8, v9)
	}
}

// Builder10 provides a highly efficient, type-safe API for creating entities
// with a predefined set of 10 components: T1, T2, T3, T4, T5, T6, T7, T8, T9, T10.
type Builder10[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any, T10 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
	id3   uint8
	id4   uint8
	id5   uint8
	id6   uint8
	id7   uint8
	id8   uint8
	id9   uint8
	id10   uint8
	
}

// NewBuilder10 creates a new `Builder` for entities with the 10 components
// T1, T2, T3, T4, T5, T6, T7, T8, T9, T10. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder10`.
func NewBuilder10[T1 any, T2 any, T3 any, T4 any, T5 any, T6 any, T7 any, T8 any, T9 any, T10 any](w *World) *Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10] {
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
		panic("ecs: duplicate component types in Builder10")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	mask.set(id4)
	mask.set(id5)
	mask.set(id6)
	mask.set(id7)
	mask.set(id8)
	mask.set(id9)
	mask.set(id10)
	
	w.components.mu.RLock()
	specs := []compSpec{
		{id: id1, typ: t1, size: w.components.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.components.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.components.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.components.compIDToSize[id4]},
		{id: id5, typ: t5, size: w.components.compIDToSize[id5]},
		{id: id6, typ: t6, size: w.components.compIDToSize[id6]},
		{id: id7, typ: t7, size: w.components.compIDToSize[id7]},
		{id: id8, typ: t8, size: w.components.compIDToSize[id8]},
		{id: id9, typ: t9, size: w.components.compIDToSize[id9]},
		{id: id10, typ: t10, size: w.components.compIDToSize[id10]},
		
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5, id6: id6, id7: id7, id8: id8, id9: id9, id10: id10}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder10`.
func (b *Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) New(w *World) *Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10] {
	return NewBuilder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10](w)
}

// NewEntity creates a single new entity with the 10 components defined by the
// builder: T1, T2, T3, T4, T5, T6, T7, T8, T9, T10. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 10 components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// NewEntitiesWithValueSet creates a batch of `count` entities and initializes
// their components to the provided values.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp1: The initial value for the component T1.
//   - comp2: The initial value for the component T2.
//   - comp3: The initial value for the component T3.
//   - comp4: The initial value for the component T4.
//   - comp5: The initial value for the component T5.
//   - comp6: The initial value for the component T6.
//   - comp7: The initial value for the component T7.
//   - comp8: The initial value for the component T8.
//   - comp9: The initial value for the component T9.
//   - comp10: The initial value for the component T10.
func (b *Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) NewEntitiesWithValueSet(count int, comp1 T1, comp2 T2, comp3 T3, comp4 T4, comp5 T5, comp6 T6, comp7 T7, comp8 T8, comp9 T9, comp10 T10) {
	if count == 0 {
		return
	}
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	a := b.arch
	for len(w.entities.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.entities.freeIDs[len(w.entities.freeIDs)-count:]
	w.entities.freeIDs = w.entities.freeIDs[:len(w.entities.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.entities.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.entities.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])) = comp1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])) = comp2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])) = comp3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])) = comp4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(startSize+k)*a.compSizes[b.id5])) = comp5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(startSize+k)*a.compSizes[b.id6])) = comp6
		*(*T7)(unsafe.Pointer(uintptr(a.compPointers[b.id7]) + uintptr(startSize+k)*a.compSizes[b.id7])) = comp7
		*(*T8)(unsafe.Pointer(uintptr(a.compPointers[b.id8]) + uintptr(startSize+k)*a.compSizes[b.id8])) = comp8
		*(*T9)(unsafe.Pointer(uintptr(a.compPointers[b.id9]) + uintptr(startSize+k)*a.compSizes[b.id9])) = comp9
		*(*T10)(unsafe.Pointer(uintptr(a.compPointers[b.id10]) + uintptr(startSize+k)*a.compSizes[b.id10])) = comp10
		
		w.entities.nextEntityVer++
	}
	w.mutationVersion.Add(1)
}

// Get retrieves pointers to the components for the given entity.
//
// If the entity is invalid or does not have all the components, this returns nils.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data, or nils if not found.
func (b *Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Get(e Entity) (*T1, *T2, *T3, *T4, *T5, *T6, *T7, *T8, *T9, *T10) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	i1 := b.id1 >> 6
	o1 := b.id1 & 63
	i2 := b.id2 >> 6
	o2 := b.id2 & 63
	i3 := b.id3 >> 6
	o3 := b.id3 & 63
	i4 := b.id4 >> 6
	o4 := b.id4 & 63
	i5 := b.id5 >> 6
	o5 := b.id5 & 63
	i6 := b.id6 >> 6
	o6 := b.id6 & 63
	i7 := b.id7 >> 6
	o7 := b.id7 & 63
	i8 := b.id8 >> 6
	o8 := b.id8 & 63
	i9 := b.id9 >> 6
	o9 := b.id9 & 63
	i10 := b.id10 >> 6
	o10 := b.id10 & 63
	
	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 || (a.mask[i7]&(uint64(1)<<uint64(o7))) == 0 || (a.mask[i8]&(uint64(1)<<uint64(o8))) == 0 || (a.mask[i9]&(uint64(1)<<uint64(o9))) == 0 || (a.mask[i10]&(uint64(1)<<uint64(o10))) == 0 {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil
	}
	return (*T1)(unsafe.Add(a.compPointers[b.id1], uintptr(meta.index)*a.compSizes[b.id1])),
		(*T2)(unsafe.Add(a.compPointers[b.id2], uintptr(meta.index)*a.compSizes[b.id2])),
		(*T3)(unsafe.Add(a.compPointers[b.id3], uintptr(meta.index)*a.compSizes[b.id3])),
		(*T4)(unsafe.Add(a.compPointers[b.id4], uintptr(meta.index)*a.compSizes[b.id4])),
		(*T5)(unsafe.Add(a.compPointers[b.id5], uintptr(meta.index)*a.compSizes[b.id5])),
		(*T6)(unsafe.Add(a.compPointers[b.id6], uintptr(meta.index)*a.compSizes[b.id6])),
		(*T7)(unsafe.Add(a.compPointers[b.id7], uintptr(meta.index)*a.compSizes[b.id7])),
		(*T8)(unsafe.Add(a.compPointers[b.id8], uintptr(meta.index)*a.compSizes[b.id8])),
		(*T9)(unsafe.Add(a.compPointers[b.id9], uintptr(meta.index)*a.compSizes[b.id9])),
		(*T10)(unsafe.Add(a.compPointers[b.id10], uintptr(meta.index)*a.compSizes[b.id10]))
}

// Set adds or updates the components for a given entity with the specified
// values.
//
// If the entity already has all the components, their values are updated. If not,
// the missing components are added, which may trigger an archetype change.
//
// It is safe to call this on an invalid entity; the operation will be ignored.
//
// Parameters:
//   - e: The entity to modify.
//   - v1: The value for T1.
//   - v2: The value for T2.
//   - v3: The value for T3.
//   - v4: The value for T4.
//   - v5: The value for T5.
//   - v6: The value for T6.
//   - v7: The value for T7.
//   - v8: The value for T8.
//   - v9: The value for T9.
//   - v10: The value for T10.
func (b *Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) Set(e Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8, v9 T9, v10 T10) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	has1 := (a.mask[b.id1>>6] & (uint64(1) << uint64(b.id1&63))) != 0
	has2 := (a.mask[b.id2>>6] & (uint64(1) << uint64(b.id2&63))) != 0
	has3 := (a.mask[b.id3>>6] & (uint64(1) << uint64(b.id3&63))) != 0
	has4 := (a.mask[b.id4>>6] & (uint64(1) << uint64(b.id4&63))) != 0
	has5 := (a.mask[b.id5>>6] & (uint64(1) << uint64(b.id5&63))) != 0
	has6 := (a.mask[b.id6>>6] & (uint64(1) << uint64(b.id6&63))) != 0
	has7 := (a.mask[b.id7>>6] & (uint64(1) << uint64(b.id7&63))) != 0
	has8 := (a.mask[b.id8>>6] & (uint64(1) << uint64(b.id8&63))) != 0
	has9 := (a.mask[b.id9>>6] & (uint64(1) << uint64(b.id9&63))) != 0
	has10 := (a.mask[b.id10>>6] & (uint64(1) << uint64(b.id10&63))) != 0
	
	if has1 && has2 && has3 && has4 && has5 && has6 && has7 && has8 && has9 && has10 {
		*(*T1)(unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])) = v1
		*(*T2)(unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])) = v2
		*(*T3)(unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(meta.index)*a.compSizes[b.id3])) = v3
		*(*T4)(unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(meta.index)*a.compSizes[b.id4])) = v4
		*(*T5)(unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(meta.index)*a.compSizes[b.id5])) = v5
		*(*T6)(unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(meta.index)*a.compSizes[b.id6])) = v6
		*(*T7)(unsafe.Pointer(uintptr(a.compPointers[b.id7]) + uintptr(meta.index)*a.compSizes[b.id7])) = v7
		*(*T8)(unsafe.Pointer(uintptr(a.compPointers[b.id8]) + uintptr(meta.index)*a.compSizes[b.id8])) = v8
		*(*T9)(unsafe.Pointer(uintptr(a.compPointers[b.id9]) + uintptr(meta.index)*a.compSizes[b.id9])) = v9
		*(*T10)(unsafe.Pointer(uintptr(a.compPointers[b.id10]) + uintptr(meta.index)*a.compSizes[b.id10])) = v10
		
		return
	}
	newMask := a.mask
	if !has1 {
		newMask.set(b.id1)
	}
	if !has2 {
		newMask.set(b.id2)
	}
	if !has3 {
		newMask.set(b.id3)
	}
	if !has4 {
		newMask.set(b.id4)
	}
	if !has5 {
		newMask.set(b.id5)
	}
	if !has6 {
		newMask.set(b.id6)
	}
	if !has7 {
		newMask.set(b.id7)
	}
	if !has8 {
		newMask.set(b.id8)
	}
	if !has9 {
		newMask.set(b.id9)
	}
	if !has10 {
		newMask.set(b.id10)
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
			tempSpecs[count] = compSpec{id: b.id1, typ: w.components.compIDToType[b.id1], size: w.components.compIDToSize[b.id1]}
			count++
		}
		if !has2 {
			tempSpecs[count] = compSpec{id: b.id2, typ: w.components.compIDToType[b.id2], size: w.components.compIDToSize[b.id2]}
			count++
		}
		if !has3 {
			tempSpecs[count] = compSpec{id: b.id3, typ: w.components.compIDToType[b.id3], size: w.components.compIDToSize[b.id3]}
			count++
		}
		if !has4 {
			tempSpecs[count] = compSpec{id: b.id4, typ: w.components.compIDToType[b.id4], size: w.components.compIDToSize[b.id4]}
			count++
		}
		if !has5 {
			tempSpecs[count] = compSpec{id: b.id5, typ: w.components.compIDToType[b.id5], size: w.components.compIDToSize[b.id5]}
			count++
		}
		if !has6 {
			tempSpecs[count] = compSpec{id: b.id6, typ: w.components.compIDToType[b.id6], size: w.components.compIDToSize[b.id6]}
			count++
		}
		if !has7 {
			tempSpecs[count] = compSpec{id: b.id7, typ: w.components.compIDToType[b.id7], size: w.components.compIDToSize[b.id7]}
			count++
		}
		if !has8 {
			tempSpecs[count] = compSpec{id: b.id8, typ: w.components.compIDToType[b.id8], size: w.components.compIDToSize[b.id8]}
			count++
		}
		if !has9 {
			tempSpecs[count] = compSpec{id: b.id9, typ: w.components.compIDToType[b.id9], size: w.components.compIDToSize[b.id9]}
			count++
		}
		if !has10 {
			tempSpecs[count] = compSpec{id: b.id10, typ: w.components.compIDToType[b.id10], size: w.components.compIDToSize[b.id10]}
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
	*(*T1)(unsafe.Pointer(uintptr(targetA.compPointers[b.id1]) + uintptr(newIdx)*targetA.compSizes[b.id1])) = v1
	*(*T2)(unsafe.Pointer(uintptr(targetA.compPointers[b.id2]) + uintptr(newIdx)*targetA.compSizes[b.id2])) = v2
	*(*T3)(unsafe.Pointer(uintptr(targetA.compPointers[b.id3]) + uintptr(newIdx)*targetA.compSizes[b.id3])) = v3
	*(*T4)(unsafe.Pointer(uintptr(targetA.compPointers[b.id4]) + uintptr(newIdx)*targetA.compSizes[b.id4])) = v4
	*(*T5)(unsafe.Pointer(uintptr(targetA.compPointers[b.id5]) + uintptr(newIdx)*targetA.compSizes[b.id5])) = v5
	*(*T6)(unsafe.Pointer(uintptr(targetA.compPointers[b.id6]) + uintptr(newIdx)*targetA.compSizes[b.id6])) = v6
	*(*T7)(unsafe.Pointer(uintptr(targetA.compPointers[b.id7]) + uintptr(newIdx)*targetA.compSizes[b.id7])) = v7
	*(*T8)(unsafe.Pointer(uintptr(targetA.compPointers[b.id8]) + uintptr(newIdx)*targetA.compSizes[b.id8])) = v8
	*(*T9)(unsafe.Pointer(uintptr(targetA.compPointers[b.id9]) + uintptr(newIdx)*targetA.compSizes[b.id9])) = v9
	*(*T10)(unsafe.Pointer(uintptr(targetA.compPointers[b.id10]) + uintptr(newIdx)*targetA.compSizes[b.id10])) = v10
	
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
	w.mutationVersion.Add(1)
}

// SetBatch efficiently sets the component values for a slice of entities.
// It iterates over the entities and calls `Set` for each one.
//
// Parameters:
//   - entities: A slice of entities to modify.
//   - v1: The component value to set for type T1.
//   - v2: The component value to set for type T2.
//   - v3: The component value to set for type T3.
//   - v4: The component value to set for type T4.
//   - v5: The component value to set for type T5.
//   - v6: The component value to set for type T6.
//   - v7: The component value to set for type T7.
//   - v8: The component value to set for type T8.
//   - v9: The component value to set for type T9.
//   - v10: The component value to set for type T10.
func (b *Builder10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10]) SetBatch(entities []Entity, v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, v6 T6, v7 T7, v8 T8, v9 T9, v10 T10) {
	for _, e := range entities {
		b.Set(e, v1, v2, v3, v4, v5, v6, v7, v8, v9, v10)
	}
}

