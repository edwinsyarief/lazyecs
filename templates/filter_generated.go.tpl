// Filter{{.N}} provides a fast, cache-friendly iterator over all entities that
// have the {{.N}} components: {{.TypeVars}}.
type Filter{{.N}}[{{.Types}}] struct {
	queryCache
	curBases     [{{.N}}]unsafe.Pointer
	curEntityIDs []Entity
	curMatchIdx  int // index into matchingArches
	curIdx       int // index into the current archetype's entity/component array
	compSizes    [{{.N}}]uintptr
	curArchSize  int
	ids          [{{.N}}]uint8
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
		ids: [{{.N}}]uint8{ {{range $i, $e := .Components}}{{if $i}}, {{end}}id{{$e.Index}}{{end}} },
		curMatchIdx: 0,
		curIdx:      -1,
	}
	{{range $i, $e := .Components}}f.compSizes[{{$i}}] = w.components.compIDToSize[id{{$e.Index}}]
	{{end}}
	f.updateMatching()
	f.updateCachedEntities()
	f.Reset()
	return f
}

// New is a convenience method that constructs a new `Filter` instance for the
// same component types, equivalent to calling `NewFilter{{.N}}`.
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
		for i := 0; i < {{.N}}; i++ {
			f.curBases[i] = a.compPointers[f.ids[i]]
		}
		f.curEntityIDs = a.entityIDs
		f.curArchSize = a.size
	} else {
		f.curArchSize = 0
	}
}

// Next advances the filter to the next matching entity. It returns true if an
// entity was found, and false if the iteration is complete. This method must
// be called before accessing the entity or its components.
//
// Returns:
//   - true if another matching entity was found, false otherwise.
func (f *Filter{{.N}}[{{.TypeVars}}]) Next() bool {
	f.curIdx++
	if f.curIdx < f.curArchSize {
		return true
	}
	f.curMatchIdx++
	if f.curMatchIdx >= len(f.matchingArches) {
		return false
	}
	a := f.matchingArches[f.curMatchIdx]
	for i := 0; i < {{.N}}; i++ {
		f.curBases[i] = a.compPointers[f.ids[i]]
	}
	f.curEntityIDs = a.entityIDs
	f.curArchSize = a.size
	f.curIdx = 0
	return true
}

// Entity returns the current `Entity` in the iteration. This should only be
// called after `Next()` has returned true.
//
// Returns:
//   - The current Entity.
func (f *Filter{{.N}}[{{.TypeVars}}]) Entity() Entity {
	return f.curEntityIDs[f.curIdx]
}

// Get returns pointers to the {{.N}} components ({{.TypeVars}}) for the
// current entity in the iteration. This should only be called after `Next()`
// has returned true.
//
// Returns:
//   - Pointers to the component data ({{.ReturnTypes}}).
func (f *Filter{{.N}}[{{.TypeVars}}]) Get() ({{.ReturnTypes}}) {
	{{range $i, $e := .Components}}{{$e.PtrName}} := unsafe.Pointer(uintptr(f.curBases[{{$i}}]) + uintptr(f.curIdx)*f.compSizes[{{$i}}])
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
		for i := 0; i < a.size; i++ {
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
