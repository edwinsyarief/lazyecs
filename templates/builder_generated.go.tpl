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
// Builder{{.N}} provides a highly efficient, type-safe API for creating entities
// with a predefined set of {{.N}} components: {{.TypeVars}}.
type Builder{{.N}}[{{.Types}}] struct {
	world *World
	arch  *archetype
	{{range .Components}}id{{.Index}}   uint8
	{{end}}
}

// NewBuilder{{.N}} creates a new `Builder` for entities with the {{.N}} components
// {{.TypeVars}}. It pre-calculates and caches the archetype for peak
// performance.
//
// Parameters:
//   - w: The World in which to create entities.
//
// Returns:
//   - A pointer to the configured `Builder{{.N}}`.
func NewBuilder{{.N}}[{{.Types}}](w *World) *Builder{{.N}}[{{.TypeVars}}] {
	{{range .Components}}t{{.Index}} := reflect.TypeFor[{{.TypeName}}]()
	{{end}}
	{{range .Components}}id{{.Index}} := w.getCompTypeID(t{{.Index}})
	{{end}}
	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in Builder{{.N}}")
	}
	var mask bitmask256
	{{range .Components}}mask.set(id{{.Index}})
	{{end}}
	specs := []compSpec{
		{{range .Components}}{id: id{{.Index}}, typ: t{{.Index}}, size: w.compIDToSize[id{{.Index}}]},
		{{end}}
	}
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder{{.N}}[{{.TypeVars}}]{world: w, arch: arch, {{range $i, $e := .Components}}{{if $i}}, {{end}}id{{$e.Index}}: id{{$e.Index}}{{end}}}
}

// New is a convenience function that creates a new builder instance.
func (b *Builder{{.N}}[{{.TypeVars}}]) New(w *World) *Builder{{.N}}[{{.TypeVars}}] {
	return NewBuilder{{.N}}[{{.TypeVars}}](w)
}

// NewEntity creates a single new entity with the {{.N}} components defined by the
// builder: {{.TypeVars}}.
//
// Returns:
//   - The newly created Entity.
func (b *Builder{{.N}}[{{.TypeVars}}]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the {{.N}} components
// defined by the builder. This is the most performant method for creating many
// entities at once.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder{{.N}}[{{.TypeVars}}]) NewEntities(count int) {
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
{{range .Components}}//   - comp{{.Index}}: The initial value for the component {{.TypeName}}.
{{end}}
func (b *Builder{{.N}}[{{.TypeVars}}]) NewEntitiesWithValueSet(count int, {{.BuilderVars}}) {
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
		{{range .Components}}ptr{{.Index}} := unsafe.Pointer(uintptr(a.compPointers[b.id{{.Index}}]) + uintptr(startSize+k)*a.compSizes[b.id{{.Index}}])
		*(*{{.TypeName}})(ptr{{.Index}}) = {{.BuilderVarName}}
		{{end}}
		w.nextEntityVer++
	}
}

// Get retrieves pointers to the {{.N}} components ({{.TypeVars}}) for the
// given entity.
//
// If the entity is invalid or does not have all the required components, this
// returns nil for all pointers.
//
// Parameters:
//   - e: The entity to get the components from.
//
// Returns:
//   - Pointers to the component data ({{.ReturnTypes}}), or nils if not found.
func (b *Builder{{.N}}[{{.TypeVars}}]) Get(e Entity) ({{.ReturnTypes}}) {
	w := b.world
	if !w.IsValid(e) {
		return {{.ReturnNil}}
	}
	meta := w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	{{range .Components}}id{{.Index}} := b.id{{.Index}}
	i{{.Index}} := id{{.Index}} >> 6
	o{{.Index}} := id{{.Index}} & 63
	{{end}}
	if {{.BuilderMaskCheck}} {
		return {{.ReturnNil}}
	}
	{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(a.compPointers[id{{.Index}}]) + uintptr(meta.index)*a.compSizes[id{{.Index}}])
	{{end}}
	return {{.ReturnPtrs}}
}
