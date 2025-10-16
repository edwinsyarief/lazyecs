// Filter{{.N}} provides a fast, cache-friendly iterator over all entities that
// have the {{.N}} components: {{.TypeVars}}.
type Filter{{.N}}[{{.Types}}] struct {
	world          *World
	mask           bitmask256
	{{range .Components}}id{{.Index}}            uint8
	{{end}}
	matchingArches []*archetype
	lastVersion    uint32 // world.archetypeVersion when matchingArches was last updated
	curMatchIdx    int    // index into matchingArches
	curIdx         int    // index into the current archetype's entity/component array
	curEnt         Entity
}

// NewFilter{{.N}} creates a new `Filter` that iterates over all entities
// possessing at least the {{.N}} components: {{.TypeVars}}.
//
// Parameters:
//   - w: The World to query.
//
// Returns:
//   - A pointer to the newly created `Filter{{.N}}`.
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
	f := &Filter{{.N}}[{{.TypeVars}}]{world: w, mask: m, {{range $i, $e := .Components}}{{if $i}}, {{end}}id{{$e.Index}}: id{{$e.Index}}{{end}}, curMatchIdx: 0, curIdx: -1, matchingArches: make([]*archetype, 0, 4)}
	f.updateMatching()
	return f
}

// New is a convenience function that creates a new filter instance.
func (f *Filter{{.N}}[{{.TypeVars}}]) New(w *World) *Filter{{.N}}[{{.TypeVars}}] {
	return NewFilter{{.N}}[{{.TypeVars}}](w)
}

// updateMatching rebuilds the filter's list of archetypes that match its
// component mask.
func (f *Filter{{.N}}[{{.TypeVars}}]) updateMatching() {
	f.matchingArches = f.matchingArches[:0]
	for _, a := range f.world.archetypes {
		if a.mask.contains(f.mask) {
			f.matchingArches = append(f.matchingArches, a)
		}
	}
	f.lastVersion = f.world.archetypeVersion
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter{{.N}}[{{.TypeVars}}]) Reset() {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
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

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
func (f *Filter{{.N}}[{{.TypeVars}}]) Entity() Entity {
	return f.curEnt
}

// Get returns pointers to the {{.N}} components ({{.TypeVars}}) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data ({{.ReturnTypes}}).
func (f *Filter{{.N}}[{{.TypeVars}}]) Get() ({{.ReturnTypes}}) {
	a := f.matchingArches[f.curMatchIdx]
	{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(a.compPointers[f.id{{.Index}}]) + uintptr(f.curIdx)*a.compSizes[f.id{{.Index}}])
	{{end}}
	return {{.ReturnPtrs}}
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
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

// Entities returns all entities that match the filter.
func (f *Filter{{.N}}[{{.TypeVars}}]) Entities() []Entity {
	if f.world.archetypeVersion != f.lastVersion {
		f.updateMatching()
	}
	total := 0
	for _, a := range f.matchingArches {
		total += a.size
	}
	ents := make([]Entity, total)
	idx := 0
	for _, a := range f.matchingArches {
		copy(ents[idx:idx+a.size], a.entityIDs[:a.size])
		idx += a.size
	}
	return ents
}
