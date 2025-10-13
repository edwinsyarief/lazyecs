package lazyecs

import (
	"reflect"
	"unsafe"
)

// This template generates the code for N-ary Builders (Builder2, Builder3, etc.).
// A Builder is a highly optimized factory for creating entities with a fixed set
// of components. By pre-calculating the archetype, it makes entity creation an
// extremely fast, allocation-free operation.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types, e.g., "id1 == id2".
// - .Components: A slice of ComponentInfo structs, used for loops.
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

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)

	if id2 == id1 {
		panic("ecs: duplicate component types in Builder2")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)

	specs := []compSpec{
		{id: id1, typ: t1, size: w.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.compIDToSize[id2]},
	}
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder2[T1, T2]{world: w, arch: arch, id1: id1, id2: id2}
}

// New is a convenience function that creates a new builder instance.
func (b *Builder2[T1, T2]) New(w *World) *Builder2[T1, T2] {
	return NewBuilder2[T1, T2](w)
}

// NewEntity creates a single new entity with the 2 components defined by the
// builder: T1, T2.
//
// Returns:
//   - The newly created Entity.
func (b *Builder2[T1, T2]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 2 components
// defined by the builder. This is the most performant method for creating many
// entities at once.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder2[T1, T2]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.nextEntityVer++
	}
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
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])
		*(*T1)(ptr1) = comp1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])
		*(*T2)(ptr2) = comp2

		w.nextEntityVer++
	}
}

// Get retrieves pointers to the 2 components (T1, T2) for the
// given entity.
//
// If the entity is invalid or does not have all the required components, this
// returns nil for all pointers.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data (*T1, *T2), or nils if not found.
func (b *Builder2[T1, T2]) Get(e Entity) (*T1, *T2) {
	w := b.world
	if !w.IsValid(e) {
		return nil, nil
	}
	meta := w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	id1 := b.id1
	i1 := id1 >> 6
	o1 := id1 & 63
	id2 := b.id2
	i2 := id2 >> 6
	o2 := id2 & 63

	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 {
		return nil, nil
	}
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])

	return (*T1)(ptr1), (*T2)(ptr2)
}

// This template generates the code for N-ary Builders (Builder2, Builder3, etc.).
// A Builder is a highly optimized factory for creating entities with a fixed set
// of components. By pre-calculating the archetype, it makes entity creation an
// extremely fast, allocation-free operation.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types, e.g., "id1 == id2".
// - .Components: A slice of ComponentInfo structs, used for loops.
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

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)

	if id2 == id1 || id3 == id1 || id3 == id2 {
		panic("ecs: duplicate component types in Builder3")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)

	specs := []compSpec{
		{id: id1, typ: t1, size: w.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.compIDToSize[id3]},
	}
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder3[T1, T2, T3]{world: w, arch: arch, id1: id1, id2: id2, id3: id3}
}

// New is a convenience function that creates a new builder instance.
func (b *Builder3[T1, T2, T3]) New(w *World) *Builder3[T1, T2, T3] {
	return NewBuilder3[T1, T2, T3](w)
}

// NewEntity creates a single new entity with the 3 components defined by the
// builder: T1, T2, T3.
//
// Returns:
//   - The newly created Entity.
func (b *Builder3[T1, T2, T3]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 3 components
// defined by the builder. This is the most performant method for creating many
// entities at once.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder3[T1, T2, T3]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.nextEntityVer++
	}
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
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])
		*(*T1)(ptr1) = comp1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])
		*(*T2)(ptr2) = comp2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])
		*(*T3)(ptr3) = comp3

		w.nextEntityVer++
	}
}

// Get retrieves pointers to the 3 components (T1, T2, T3) for the
// given entity.
//
// If the entity is invalid or does not have all the required components, this
// returns nil for all pointers.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3), or nils if not found.
func (b *Builder3[T1, T2, T3]) Get(e Entity) (*T1, *T2, *T3) {
	w := b.world
	if !w.IsValid(e) {
		return nil, nil, nil
	}
	meta := w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	id1 := b.id1
	i1 := id1 >> 6
	o1 := id1 & 63
	id2 := b.id2
	i2 := id2 >> 6
	o2 := id2 & 63
	id3 := b.id3
	i3 := id3 >> 6
	o3 := id3 & 63

	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 {
		return nil, nil, nil
	}
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
	ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3)
}

// This template generates the code for N-ary Builders (Builder2, Builder3, etc.).
// A Builder is a highly optimized factory for creating entities with a fixed set
// of components. By pre-calculating the archetype, it makes entity creation an
// extremely fast, allocation-free operation.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types, e.g., "id1 == id2".
// - .Components: A slice of ComponentInfo structs, used for loops.
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

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 {
		panic("ecs: duplicate component types in Builder4")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	mask.set(id4)

	specs := []compSpec{
		{id: id1, typ: t1, size: w.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.compIDToSize[id4]},
	}
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder4[T1, T2, T3, T4]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4}
}

// New is a convenience function that creates a new builder instance.
func (b *Builder4[T1, T2, T3, T4]) New(w *World) *Builder4[T1, T2, T3, T4] {
	return NewBuilder4[T1, T2, T3, T4](w)
}

// NewEntity creates a single new entity with the 4 components defined by the
// builder: T1, T2, T3, T4.
//
// Returns:
//   - The newly created Entity.
func (b *Builder4[T1, T2, T3, T4]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 4 components
// defined by the builder. This is the most performant method for creating many
// entities at once.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder4[T1, T2, T3, T4]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.nextEntityVer++
	}
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
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])
		*(*T1)(ptr1) = comp1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])
		*(*T2)(ptr2) = comp2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])
		*(*T3)(ptr3) = comp3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])
		*(*T4)(ptr4) = comp4

		w.nextEntityVer++
	}
}

// Get retrieves pointers to the 4 components (T1, T2, T3, T4) for the
// given entity.
//
// If the entity is invalid or does not have all the required components, this
// returns nil for all pointers.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4), or nils if not found.
func (b *Builder4[T1, T2, T3, T4]) Get(e Entity) (*T1, *T2, *T3, *T4) {
	w := b.world
	if !w.IsValid(e) {
		return nil, nil, nil, nil
	}
	meta := w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	id1 := b.id1
	i1 := id1 >> 6
	o1 := id1 & 63
	id2 := b.id2
	i2 := id2 >> 6
	o2 := id2 & 63
	id3 := b.id3
	i3 := id3 >> 6
	o3 := id3 & 63
	id4 := b.id4
	i4 := id4 >> 6
	o4 := id4 & 63

	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 {
		return nil, nil, nil, nil
	}
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
	ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
	ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4)
}

// This template generates the code for N-ary Builders (Builder2, Builder3, etc.).
// A Builder is a highly optimized factory for creating entities with a fixed set
// of components. By pre-calculating the archetype, it makes entity creation an
// extremely fast, allocation-free operation.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types, e.g., "id1 == id2".
// - .Components: A slice of ComponentInfo structs, used for loops.
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

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)
	id5 := w.getCompTypeID(t5)

	if id2 == id1 || id3 == id1 || id3 == id2 || id4 == id1 || id4 == id2 || id4 == id3 || id5 == id1 || id5 == id2 || id5 == id3 || id5 == id4 {
		panic("ecs: duplicate component types in Builder5")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	mask.set(id3)
	mask.set(id4)
	mask.set(id5)

	specs := []compSpec{
		{id: id1, typ: t1, size: w.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.compIDToSize[id4]},
		{id: id5, typ: t5, size: w.compIDToSize[id5]},
	}
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder5[T1, T2, T3, T4, T5]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5}
}

// New is a convenience function that creates a new builder instance.
func (b *Builder5[T1, T2, T3, T4, T5]) New(w *World) *Builder5[T1, T2, T3, T4, T5] {
	return NewBuilder5[T1, T2, T3, T4, T5](w)
}

// NewEntity creates a single new entity with the 5 components defined by the
// builder: T1, T2, T3, T4, T5.
//
// Returns:
//   - The newly created Entity.
func (b *Builder5[T1, T2, T3, T4, T5]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 5 components
// defined by the builder. This is the most performant method for creating many
// entities at once.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder5[T1, T2, T3, T4, T5]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.nextEntityVer++
	}
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
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])
		*(*T1)(ptr1) = comp1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])
		*(*T2)(ptr2) = comp2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])
		*(*T3)(ptr3) = comp3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])
		*(*T4)(ptr4) = comp4
		ptr5 := unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(startSize+k)*a.compSizes[b.id5])
		*(*T5)(ptr5) = comp5

		w.nextEntityVer++
	}
}

// Get retrieves pointers to the 5 components (T1, T2, T3, T4, T5) for the
// given entity.
//
// If the entity is invalid or does not have all the required components, this
// returns nil for all pointers.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5), or nils if not found.
func (b *Builder5[T1, T2, T3, T4, T5]) Get(e Entity) (*T1, *T2, *T3, *T4, *T5) {
	w := b.world
	if !w.IsValid(e) {
		return nil, nil, nil, nil, nil
	}
	meta := w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	id1 := b.id1
	i1 := id1 >> 6
	o1 := id1 & 63
	id2 := b.id2
	i2 := id2 >> 6
	o2 := id2 & 63
	id3 := b.id3
	i3 := id3 >> 6
	o3 := id3 & 63
	id4 := b.id4
	i4 := id4 >> 6
	o4 := id4 & 63
	id5 := b.id5
	i5 := id5 >> 6
	o5 := id5 & 63

	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 {
		return nil, nil, nil, nil, nil
	}
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
	ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
	ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
	ptr5 := unsafe.Pointer(uintptr(a.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4), (*T5)(ptr5)
}

// This template generates the code for N-ary Builders (Builder2, Builder3, etc.).
// A Builder is a highly optimized factory for creating entities with a fixed set
// of components. By pre-calculating the archetype, it makes entity creation an
// extremely fast, allocation-free operation.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .DuplicateIDs: A condition to check for duplicate component types, e.g., "id1 == id2".
// - .Components: A slice of ComponentInfo structs, used for loops.
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

	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	id3 := w.getCompTypeID(t3)
	id4 := w.getCompTypeID(t4)
	id5 := w.getCompTypeID(t5)
	id6 := w.getCompTypeID(t6)

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

	specs := []compSpec{
		{id: id1, typ: t1, size: w.compIDToSize[id1]},
		{id: id2, typ: t2, size: w.compIDToSize[id2]},
		{id: id3, typ: t3, size: w.compIDToSize[id3]},
		{id: id4, typ: t4, size: w.compIDToSize[id4]},
		{id: id5, typ: t5, size: w.compIDToSize[id5]},
		{id: id6, typ: t6, size: w.compIDToSize[id6]},
	}
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder6[T1, T2, T3, T4, T5, T6]{world: w, arch: arch, id1: id1, id2: id2, id3: id3, id4: id4, id5: id5, id6: id6}
}

// New is a convenience function that creates a new builder instance.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) New(w *World) *Builder6[T1, T2, T3, T4, T5, T6] {
	return NewBuilder6[T1, T2, T3, T4, T5, T6](w)
}

// NewEntity creates a single new entity with the 6 components defined by the
// builder: T1, T2, T3, T4, T5, T6.
//
// Returns:
//   - The newly created Entity.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the 6 components
// defined by the builder. This is the most performant method for creating many
// entities at once.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) NewEntities(count int) {
	if count == 0 {
		return
	}
	w := b.world
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		w.nextEntityVer++
	}
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
	a := b.arch
	for len(w.freeIDs) < count {
		w.expand()
	}
	startSize := a.size
	a.size += count
	popped := w.freeIDs[len(w.freeIDs)-count:]
	w.freeIDs = w.freeIDs[:len(w.freeIDs)-count]
	for k := 0; k < count; k++ {
		id := popped[k]
		meta := &w.metas[id]
		meta.archetypeIndex = a.index
		meta.index = startSize + k
		meta.version = w.nextEntityVer
		ent := Entity{ID: id, Version: meta.version}
		a.entityIDs[startSize+k] = ent
		ptr1 := unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(startSize+k)*a.compSizes[b.id1])
		*(*T1)(ptr1) = comp1
		ptr2 := unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(startSize+k)*a.compSizes[b.id2])
		*(*T2)(ptr2) = comp2
		ptr3 := unsafe.Pointer(uintptr(a.compPointers[b.id3]) + uintptr(startSize+k)*a.compSizes[b.id3])
		*(*T3)(ptr3) = comp3
		ptr4 := unsafe.Pointer(uintptr(a.compPointers[b.id4]) + uintptr(startSize+k)*a.compSizes[b.id4])
		*(*T4)(ptr4) = comp4
		ptr5 := unsafe.Pointer(uintptr(a.compPointers[b.id5]) + uintptr(startSize+k)*a.compSizes[b.id5])
		*(*T5)(ptr5) = comp5
		ptr6 := unsafe.Pointer(uintptr(a.compPointers[b.id6]) + uintptr(startSize+k)*a.compSizes[b.id6])
		*(*T6)(ptr6) = comp6

		w.nextEntityVer++
	}
}

// Get retrieves pointers to the 6 components (T1, T2, T3, T4, T5, T6) for the
// given entity.
//
// If the entity is invalid or does not have all the required components, this
// returns nil for all pointers.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data (*T1, *T2, *T3, *T4, *T5, *T6), or nils if not found.
func (b *Builder6[T1, T2, T3, T4, T5, T6]) Get(e Entity) (*T1, *T2, *T3, *T4, *T5, *T6) {
	w := b.world
	if !w.IsValid(e) {
		return nil, nil, nil, nil, nil, nil
	}
	meta := w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	id1 := b.id1
	i1 := id1 >> 6
	o1 := id1 & 63
	id2 := b.id2
	i2 := id2 >> 6
	o2 := id2 & 63
	id3 := b.id3
	i3 := id3 >> 6
	o3 := id3 & 63
	id4 := b.id4
	i4 := id4 >> 6
	o4 := id4 & 63
	id5 := b.id5
	i5 := id5 >> 6
	o5 := id5 & 63
	id6 := b.id6
	i6 := id6 >> 6
	o6 := id6 & 63

	if (a.mask[i1]&(uint64(1)<<uint64(o1))) == 0 || (a.mask[i2]&(uint64(1)<<uint64(o2))) == 0 || (a.mask[i3]&(uint64(1)<<uint64(o3))) == 0 || (a.mask[i4]&(uint64(1)<<uint64(o4))) == 0 || (a.mask[i5]&(uint64(1)<<uint64(o5))) == 0 || (a.mask[i6]&(uint64(1)<<uint64(o6))) == 0 {
		return nil, nil, nil, nil, nil, nil
	}
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[id1]) + uintptr(meta.index)*a.compSizes[id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[id2]) + uintptr(meta.index)*a.compSizes[id2])
	ptr3 := unsafe.Pointer(uintptr(a.compPointers[id3]) + uintptr(meta.index)*a.compSizes[id3])
	ptr4 := unsafe.Pointer(uintptr(a.compPointers[id4]) + uintptr(meta.index)*a.compSizes[id4])
	ptr5 := unsafe.Pointer(uintptr(a.compPointers[id5]) + uintptr(meta.index)*a.compSizes[id5])
	ptr6 := unsafe.Pointer(uintptr(a.compPointers[id6]) + uintptr(meta.index)*a.compSizes[id6])

	return (*T1)(ptr1), (*T2)(ptr2), (*T3)(ptr3), (*T4)(ptr4), (*T5)(ptr5), (*T6)(ptr6)
}
