// GetComponent{{.N}} returns pointers to the components of type {{.TypeVars}} for the entity, or nil if not present or invalid.
func GetComponent{{.N}}[{{.Types}}](w *World, e Entity) ({{.ReturnTypes}}) {
	if int(e.ID) >= len(w.metas) {
		return {{.ReturnNil}}
	}
	meta := w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return {{.ReturnNil}}
	}
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

// SetComponent{{.N}} sets the components of type {{.TypeVars}} on the entity, adding them if not present.
func SetComponent{{.N}}[{{.Types}}](w *World, e Entity, {{.Vars}}) {
	if int(e.ID) >= len(w.metas) {
		return
	}
	meta := &w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return
	}
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

// RemoveComponent{{.N}} removes the components of type {{.TypeVars}} from the entity if present.
func RemoveComponent{{.N}}[{{.Types}}](w *World, e Entity) {
	if int(e.ID) >= len(w.metas) {
		return
	}
	meta := &w.metas[e.ID]
	if meta.version == 0 || meta.version != e.Version {
		return
	}
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