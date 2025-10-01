package lazyecs

import (
	"reflect"
	"unsafe"
)

// Builder provides a highly efficient, type-safe API for creating entities
// with a predefined set of components. By pre-calculating the target
// archetype, it minimizes overhead and avoids allocations when creating
// entities, making it the ideal choice for spawning large numbers of entities
// with the same component layout.
//
// This is the builder for entities with one component. Generated builders for
// multiple components (e.g., Builder2, Builder3) follow a similar pattern.
type Builder[T any] struct {
	world  *World
	arch   *archetype
	compID uint8
}

// NewBuilder creates a new `Builder` for entities with a single component of
// type `T`. It finds or creates the corresponding archetype and caches it for
// future entity creation.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder[T]`.
func NewBuilder[T any](w *World) *Builder[T] {
	t := reflect.TypeFor[T]()
	id := w.getCompTypeID(t)
	var mask bitmask256
	mask.set(id)
	sp := compSpec{id: id, typ: t, size: w.compIDToSize[id]}
	arch := w.getOrCreateArchetype(mask, []compSpec{sp})
	return &Builder[T]{world: w, arch: arch, compID: id}
}

// New is a convenience function that creates a new builder instance.
func (b *Builder[T]) New(w *World) *Builder[T] {
	return NewBuilder[T](w)
}

// NewEntity creates a single new entity with the component layout defined by the
// builder. This method is highly optimized and should not cause any garbage
// collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder[T]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the component layout
// defined by the builder. This is the most performant way to create many
// entities at once, as it minimizes overhead by processing them in a single
// operation.
//
// This method does not return the created entities to avoid allocations. Use a
// `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder[T]) NewEntities(count int) {
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
// their component of type `T` to the provided value. This is useful for
// creating and setting up entities in one step.
//
// Parameters:
//   - count: The number of entities to create.
//   - comp: The initial value for the component `T`.
func (b *Builder[T]) NewEntitiesWithValueSet(count int, comp T) {
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
		ptr := unsafe.Pointer(uintptr(a.compPointers[b.compID]) + uintptr(startSize+k)*a.compSizes[b.compID])
		*(*T)(ptr) = comp
		w.nextEntityVer++
	}
}

// Get retrieves a pointer to the component of type `T` for the given entity.
// This method is most efficient when the entity was created by this same
// builder, as the archetype is already known.
//
// If the entity is invalid or does not have the component, this returns nil.
//
// Parameters:
//   - e: The entity to get the component from.
//
// Returns:
//   - A pointer to the component data (*T), or nil if not found.
func (b *Builder[T]) Get(e Entity) *T {
	w := b.world
	if !w.IsValid(e) {
		return nil
	}
	meta := w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	id := b.compID
	i := id >> 6
	o := id & 63
	if (a.mask[i] & (uint64(1) << uint64(o))) == 0 {
		return nil
	}
	ptr := unsafe.Pointer(uintptr(a.compPointers[id]) + uintptr(meta.index)*a.compSizes[id])
	return (*T)(ptr)
}
