// AddComponent{{.N}} adds {{.N}} components to an entity if not already present.
// It returns pointers to the components (existing or new) and a boolean indicating success.
func AddComponent{{.N}}[{{.Types}}](w *World, e Entity) ({{.ReturnSinglePtrs}}, bool) {
	if int(e.ID) >= len(w.entitiesSlice) {
		return {{.ReturnNil}}, false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return {{.ReturnNil}}, false
	}

	{{range .Components}}{{.IDName}}, {{.OKName}} := TryGetID[{{.TypeName}}]()
	{{end}}
	if {{.OKIDs}} {
		return {{.ReturnNil}}, false
	}
	{{range .Components}}{{.SizeName}} := int(componentSizes[{{.IDName}}])
	{{end}}
	oldArch := meta.Archetype
	addMask := makeMask{{.N}}({{.IDs}})
	if includesAll(oldArch.mask, addMask) {
		{{range .Components}}{{.SlotName}} := oldArch.getSlot({{.IDName}})
		{{end}}
		if {{.SlotCheckCondition}} {
			return {{$.ReturnNil}}, false
		}
		{{range .Components}}{{.BytesName}} := oldArch.componentData[{{.SlotName}}]
		if meta.Index*{{.SizeName}} >= len({{.BytesName}}) {
			return {{$.ReturnNil}}, false
		}
		{{end}}
		return {{$.ReturnPtrsFromBytes}}, true
	}

	newMask := orMask(oldArch.mask, addMask)

	var transition Transition
	addMap, ok := w.addTransitions[oldArch]
	if ok {
		if tr, ok := addMap[addMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.addTransitions[oldArch]; !ok {
			w.addTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.addTransitions[oldArch][addMask] = transition
	} else {
		newArch = transition.target
	}

	oldIndex := meta.Index
	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

	ids := []ComponentID{ {{.IDs}} }
	for _, id := range ids {
		if !oldArch.mask.has(id) {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return {{$.ReturnNil}}, false
			}
			bytes := newArch.componentData[idx]
			bytes = extendByteSlice(bytes, int(componentSizes[id]))
			newArch.componentData[idx] = bytes
		}
	}

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entitiesSlice[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	{{range .Components}}{{.SlotName}} := newArch.getSlot({{.IDName}})
	{{end}}
	if {{.SlotCheckCondition}} {
		return {{.ReturnNil}}, false
	}
	{{range .Components}}{{.BytesName}} := newArch.componentData[{{.SlotName}}]
	{{end}}
	return {{range .Components}}(*{{.TypeName}})(unsafe.Pointer(&{{.BytesName}}[newIndex*{{.SizeName}}])), {{end}}true
}

// SetComponent{{.N}} sets {{.N}} components for an entity.
// If any component is missing, it adds them; otherwise, updates existing ones.
// It returns a boolean indicating success.
func SetComponent{{.N}}[{{.Types}}](w *World, e Entity, {{.Vars}}) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	{{range .Components}}{{.IDName}}, {{.OKName}} := TryGetID[{{.TypeName}}]()
	{{end}}
	if {{.OKIDs}} {
		return false
	}
	{{range .Components}}{{.SizeName}} := int(componentSizes[{{.IDName}}])
	{{end}}
	{{range .Components}}{{.SrcName}} := unsafe.Slice((*byte)(unsafe.Pointer(&{{.VarName}})), {{.SizeName}})
	{{end}}
	oldArch := meta.Archetype
	setMask := makeMask{{.N}}({{.IDs}})
	if includesAll(oldArch.mask, setMask) {
		{{range .Components}}{{.SlotName}} := oldArch.getSlot({{.IDName}})
		{{end}}
		if {{.SlotCheckCondition}} {
			return false
		}
		{{range .Components}}{{.BytesName}} := oldArch.componentData[{{.SlotName}}]
		if meta.Index*{{.SizeName}} >= len({{.BytesName}}) {
			return false
		}
		copy({{.BytesName}}[meta.Index*{{.SizeName}}:(meta.Index+1)*{{.SizeName}}], {{.SrcName}})
		{{end}}
		return true
	} else {
		newMask := orMask(oldArch.mask, setMask)
		var transition Transition
		addMap, ok := w.addTransitions[oldArch]
		if ok {
			if tr, ok := addMap[setMask]; ok {
				transition = tr
			}
		}
		var newArch *Archetype
		if transition.target == nil {
			newArch = w.getOrCreateArchetype(newMask)
			copies := make([]CopyOp, 0, len(oldArch.componentIDs))
			for from, id := range oldArch.componentIDs {
				to := newArch.getSlot(id)
				if to >= 0 {
					copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
				}
			}
			transition = Transition{target: newArch, copies: copies}
			if _, ok := w.addTransitions[oldArch]; !ok {
				w.addTransitions[oldArch] = make(map[maskType]Transition)
			}
			w.addTransitions[oldArch][setMask] = transition
		} else {
			newArch = transition.target
		}

		oldIndex := meta.Index
		newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

		ids := []ComponentID{ {{.IDs}} }
		srcs := [][]byte{ {{range .Components}}{{.SrcName}}{{if ne .Index $.N}}, {{end}}{{end}} }
		sizes := []int{ {{range .Components}}{{.SizeName}}{{if ne .Index $.N}}, {{end}}{{end}} }
		for i, id := range ids {
			idx := newArch.getSlot(id)
			if idx == -1 {
				return false
			}
			bytes := newArch.componentData[idx]
			if oldArch.mask.has(id) {
				copy(bytes[newIndex*sizes[i]:(newIndex+1)*sizes[i]], srcs[i])
			} else {
				bytes = extendByteSlice(bytes, sizes[i])
				copy(bytes[len(bytes)-sizes[i]:], srcs[i])
				newArch.componentData[idx] = bytes
			}
		}

		meta.Archetype = newArch
		meta.Index = newIndex
		w.entitiesSlice[e.ID] = meta

		w.removeEntityFromArchetype(e, oldArch, oldIndex)
		return true
	}
}

// RemoveComponent{{.N}} removes {{.N}} components from an entity if present.
// It returns a boolean indicating success.
func RemoveComponent{{.N}}[{{.Types}}](w *World, e Entity) bool {
	if int(e.ID) >= len(w.entitiesSlice) {
		return false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return false
	}

	{{range .Components}}{{.IDName}}, {{.OKName}} := TryGetID[{{.TypeName}}]()
	{{end}}
	if {{.OKIDs}} {
		return false
	}

	oldArch := meta.Archetype
	removeMask := makeMask{{.N}}({{.IDs}})
	if !intersects(oldArch.mask, removeMask) {
		return true
	}

	oldIndex := meta.Index

	newMask := andNotMask(oldArch.mask, removeMask)

	var transition Transition
	removeMap, ok := w.removeTransitions[oldArch]
	if ok {
		if tr, ok := removeMap[removeMask]; ok {
			transition = tr
		}
	}
	var newArch *Archetype
	if transition.target == nil {
		newArch = w.getOrCreateArchetype(newMask)
		copies := make([]CopyOp, 0, len(oldArch.componentIDs))
		for from, id := range oldArch.componentIDs {
			if removeMask.has(id) {
				continue
			}
			to := newArch.getSlot(id)
			if to >= 0 {
				copies = append(copies, CopyOp{from: from, to: to, size: int(componentSizes[id])})
			}
		}
		transition = Transition{target: newArch, copies: copies}
		if _, ok := w.removeTransitions[oldArch]; !ok {
			w.removeTransitions[oldArch] = make(map[maskType]Transition)
		}
		w.removeTransitions[oldArch][removeMask] = transition
	} else {
		newArch = transition.target
	}

	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, transition.copies)

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entitiesSlice[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	return true
}