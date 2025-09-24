package lazyecs

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
)

// ComponentID represents a unique identifier for a component type.
// We use uint32 to allow for a reasonable number of component types without wasting space.
type ComponentID uint32

const (
	bitsPerWord            = 64                      // Number of bits in a uint64 word.
	maskWords              = 4                       // Number of words in the bitmask; supports up to 256 components.
	maxComponentTypes      = maskWords * bitsPerWord // Maximum supported component types.
	defaultInitialCapacity = 4096                    // Default initial capacity for slices; can be overridden.
)

// maskType is a bitmask for component sets.
// This fixed-size array allows fast bit operations for checking component presence.
type maskType [maskWords]uint64

// has checks if the mask has the bit set for the given ID.
// It returns false if the ID is out of range to prevent panics.
func (m maskType) has(id ComponentID) bool {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		return false
	}
	bit := id % bitsPerWord
	return (m[word] & (1 << bit)) != 0
}

// set returns a new mask with the bit set for the given ID.
// Panics if the ID exceeds the maximum allowed.
func setMask(m maskType, id ComponentID) maskType {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		panic(fmt.Sprintf("component ID %d exceeds maximum (%d)", id, maxComponentTypes))
	}
	bit := id % bitsPerWord
	nm := m
	nm[word] |= (1 << bit)
	return nm
}

// unset returns a new mask with the bit unset for the given ID.
// If ID is out of range, it returns the original mask unchanged.
func unsetMask(m maskType, id ComponentID) maskType {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		return m
	}
	bit := id % bitsPerWord
	nm := m
	nm[word] &^= (1 << bit)
	return nm
}

// makeMask creates a mask from a list of IDs.
// It iterates over the IDs and sets the corresponding bits.
func makeMask(ids []ComponentID) maskType {
	var m maskType
	for _, id := range ids {
		word := int(id / bitsPerWord)
		bit := id % bitsPerWord
		m[word] |= (1 << bit)
	}
	return m
}

// includesAll checks if m includes all bits set in include.
// This is used to verify if an archetype matches a query's required components.
func includesAll(m, include maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & include[i]) != include[i] {
			return false
		}
	}
	return true
}

// intersects checks if m and exclude have any overlapping bits.
// Used for exclusion filters in queries.
func intersects(m, exclude maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & exclude[i]) != 0 {
			return true
		}
	}
	return false
}

var (
	registryMutex   sync.RWMutex                         // Protects the component registry from concurrent access.
	nextComponentID ComponentID                          // Next available component ID, starts at 0.
	typeToID        = make(map[reflect.Type]ComponentID) // Maps component types to their IDs.
	idToType        = make(map[ComponentID]reflect.Type) // Maps IDs back to component types.
)

// ResetGlobalRegistry resets the component registry for testing purposes.
// This clears all mappings and resets the next ID counter.
func ResetGlobalRegistry() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	nextComponentID = 0
	typeToID = make(map[reflect.Type]ComponentID)
	idToType = make(map[ComponentID]reflect.Type)
}

// RegisterComponent registers a component type and returns its unique ID.
// It panics if the type is already registered or if we've hit the max component limit.
func RegisterComponent[T any]() ComponentID {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	var t T
	compType := reflect.TypeOf(t)

	if id, ok := typeToID[compType]; ok {
		return id
	}

	if int(nextComponentID) >= maxComponentTypes {
		panic(fmt.Sprintf("cannot register component %s: maximum number of component types (%d) reached", compType.Name(), maxComponentTypes))
	}

	id := nextComponentID
	typeToID[compType] = id
	idToType[id] = compType
	nextComponentID++
	return id
}

// Entity is a unique identifier for an entity, including a version for safety.
// The version helps detect use-after-free errors.
type Entity struct {
	ID      uint32
	Version uint32
}

// entityMeta stores internal metadata for each entity.
// This includes its current archetype, index within it, and version.
type entityMeta struct {
	Archetype *Archetype
	Index     int
	Version   uint32
}

// WorldOptions allows configuring the World.
// Currently, it supports setting the initial capacity for archetypes.
type WorldOptions struct {
	InitialCapacity int // Initial capacity for slices; defaults to 4096 if <=0.
}

// World manages all entities, components, and systems.
// It uses archetypes for efficient storage and querying.
type World struct {
	mu              sync.RWMutex            // Main lock for world operations.
	archetypesMu    sync.RWMutex            // Separate lock for archetype map to reduce contention.
	nextEntityID    uint32                  // Next available entity ID.
	freeEntityIDs   []uint32                // Recycled entity IDs for reuse.
	entities        map[uint32]entityMeta   // Maps entity IDs to their metadata.
	archetypes      map[maskType]*Archetype // Maps component masks to archetypes.
	toRemove        []Entity                // Entities queued for removal.
	Resources       sync.Map                // General-purpose resource storage.
	initialCapacity int                     // Configurable initial capacity.
}

// NewWorld creates a new ECS world with default options.
// It initializes with an empty archetype.
func NewWorld() *World {
	return NewWorldWithOptions(WorldOptions{})
}

// NewWorldWithOptions creates a new ECS world with custom options.
// This allows setting initial capacities without changing the API.
func NewWorldWithOptions(opts WorldOptions) *World {
	cap := defaultInitialCapacity
	if opts.InitialCapacity > 0 {
		cap = opts.InitialCapacity
	}
	w := &World{
		entities:        make(map[uint32]entityMeta),
		archetypes:      make(map[maskType]*Archetype),
		toRemove:        make([]Entity, 0, cap),
		freeEntityIDs:   make([]uint32, 0, cap),
		initialCapacity: cap,
	}
	// Create the initial empty archetype with configurable capacity.
	w.getOrCreateArchetype(maskType{})
	return w
}

// getOrCreateArchetype finds or creates an archetype for a given mask.
// It uses double-checked locking to minimize contention.
func (self *World) getOrCreateArchetype(mask maskType) *Archetype {
	self.archetypesMu.RLock()
	if arch, ok := self.archetypes[mask]; ok {
		self.archetypesMu.RUnlock()
		return arch
	}
	self.archetypesMu.RUnlock()

	self.archetypesMu.Lock()
	defer self.archetypesMu.Unlock()

	// Double-check after lock.
	if arch, ok := self.archetypes[mask]; ok {
		return arch
	}

	newArch := &Archetype{
		mask:         mask,
		componentMap: make(map[ComponentID]int),
		entities:     make([]Entity, 0, self.initialCapacity),
	}

	registryMutex.RLock()
	defer registryMutex.RUnlock()

	compIDs := make([]ComponentID, 0, len(idToType))
	for id := range idToType {
		if mask.has(id) {
			compIDs = append(compIDs, id)
		}
	}

	sort.Slice(compIDs, func(i, j int) bool { return compIDs[i] < compIDs[j] })
	newArch.componentIDs = compIDs
	newArch.componentData = make([]any, len(compIDs))
	newArch.componentTypes = make([]reflect.Type, len(compIDs))

	for i, id := range compIDs {
		compType := idToType[id]
		newArch.componentMap[id] = i
		newArch.componentTypes[i] = compType
		sliceType := reflect.SliceOf(compType)
		slice := reflect.MakeSlice(sliceType, 0, self.initialCapacity)
		newArch.componentData[i] = slice.Interface()
	}

	self.archetypes[mask] = newArch
	return newArch
}

// CreateEntity creates a new entity with no components.
// It reuses free IDs if available to avoid fragmentation.
func (self *World) CreateEntity() Entity {
	self.mu.Lock()
	defer self.mu.Unlock()

	var id uint32
	if len(self.freeEntityIDs) > 0 {
		id = self.freeEntityIDs[len(self.freeEntityIDs)-1]
		self.freeEntityIDs = self.freeEntityIDs[:len(self.freeEntityIDs)-1]
	} else {
		if self.nextEntityID == ^uint32(0) {
			panic("entity ID overflow")
		}
		id = self.nextEntityID
		self.nextEntityID++
	}

	meta, exists := self.entities[id]
	version := uint32(1)
	if exists {
		version = meta.Version + 1
		if version == 0 {
			version = 1 // Wrap-around safety.
		}
	}

	e := Entity{ID: id, Version: version}
	arch := self.archetypes[maskType{}]
	index := len(arch.entities)

	self.entities[id] = entityMeta{Archetype: arch, Index: index, Version: e.Version}
	arch.entities = append(arch.entities, e)

	return e
}

// CreateEntities batch-creates multiple entities with no components.
// This is more efficient than calling CreateEntity in a loop.
func (self *World) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}
	self.mu.Lock()
	defer self.mu.Unlock()

	entities := make([]Entity, count)
	arch := self.archetypes[maskType{}]
	startIndex := len(arch.entities)

	for i := 0; i < count; i++ {
		var id uint32
		if len(self.freeEntityIDs) > 0 {
			id = self.freeEntityIDs[len(self.freeEntityIDs)-1]
			self.freeEntityIDs = self.freeEntityIDs[:len(self.freeEntityIDs)-1]
		} else {
			if self.nextEntityID == ^uint32(0) {
				panic("entity ID overflow")
			}
			id = self.nextEntityID
			self.nextEntityID++
		}

		meta, exists := self.entities[id]
		version := uint32(1)
		if exists {
			version = meta.Version + 1
			if version == 0 {
				version = 1
			}
		}

		e := Entity{ID: id, Version: version}
		entities[i] = e
		index := startIndex + i
		self.entities[id] = entityMeta{Archetype: arch, Index: index, Version: e.Version}
		arch.entities = append(arch.entities, e)
	}

	return entities
}

// RemoveEntity marks an entity for removal at the end of the frame.
// Actual removal is deferred to ProcessRemovals for batch processing.
func (self *World) RemoveEntity(e Entity) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.toRemove = append(self.toRemove, e)
}

// ProcessRemovals cleans up entities marked for removal.
// It processes them in batch to improve efficiency.
func (self *World) ProcessRemovals() {
	self.mu.Lock()
	defer self.mu.Unlock()

	if len(self.toRemove) == 0 {
		return
	}

	removeSet := make(map[uint32]Entity, len(self.toRemove))
	for _, e := range self.toRemove {
		meta, ok := self.entities[e.ID]
		if ok && e.Version == meta.Version {
			removeSet[e.ID] = e
		}
	}

	for id, e := range removeSet {
		if meta, ok := self.entities[id]; ok {
			self.removeEntityFromArchetype(e, meta.Archetype, meta.Index)
			self.freeEntityIDs = append(self.freeEntityIDs, id)
			delete(self.entities, id)
		}
	}

	self.toRemove = self.toRemove[:0]
}

// removeEntityFromArchetype performs an efficient "swap and pop" removal from an archetype.
// This avoids shifting elements by swapping with the last one and truncating.
func (self *World) removeEntityFromArchetype(e Entity, arch *Archetype, index int) {
	lastIndex := len(arch.entities) - 1
	if lastIndex < 0 || index > lastIndex {
		return
	}
	lastEntity := arch.entities[lastIndex]

	arch.entities[index] = lastEntity
	arch.entities = arch.entities[:lastIndex]

	if e.ID != lastEntity.ID {
		meta := self.entities[lastEntity.ID]
		meta.Index = index
		self.entities[lastEntity.ID] = meta
	}

	for i := range arch.componentData {
		sliceVal := reflect.ValueOf(arch.componentData[i])
		if sliceVal.Len() <= lastIndex {
			continue
		}
		sliceVal.Index(index).Set(sliceVal.Index(lastIndex))
		newSlice := sliceVal.Slice(0, lastIndex)
		arch.componentData[i] = newSlice.Interface()
	}
}

// Query creates a new query iterator for systems.
// It matches archetypes that have all the included components.
func (self *World) Query(includes ...ComponentID) *Query {
	return self.queryInternal(includes, nil)
}

// QueryWithExclusions creates a query with includes and excludes.
// Archetypes must have all includes and none of the excludes.
func (self *World) QueryWithExclusions(includes, excludes []ComponentID) *Query {
	return self.queryInternal(includes, excludes)
}

func (self *World) queryInternal(includes, excludes []ComponentID) *Query {
	self.mu.RLock()
	defer self.mu.RUnlock()
	self.archetypesMu.RLock()
	defer self.archetypesMu.RUnlock()

	includeMask := makeMask(includes)
	excludeMask := makeMask(excludes)

	matchingArchetypes := make([]*Archetype, 0, len(self.archetypes)/2) // Pre-allocate estimate.
	for mask, arch := range self.archetypes {
		if len(arch.entities) > 0 && includesAll(mask, includeMask) && !intersects(mask, excludeMask) {
			matchingArchetypes = append(matchingArchetypes, arch)
		}
	}

	return &Query{
		archetypes:          matchingArchetypes,
		currentArchetypeIdx: -1,
	}
}

// AddComponent adds a component of type T to an entity.
// Returns pointer to the component and success flag.
// If the component already exists, it returns the existing one.
func AddComponent[T any](w *World, e Entity) (*T, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return nil, false
	}

	var t T
	compType := reflect.TypeOf(t)
	registryMutex.RLock()
	compID, ok := typeToID[compType]
	registryMutex.RUnlock()
	if !ok {
		return nil, false
	}

	oldArch := meta.Archetype
	if oldArch.mask.has(compID) {
		idx := oldArch.componentMap[compID]
		slice := oldArch.componentData[idx].([]T)
		return &slice[meta.Index], true
	}

	newMask := setMask(oldArch.mask, compID)
	newArch := w.getOrCreateArchetype(newMask)

	oldIndex := meta.Index
	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch)

	newIdx := newArch.componentMap[compID]
	newSliceVal := reflect.ValueOf(newArch.componentData[newIdx])
	newSliceVal = reflect.Append(newSliceVal, reflect.Zero(compType))
	newArch.componentData[newIdx] = newSliceVal.Interface()

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entities[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	finalIdx := newArch.componentMap[compID]
	finalSlice := newArch.componentData[finalIdx].([]T)
	return &finalSlice[newIndex], true
}

// SetComponent adds a component with specific data to an entity, or updates it if it already exists.
// This is more convenient than AddComponent + manual set for initialization.
func SetComponent[T any](w *World, e Entity, comp T) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 1. Validate the entity and get its metadata.
	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return false
	}

	// 2. Get the Component ID for type T.
	compType := reflect.TypeOf(comp)
	registryMutex.RLock()
	compID, ok := typeToID[compType]
	registryMutex.RUnlock()
	if !ok {
		// Component type not registered.
		return false
	}

	oldArch := meta.Archetype
	// 3. Check if the entity already has this component.
	if oldArch.mask.has(compID) {
		// --- SCENARIO A: UPDATE EXISTING COMPONENT ---
		// The entity is already in the correct archetype. We just need to update the data.
		componentIndexInArchetype := oldArch.componentMap[compID]
		// Directly access the typed slice and update the value at the entity's index.
		componentSlice := oldArch.componentData[componentIndexInArchetype].([]T)
		componentSlice[meta.Index] = comp
		return true
	} else {
		// --- SCENARIO B: ADD NEW COMPONENT ---
		// The entity must move to a new archetype.
		// a. Determine the new archetype's mask and get/create it.
		newMask := setMask(oldArch.mask, compID)
		newArch := w.getOrCreateArchetype(newMask)

		// b. Move the entity and all its existing component data to the new archetype.
		oldIndex := meta.Index
		newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch)

		// c. Append the new component data (passed in as `comp`) to the correct slice in the new archetype.
		newCompSliceVal := reflect.ValueOf(newArch.componentData[newArch.componentMap[compID]])
		newCompSliceVal = reflect.Append(newCompSliceVal, reflect.ValueOf(comp))
		newArch.componentData[newArch.componentMap[compID]] = newCompSliceVal.Interface()

		// d. Update the entity's metadata to point to its new location.
		meta.Archetype = newArch
		meta.Index = newIndex
		w.entities[e.ID] = meta

		// e. Remove the entity from its old archetype using swap-and-pop.
		w.removeEntityFromArchetype(e, oldArch, oldIndex)
		return true
	}
}

// RemoveComponent removes a component of type T from an entity.
// If the component doesn't exist, it returns true anyway.
func RemoveComponent[T any](w *World, e Entity) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return false
	}

	var t T
	compType := reflect.TypeOf(t)
	registryMutex.RLock()
	compID, ok := typeToID[compType]
	registryMutex.RUnlock()
	if !ok {
		return false
	}

	oldArch := meta.Archetype
	if !oldArch.mask.has(compID) {
		return true // Already removed.
	}

	oldIndex := meta.Index
	newMask := unsetMask(oldArch.mask, compID)
	newArch := w.getOrCreateArchetype(newMask)

	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch, compID)

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entities[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	return true
}

// GetComponent retrieves a pointer to a component of type T for an entity.
// Returns nil and false if not found or entity invalid.
func GetComponent[T any](w *World, e Entity) (*T, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return nil, false
	}

	var t T
	registryMutex.RLock()
	compID, ok := typeToID[reflect.TypeOf(t)]
	registryMutex.RUnlock()
	if !ok {
		return nil, false
	}

	arch := meta.Archetype
	if idx, ok := arch.componentMap[compID]; ok {
		slice := arch.componentData[idx].([]T)
		if meta.Index >= len(slice) {
			return nil, false // Edge case: index out of bounds.
		}
		return &slice[meta.Index], true
	}
	return nil, false
}

// moveEntityBetweenArchetypes copies an entity's data from one archetype to another, optionally excluding IDs.
// This is used when adding or removing components.
func moveEntityBetweenArchetypes(e Entity, oldIndex int, oldArch, newArch *Archetype, excludeIDs ...ComponentID) int {
	newIndex := len(newArch.entities)
	newArch.entities = append(newArch.entities, e)

	excludeSet := make(map[ComponentID]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		excludeSet[id] = struct{}{}
	}

	for _, id := range oldArch.componentIDs {
		if _, excluded := excludeSet[id]; excluded {
			continue
		}
		oldIdx := oldArch.componentMap[id]
		oldSliceVal := reflect.ValueOf(oldArch.componentData[oldIdx])
		newIdx := newArch.componentMap[id]
		newSliceVal := reflect.ValueOf(newArch.componentData[newIdx])
		if oldIndex >= oldSliceVal.Len() {
			continue // Edge case: invalid index.
		}
		componentToMove := oldSliceVal.Index(oldIndex)
		newSliceVal = reflect.Append(newSliceVal, componentToMove)
		newArch.componentData[newIdx] = newSliceVal.Interface()
	}
	return newIndex
}

// Archetype represents a unique combination of components.
// It stores entities and their component data in parallel slices for cache efficiency.
type Archetype struct {
	mask           maskType            // Bitmask of components in this archetype.
	componentData  []any               // Slices of component data.
	componentTypes []reflect.Type      // Types of the components.
	componentIDs   []ComponentID       // Sorted list of component IDs.
	componentMap   map[ComponentID]int // Map from ID to index in data slices.
	entities       []Entity            // List of entities in this archetype.
}

// Query is an iterator for efficiently accessing entities and components in matching archetypes.
// It iterates over archetypes, not individual entities, for batch processing.
type Query struct {
	archetypes          []*Archetype // Matching archetypes.
	currentArchetypeIdx int          // Index of the current archetype.
}

// Next advances the iterator to the next archetype. Returns false if no more archetypes.
func (self *Query) Next() bool {
	self.currentArchetypeIdx++
	return self.currentArchetypeIdx < len(self.archetypes)
}

// Count returns the number of entities in the current archetype.
func (self *Query) Count() int {
	if self.currentArchetypeIdx < 0 || self.currentArchetypeIdx >= len(self.archetypes) {
		return 0 // Edge case: invalid index.
	}
	return len(self.archetypes[self.currentArchetypeIdx].entities)
}

// Entities returns the slice of entities for the current archetype.
func (self *Query) Entities() []Entity {
	if self.currentArchetypeIdx < 0 || self.currentArchetypeIdx >= len(self.archetypes) {
		return nil // Edge case.
	}
	return self.archetypes[self.currentArchetypeIdx].entities
}

// GetComponentSlice provides direct, typed access to the component slice for the current archetype.
// This allows systems to process components in batches efficiently.
func GetComponentSlice[T any](q *Query) ([]T, bool) {
	if q.currentArchetypeIdx < 0 || q.currentArchetypeIdx >= len(q.archetypes) {
		return nil, false // Edge case.
	}
	var t T
	registryMutex.RLock()
	compID, ok := typeToID[reflect.TypeOf(t)]
	registryMutex.RUnlock()
	if !ok {
		return nil, false
	}

	arch := q.archetypes[q.currentArchetypeIdx]
	if idx, ok := arch.componentMap[compID]; ok {
		slice, ok := arch.componentData[idx].([]T)
		if !ok {
			return nil, false // Type assertion fail.
		}
		return slice, true
	}
	return nil, false
}
