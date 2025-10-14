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

// Set replaces the existing component values if they exist, or adds the components to the entity if it doesn't have them.
//
// Parameters:
//   - entity: The entity to set the components for.
{{range .Components}}//   - comp{{.Index}}: The initial value for the component {{.TypeName}}.
{{end}}
func (b *Builder{{.N}}[{{.TypeVars}}]) Set(e Entity, {{.SetVars}}) {
	w := b.world
	if !w.IsValid(e) {
		return
	}
	meta := &w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	{{range .Components}}id{{.Index}} := b.id{{.Index}}
	i{{.Index}} := id{{.Index}} >> 6
	o{{.Index}} := id{{.Index}} & 63
	{{end}}
	{{range .Components}}has{{.Index}} := (a.mask[i{{.Index}}] & (uint64(1) << uint64(o{{.Index}}))) != 0
	{{end}}
	if {{.SetHasVars}} {
		{{range .Components}}ptr{{.Index}} := unsafe.Pointer(uintptr(a.compPointers[id{{.Index}}]) + uintptr(meta.index)*a.compSizes[id{{.Index}}])
		*(*{{.TypeName}})(ptr{{.Index}}) = v{{.Index}}
		{{end}}
		return
	}
	newMask := a.mask
	{{range .Components}}if !has{{.Index}} {
		newMask.set(id{{.Index}})
	}
	{{end}}
	var targetA *archetype
	if idx, ok := w.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			tempSpecs[count] = compSpec{id: cid, typ: w.compIDToType[cid], size: w.compIDToSize[cid]}
			count++
		}
		{{range .Components}}if !has{{.Index}} {
			tempSpecs[count] = compSpec{id: id{{.Index}}, typ: w.compIDToType[id{{.Index}}], size: w.compIDToSize[id{{.Index}}]}
			count++
		}
		{{end}}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	{{range .Components}}ptr{{.Index}} := unsafe.Pointer(uintptr(targetA.compPointers[id{{.Index}}]) + uintptr(newIdx)*targetA.compSizes[id{{.Index}}])
	*(*{{.TypeName}})(ptr{{.Index}}) = v{{.Index}}
	{{end}}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
}

// SetBatch sets the component values for multiple entities.
// 
// Parameters:
//   - entities: The entities slices to set the components for.
{{range .Components}}//   - comp{{.Index}}: The initial value for the component {{.TypeName}}.
{{end}}
func (b *Builder{{.N}}[{{.TypeVars}}]) SetBatch(entities []Entity, {{.SetVars}}) {
	for _, e := range entities {
		b.Set(e, {{.SetVarNames}})
	}
}
