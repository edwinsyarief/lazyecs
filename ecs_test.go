package teishoku

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"
)

// Define constants for configurability
const (
	TestCap      = 100000 // Capacity for world in tests
	TestEntities = 100000 // Number of entities for data integrity tests
)

// Define some test components
type Position struct {
	X, Y float32
}

type Velocity struct {
	DX, DY float32
}

type Health struct {
	HP int
}

type WithPointer struct {
	Data *int
}

// World Creation and Basic Operations
func TestNewWorld(t *testing.T) {
	w := NewWorld(TestCap)
	if w.entities.capacity != TestCap {
		t.Errorf("expected capacity %d, got %d", TestCap, w.entities.capacity)
	}
	if len(w.entities.freeIDs) != TestCap {
		t.Errorf("expected %d free IDs, got %d", TestCap, len(w.entities.freeIDs))
	}
	if len(w.entities.metas) != TestCap {
		t.Errorf("expected %d metas, got %d", TestCap, len(w.entities.metas))
	}
	if len(w.archetypes.archetypes) != 1 {
		t.Errorf("expected 1 archetypes, got %d", len(w.archetypes.archetypes))
	}
}

func TestAutoExpand(t *testing.T) {
	initialCap := 10
	w := NewWorld(initialCap)
	if w.entities.capacity != initialCap || w.entities.initialCapacity != initialCap {
		t.Errorf("expected initial capacity %d, got %d/%d", initialCap, w.entities.capacity, w.entities.initialCapacity)
	}
	builder := NewBuilder[Position](&w)
	// Create initial cap entities
	for i := 0; i < initialCap; i++ {
		ent := builder.NewEntity()
		if !w.IsValid(ent) {
			t.Errorf("entity %d invalid", i)
		}
	}
	// Create extra to trigger expand
	extra := 5
	for i := 0; i < extra; i++ {
		ent := builder.NewEntity()
		if !w.IsValid(ent) {
			t.Errorf("extra entity %d invalid", i)
		}
	}
	expectedCap := initialCap * 2
	if w.entities.capacity != expectedCap {
		t.Errorf("expected expanded capacity %d, got %d", expectedCap, w.entities.capacity)
	}
	if len(w.entities.metas) != expectedCap {
		t.Errorf("expected metas len %d, got %d", expectedCap, len(w.entities.metas))
	}
	if len(w.entities.freeIDs) != expectedCap-(initialCap+extra) {
		t.Errorf("expected freeIDs len %d, got %d", expectedCap-(initialCap+extra), len(w.entities.freeIDs))
	}
	// Verify archetype resized
	a := builder.arch
	if cap(a.entityIDs) != expectedCap {
		t.Errorf("expected archetype entityIDs cap %d, got %d", expectedCap, cap(a.entityIDs))
	}
	// Check data integrity after expand
	for i := 0; i < initialCap+extra; i++ {
		pos := GetComponent[Position](&w, a.entityIDs[i])
		if pos == nil {
			t.Errorf("position nil for entity %d after expand", i)
		}
	}
}

func TestGetCompTypeID(t *testing.T) {
	w := NewWorld(TestCap)
	id1 := w.getCompTypeID(reflect.TypeFor[Position]())
	id2 := w.getCompTypeID(reflect.TypeFor[Velocity]())
	id3 := w.getCompTypeID(reflect.TypeFor[Position]())
	if id1 != id3 {
		t.Errorf("expected same ID for same type, got %d and %d", id1, id3)
	}
	if id1 == id2 {
		t.Errorf("expected different IDs for different types, got %d", id1)
	}
	if w.components.nextCompTypeID != 2 {
		t.Errorf("expected nextCompTypeID 2, got %d", w.components.nextCompTypeID)
	}
}

func TestGetOrCreateArchetype(t *testing.T) {
	w := NewWorld(TestCap)
	var mask bitmask256
	mask.set(0)
	specs := []compSpec{{id: 0, typ: reflect.TypeFor[Position](), size: unsafe.Sizeof(Position{})}}
	a1 := w.getOrCreateArchetype(mask, specs)
	if a1 == nil {
		t.Fatal("archetype not created")
	}
	if len(w.archetypes.archetypes) != 2 {
		t.Errorf("expected 2 archetype, got %d", len(w.archetypes.archetypes))
	}
	a2 := w.getOrCreateArchetype(mask, specs)
	if a1 != a2 {
		t.Errorf("expected same archetype, got different")
	}
}

// Builder Tests
func TestBuilderNewEntity(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ent := builder.NewEntity()
	if !w.IsValid(ent) {
		t.Error("entity should be valid")
	}
	if w.entities.metas[ent.ID].archetypeIndex == -1 {
		t.Error("archetypeIndex not set")
	}
	if w.entities.metas[ent.ID].index != 0 {
		t.Errorf("expected index 0, got %d", w.entities.metas[ent.ID].index)
	}
	if w.entities.metas[ent.ID].version != ent.Version {
		t.Error("version mismatch")
	}
	a := w.archetypes.archetypes[w.entities.metas[ent.ID].archetypeIndex]
	if a.size != 1 {
		t.Errorf("expected size 1, got %d", a.size)
	}
	if a.entityIDs[0] != ent {
		t.Error("entity not in archetype")
	}
}

func TestBuilderNewEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(5)
	a := builder.arch
	if a.size != 5 {
		t.Errorf("expected size 5, got %d", a.size)
	}
	for i := 0; i < 5; i++ {
		ent := a.entityIDs[i]
		if !w.IsValid(ent) {
			t.Errorf("entity %d invalid", i)
		}
	}
}

func TestBuilderNewEntitiesWithValueSet(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	posVal := Position{10, 20}
	builder.NewEntitiesWithValueSet(5, posVal)
	a := builder.arch
	if a.size != 5 {
		t.Errorf("expected size 5, got %d", a.size)
	}
	for i := 0; i < 5; i++ {
		ent := a.entityIDs[i]
		if !w.IsValid(ent) {
			t.Errorf("entity %d invalid", i)
		}
		pos := builder.Get(ent)
		if pos == nil || pos.X != 10 || pos.Y != 20 {
			t.Errorf("position incorrect for entity %d: got (%f,%f)", i, pos.X, pos.Y)
		}
	}
}

func TestBuilderNewEntitiesWithValueSet2(t *testing.T) {
	w := NewWorld(TestCap)
	builder2 := NewBuilder2[Position, Velocity](&w)
	posVal := Position{10, 20}
	velVal := Velocity{30, 40}
	builder2.NewEntitiesWithValueSet(5, posVal, velVal)
	a := builder2.arch
	if a.size != 5 {
		t.Errorf("expected size 5, got %d", a.size)
	}
	for i := 0; i < 5; i++ {
		ent := a.entityIDs[i]
		if !w.IsValid(ent) {
			t.Errorf("entity %d invalid", i)
		}
		pos, vel := builder2.Get(ent)
		if pos == nil || pos.X != 10 || pos.Y != 20 {
			t.Errorf("position incorrect for entity %d", i)
		}
		if vel == nil || vel.DX != 30 || vel.DY != 40 {
			t.Errorf("velocity incorrect for entity %d", i)
		}
	}
}

// World Entity Creation Tests
func TestWorldCreateEntity(t *testing.T) {
	w := NewWorld(TestCap)
	ent := w.CreateEntity()
	if !w.IsValid(ent) {
		t.Errorf("created entity is invalid")
	}
	if len(w.archetypes.archetypes) != 1 {
		t.Errorf("expected 1 archetype, got %d", len(w.archetypes.archetypes))
	}
	a := w.archetypes.archetypes[0]
	if a.size != 1 {
		t.Errorf("expected archetype size 1, got %d", a.size)
	}
	if a.mask != (bitmask256{}) {
		t.Errorf("archetype mask not empty")
	}
	if GetComponent[Position](&w, ent) != nil {
		t.Errorf("empty entity has component")
	}
}

func TestWorldCreateEntityVersion(t *testing.T) {
	w := NewWorld(TestCap)
	ent1 := w.CreateEntity()
	w.RemoveEntity(ent1)
	ent2 := w.CreateEntity()
	if ent1.ID != ent2.ID {
		t.Errorf("expected same ID after recycle, got %d and %d", ent1.ID, ent2.ID)
	}
	if ent1.Version >= ent2.Version {
		t.Errorf("version not incremented: %d >= %d", ent1.Version, ent2.Version)
	}
}

func TestWorldCreateEntities(t *testing.T) {
	w := NewWorld(TestCap)
	ents := w.CreateEntities(0)
	if ents != nil {
		t.Errorf("expected nil for count 0, got %v", ents)
	}

	ents = w.CreateEntities(5)
	if len(ents) != 5 {
		t.Errorf("expected 5 entities, got %d", len(ents))
	}
	for i, e := range ents {
		if !w.IsValid(e) {
			t.Errorf("entity %d invalid", i)
		}
		if GetComponent[Position](&w, e) != nil {
			t.Errorf("entity %d has unexpected component", i)
		}
	}
	if len(w.archetypes.archetypes) != 1 {
		t.Errorf("expected 1 archetype, got %d", len(w.archetypes.archetypes))
	}
	a := w.archetypes.archetypes[0]
	if a.size != 5 {
		t.Errorf("expected archetype size 5, got %d", a.size)
	}
	if a.mask != (bitmask256{}) {
		t.Errorf("archetype mask not empty")
	}
}

func TestWorldCreateEntitiesExpand(t *testing.T) {
	w := NewWorld(1)
	ents := w.CreateEntities(2)
	if len(ents) != 2 {
		t.Errorf("expected 2 entities, got %d", len(ents))
	}
	for i, e := range ents {
		if !w.IsValid(e) {
			t.Errorf("entity %d invalid", i)
		}
	}
	if w.entities.capacity != 2 {
		t.Errorf("expected capacity 2 after expand, got %d", w.entities.capacity)
	}
	if len(w.archetypes.archetypes) != 1 {
		t.Errorf("expected 1 archetype, got %d", len(w.archetypes.archetypes))
	}
	a := w.archetypes.archetypes[0]
	if cap(a.entityIDs) != 2 {
		t.Errorf("expected archetype entityIDs cap 2, got %d", cap(a.entityIDs))
	}
}

// Component Operations
func TestGetComponent(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ent := builder.NewEntity()
	pos := builder.Get(ent)
	if pos == nil {
		t.Fatal("Get returned nil")
	}
	*pos = Position{1, 2}
	got := builder.Get(ent)
	if got.X != 1 || got.Y != 2 {
		t.Error("component not set correctly")
	}
	// invalid entity
	invalid := Entity{ID: 999, Version: 1}
	if builder.Get(invalid) != nil {
		t.Error("expected nil for invalid entity")
	}
	// wrong component
	velBuilder := NewBuilder[Velocity](&w)
	if velBuilder.Get(ent) != nil {
		t.Error("expected nil for missing component")
	}
}

func TestSetComponent(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ent := builder.NewEntity()
	SetComponent(&w, ent, Velocity{DX: 3, DY: 4})
	if !w.IsValid(ent) {
		t.Error("entity invalid after set")
	}
	vel := GetComponent[Velocity](&w, ent)
	if vel == nil {
		t.Fatal("velocity not set")
	}
	if vel.DX != 3 || vel.DY != 4 {
		t.Error("velocity values incorrect")
	}
	pos := GetComponent[Position](&w, ent)
	if pos == nil {
		t.Error("position lost after set")
	}
	// set existing
	SetComponent(&w, ent, Position{X: 5, Y: 6})
	pos = GetComponent[Position](&w, ent)
	if pos.X != 5 || pos.Y != 6 {
		t.Error("position not updated")
	}
}

func TestRemoveComponent(t *testing.T) {
	w := NewWorld(TestCap)
	builder2 := NewBuilder2[Position, Velocity](&w)
	ent := builder2.NewEntity()
	RemoveComponent[Velocity](&w, ent)
	if v := GetComponent[Velocity](&w, ent); v != nil {
		t.Error("velocity not removed")
	}
	if p := GetComponent[Position](&w, ent); p == nil {
		t.Error("position lost")
	}
	// remove non-existing
	RemoveComponent[Health](&w, ent)
	if !w.IsValid(ent) {
		t.Error("entity invalid after removing non-existing")
	}
}

// Entity Removal Tests
func TestRemoveEntity(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ent1 := builder.NewEntity()
	ent2 := builder.NewEntity()
	w.RemoveEntity(ent1)
	if w.IsValid(ent1) {
		t.Error("ent1 still valid after remove")
	}
	if !w.IsValid(ent2) {
		t.Error("ent2 invalid after removing ent1")
	}
	a := builder.arch
	if a.size != 1 {
		t.Errorf("expected size 1, got %d", a.size)
	}
	if a.entityIDs[0] != ent2 {
		t.Error("ent2 not swapped")
	}
	if w.entities.metas[ent2.ID].index != 0 {
		t.Error("ent2 index not updated")
	}
	// remove stale
	w.RemoveEntity(ent1)
	if a.size != 1 {
		t.Error("size changed on stale remove")
	}
}

func TestRemoveEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ents := make([]Entity, 5)
	for i := 0; i < 5; i++ {
		ents[i] = builder.NewEntity()
	}
	w.RemoveEntities(ents)
	for _, ent := range ents {
		if w.IsValid(ent) {
			t.Error("entity still valid after batch remove")
		}
	}
	// Test with invalid entity
	invalidEnt := Entity{ID: 9999, Version: 1}
	w.RemoveEntities([]Entity{invalidEnt}) // should do nothing
}

func TestClearEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(5)
	w.ClearEntities()
	if len(w.entities.freeIDs) != TestCap {
		t.Errorf("expected %d free IDs after clear, got %d", TestCap, len(w.entities.freeIDs))
	}
	for _, meta := range w.entities.metas {
		if meta.version != 0 || meta.archetypeIndex != -1 {
			t.Error("meta not reset")
		}
	}
	for _, a := range w.archetypes.archetypes {
		if a.size != 0 {
			t.Error("archetype size not reset")
		}
	}
	// Check new entity creation after clear
	ent := builder.NewEntity()
	if !w.IsValid(ent) {
		t.Error("cannot create entity after clear")
	}
}

// Filter Tests
func TestFilter(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(3)
	builder2 := NewBuilder2[Position, Velocity](&w)
	builder2.NewEntities(2)
	filter := NewFilter[Position](&w)
	count := 0
	for filter.Next() {
		count++
		ent := filter.Entity()
		if !w.IsValid(ent) {
			t.Error("invalid entity in filter")
		}
		pos := filter.Get()
		if pos == nil {
			t.Error("nil component in filter")
		}
	}
	if count != 5 {
		t.Errorf("expected 5 entities, got %d", count)
	}
	filter.Reset()
	count = 0
	for filter.Next() {
		count++
	}
	if count != 5 {
		t.Error("reset failed")
	}
	builder2.NewEntities(2)
	filter.Reset()
	for filter.Next() {
		p := filter.Get()
		p.X += 10
		p.Y += 10
	}
	filter.Reset()
	count = 0
	for filter.Next() {
		count++
		p := filter.Get()
		if p.X != 10 || p.Y != 10 {
			t.Errorf("component data incorrect after update, expected (10, 10), got (%f,%f)", p.X, p.Y)
		}
	}
	if count != 7 {
		t.Errorf("expected 7 entities, got %d", count)
	}
}

func TestFilter2(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(3)
	builder2 := NewBuilder2[Position, Velocity](&w)
	builder2.NewEntities(2)
	filter2 := NewFilter2[Position, Velocity](&w)
	count := 0
	for filter2.Next() {
		count++
		pos, vel := filter2.Get()
		if pos == nil || vel == nil {
			t.Error("nil components in filter2")
		}
	}
	if count != 2 {
		t.Errorf("expected 2 entities, got %d", count)
	}
}

func TestFilterRemoveEntities(t *testing.T) {
	w := NewWorld(10)
	initialFree := len(w.entities.freeIDs)
	builderPos := NewBuilder[Position](&w)
	posEnts := make([]Entity, 3)
	for i := 0; i < 3; i++ {
		posEnts[i] = builderPos.NewEntity()
	}
	builderVel := NewBuilder[Velocity](&w)
	velEnts := make([]Entity, 2)
	for i := 0; i < 2; i++ {
		velEnts[i] = builderVel.NewEntity()
	}
	filter := NewFilter[Position](&w)
	filter.RemoveEntities()
	if builderPos.arch.size != 0 {
		t.Errorf("expected pos arch size 0, got %d", builderPos.arch.size)
	}
	if builderVel.arch.size != 2 {
		t.Errorf("expected vel arch size 2, got %d", builderVel.arch.size)
	}
	for _, ent := range posEnts {
		if w.IsValid(ent) {
			t.Error("pos entity still valid after remove")
		}
	}
	for _, ent := range velEnts {
		if !w.IsValid(ent) {
			t.Error("vel entity invalid after remove")
		}
	}
	if len(w.entities.freeIDs) != initialFree-2 {
		t.Errorf("expected freeIDs %d, got %d", initialFree-2, len(w.entities.freeIDs))
	}
	// Check filter after remove
	count := 0
	for filter.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 after remove, got %d", count)
	}
}

func TestFilter2RemoveEntities(t *testing.T) {
	w := NewWorld(10)
	initialFree := len(w.entities.freeIDs)
	builderPV := NewBuilder2[Position, Velocity](&w)
	pvEnts := make([]Entity, 2)
	for i := 0; i < 2; i++ {
		pvEnts[i] = builderPV.NewEntity()
	}
	builderPos := NewBuilder[Position](&w)
	posEnts := make([]Entity, 3)
	for i := 0; i < 3; i++ {
		posEnts[i] = builderPos.NewEntity()
	}
	filter2 := NewFilter2[Position, Velocity](&w)
	filter2.RemoveEntities()
	if builderPV.arch.size != 0 {
		t.Errorf("expected pv arch size 0, got %d", builderPV.arch.size)
	}
	if builderPos.arch.size != 3 {
		t.Errorf("expected pos arch size 3, got %d", builderPos.arch.size)
	}
	for _, ent := range pvEnts {
		if w.IsValid(ent) {
			t.Error("pv entity still valid after remove")
		}
	}
	for _, ent := range posEnts {
		if !w.IsValid(ent) {
			t.Error("pos entity invalid after remove")
		}
	}
	if len(w.entities.freeIDs) != initialFree-3 {
		t.Errorf("expected freeIDs %d, got %d", initialFree-3, len(w.entities.freeIDs))
	}
	// Check filter after remove
	count := 0
	for filter2.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 after remove, got %d", count)
	}
}

// Data Integrity Tests
func TestDataIntegrityAfterRemoveEntity(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ents := make([]Entity, TestEntities)
	for i := 0; i < TestEntities; i++ {
		ents[i] = builder.NewEntity()
		pos := builder.Get(ents[i])
		*pos = Position{X: float32(i), Y: float32(i * 2)}
	}
	// Remove every other entity
	for i := 0; i < TestEntities; i += 2 {
		w.RemoveEntity(ents[i])
	}
	// Check remaining entities' data
	for i := 1; i < TestEntities; i += 2 {
		if !w.IsValid(ents[i]) {
			t.Errorf("entity %d should be valid", i)
		}
		pos := GetComponent[Position](&w, ents[i])
		if pos == nil {
			t.Errorf("position nil for entity %d", i)
		} else if pos.X != float32(i) || pos.Y != float32(i*2) {
			t.Errorf("data corrupted for entity %d: got (%f,%f), expected (%f,%f)", i, pos.X, pos.Y, float32(i), float32(i*2))
		}
	}
}

func TestDataIntegrityAfterSetComponentNew(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ents := make([]Entity, TestEntities)
	for i := 0; i < TestEntities; i++ {
		ents[i] = builder.NewEntity()
		pos := builder.Get(ents[i])
		*pos = Position{X: float32(i), Y: float32(i * 2)}
	}
	// Add velocity to every entity
	for i := 0; i < TestEntities; i++ {
		SetComponent(&w, ents[i], Velocity{DX: float32(i * 3), DY: float32(i * 4)})
	}
	// Check data
	for i := 0; i < TestEntities; i++ {
		pos := GetComponent[Position](&w, ents[i])
		if pos == nil || pos.X != float32(i) || pos.Y != float32(i*2) {
			t.Errorf("position corrupted for entity %d", i)
		}
		vel := GetComponent[Velocity](&w, ents[i])
		if vel == nil || vel.DX != float32(i*3) || vel.DY != float32(i*4) {
			t.Errorf("velocity incorrect for entity %d", i)
		}
	}
}

func TestDataIntegrityAfterRemoveComponent(t *testing.T) {
	w := NewWorld(TestCap)
	builder2 := NewBuilder2[Position, Velocity](&w)
	ents := make([]Entity, TestEntities)
	for i := 0; i < TestEntities; i++ {
		ents[i] = builder2.NewEntity()
		pos, vel := builder2.Get(ents[i])
		*pos = Position{X: float32(i), Y: float32(i * 2)}
		*vel = Velocity{DX: float32(i * 3), DY: float32(i * 4)}
	}
	// Remove velocity from every entity
	for i := 0; i < TestEntities; i++ {
		RemoveComponent[Velocity](&w, ents[i])
	}
	// Check data
	for i := 0; i < TestEntities; i++ {
		pos := GetComponent[Position](&w, ents[i])
		if pos == nil || pos.X != float32(i) || pos.Y != float32(i*2) {
			t.Errorf("position corrupted for entity %d", i)
		}
		vel := GetComponent[Velocity](&w, ents[i])
		if vel != nil {
			t.Errorf("velocity not removed for entity %d", i)
		}
	}
}

// Special Cases
func TestComponentWithPointer(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[WithPointer](&w)
	ent := builder.NewEntity()
	data := 42
	comp := builder.Get(ent)
	comp.Data = &data
	got := builder.Get(ent)
	if *got.Data != 42 {
		t.Error("pointer data not preserved")
	}
	// Add another component
	SetComponent(&w, ent, Position{X: 1, Y: 2})
	got = GetComponent[WithPointer](&w, ent)
	if got == nil || *got.Data != 42 {
		t.Error("pointer data lost after archetype move")
	}
}

func TestBuilderSet(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ent := w.CreateEntity()
	builder.Set(ent, Position{X: 1, Y: 2})
	pos := builder.Get(ent)
	if pos == nil || pos.X != 1 || pos.Y != 2 {
		t.Error("Set failed")
	}
	builder.Set(ent, Position{X: 3, Y: 4})
	pos = builder.Get(ent)
	if pos.X != 3 || pos.Y != 4 {
		t.Error("Update failed")
	}
}

func TestBuilderSetBatch(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ents := []Entity{w.CreateEntity(), w.CreateEntity()}
	builder.SetBatch(ents, Position{X: 5, Y: 6})
	for _, e := range ents {
		pos := builder.Get(e)
		if pos == nil || pos.X != 5 || pos.Y != 6 {
			t.Error("SetBatch failed")
		}
	}
}

func TestFilterEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(5)
	filter := NewFilter[Position](&w)
	ents := filter.Entities()
	if len(ents) != 5 {
		t.Errorf("expected 5, got %d", len(ents))
	}
}

// World Creation Benchmarks
func BenchmarkCreateWorld(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = NewWorld(size)
			}
		})
	}
}

// Expansion Benchmarks
func BenchmarkAutoExpand(b *testing.B) {
	initialSizes := []int{1000, 10000, 100000}
	expandMultiplier := 2
	for _, initSize := range initialSizes {
		name := fmt.Sprintf("%dK_init_x%d", initSize/1000, expandMultiplier)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			targetEntities := initSize * expandMultiplier
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(initSize)
				builder := NewBuilder[Position](&w)
				b.StartTimer()
				for j := 0; j < targetEntities; j++ {
					builder.NewEntity()
				}
			}
		})
	}
}

// World Entity Creation Benchmarks
func BenchmarkWorldCreateEntity(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				b.StartTimer()
				for j := 0; j < size; j++ {
					w.CreateEntity()
				}
			}
		})
	}
}

func BenchmarkWorldCreateEntities(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				b.StartTimer()
				w.CreateEntities(size)
			}
		})
	}
}

// Builder Benchmarks
func BenchmarkBuilderNewEntity(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](&w)
				b.StartTimer()
				for j := 0; j < size; j++ {
					builder.NewEntity()
				}
			}
		})
	}
}

func BenchmarkBuilderNewEntities(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](&w)
				b.StartTimer()
				builder.NewEntities(size)
			}
		})
	}
}

func BenchmarkBuilderNewEntitiesWithValueSet(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			val := Position{1, 2}
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](&w)
				b.StartTimer()
				builder.NewEntitiesWithValueSet(size, val)
			}
		})
	}
}

func BenchmarkBuilderNewEntitiesWithValueSet2(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			pos := Position{1, 2}
			vel := Velocity{3, 4}
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder2 := NewBuilder2[Position, Velocity](&w)
				b.StartTimer()
				builder2.NewEntitiesWithValueSet(size, pos, vel)
			}
		})
	}
}

func BenchmarkBuilderSet(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				ents := make([]Entity, size)
				for j := 0; j < size; j++ {
					ents[j] = w.CreateEntity()
				}
				builder := NewBuilder[Position](&w)
				b.StartTimer()
				for j := 0; j < size; j++ {
					builder.Set(ents[j], Position{})
				}
			}
		})
	}
}

// Component Operation Benchmarks
func BenchmarkBuilderGetComponent(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](&w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder.Get(ents[i%size])
			}
		})
	}
}

func BenchmarkBuilderGetComponent2(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder2[Position, Velocity](&w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder.Get(ents[i%size])
			}
		})
	}
}

func BenchmarkGlobalGetComponent(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](&w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				GetComponent[Position](&w, ents[i%size])
			}
		})
	}
}

func BenchmarkGlobalGetComponent2(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder2[Position, Velocity](&w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				GetComponent2[Position, Velocity](&w, ents[i%size])
			}
		})
	}
}

func BenchmarkSetComponentExisting(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](&w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			val := Position{1, 2}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SetComponent(&w, ents[i%size], val)
			}
		})
	}
}

func BenchmarkSetComponentNew(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			val := Velocity{3, 4}
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](&w)
				dummyBuilder := NewBuilder2[Position, Velocity](&w)
				dummy := dummyBuilder.NewEntity()
				w.RemoveEntity(dummy)
				builder.NewEntities(size)
				ents := builder.arch.entityIDs[:size]
				b.StartTimer()
				for j := 0; j < size; j++ {
					SetComponent(&w, ents[j], val)
				}
			}
		})
	}
}

// Removal Benchmarks
func BenchmarkRemoveComponent(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder2 := NewBuilder2[Position, Velocity](&w)
				dummyBuilder := NewBuilder[Position](&w)
				dummy := dummyBuilder.NewEntity()
				w.RemoveEntity(dummy)
				builder2.NewEntities(size)
				ents := builder2.arch.entityIDs[:size]
				b.StartTimer()
				for j := 0; j < size; j++ {
					RemoveComponent[Velocity](&w, ents[j])
				}
			}
		})
	}
}

func BenchmarkWorldRemoveEntity(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](&w)
				builder.NewEntities(size)
				ents := make([]Entity, size)
				copy(ents, builder.arch.entityIDs[:size])
				b.StartTimer()
				for j := 0; j < size; j++ {
					w.RemoveEntity(ents[j])
				}
			}
		})
	}
}

func BenchmarkWorldRemoveEntities(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](&w)
				builder.NewEntities(size)
				ents := make([]Entity, size)
				copy(ents, builder.arch.entityIDs[:size])
				b.StartTimer()
				for j := 0; j < size; j++ {
					w.RemoveEntity(ents[j])
				}
			}
		})
	}
}

func BenchmarkWorldClearEntities(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](&w)
				builder.NewEntities(size)
				b.StartTimer()
				w.ClearEntities()
			}
		})
	}
}

func BenchmarkFilterRemoveEntities(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](&w)
				builder.NewEntities(size)
				filter := NewFilter[Position](&w)
				b.StartTimer()
				filter.RemoveEntities()
			}
		})
	}
}

func BenchmarkFilter2RemoveEntities(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder2 := NewBuilder2[Position, Velocity](&w)
				builder2.NewEntities(size)
				filter2 := NewFilter2[Position, Velocity](&w)
				b.StartTimer()
				filter2.RemoveEntities()
			}
		})
	}
}

// Filter Iteration Benchmarks
func BenchmarkFilterIterate(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](&w)
			builder.NewEntities(size)
			filter := NewFilter[Position](&w)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filter.Reset()
				for filter.Next() {
					_ = filter.Get()
				}
			}
		})
	}
}

func BenchmarkFilter2Iterate(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder2 := NewBuilder2[Position, Velocity](&w)
			builder2.NewEntities(size)
			filter2 := NewFilter2[Position, Velocity](&w)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filter2.Reset()
				for filter2.Next() {
					_, _ = filter2.Get()
				}
			}
		})
	}
}

func BenchmarkFilterGetEntitiesCached(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](&w)
			builder.NewEntities(size)
			filter := NewFilter[Position](&w)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filter.Entities()
			}
		})
	}
}

func BenchmarkFilterGetEntitiesUncached(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](&w)
			filter := NewFilter[Position](&w)

			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w.ClearEntities()
				builder.NewEntities(size)
				b.StartTimer()
				filter.Entities()
			}
		})
	}
}
