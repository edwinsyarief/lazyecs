// Builder{{.N}} provides a simple API to create entities with {{.N}} specific components.
type Builder{{.N}}[{{.Types}}] struct {
	world *World
	arch  *archetype
	{{range .Components}}id{{.Index}} uint8
	{{end}}
}

// NewBuilder{{.N}} creates a builder for entities with components {{.TypeVars}}, pre-creating the archetype.
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
		{{range .Components}}{id: id{{.Index}}, typ: t{{.Index}}, size: t{{.Index}}.Size()},
		{{end}}
	}
	arch := w.getOrCreateArchetype(mask, specs)
	return &Builder{{.N}}[{{.TypeVars}}]{world: w, arch: arch, {{range .Components}}id{{.Index}}: id{{.Index}},{{end}}}
}

// New create a builder for entities with components {{.TypeVars}}, pre-creating the archetype
func (b *Builder{{.N}}[{{.TypeVars}}]) New(w *World) *Builder{{.N}}[{{.TypeVars}}] {
	return NewBuilder{{.N}}[{{.TypeVars}}](w)
}

// NewEntity creates a new entity with components {{.TypeVars}}.
func (b *Builder{{.N}}[{{.TypeVars}}]) NewEntity() Entity {
	return b.world.createEntity(b.arch)
}

// NewEntities creates count entities with components {{.TypeVars}} (void return to avoid allocations).
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

// NewEntitiesWithValueSet creates count entities and sets the component to the given value.
func (b *Builder{{.N}}[{{.TypeVars}}]) NewEntitiesWithValueSet(count int, {{.Vars}}) {
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
		*(*{{.TypeName}})(ptr{{.Index}}) = c{{.Index}}
		{{end}}
		w.nextEntityVer++
	}
}

// Get returns pointers to components {{.TypeVars}} for the entity, or nil if not present or invalid.
func (b *Builder{{.N}}[{{.TypeVars}}]) Get(e Entity) ({{.ReturnTypes}}) {
	w := b.world
	meta := &w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return {{.ReturnNil}}
	}
	a := w.archetypes[meta.archetypeIndex]
	var m bitmask256
	{{range .Components}}m.set(b.id{{.Index}})
	{{end}}
	if !a.mask.contains(m) {
		return {{.ReturnNil}}
	}
	{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(a.compPointers[b.id{{.Index}}]) + uintptr(meta.index)*a.compSizes[b.id{{.Index}}])
	{{end}}
	return {{.ReturnPtrs}}
}