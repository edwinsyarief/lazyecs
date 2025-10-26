// Filter{{.N}} provides a fast, cache-friendly iterator over all entities that
// have the {{.N}} components: {{.TypeVars}}.
type Filter{{.N}}[{{.Types}}] struct {
	queryCache
	{{range .Components}}curBase{{.Index}} unsafe.Pointer
	{{end}}
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	{{range .Components}}compSize{{.Index}} uintptr
	{{end}}
	curArchSize  int
	curEnt       Entity
	{{range .Components}}id{{.Index}}       uint8
	{{end}}
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
	{{range .Components}}id{{.Index}} := w.getCompTypeID(reflect.TypeFor[{{.TypeName}}]())
	{{end}}
	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in Filter{{.N}}")
	}
	var m bitmask256
	{{range .Components}}m.set(id{{.Index}})
	{{end}}
	f := &Filter{{.N}}[{{.TypeVars}}]{
		queryCache: newQueryCache(w, m),
		{{range $i, $e := .Components}}id{{$e.Index}}: id{{$e.Index}},
		{{end}}curMatchIdx: 0,
		curIdx:      -1,
	}
	{{range .Components}}f.compSize{{.Index}} = w.components.compIDToSize[id{{.Index}}]
	{{end}}
	f.updateMatching()
	f.updateCachedEntities()
	f.Reset()
	return f
}

// New is a convenience function that creates a new filter instance.
func (f *Filter{{.N}}[{{.TypeVars}}]) New(w *World) *Filter{{.N}}[{{.TypeVars}}] {
	return NewFilter{{.N}}[{{.TypeVars}}](w)
}

// Reset rewinds the filter's iterator to the beginning. It should be called if
// you need to iterate over the same set of entities multiple times.
func (f *Filter{{.N}}[{{.TypeVars}}]) Reset() {
	if f.IsStale() {
		f.updateMatching()
		f.updateCachedEntities()
	}
	f.curMatchIdx = 0
	f.curIdx = -1
	if len(f.matchingArches) > 0 {
		a := f.matchingArches[0]
		{{range .Components}}f.curBase{{.Index}} = a.compPointers[f.id{{.Index}}]
		{{end}}
		f.curEntityIDs = a.entityIDs
		f.curArchSize = a.size
	}
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter{{.N}}[{{.TypeVars}}]) Next() bool {
	for {
		f.curIdx++
		if f.curIdx >= f.curArchSize {
			f.curMatchIdx++
			if f.curMatchIdx >= len(f.matchingArches) {
				return false
			}
			a := f.matchingArches[f.curMatchIdx]
			{{range .Components}}f.curBase{{.Index}} = a.compPointers[f.id{{.Index}}]
			{{end}}
			f.curEntityIDs = a.entityIDs
			f.curArchSize = a.size
			f.curIdx = -1
			continue
		}
		f.curEnt = f.curEntityIDs[f.curIdx]
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
	{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(f.curBase{{.Index}}) + uintptr(f.curIdx)*f.compSize{{.Index}})
	{{end}}
	return {{.ReturnPtrs}}
}

// RemoveEntities efficiently removes all entities that match the filter's
// query. This operation is performed in a batch, invalidating all matching
// entities and recycling their IDs without moving any memory.
func (f *Filter{{.N}}[{{.TypeVars}}]) RemoveEntities() {
	if f.IsStale() {
		f.updateMatching()
	}
	for _, a := range f.matchingArches {
		for i := range a.size {
			ent := a.entityIDs[i]
			meta := &f.world.entities.metas[ent.ID]
			meta.archetypeIndex = -1
			meta.index = -1
			meta.version = 0
			f.world.entities.freeIDs = append(f.world.entities.freeIDs, ent.ID)
		}
		a.size = 0
	}
	f.world.mutationVersion++
	f.Reset()
}

// Entities returns all entities that match the filter.
func (f *Filter{{.N}}[{{.TypeVars}}]) Entities() []Entity {
	return f.queryCache.Entities()
}
