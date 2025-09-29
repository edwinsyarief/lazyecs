package lazyecs

import (
	"reflect"
	"unsafe"
)

//----------------------------------------
// Builder for 1 component
//----------------------------------------

// Builder provides a simple API to create entities with a specific component.
type Builder[T any] struct {
	world  *World
	arch   *archetype
	compID uint8
}

// NewBuilder creates a builder for entities with component T, pre-creating the archetype.
func NewBuilder[T any](w *World) *Builder[T] {
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	var mask bitmask256
	mask.set(id)
	sp := compSpec{id: id, typ: t, size: t.Size()}
	arch := w.getOrCreateArchetype(mask, []compSpec{sp})
	return &Builder[T]{world: w, arch: arch, compID: id}
}

// NewEntity creates a new entity with the component T.
func (b *Builder[T]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates count entities with the component T (void return to avoid allocations).
func (b *Builder[T]) NewEntities(count int) {
	for i := 0; i < count; i++ {
		b.world.createEntity(b.arch)
	}
}

// Get returns a pointer to the component T for the entity, or nil if not present or invalid.
func (b *Builder[T]) Get(e Entity) *T {
	w := b.world
	meta := &w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return nil
	}
	a := w.archetypes[meta.archetypeIndex]
	var m bitmask256
	m.set(b.compID)
	if !a.mask.contains(m) {
		return nil
	}
	ptr := unsafe.Pointer(uintptr(a.compPointers[b.compID]) + uintptr(meta.index)*a.compSizes[b.compID])
	return (*T)(ptr)
}

//----------------------------------------
// Builder for 2 components
//----------------------------------------

// Builder2 provides a simple API to create entities with two specific components.
type Builder2[T1 any, T2 any] struct {
	world *World
	arch  *archetype
	id1   uint8
	id2   uint8
}

// NewBuilder2 creates a builder for entities with components T1 and T2, pre-creating the archetype.
func NewBuilder2[T1 any, T2 any](w *World) *Builder2[T1, T2] {
	t1 := reflect.TypeFor[T1]()
	t2 := reflect.TypeFor[T2]()
	id1 := w.getCompTypeID(t1)
	id2 := w.getCompTypeID(t2)
	if id1 == id2 {
		panic("ecs: duplicate component types in Builder2")
	}
	var mask bitmask256
	mask.set(id1)
	mask.set(id2)
	specs := []compSpec{
		{id: id1, typ: t1, size: t1.Size()},
		{id: id2, typ: t2, size: t2.Size()},
	}
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder2[T1, T2]{world: w, arch: arch, id1: id1, id2: id2}
}

// NewEntity creates a new entity with components T1 and T2.
func (b *Builder2[T1, T2]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates count entities with components T1 and T2 (void return to avoid allocations).
func (b *Builder2[T1, T2]) NewEntities(count int) {
	for i := 0; i < count; i++ {
		b.world.createEntity(b.arch)
	}
}

// Get returns pointers to components T1 and T2 for the entity, or nil if not present or invalid.
func (b *Builder2[T1, T2]) Get(e Entity) (*T1, *T2) {
	w := b.world
	meta := &w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return nil, nil
	}
	a := w.archetypes[meta.archetypeIndex]
	var m bitmask256
	m.set(b.id1)
	m.set(b.id2)
	if !a.mask.contains(m) {
		return nil, nil
	}
	ptr1 := unsafe.Pointer(uintptr(a.compPointers[b.id1]) + uintptr(meta.index)*a.compSizes[b.id1])
	ptr2 := unsafe.Pointer(uintptr(a.compPointers[b.id2]) + uintptr(meta.index)*a.compSizes[b.id2])
	return (*T1)(ptr1), (*T2)(ptr2)
}
