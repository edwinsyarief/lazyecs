// GetComponents{{.N}} returns pointers to the components of type {{.TypeVars}} for the entity, or nil if not present or invalid.
func GetComponents{{.N}}[{{.Types}}](w *World, e Entity) ({{.ReturnTypes}}, bool) {
	if !w.IsValid(e) {
		return {{.ReturnNil}}, false
	}
	{{range .Components}}t{{.Index}} := reflect.TypeFor[{{.TypeName}}]()
	{{end}}
	{{range .Components}}id{{.Index}} := w.getCompTypeID(t{{.Index}})
	{{end}}
	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in GetComponents{{.N}}")
	}
	meta := w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	var m bitmask256
	{{range .Components}}m.set(id{{.Index}})
	{{end}}
	if !a.mask.contains(m) {
		return {{.ReturnNil}}, false
	}
	{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(a.compPointers[id{{.Index}}]) + uintptr(meta.index)*a.compSizes[id{{.Index}}])
	{{end}}
	return {{.ReturnPtrs}}, true
}

// SetComponents{{.N}} sets the components of type {{.TypeVars}} on the entity, adding them if not present.
func SetComponents{{.N}}[{{.Types}}](w *World, e Entity, {{.Vars}}) {
	if !w.IsValid(e) {
		return
	}
	{{range .Components}}t{{.Index}} := reflect.TypeFor[{{.TypeName}}]()
	{{end}}
	{{range .Components}}id{{.Index}} := w.getCompTypeID(t{{.Index}})
	{{end}}
	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in SetComponents{{.N}}")
	}
	meta := &w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	var setMask bitmask256
	{{range .Components}}setMask.set(id{{.Index}})
	{{end}}
	if a.mask.contains(setMask) {
		// already has all, just set
		{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(a.compPointers[id{{.Index}}]) + uintptr(meta.index)*a.compSizes[id{{.Index}}])
		*(*{{.TypeName}})({{.PtrName}}) = {{.VarName}}
		{{end}}
		return
	}
	newMask := a.mask
	{{range .Components}}newMask.set(id{{.Index}})
	{{end}}
	var tempSpecs [MaxComponentTypes]compSpec
	count := 0
	for wi := 0; wi < 4; wi++ {
		word := newMask[wi]
		for word != 0 {
			bit := bits.TrailingZeros64(word)
			cid := uint8(wi*64 + bit)
			typ := w.compIDToType[cid]
			tempSpecs[count] = compSpec{typ, typ.Size(), cid}
			count++
			word &= word - 1 // clear lowest set bit
		}
	}
	specs := tempSpecs[:count]
	targetA := w.getOrCreateArchetype(newMask, specs)
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		src := unsafe.Pointer(uintptr(a.compPointers[cid]) + uintptr(meta.index)*a.compSizes[cid])
		dst := unsafe.Pointer(uintptr(targetA.compPointers[cid]) + uintptr(newIdx)*targetA.compSizes[cid])
		memCopy(dst, src, a.compSizes[cid])
	}
	{{range .Components}}
	{
		var singleMask bitmask256
		singleMask.set(id{{.Index}})
		if !a.mask.contains(singleMask) {
			dst := unsafe.Pointer(uintptr(targetA.compPointers[id{{.Index}}]) + uintptr(newIdx)*targetA.compSizes[id{{.Index}}])
			*(*{{.TypeName}})(dst) = {{.VarName}}
		}
	}
	{{end}}
	w.removeFromArchetype(a, meta)
	meta.archetypeIndex = targetA.index
	meta.index = newIdx
}

// RemoveComponents{{.N}} removes the components of type {{.TypeVars}} from the entity if present.
func RemoveComponents{{.N}}[{{.Types}}](w *World, e Entity) {
	if !w.IsValid(e) {
		return
	}
	{{range .Components}}t{{.Index}} := reflect.TypeFor[{{.TypeName}}]()
	{{end}}
	{{range .Components}}id{{.Index}} := w.getCompTypeID(t{{.Index}})
	{{end}}
	if {{.DuplicateIDs}} {
		panic("ecs: duplicate component types in RemoveComponents{{.N}}")
	}
	meta := &w.metas[e.ID]
	a := w.archetypes[meta.archetypeIndex]
	var removeMask bitmask256
	{{range .Components}}removeMask.set(id{{.Index}})
	{{end}}
	if !a.mask.intersects(removeMask) {
		return
	}
	newMask := a.mask
	{{range .Components}}newMask.unset(id{{.Index}})
	{{end}}
	var tempSpecs [MaxComponentTypes]compSpec
	count := 0
	for wi := 0; wi < 4; wi++ {
		word := newMask[wi]
		for word != 0 {
			bit := bits.TrailingZeros64(word)
			cid := uint8(wi*64 + bit)
			typ := w.compIDToType[cid]
			tempSpecs[count] = compSpec{typ, typ.Size(), cid}
			count++
			word &= word - 1 // clear lowest set bit
		}
	}
	specs := tempSpecs[:count]
	targetA := w.getOrCreateArchetype(newMask, specs)
	newIdx := targetA.size
	targetA.entityIDs[newIdx] = e
	targetA.size++
	for _, cid := range a.compOrder {
		var bm bitmask256
		bm.set(cid)
		if removeMask.contains(bm) {
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