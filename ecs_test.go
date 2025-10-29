package teishoku

import (
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

type Dummy1 struct {
	Val float32
}

type Dummy2 struct {
	Val float32
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
}

func TestBuilderNewEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	count := 5
	builder.NewEntities(count)
	if builder.arch.size != count {
		t.Errorf("expected size %d, got %d", count, builder.arch.size)
	}
}

func TestBuilderNewEntitiesWithValueSet(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	count := 5
	val := Position{X: 1.0, Y: 2.0}
	builder.NewEntitiesWithValueSet(count, val)
	if builder.arch.size != count {
		t.Errorf("expected size %d, got %d", count, builder.arch.size)
	}
	for i := 0; i < count; i++ {
		pos := builder.Get(builder.arch.entityIDs[i])
		if *pos != val {
			t.Errorf("expected pos %+v, got %+v", val, *pos)
		}
	}
}

func TestBuilderGet(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ent := builder.NewEntity()
	val := Position{X: 1.0, Y: 2.0}
	builder.Set(ent, val)
	pos := builder.Get(ent)
	if *pos != val {
		t.Errorf("expected pos %+v, got %+v", val, *pos)
	}
}

func TestBuilderSet(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	ent := builder.NewEntity()
	val := Position{X: 1.0, Y: 2.0}
	builder.Set(ent, val)
	pos := GetComponent[Position](&w, ent)
	if *pos != val {
		t.Errorf("expected pos %+v, got %+v", val, *pos)
	}
}

// Filter Tests
func TestFilter(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(5)
	filter := NewFilter[Position](&w)
	count := 0
	for filter.Next() {
		count++
	}
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}

func TestFilter2(t *testing.T) {
	w := NewWorld(TestCap)
	builder2 := NewBuilder2[Position, Velocity](&w)
	builder2.NewEntities(5)
	filter2 := NewFilter2[Position, Velocity](&w)
	count := 0
	for filter2.Next() {
		count++
	}
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}

func TestFilter0(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(5) // Entities with components
	w.CreateEntities(3)    // Entities without components

	filter0 := NewFilter0(&w)
	count := 0
	for filter0.Next() {
		count++
	}
	if count != 3 {
		t.Errorf("expected 3 entities with no components, got %d", count)
	}
}

func TestFilterRemoveEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(3)
	filter := NewFilter[Position](&w)
	filter.RemoveEntities()
	count := 0
	for filter.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 after remove, got %d", count)
	}
}

func TestFilter2RemoveEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder2 := NewBuilder2[Position, Velocity](&w)
	builder2.NewEntities(2)
	filter2 := NewFilter2[Position, Velocity](&w)
	filter2.RemoveEntities()
	count := 0
	for filter2.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 after remove, got %d", count)
	}
}

func TestFilterDataIntegrity(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	count := 10

	// Create entities with initial value
	initialValue := Position{X: 1.0, Y: 1.0}
	builder.NewEntitiesWithValueSet(count, initialValue)

	// Verify initial values
	filter := NewFilter[Position](&w)
	entityCount := 0
	for filter.Next() {
		pos := filter.Get()
		if *pos != initialValue {
			t.Errorf("Expected initial value %+v, got %+v", initialValue, *pos)
		}
		entityCount++
	}
	if entityCount != count {
		t.Errorf("Expected to filter %d entities, but got %d", count, entityCount)
	}

	// Modify component values
	updatedValue := Position{X: 2.0, Y: 2.0}
	filter.Reset()
	for filter.Next() {
		pos := filter.Get()
		*pos = updatedValue
	}

	// Verify updated values
	filter.Reset()
	for filter.Next() {
		pos := filter.Get()
		if *pos != updatedValue {
			t.Errorf("Expected updated value %+v, got %+v", updatedValue, *pos)
		}
	}

	// Remove one entity and check count
	entityToRemove := filter.Entities()[0]
	w.RemoveEntity(entityToRemove)
	filter.Reset() // Reset to re-evaluate archetypes and entities
	entityCount = 0
	for filter.Next() {
		entityCount++
	}
	if entityCount != count-1 {
		t.Errorf("Expected %d entities after removal, but got %d", count-1, entityCount)
	}
}
