// This template generates N-ary functions for the public World API, specifically
// for getting, setting, and removing multiple components at once. These functions
// (e.g., GetComponent2, SetComponent3) provide a convenient and type-safe way to
// interact with entities that have several components, abstracting away the
// underlying archetype and bitmask operations.
//
// Placeholders:
// - .N: The number of components (e.g., 2, 3).
// - .Types: The generic type parameters, e.g., "T1 any, T2 any".
// - .TypeVars: The type names themselves, e.g., "T1, T2".
// - .Vars: Variable declarations for SetComponentN, e.g., "v1 T1, v2 T2".
// - .ReturnTypes: Pointer return types for GetComponentN, e.g., "*T1, *T2".
// - .ReturnNil: A list of nil values for returns, e.g., "nil, nil".
// - .MaskCheck: The condition to check if an archetype has all the components.
// - .HasAll: A boolean check for SetComponentN, e.g., "has1 && has2".
// - .IsRemovedID: A condition to check if a component is being removed.
// GetComponent{{.N}} retrieves pointers to the {{.N}} components of type
// ({{.TypeVars}}) for the given entity.
//
// If the entity is invalid or does not have all the requested components, this
// function returns nil for all pointers.
//
// Parameters:
//   - w: The World containing the entity.
//   - e: The Entity from which to retrieve the components.
//
// Returns:
//   - Pointers to the component data ({{.ReturnTypes}}), or nils if not found.
func GetComponent{{.N}}[{{.Types}}](w *World, e Entity) ({{.ReturnTypes}}) {
	if !w.IsValid(e) {
		return {{.ReturnNil}}
	}
	meta := w.metas[e.ID]
	{{range .Components}}t{{.Index}} := reflect.TypeFor[{{.TypeName}}]()
	{{end}}
	{{range .Components}}id{{.Index}} := w.getCompTypeID(t{{.Index}})
	{{end}}
	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in GetComponent{{.N}}")
	}
	a := w.archetypes[meta.archetypeIndex]
	{{range .Components}}i{{.Index}} := id{{.Index}} >> 6
	o{{.Index}} := id{{.Index}} & 63
	{{end}}
	if {{.MaskCheck}} {
		return {{.ReturnNil}}
	}
	{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(a.compPointers[id{{.Index}}]) + uintptr(meta.index)*a.compSizes[id{{.Index}}])
	{{end}}
	return {{.ReturnPtrs}}
}

// SetComponent{{.N}} adds or updates the {{.N}} components ({{.TypeVars}}) on the
// specified entity.
//
// If the entity does not already have all the components, this operation will
// cause the entity to move to a different archetype. If the entity is invalid,
// this function does nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
{{range .Components}}//   - v{{.Index}}: The component data of type {{.TypeName}} to set.
{{end}}
func SetComponent{{.N}}[{{.Types}}](w *World, e Entity, {{.Vars}}) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.metas[e.ID]
	{{range .Components}}t{{.Index}} := reflect.TypeFor[{{.TypeName}}]()
	{{end}}
	{{range .Components}}id{{.Index}} := w.getCompTypeID(t{{.Index}})
	{{end}}
	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in SetComponent{{.N}}")
	}
	a := w.archetypes[meta.archetypeIndex]
	{{range .Components}}i{{.Index}} := id{{.Index}} >> 6
	o{{.Index}} := id{{.Index}} & 63
	{{end}}
	{{range .Components}}has{{.Index}} := (a.mask[i{{.Index}}] & (uint64(1) << uint64(o{{.Index}}))) != 0
	{{end}}
	if {{.HasAll}} {
		{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(a.compPointers[id{{.Index}}]) + uintptr(meta.index)*a.compSizes[id{{.Index}}])
		*(*{{.TypeName}})({{.PtrName}}) = {{.VarName}}
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
	{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(targetA.compPointers[id{{.Index}}]) + uintptr(newIdx)*targetA.compSizes[id{{.Index}}])
	*(*{{.TypeName}})({{.PtrName}}) = {{.VarName}}
	{{end}}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
}

// RemoveComponent{{.N}} removes the {{.N}} components ({{.TypeVars}}) from the
// specified entity.
//
// This operation will cause the entity to move to a new archetype. If the
// entity is invalid or does not have all the components, this function does
// nothing.
//
// Parameters:
//   - w: The World where the entity resides.
//   - e: The Entity to modify.
func RemoveComponent{{.N}}[{{.Types}}](w *World, e Entity) {
	if !w.IsValid(e) {
		return
	}
	meta := &w.metas[e.ID]
	{{range .Components}}t{{.Index}} := reflect.TypeFor[{{.TypeName}}]()
	{{end}}
	{{range .Components}}id{{.Index}} := w.getCompTypeID(t{{.Index}})
	{{end}}
	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in RemoveComponent{{.N}}")
	}
	a := w.archetypes[meta.archetypeIndex]
	{{range .Components}}i{{.Index}} := id{{.Index}} >> 6
	o{{.Index}} := id{{.Index}} & 63
	{{end}}
	{{range .Components}}has{{.Index}} := (a.mask[i{{.Index}}] & (uint64(1) << uint64(o{{.Index}}))) != 0
	{{end}}
	if {{.HasNone}} {
		return
	}
	newMask := a.mask
	{{range .Components}}newMask.unset(id{{.Index}})
	{{end}}
	var targetA *archetype
	if idx, ok := w.maskToArcIndex[newMask]; ok {
		targetA = w.archetypes[idx]
	} else {
		var tempSpecs [MaxComponentTypes]compSpec
		count := 0
		for _, cid := range a.compOrder {
			if {{.IsRemovedID}} {
				continue
			}
			tempSpecs[count] = compSpec{id: cid, typ: w.compIDToType[cid], size: w.compIDToSize[cid]}
			count++
		}
		specs := tempSpecs[:count]
		targetA = w.getOrCreateArchetype(newMask, specs)
	}
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		if {{.IsRemovedID}} {
			continue
		}
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
}
