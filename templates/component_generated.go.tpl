// GetComponent{{.N}} retrieves {{.N}} components for an entity.
// It returns the component values directly and a boolean indicating success.
// This function is designed for high-performance read-only access and is allocation-free.
func GetComponent{{.N}}[{{.Types}}](w *World, e Entity) ({{.TypeVars}}, bool) {
	{{range .Components}}var {{.VarName}} {{.TypeName}}
	{{end}}
	if int(e.ID) >= len(w.entitiesSlice) {
		return {{.ReturnVars}}, false
	}
	meta := w.entitiesSlice[e.ID]
	if meta.Version != e.Version {
		return {{.ReturnVars}}, false
	}

	{{range .Components}}{{.IDName}}, {{.OKName}} := TryGetID[{{.TypeName}}]()
	{{end}}
	if {{.OKIDs}} {
		return {{.ReturnVars}}, false
	}

	arch := meta.Archetype
	mask := makeMask{{.N}}({{.IDs}})
	if !includesAll(arch.mask, mask) {
		return {{.ReturnVars}}, false
	}

	{{range .Components}}{{.SlotName}} := arch.getSlot({{.IDName}})
	{{end}}
	{{range .Components}}{{.SizeName}} := int(componentSizes[{{.IDName}}])
	{{end}}
	{{range .Components}}{{.BytesName}} := arch.componentData[{{.SlotName}}]
	{{end}}
	{{range .Components}}{{.VarName}} = *(*{{.TypeName}})(unsafe.Pointer(&{{.BytesName}}[meta.Index*{{.SizeName}}]))
	{{end}}
	return {{.ReturnVars}}, true
}