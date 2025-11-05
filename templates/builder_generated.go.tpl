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
	w.components.mu.RLock()
	{{range .Components}}id{{.Index}} := w.getCompTypeIDNoLock(t{{.Index}})
	{{end}}
	w.components.mu.RUnlock()

	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in Builder{{.N}}")
	}
	var mask bitmask256
	{{range .Components}}mask.set(id{{.Index}})
	{{end}}
	w.components.mu.RLock()
	specs := []compSpec{
		{{range .Components}}{id: id{{.Index}}, typ: t{{.Index}}, size: w.components.compIDToSize[id{{.Index}}]},
		{{end}}
	}
	w.components.mu.RUnlock()
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder{{.N}}[{{.TypeVars}}]{world: w, arch: arch, {{range $i, $e := .Components}}{{if $i}}, {{end}}id{{$e.Index}}: id{{$e.Index}}{{end}}}
}

// New is a convenience method that constructs a new `Builder` instance for the
// same component types, equivalent to calling `NewBuilder{{.N}}`.
func (b *Builder{{.N}}[{{.TypeVars}}]) New(w *World) *Builder{{.N}}[{{.TypeVars}}] {
	return NewBuilder{{.N}}[{{.TypeVars}}](w)
}

// NewEntity creates a single new entity with the {{.N}} components defined by the
// builder: {{.TypeVars}}. This method is highly optimized and should not cause
// any garbage collection overhead.
//
// Returns:
//   - The newly created Entity.
func (b *Builder{{.N}}[{{.TypeVars}}]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates a batch of `count` entities with the {{.N}} components
// defined by the builder. This is the most performant method for creating many
// entities at once. This method does not return the created entities to avoid
// allocations. Use a `Filter` to query for and initialize them afterward.
//
// Parameters:
//   - count: The number of entities to create.
func (b *Builder{{.N}}[{{.TypeVars}}]) NewEntities(count int) {
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
{{range .Components}}//   - comp{{.Index}}: The initial value for the component {{.TypeName}}.
{{end}}func (b *Builder{{.N}}[{{.TypeVars}}]) NewEntitiesWithValueSet(count int, {{.BuilderVars}}) {
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
		{{range .Components}}*(*{{.TypeName}})(unsafe.Pointer(uintptr(a.compPointers[b.id{{.Index}}]) + uintptr(startSize+k)*a.compSizes[b.id{{.Index}}])) = {{.BuilderVarName}}
		{{end}}
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
func (b *Builder{{.N}}[{{.TypeVars}}]) Get(e Entity) ({{.ReturnTypes}}) {
	w := b.world
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.IsValidNoLock(e) {
		return {{.ReturnNil}}
	}
	meta := w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	{{range .Components}}i{{.Index}} := b.id{{.Index}} >> 6
	o{{.Index}} := b.id{{.Index}} & 63
	{{end}}
	if {{.BuilderMaskCheck}} {
		return {{.ReturnNil}}
	}
	return {{range $i, $e := .Components}}{{if $i}},
		{{end}}(*{{$e.TypeName}})(unsafe.Add(a.compPointers[b.id{{$e.Index}}], uintptr(meta.index)*a.compSizes[b.id{{$e.Index}}])){{end}}
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
{{range .Components}}//   - v{{.Index}}: The value for {{.TypeName}}.
{{end}}func (b *Builder{{.N}}[{{.TypeVars}}]) Set(e Entity, {{.SetVars}}) {
	w := b.world
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.IsValidNoLock(e) {
		return
	}
	meta := &w.entities.metas[e.ID]
	a := w.archetypes.archetypes[meta.archetypeIndex]
	{{range .Components}}has{{.Index}} := (a.mask[b.id{{.Index}}>>6] & (uint64(1) << uint64(b.id{{.Index}}&63))) != 0
	{{end}}
	if {{.SetHasVars}} {
		{{range .Components}}*(*{{.TypeName}})(unsafe.Pointer(uintptr(a.compPointers[b.id{{.Index}}]) + uintptr(meta.index)*a.compSizes[b.id{{.Index}}])) = v{{.Index}}
		{{end}}
		return
	}
	newMask := a.mask
	{{range .Components}}if !has{{.Index}} {
		newMask.set(b.id{{.Index}})
	}
	{{end}}
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
		{{range .Components}}if !has{{.Index}} {
			tempSpecs[count] = compSpec{id: b.id{{.Index}}, typ: w.components.compIDToType[b.id{{.Index}}], size: w.components.compIDToSize[b.id{{.Index}}]}
			count++
		}
		{{end}}
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
	{{range .Components}}*(*{{.TypeName}})(unsafe.Pointer(uintptr(targetA.compPointers[b.id{{.Index}}]) + uintptr(newIdx)*targetA.compSizes[b.id{{.Index}}])) = v{{.Index}}
	{{end}}
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
{{range .Components}}//   - v{{.Index}}: The component value to set for type {{.TypeName}}.
{{end}}func (b *Builder{{.N}}[{{.TypeVars}}]) SetBatch(entities []Entity, {{.SetVars}}) {
	for _, e := range entities {
		b.Set(e, {{.SetVarNames}})
	}
}
