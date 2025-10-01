// Filter{{.N}} provides a fast iterator over entities with components {{.TypeVars}}.
type Filter{{.N}}[{{.Types}}] struct {
	world          *World
	mask           bitmask256
	{{range .Components}}id{{.Index}}            uint8
	{{end}}
	matchingArches []*archetype
	lastVersion    uint32
	curMatchIdx    int
	curIdx         int
	curEnt         Entity
}

// NewFilter{{.N}} creates a filter for entities with components {{.TypeVars}}.
func NewFilter{{.N}}[{{.Types}}](w *World) *Filter{{.N}}[{{.TypeVars}}] {
	{{range .Components}}t{{.Index}} := reflect.TypeFor[{{.TypeName}}]()
	{{end}}
	{{range .Components}}id{{.Index}} := w.getCompTypeID(t{{.Index}})
	{{end}}
	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in Filter{{.N}}")
	}
	var m bitmask256
	{{range .Components}}m.set(id{{.Index}})
	{{end}}
	f := &Filter{{.N}}[{{.TypeVars}}]{world: w, mask: m, {{range .Components}}id{{.Index}}: id{{.Index}},{{end}} curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
	return f
}

// New creates a filter for entities with components {{.TypeVars}}.
func (f *Filter{{.N}}[{{.TypeVars}}]) New(w *World) *Filter{{.N}}[{{.TypeVars}}] {
	return NewFilter{{.N}}[{{.TypeVars}}](w)
}

// updateMatching updates the list of matching archetypes.
func (f *Filter{{.N}}[{{.TypeVars}}]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// Reset resets the filter iterator.
func (f *Filter{{.N}}[{{.TypeVars}}]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
}

// Next advances to the next entity with the components, returning true if found.
func (f *Filter{{.N}}[{{.TypeVars}}]) Next() bool {
	for {
		f.curIdx++
		if f.curMatchIdx >= len(f.matchingArches) {
			return false
		}
		a := f.matchingArches[f.curMatchIdx]
		if f.curIdx >= a.size {
			f.curMatchIdx++
			f.curIdx = -1
			continue
		}
		f.curEnt = a.entityIDs[f.curIdx]
		return true
	}
}

// Entity returns the current entity.
func (f *Filter{{.N}}[{{.TypeVars}}]) Entity() Entity {
	return f.curEnt
}

// Get returns pointers to the current components {{.TypeVars}}.
func (f *Filter{{.N}}[{{.TypeVars}}]) Get() ({{.ReturnTypes}}) {
	a := f.matchingArches[f.curMatchIdx]
	{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(a.compPointers[f.id{{.Index}}]) + uintptr(f.curIdx)*a.compSizes[f.id{{.Index}}])
	{{end}}
	return {{.ReturnPtrs}}
}

// RemoveEntities batch-removes all entities matching the filter with zero allocations or memory moves.
func (f *Filter{{.N}}[{{.TypeVars}}]) RemoveEntities() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := 0; i < a.size; i++ {
			ent := a.entityIDs[i]
			meta := &f.world.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.freeIDs = append(f.world.freeIDs, ent.ID)
		}
		a.size = 0
	}
	f.Reset()
}