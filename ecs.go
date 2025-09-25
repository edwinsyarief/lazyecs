package lazyecs

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"unsafe"
)

type ComponentID uint32

const (
	bitsPerWord            = 64
	maskWords              = 4
	maxComponentTypes      = maskWords * bitsPerWord
	defaultInitialCapacity = 4096
)

type maskType [maskWords]uint64

func (m maskType) has(id ComponentID) bool {
	word := int(id / bitsPerWord)
	if word >= maskWords {
		return false
	}
	bit := id % bitsPerWord
	return (m[word] & (1 << bit)) != 0
}

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

func makeMask(ids []ComponentID) maskType {
	var m maskType
	for _, id := range ids {
		word := int(id / bitsPerWord)
		bit := id % bitsPerWord
		m[word] |= (1 << bit)
	}
	return m
}

func makeMask1(id1 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	return m
}

func makeMask2(id1, id2 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	return m
}

func makeMask3(id1, id2, id3 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	word3 := int(id3 / bitsPerWord)
	bit3 := id3 % bitsPerWord
	m[word3] |= (1 << bit3)
	return m
}

func makeMask4(id1, id2, id3, id4 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	word3 := int(id3 / bitsPerWord)
	bit3 := id3 % bitsPerWord
	m[word3] |= (1 << bit3)
	word4 := int(id4 / bitsPerWord)
	bit4 := id4 % bitsPerWord
	m[word4] |= (1 << bit4)
	return m
}

func makeMask5(id1, id2, id3, id4, id5 ComponentID) maskType {
	var m maskType
	word1 := int(id1 / bitsPerWord)
	bit1 := id1 % bitsPerWord
	m[word1] |= (1 << bit1)
	word2 := int(id2 / bitsPerWord)
	bit2 := id2 % bitsPerWord
	m[word2] |= (1 << bit2)
	word3 := int(id3 / bitsPerWord)
	bit3 := id3 % bitsPerWord
	m[word3] |= (1 << bit3)
	word4 := int(id4 / bitsPerWord)
	bit4 := id4 % bitsPerWord
	m[word4] |= (1 << bit4)
	word5 := int(id5 / bitsPerWord)
	bit5 := id5 % bitsPerWord
	m[word5] |= (1 << bit5)
	return m
}

func includesAll(m, include maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & include[i]) != include[i] {
			return false
		}
	}
	return true
}

func intersects(m, exclude maskType) bool {
	for i := 0; i < maskWords; i++ {
		if (m[i] & exclude[i]) != 0 {
			return true
		}
	}
	return false
}

var (
	nextComponentID ComponentID
	typeToID        = make(map[reflect.Type]ComponentID)
	idToType        = make(map[ComponentID]reflect.Type)
	componentSizes  [maxComponentTypes]uintptr
)

func ResetGlobalRegistry() {
	nextComponentID = 0
	typeToID = make(map[reflect.Type]ComponentID)
	idToType = make(map[ComponentID]reflect.Type)
	componentSizes = [maxComponentTypes]uintptr{}
}

func RegisterComponent[T any]() ComponentID {
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
	componentSizes[id] = unsafe.Sizeof(t)
	nextComponentID++
	return id
}

func GetID[T any]() ComponentID {
	var zero T
	typ := reflect.TypeOf(zero)
	id, ok := typeToID[typ]
	if !ok {
		panic(fmt.Sprintf("component type %s not registered", typ))
	}
	return id
}

func TryGetID[T any]() (ComponentID, bool) {
	var zero T
	typ := reflect.TypeOf(zero)
	id, ok := typeToID[typ]
	return id, ok
}

type Entity struct {
	ID      uint32
	Version uint32
}

type entityMeta struct {
	Archetype *Archetype
	Index     int
	Version   uint32
}

type WorldOptions struct {
	InitialCapacity int
}

type World struct {
	nextEntityID    uint32
	freeEntityIDs   []uint32
	entities        map[uint32]entityMeta
	archetypes      map[maskType]*Archetype
	archetypesList  []*Archetype
	toRemove        []Entity
	Resources       sync.Map
	initialCapacity int
}

func NewWorld() *World {
	return NewWorldWithOptions(WorldOptions{})
}

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
	w.getOrCreateArchetype(maskType{})
	return w
}

func (self *World) getOrCreateArchetype(mask maskType) *Archetype {
	if arch, ok := self.archetypes[mask]; ok {
		return arch
	}

	newArch := &Archetype{
		mask:     mask,
		entities: make([]Entity, 0, self.initialCapacity),
	}

	compIDs := make([]ComponentID, 0, len(idToType))
	for id := range idToType {
		if mask.has(id) {
			compIDs = append(compIDs, id)
		}
	}
	sort.Slice(compIDs, func(i, j int) bool { return compIDs[i] < compIDs[j] })
	newArch.componentIDs = compIDs
	newArch.componentData = make([][]byte, len(compIDs))

	for i, id := range compIDs {
		size := int(componentSizes[id])
		newArch.componentData[i] = make([]byte, 0, self.initialCapacity*size)
	}

	self.archetypes[mask] = newArch
	self.archetypesList = append(self.archetypesList, newArch)
	return newArch
}

func extendSlice[T any](s []T, n int) []T {
	newLen := len(s) + n
	if cap(s) < newLen {
		panic("slice capacity exceeded, increase initialCapacity")
	}
	return s[:newLen]
}

func extendByteSlice(s []byte, n int) []byte {
	newLen := len(s) + n
	if cap(s) < newLen {
		panic("byte slice capacity exceeded, increase initialCapacity")
	}
	return s[:newLen]
}

func (self *World) CreateEntity() Entity {
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
	arch := self.archetypes[maskType{}]
	index := len(arch.entities)
	arch.entities = extendSlice(arch.entities, 1)
	arch.entities[index] = e

	self.entities[id] = entityMeta{Archetype: arch, Index: index, Version: e.Version}
	return e
}

func (self *World) CreateEntities(count int) []Entity {
	if count <= 0 {
		return nil
	}

	entities := make([]Entity, count)
	arch := self.archetypes[maskType{}]
	startIndex := len(arch.entities)
	arch.entities = extendSlice(arch.entities, count)

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
		idx := startIndex + i
		arch.entities[idx] = e
		self.entities[id] = entityMeta{Archetype: arch, Index: idx, Version: e.Version}
	}
	return entities
}

func (self *World) RemoveEntity(e Entity) {
	self.toRemove = extendSlice(self.toRemove, 1)
	self.toRemove[len(self.toRemove)-1] = e
}

func (self *World) ProcessRemovals() {
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
			self.freeEntityIDs = extendSlice(self.freeEntityIDs, 1)
			self.freeEntityIDs[len(self.freeEntityIDs)-1] = id
			delete(self.entities, id)
		}
	}
	self.toRemove = self.toRemove[:0]
}

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
		id := arch.componentIDs[i]
		size := int(componentSizes[id])
		bytes := arch.componentData[i]
		copy(bytes[index*size:(index+1)*size], bytes[lastIndex*size:(lastIndex+1)*size])
		arch.componentData[i] = bytes[:lastIndex*size]
	}
}

func AddComponent[T any](w *World, e Entity) (*T, bool) {
	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return nil, false
	}

	compID, ok := TryGetID[T]()
	if !ok {
		return nil, false
	}
	size := int(componentSizes[compID])

	oldArch := meta.Archetype
	if oldArch.mask.has(compID) {
		idx := oldArch.getSlot(compID)
		if idx == -1 {
			return nil, false
		}
		bytes := oldArch.componentData[idx]
		if meta.Index*size >= len(bytes) {
			return nil, false
		}
		return (*T)(unsafe.Pointer(&bytes[meta.Index*size])), true
	}

	newMask := setMask(oldArch.mask, compID)
	newArch := w.getOrCreateArchetype(newMask)

	oldIndex := meta.Index
	newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch)

	newIdx := newArch.getSlot(compID)
	if newIdx == -1 {
		return nil, false
	}
	newBytes := newArch.componentData[newIdx]
	newBytes = extendByteSlice(newBytes, size)
	newArch.componentData[newIdx] = newBytes

	meta.Archetype = newArch
	meta.Index = newIndex
	w.entities[e.ID] = meta

	w.removeEntityFromArchetype(e, oldArch, oldIndex)

	finalIdx := newArch.getSlot(compID)
	finalBytes := newArch.componentData[finalIdx]
	return (*T)(unsafe.Pointer(&finalBytes[newIndex*size])), true
}

func SetComponent[T any](w *World, e Entity, comp T) bool {
	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return false
	}

	compID, ok := TryGetID[T]()
	if !ok {
		return false
	}
	size := int(componentSizes[compID])
	src := unsafe.Slice((*byte)(unsafe.Pointer(&comp)), size)

	oldArch := meta.Archetype
	if oldArch.mask.has(compID) {
		componentIndexInArchetype := oldArch.getSlot(compID)
		if componentIndexInArchetype == -1 {
			return false
		}
		bytes := oldArch.componentData[componentIndexInArchetype]
		if meta.Index*size >= len(bytes) {
			return false
		}
		copy(bytes[meta.Index*size:(meta.Index+1)*size], src)
		return true
	} else {
		newMask := setMask(oldArch.mask, compID)
		newArch := w.getOrCreateArchetype(newMask)

		oldIndex := meta.Index
		newIndex := moveEntityBetweenArchetypes(e, oldIndex, oldArch, newArch)

		newCompIdx := newArch.getSlot(compID)
		if newCompIdx == -1 {
			return false
		}
		newBytes := newArch.componentData[newCompIdx]
		newBytes = extendByteSlice(newBytes, size)
		copy(newBytes[len(newBytes)-size:], src)
		newArch.componentData[newCompIdx] = newBytes

		meta.Archetype = newArch
		meta.Index = newIndex
		w.entities[e.ID] = meta

		w.removeEntityFromArchetype(e, oldArch, oldIndex)
		return true
	}
}

func RemoveComponent[T any](w *World, e Entity) bool {
	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return false
	}

	compID, ok := TryGetID[T]()
	if !ok {
		return false
	}

	oldArch := meta.Archetype
	if !oldArch.mask.has(compID) {
		return true
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

func GetComponent[T any](w *World, e Entity) (*T, bool) {
	meta, ok := w.entities[e.ID]
	if !ok || e.Version != meta.Version {
		return nil, false
	}

	compID, ok := TryGetID[T]()
	if !ok {
		return nil, false
	}
	size := int(componentSizes[compID])

	arch := meta.Archetype
	idx := arch.getSlot(compID)
	if idx == -1 {
		return nil, false
	}
	bytes := arch.componentData[idx]
	if meta.Index*size >= len(bytes) {
		return nil, false
	}
	return (*T)(unsafe.Pointer(&bytes[meta.Index*size])), true
}

func moveEntityBetweenArchetypes(e Entity, oldIndex int, oldArch, newArch *Archetype, excludeIDs ...ComponentID) int {
	newIndex := len(newArch.entities)
	newArch.entities = extendSlice(newArch.entities, 1)
	newArch.entities[newIndex] = e

	excludeSet := make(map[ComponentID]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		excludeSet[id] = struct{}{}
	}

	for i := range oldArch.componentIDs {
		id := oldArch.componentIDs[i]
		if _, excluded := excludeSet[id]; excluded {
			continue
		}
		oldBytes := oldArch.componentData[i]
		size := int(componentSizes[id])
		src := oldBytes[oldIndex*size : (oldIndex+1)*size]
		newIdx := newArch.getSlot(id)
		if newIdx == -1 {
			continue
		}
		newBytes := newArch.componentData[newIdx]
		newBytes = extendByteSlice(newBytes, size)
		copy(newBytes[len(newBytes)-size:], src)
		newArch.componentData[newIdx] = newBytes
	}
	return newIndex
}

type Archetype struct {
	mask          maskType
	componentData [][]byte
	componentIDs  []ComponentID
	entities      []Entity
}

func (a *Archetype) getSlot(id ComponentID) int {
	i := sort.Search(len(a.componentIDs), func(j int) bool {
		return a.componentIDs[j] >= id
	})
	if i < len(a.componentIDs) && a.componentIDs[i] == id {
		return i
	}
	return -1
}

// Query is an iterator over entities matching 1 component type.
type Query[T1 any] struct {
	world         *World
	includeMask   maskType
	excludeMask   maskType
	id1           ComponentID
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	currentEntity Entity
}

// Reset resets the query for reuse.
func (q *Query[T1]) Reset() {
	q.archIdx = 0
	q.index = -1
	q.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query[T1]) Next() bool {
	q.index++
	if q.currentArch != nil && q.index < len(q.currentArch.entities) {
		q.currentEntity = q.currentArch.entities[q.index]
		return true
	}

	for q.archIdx < len(q.world.archetypesList) {
		arch := q.world.archetypesList[q.archIdx]
		q.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, q.includeMask) || intersects(arch.mask, q.excludeMask) {
			continue
		}
		q.currentArch = arch
		slot1 := arch.getSlot(q.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			q.base1 = nil
		}
		q.stride1 = componentSizes[q.id1]
		q.index = 0
		q.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns a pointer to the component for the current entity.
func (q *Query[T1]) Get() *T1 {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	return (*T1)(p1)
}

// Entity returns the current entity.
func (q *Query[T1]) Entity() Entity {
	return q.currentEntity
}

// Query2 is an iterator over entities matching 2 component types.
type Query2[T1 any, T2 any] struct {
	world         *World
	includeMask   maskType
	excludeMask   maskType
	id1           ComponentID
	id2           ComponentID
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	base2         unsafe.Pointer
	stride2       uintptr
	currentEntity Entity
}

// Reset resets the query for reuse.
func (q *Query2[T1, T2]) Reset() {
	q.archIdx = 0
	q.index = -1
	q.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query2[T1, T2]) Next() bool {
	q.index++
	if q.currentArch != nil && q.index < len(q.currentArch.entities) {
		q.currentEntity = q.currentArch.entities[q.index]
		return true
	}

	for q.archIdx < len(q.world.archetypesList) {
		arch := q.world.archetypesList[q.archIdx]
		q.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, q.includeMask) || intersects(arch.mask, q.excludeMask) {
			continue
		}
		q.currentArch = arch
		slot1 := arch.getSlot(q.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			q.base1 = nil
		}
		q.stride1 = componentSizes[q.id1]
		slot2 := arch.getSlot(q.id2)
		if slot2 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot2]) > 0 {
			q.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
		} else {
			q.base2 = nil
		}
		q.stride2 = componentSizes[q.id2]
		q.index = 0
		q.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (q *Query2[T1, T2]) Get() (*T1, *T2) {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	p2 := unsafe.Pointer(uintptr(q.base2) + uintptr(q.index)*q.stride2)
	return (*T1)(p1), (*T2)(p2)
}

// Entity returns the current entity.
func (q *Query2[T1, T2]) Entity() Entity {
	return q.currentEntity
}

// Query3 is an iterator over entities matching 3 component types.
type Query3[T1 any, T2 any, T3 any] struct {
	world         *World
	includeMask   maskType
	excludeMask   maskType
	id1           ComponentID
	id2           ComponentID
	id3           ComponentID
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	base2         unsafe.Pointer
	stride2       uintptr
	base3         unsafe.Pointer
	stride3       uintptr
	currentEntity Entity
}

// Reset resets the query for reuse.
func (q *Query3[T1, T2, T3]) Reset() {
	q.archIdx = 0
	q.index = -1
	q.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query3[T1, T2, T3]) Next() bool {
	q.index++
	if q.currentArch != nil && q.index < len(q.currentArch.entities) {
		q.currentEntity = q.currentArch.entities[q.index]
		return true
	}

	for q.archIdx < len(q.world.archetypesList) {
		arch := q.world.archetypesList[q.archIdx]
		q.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, q.includeMask) || intersects(arch.mask, q.excludeMask) {
			continue
		}
		q.currentArch = arch
		slot1 := arch.getSlot(q.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			q.base1 = nil
		}
		q.stride1 = componentSizes[q.id1]
		slot2 := arch.getSlot(q.id2)
		if slot2 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot2]) > 0 {
			q.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
		} else {
			q.base2 = nil
		}
		q.stride2 = componentSizes[q.id2]
		slot3 := arch.getSlot(q.id3)
		if slot3 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot3]) > 0 {
			q.base3 = unsafe.Pointer(&arch.componentData[slot3][0])
		} else {
			q.base3 = nil
		}
		q.stride3 = componentSizes[q.id3]
		q.index = 0
		q.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (q *Query3[T1, T2, T3]) Get() (*T1, *T2, *T3) {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	p2 := unsafe.Pointer(uintptr(q.base2) + uintptr(q.index)*q.stride2)
	p3 := unsafe.Pointer(uintptr(q.base3) + uintptr(q.index)*q.stride3)
	return (*T1)(p1), (*T2)(p2), (*T3)(p3)
}

// Entity returns the current entity.
func (q *Query3[T1, T2, T3]) Entity() Entity {
	return q.currentEntity
}

// Query4 is an iterator over entities matching 4 component types.
type Query4[T1 any, T2 any, T3 any, T4 any] struct {
	world         *World
	includeMask   maskType
	excludeMask   maskType
	id1           ComponentID
	id2           ComponentID
	id3           ComponentID
	id4           ComponentID
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	base2         unsafe.Pointer
	stride2       uintptr
	base3         unsafe.Pointer
	stride3       uintptr
	base4         unsafe.Pointer
	stride4       uintptr
	currentEntity Entity
}

// Reset resets the query for reuse.
func (q *Query4[T1, T2, T3, T4]) Reset() {
	q.archIdx = 0
	q.index = -1
	q.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query4[T1, T2, T3, T4]) Next() bool {
	q.index++
	if q.currentArch != nil && q.index < len(q.currentArch.entities) {
		q.currentEntity = q.currentArch.entities[q.index]
		return true
	}

	for q.archIdx < len(q.world.archetypesList) {
		arch := q.world.archetypesList[q.archIdx]
		q.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, q.includeMask) || intersects(arch.mask, q.excludeMask) {
			continue
		}
		q.currentArch = arch
		slot1 := arch.getSlot(q.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			q.base1 = nil
		}
		q.stride1 = componentSizes[q.id1]
		slot2 := arch.getSlot(q.id2)
		if slot2 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot2]) > 0 {
			q.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
		} else {
			q.base2 = nil
		}
		q.stride2 = componentSizes[q.id2]
		slot3 := arch.getSlot(q.id3)
		if slot3 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot3]) > 0 {
			q.base3 = unsafe.Pointer(&arch.componentData[slot3][0])
		} else {
			q.base3 = nil
		}
		q.stride3 = componentSizes[q.id3]
		slot4 := arch.getSlot(q.id4)
		if slot4 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot4]) > 0 {
			q.base4 = unsafe.Pointer(&arch.componentData[slot4][0])
		} else {
			q.base4 = nil
		}
		q.stride4 = componentSizes[q.id4]
		q.index = 0
		q.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (q *Query4[T1, T2, T3, T4]) Get() (*T1, *T2, *T3, *T4) {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	p2 := unsafe.Pointer(uintptr(q.base2) + uintptr(q.index)*q.stride2)
	p3 := unsafe.Pointer(uintptr(q.base3) + uintptr(q.index)*q.stride3)
	p4 := unsafe.Pointer(uintptr(q.base4) + uintptr(q.index)*q.stride4)
	return (*T1)(p1), (*T2)(p2), (*T3)(p3), (*T4)(p4)
}

// Entity returns the current entity.
func (q *Query4[T1, T2, T3, T4]) Entity() Entity {
	return q.currentEntity
}

// Query5 is an iterator over entities matching 5 component types.
type Query5[T1 any, T2 any, T3 any, T4 any, T5 any] struct {
	world         *World
	includeMask   maskType
	excludeMask   maskType
	id1           ComponentID
	id2           ComponentID
	id3           ComponentID
	id4           ComponentID
	id5           ComponentID
	archIdx       int
	index         int
	currentArch   *Archetype
	base1         unsafe.Pointer
	stride1       uintptr
	base2         unsafe.Pointer
	stride2       uintptr
	base3         unsafe.Pointer
	stride3       uintptr
	base4         unsafe.Pointer
	stride4       uintptr
	base5         unsafe.Pointer
	stride5       uintptr
	currentEntity Entity
}

// Reset resets the query for reuse.
func (q *Query5[T1, T2, T3, T4, T5]) Reset() {
	q.archIdx = 0
	q.index = -1
	q.currentArch = nil
}

// Next advances to the next entity. Returns false if no more entities.
func (q *Query5[T1, T2, T3, T4, T5]) Next() bool {
	q.index++
	if q.currentArch != nil && q.index < len(q.currentArch.entities) {
		q.currentEntity = q.currentArch.entities[q.index]
		return true
	}

	for q.archIdx < len(q.world.archetypesList) {
		arch := q.world.archetypesList[q.archIdx]
		q.archIdx++
		if len(arch.entities) == 0 || !includesAll(arch.mask, q.includeMask) || intersects(arch.mask, q.excludeMask) {
			continue
		}
		q.currentArch = arch
		slot1 := arch.getSlot(q.id1)
		if slot1 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot1]) > 0 {
			q.base1 = unsafe.Pointer(&arch.componentData[slot1][0])
		} else {
			q.base1 = nil
		}
		q.stride1 = componentSizes[q.id1]
		slot2 := arch.getSlot(q.id2)
		if slot2 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot2]) > 0 {
			q.base2 = unsafe.Pointer(&arch.componentData[slot2][0])
		} else {
			q.base2 = nil
		}
		q.stride2 = componentSizes[q.id2]
		slot3 := arch.getSlot(q.id3)
		if slot3 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot3]) > 0 {
			q.base3 = unsafe.Pointer(&arch.componentData[slot3][0])
		} else {
			q.base3 = nil
		}
		q.stride3 = componentSizes[q.id3]
		slot4 := arch.getSlot(q.id4)
		if slot4 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot4]) > 0 {
			q.base4 = unsafe.Pointer(&arch.componentData[slot4][0])
		} else {
			q.base4 = nil
		}
		q.stride4 = componentSizes[q.id4]
		slot5 := arch.getSlot(q.id5)
		if slot5 < 0 {
			panic("missing component in matching archetype")
		}
		if len(arch.componentData[slot5]) > 0 {
			q.base5 = unsafe.Pointer(&arch.componentData[slot5][0])
		} else {
			q.base5 = nil
		}
		q.stride5 = componentSizes[q.id5]
		q.index = 0
		q.currentEntity = arch.entities[0]
		return true
	}
	return false
}

// Get returns pointers to the components for the current entity.
func (q *Query5[T1, T2, T3, T4, T5]) Get() (*T1, *T2, *T3, *T4, *T5) {
	p1 := unsafe.Pointer(uintptr(q.base1) + uintptr(q.index)*q.stride1)
	p2 := unsafe.Pointer(uintptr(q.base2) + uintptr(q.index)*q.stride2)
	p3 := unsafe.Pointer(uintptr(q.base3) + uintptr(q.index)*q.stride3)
	p4 := unsafe.Pointer(uintptr(q.base4) + uintptr(q.index)*q.stride4)
	p5 := unsafe.Pointer(uintptr(q.base5) + uintptr(q.index)*q.stride5)
	return (*T1)(p1), (*T2)(p2), (*T3)(p3), (*T4)(p4), (*T5)(p5)
}

// Entity returns the current entity.
func (q *Query5[T1, T2, T3, T4, T5]) Entity() Entity {
	return q.currentEntity
}

// Query1 creates a query for entities with the specified component.
func CreateQuery[T1 any](w *World, excludes ...ComponentID) *Query[T1] {
	id1 := GetID[T1]()
	return &Query[T1]{
		world:       w,
		includeMask: makeMask1(id1),
		excludeMask: makeMask(excludes),
		id1:         id1,
		archIdx:     0,
		index:       -1,
	}
}

// Query2 creates a query for entities with the specified components.
func CreateQuery2[T1 any, T2 any](w *World, excludes ...ComponentID) *Query2[T1, T2] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	return &Query2[T1, T2]{
		world:       w,
		includeMask: makeMask2(id1, id2),
		excludeMask: makeMask(excludes),
		id1:         id1,
		id2:         id2,
		archIdx:     0,
		index:       -1,
	}
}

// Query3 creates a query for entities with the specified components.
func CreateQuery3[T1 any, T2 any, T3 any](w *World, excludes ...ComponentID) *Query3[T1, T2, T3] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	id3 := GetID[T3]()
	return &Query3[T1, T2, T3]{
		world:       w,
		includeMask: makeMask3(id1, id2, id3),
		excludeMask: makeMask(excludes),
		id1:         id1,
		id2:         id2,
		id3:         id3,
		archIdx:     0,
		index:       -1,
	}
}

// Query4 creates a query for entities with the specified components.
func CreateQuery4[T1 any, T2 any, T3 any, T4 any](w *World, excludes ...ComponentID) *Query4[T1, T2, T3, T4] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	id3 := GetID[T3]()
	id4 := GetID[T4]()
	return &Query4[T1, T2, T3, T4]{
		world:       w,
		includeMask: makeMask4(id1, id2, id3, id4),
		excludeMask: makeMask(excludes),
		id1:         id1,
		id2:         id2,
		id3:         id3,
		id4:         id4,
		archIdx:     0,
		index:       -1,
	}
}

// Query5 creates a query for entities with the specified components.
func CreateQuery5[T1 any, T2 any, T3 any, T4 any, T5 any](w *World, excludes ...ComponentID) *Query5[T1, T2, T3, T4, T5] {
	id1 := GetID[T1]()
	id2 := GetID[T2]()
	id3 := GetID[T3]()
	id4 := GetID[T4]()
	id5 := GetID[T5]()
	return &Query5[T1, T2, T3, T4, T5]{
		world:       w,
		includeMask: makeMask5(id1, id2, id3, id4, id5),
		excludeMask: makeMask(excludes),
		id1:         id1,
		id2:         id2,
		id3:         id3,
		id4:         id4,
		id5:         id5,
		archIdx:     0,
		index:       -1,
	}
}
