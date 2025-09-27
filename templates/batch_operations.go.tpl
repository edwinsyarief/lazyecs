// AddComponentBatch{{.N}} adds {{.N}} components to multiple entities.
// It returns pointers to the components in order of the input entities.
func AddComponentBatch{{.N}}[{{.Types}}](w *World, entities []Entity) ({{.BatchRes}}) {
	{{range .Components}}{{.IDName}}, {{.OKName}} := TryGetID[{{.TypeName}}]()
	{{end}}
	if {{.OKIDs}} {
		return {{.ReturnNil}}
	}
	addMask := makeMask{{.N}}({{.IDs}})
	{{range .Components}}{{.SizeName}} := int(componentSizes[{{.IDName}}])
	{{end}}
	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	{{range .Components}}res{{.Index}} := make([]*{{.TypeName}}, len(entities))
	{{end}}
	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, addMask) {
			{{range .Components}}{{.SlotName}} := oldArch.getSlot({{.IDName}})
			{{end}}
			if {{$.SlotCheckCondition}} {
				continue
			}
			{{range .Components}}{{.BaseName}} := unsafe.Pointer(&oldArch.componentData[{{.SlotName}}][0])
			{{end}}
			{{range .Components}}{{.StrideName}} := uintptr({{.SizeName}})
			{{end}}
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr({{.BaseName}}) + uintptr(meta.Index)*{{.StrideName}})
				res{{.Index}}[gi] = (*{{.TypeName}})({{.PtrName}})
				{{end}}
			}
			continue
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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		{{range .Components}}{{.SlotName}} := newArch.getSlot({{.IDName}})
		{{end}}
		{{range .Components}}{{.BaseName}} := unsafe.Pointer(&newArch.componentData[{{.SlotName}}][0])
		{{end}}
		{{range .Components}}{{.StrideName}} := uintptr({{.SizeName}})
		{{end}}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr({{.BaseName}}) + uintptr(newIndex)*{{.StrideName}})
			res{{.Index}}[gi] = (*{{.TypeName}})({{.PtrName}})
			{{end}}

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}

	return {{.ReturnBatchRes}}
}

// SetComponentBatch{{.N}} sets {{.N}} components to the same values for multiple entities.
// If any component is missing in some entities, it adds them.
func SetComponentBatch{{.N}}[{{.Types}}](w *World, entities []Entity, {{.Vars}}) {
	{{range .Components}}{{.IDName}}, {{.OKName}} := TryGetID[{{.TypeName}}]()
	{{end}}
	if {{.OKIDs}} {
		return
	}
	setMask := makeMask{{.N}}({{.IDs}})
	{{range .Components}}{{.SizeName}} := int(componentSizes[{{.IDName}}])
	{{end}}
	{{range .Components}}{{.SrcName}} := unsafe.Slice((*byte)(unsafe.Pointer(&{{.VarName}})), {{.SizeName}})
	{{end}}
	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if includesAll(oldArch.mask, setMask) {
			{{range .Components}}{{.SlotName}} := oldArch.getSlot({{.IDName}})
			{{end}}
			if {{$.SlotCheckCondition}} {
				continue
			}
			{{range .Components}}{{.BytesName}} := oldArch.componentData[{{.SlotName}}]
			{{end}}
			for k := start; k < i; k++ {
				gi := temp[k].idx
				meta := w.entitiesSlice[entities[gi].ID]
				{{range .Components}}copy({{.BytesName}}[meta.Index*{{.SizeName}}:(meta.Index+1)*{{.SizeName}}], {{.SrcName}})
				{{end}}
			}
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, id := range newArch.componentIDs {
			csize := int(componentSizes[id])
			newArch.componentData[newArch.getSlot(id)] = extendByteSlice(newArch.componentData[newArch.getSlot(id)], num*csize)
		}

		{{range .Components}}{{.SlotName}} := newArch.getSlot({{.IDName}})
		{{end}}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				srcCopy := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], srcCopy)
			}

			{{range .Components}}{{.BytesName}} := newArch.componentData[{{.SlotName}}]
			dstStart{{.Index}} := len({{.BytesName}}) - num*{{.SizeName}} + j*{{.SizeName}}
			copy({{.BytesName}}[dstStart{{.Index}}:dstStart{{.Index}}+{{.SizeName}}], {{.SrcName}})
			{{end}}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}

// RemoveComponentBatch{{.N}} removes {{.N}} components from multiple entities if present.
func RemoveComponentBatch{{.N}}[{{.Types}}](w *World, entities []Entity) {
	{{range .Components}}{{.IDName}}, {{.OKName}} := TryGetID[{{.TypeName}}]()
	{{end}}
	if {{.OKIDs}} {
		return
	}
	removeMask := makeMask{{.N}}({{.IDs}})

	type entry struct {
		idx  int
		arch *Archetype
	}
	temp := make([]entry, len(entities))
	numValid := 0
	for i, e := range entities {
		if int(e.ID) >= len(w.entitiesSlice) {
			continue
		}
		meta := w.entitiesSlice[e.ID]
		if meta.Version != e.Version {
			continue
		}
		temp[numValid] = entry{idx: i, arch: meta.Archetype}
		numValid++
	}
	temp = temp[:numValid]
	sort.Slice(temp, func(i, j int) bool {
		return uintptr(unsafe.Pointer(temp[i].arch)) < uintptr(unsafe.Pointer(temp[j].arch))
	})

	var pairs []removePair

	i := 0
	for i < numValid {
		oldArch := temp[i].arch
		start := i
		for i < numValid && temp[i].arch == oldArch {
			i++
		}
		groupSize := i - start
		if groupSize == 0 {
			continue
		}

		if !intersects(oldArch.mask, removeMask) {
			continue
		}

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

		num := groupSize
		startNew := len(newArch.entities)
		newArch.entities = extendSlice(newArch.entities, num)

		for _, op := range transition.copies {
			newArch.componentData[op.to] = extendByteSlice(newArch.componentData[op.to], num*op.size)
		}

		pairs = pairs[:0]
		pairs = extendSlice(pairs, num)

		j := 0
		for k := start; k < i; k++ {
			gi := temp[k].idx
			e := entities[gi]
			meta := w.entitiesSlice[e.ID]
			oldIndex := meta.Index
			newIndex := startNew + j
			newArch.entities[newIndex] = e

			for _, op := range transition.copies {
				oldBytes := oldArch.componentData[op.from]
				src := oldBytes[oldIndex*op.size : (oldIndex+1)*op.size]
				dstBytes := newArch.componentData[op.to]
				dstStart := len(dstBytes) - num*op.size + j*op.size
				copy(dstBytes[dstStart:dstStart+op.size], src)
			}

			meta.Archetype = newArch
			meta.Index = newIndex
			w.entitiesSlice[e.ID] = meta

			pairs[j] = removePair{index: oldIndex, e: e}
			j++
		}

		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].index > pairs[b].index
		})
		for _, pair := range pairs {
			w.removeEntityFromArchetype(pair.e, oldArch, pair.index)
		}
	}
}