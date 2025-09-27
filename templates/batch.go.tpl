// Batch{{.N}} provides a way to create entities with a pre-defined set of components.
type Batch{{.N}}[{{.Types}}] struct {
	world *World
	arch  *Archetype
	{{range .Components}}{{.IDName}} ComponentID
	{{end}}
	{{range .Components}}{{.SizeName}} int
	{{end}}
}

// CreateBatch{{.N}} creates a new Batch for creating entities with {{.N}} component type(s).
func CreateBatch{{.N}}[{{.Types}}](w *World) *Batch{{.N}}[{{.TypeVars}}] {
	{{range .Components}}{{.IDName}}, {{.OKName}} := TryGetID[{{.TypeName}}]()
	{{end}}
	if {{.OKIDs}} {
		panic("one or more components in CreateBatch{{.N}} are not registered")
	}
	mask := makeMask{{.N}}({{.IDs}})
	arch := w.getOrCreateArchetype(mask)
	return &Batch{{.N}}[{{.TypeVars}}]{
		world: w,
		arch:  arch,
		{{range .Components}}{{.IDName}}: {{.IDName}},
		{{end}}
		{{range .Components}}{{.SizeName}}: int(componentSizes[{{.IDName}}]),
		{{end}}
	}
}

// CreateEntities creates a specified number of entities with the batch's components.
// The created components are zero-initialized.
func (self *Batch{{.N}}[{{.TypeVars}}]) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}
	w := self.world
	arch := self.arch

	entities := make([]Entity, count)
	startIndex := len(arch.entities)
	arch.entities = extendSlice(arch.entities, count)

	{{range .Components}}arch.componentData[arch.getSlot(self.{{.IDName}})] = extendByteSlice(arch.componentData[arch.getSlot(self.{{.IDName}})], count*self.{{.SizeName}})
	{{end}}
	maxID := uint32(0)
	for i := 0; i < count; i++ {
		var id uint32
		if len(w.freeEntityIDs) > 0 {
			id = w.freeEntityIDs[len(w.freeEntityIDs)-1]
			w.freeEntityIDs = w.freeEntityIDs[:len(w.freeEntityIDs)-1]
		} else {
			id = w.nextEntityID
			w.nextEntityID++
		}
		if id > maxID {
			maxID = id
		}
		version := uint32(1)
		if int(id) < len(w.entitiesSlice) {
			version = w.entitiesSlice[id].Version + 1
			if version == 0 {
				version = 1
			}
		}
		e := Entity{ID: id, Version: version}
		entities[i] = e
		arch.entities[startIndex+i] = e
	}

	if int(maxID) >= len(w.entitiesSlice) {
		w.entitiesSlice = extendSlice(w.entitiesSlice, int(maxID)-len(w.entitiesSlice)+1)
	}

	for i := 0; i < count; i++ {
		e := entities[i]
		idx := startIndex + i
		w.entitiesSlice[e.ID] = entityMeta{Archetype: arch, Index: idx, Version: e.Version}
	}
	return entities
}

// CreateEntitiesWithComponents creates entities with the specified component values.
func (self *Batch{{.N}}[{{.TypeVars}}]) CreateEntitiesWithComponents(count int, {{.Vars}}) []Entity {
	entities := self.CreateEntities(count)
	if len(entities) == 0 {
		return nil
	}
	arch := self.arch
	{{range .Components}}{{.SlotName}} := arch.getSlot(self.{{.IDName}})
	{{end}}
	{{range .Components}}data{{.Index}} := arch.componentData[{{.SlotName}}]
	{{end}}
	{{range .Components}}{{.SrcName}} := unsafe.Slice((*byte)(unsafe.Pointer(&{{.VarName}})), self.{{.SizeName}})
	{{end}}
	{{range .Components}}startIndex{{.Index}} := len(data{{.Index}}) - count*self.{{.SizeName}}
	{{end}}
	for i := 0; i < count; i++ {
		{{range .Components}}copy(data{{.Index}}[startIndex{{.Index}}+i*self.{{.SizeName}}:], {{.SrcName}})
		{{end}}
	}
	return entities
}