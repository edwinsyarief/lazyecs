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
