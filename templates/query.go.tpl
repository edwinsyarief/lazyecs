// Query{{.N}} is an iterator over entities that have a specific set of components.
// This query is for entities with {{.N}} component type(s).
type Query{{.N}}[{{.Types}}] struct {
	world         *World
	includeMask   maskType
	excludeMask   maskType
	{{range .Components}}{{.IDName}}          ComponentID
	{{end}}
	archIdx       int
	index         int
	currentArch   *Archetype
	{{range .Components}}{{.BaseName}}   unsafe.Pointer
	{{end}}
	{{range .Components}}{{.StrideName}} uintptr
	{{end}}
	currentEntity Entity
}

// Reset resets the query for reuse.
func (self *Query{{.N}}[{{.TypeVars}}]) Reset() {
	self.archIdx = 0
	self.index = -1
	self.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (self *Query{{.N}}[{{.TypeVars}}]) Next() bool {
	self.index++
	if self.currentArch != nil && self.index < len(self.currentArch.entities) {
		self.currentEntity = self.currentArch.entities[self.index]
		return true
	}

	for self.archIdx < len(self.world.archetypesList) {
		arch := self.world.archetypesList[self.archIdx]
		self.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, self.includeMask) || intersects(arch.mask, self.excludeMask) {
			continue
		}
		self.currentArch = arch
		{{range .Components}}{{.SlotName}} := arch.getSlot(self.{{.IDName}})
		{{end}}
		if {{.SlotCheckCondition}} {
			panic("missing component in matching archetype")
		}
		{{range .Components}}if len(arch.componentData[{{.SlotName}}]) > 0 {
			self.{{.BaseName}} = unsafe.Pointer(&arch.componentData[{{.SlotName}}][0])
		} else {
			self.{{.BaseName}} = nil
		}
		self.{{.StrideName}} = componentSizes[self.{{.IDName}}]
		{{end}}
		self.index = 0
		self.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (self *Query{{.N}}[{{.TypeVars}}]) Get() ({{.ReturnTypes}}) {
	{{range .Components}}{{.PtrName}} := unsafe.Pointer(uintptr(self.{{.BaseName}}) + uintptr(self.index)*self.{{.StrideName}})
	{{end}}
	return {{.ReturnPtrs}}
}

// Entity returns the current entity.
func (self *Query{{.N}}[{{.TypeVars}}]) Entity() Entity {
	return self.currentEntity
}

// CreateQuery{{.N}} creates a new query for entities with {{.N}} specific component type(s).
// It allows specifying component types to exclude from the query results.
func CreateQuery{{.N}}[{{.Types}}](w *World, excludes ...ComponentID) *Query{{.N}}[{{.TypeVars}}] {
	{{range .Components}}{{.IDName}} := GetID[{{.TypeName}}]()
	{{end}}
	return &Query{{.N}}[{{.TypeVars}}]{
		world:       w,
		includeMask: makeMask{{.N}}({{.IDs}}),
		excludeMask: makeMask(excludes),
		{{range .Components}}{{.IDName}}:          {{.IDName}},
		{{end}}
		archIdx:     0,
		index:       -1,
	}
}