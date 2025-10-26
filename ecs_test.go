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
	// Verify archetype chunks sufficient
	a := builder.arch
	if len(a.chunks)*ChunkSize < initialCap+extra {
		t.Errorf("expected archetype chunks sufficient for %d entities", initialCap+extra)
	}
	// Check data integrity after expand
	filter := NewFilter[Position](&w)
	count := 0
	for filter.Next() {
		pos := filter.Get()
		if pos == nil {
			t.Errorf("position nil for entity after expand")
		}
		count++
	}
	if count != initialCap+extra {
		t.Errorf("expected %d entities, got %d", initialCap+extra, count)
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
		t.Error("created entity invalid")
	}
	pos := builder.Get(ent)
	if pos == nil {
		t.Error("component not found")
	}
}

func TestBuilderNewEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(10)
	filter := NewFilter[Position](&w)
	count := 0
	for filter.Next() {
		count++
	}
	if count != 10 {
		t.Errorf("expected 10 entities, got %d", count)
	}
}

func TestBuilderNewEntitiesWithValueSet(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntitiesWithValueSet(10, Position{X: 1.0, Y: 2.0})
	filter := NewFilter[Position](&w)
	count := 0
	for filter.Next() {
		pos := filter.Get()
		if pos.X != 1.0 || pos.Y != 2.0 {
			t.Error("component value not set")
		}
		count++
	}
	if count != 10 {
		t.Errorf("expected 10 entities, got %d", count)
	}
}

func TestBuilderGetSet(t *testing.T) {
	w := NewWorld(TestCap)
	ent := w.CreateEntity()
	builder := NewBuilder[Position](&w)
	builder.Set(ent, Position{X: 3.0, Y: 4.0})
	pos := builder.Get(ent)
	if pos == nil || pos.X != 3.0 || pos.Y != 4.0 {
		t.Error("set/get failed")
	}
}

func TestBuilderSetBatch(t *testing.T) {
	w := NewWorld(TestCap)
	ents := w.CreateEntities(5)
	builder := NewBuilder[Position](&w)
	builder.SetBatch(ents, Position{X: 5.0, Y: 6.0})
	for _, ent := range ents {
		pos := builder.Get(ent)
		if pos == nil || pos.X != 5.0 || pos.Y != 6.0 {
			t.Error("set batch failed")
		}
	}
}

// Filter Tests
func TestFilterIteration(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(10)
	filter := NewFilter[Position](&w)
	count := 0
	for filter.Next() {
		count++
		pos := filter.Get()
		if pos == nil {
			t.Error("component nil")
		}
	}
	if count != 10 {
		t.Errorf("expected 10 entities, got %d", count)
	}
}

func TestFilterRemoveEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(10)
	filter := NewFilter[Position](&w)
	filter.RemoveEntities()
	count := 0
	filter.Reset()
	for filter.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 entities after remove, got %d", count)
	}
}

func TestFilterEntities(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntities(5)
	filter := NewFilter[Position](&w)
	ents := filter.Entities()
	if len(ents) != 5 {
		t.Errorf("expected 5 entities, got %d", len(ents))
	}
}

// Multi-component Tests
func TestBuilder2(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder2[Position, Velocity](&w)
	ent := builder.NewEntity()
	if !w.IsValid(ent) {
		t.Error("entity invalid")
	}
	pos, vel := builder.Get(ent)
	if pos == nil || vel == nil {
		t.Error("components not found")
	}
}

func TestFilter2Iteration(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder2[Position, Velocity](&w)
	builder.NewEntities(10)
	filter := NewFilter2[Position, Velocity](&w)
	count := 0
	for filter.Next() {
		count++
		pos, vel := filter.Get()
		if pos == nil || vel == nil {
			t.Error("components nil")
		}
	}
	if count != 10 {
		t.Errorf("expected 10 entities, got %d", count)
	}
}

// Data Integrity Test
func TestDataIntegrity(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](&w)
	builder.NewEntitiesWithValueSet(TestEntities, Position{X: 1.0, Y: 2.0})
	filter := NewFilter[Position](&w)
	count := 0
	for filter.Next() {
		pos := filter.Get()
		if pos.X != 1.0 || pos.Y != 2.0 {
			t.Error("data corruption")
		}
		count++
	}
	if count != TestEntities {
		t.Errorf("expected %d entities, got %d", TestEntities, count)
	}
}

// Mutation Tests
func TestSetComponent(t *testing.T) {
	w := NewWorld(TestCap)
	ent := w.CreateEntity()
	SetComponent(&w, ent, Position{X: 3.0, Y: 4.0})
	pos := GetComponent[Position](&w, ent)
	if pos == nil || pos.X != 3.0 || pos.Y != 4.0 {
		t.Error("set component failed")
	}
}

func TestRemoveComponent(t *testing.T) {
	w := NewWorld(TestCap)
	ent := w.CreateEntity()
	SetComponent(&w, ent, Position{})
	RemoveComponent[Position](&w, ent)
	pos := GetComponent[Position](&w, ent)
	if pos != nil {
		t.Error("remove component failed")
	}
}

func TestRemoveEntity(t *testing.T) {
	w := NewWorld(TestCap)
	ent := w.CreateEntity()
	w.RemoveEntity(ent)
	if w.IsValid(ent) {
		t.Error("entity still valid after remove")
	}
}

func TestRemoveEntities(t *testing.T) {
	w := NewWorld(TestCap)
	ents := w.CreateEntities(5)
	w.RemoveEntities(ents)
	for _, ent := range ents {
		if w.IsValid(ent) {
			t.Error("entity still valid after batch remove")
		}
	}
}

// Benchmark Creation
func BenchmarkBuilderNewEntities(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](&w)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder.NewEntities(size)
				w.ClearEntities()
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
					// _, _ = filter2.Get()
				}
			}
		})
	}
}

func BenchmarkFilter3Iterate(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder3 := NewBuilder3[Position, Velocity, Health](&w)
			builder3.NewEntities(size)
			filter3 := NewFilter3[Position, Velocity, Health](&w)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filter3.Reset()
				for filter3.Next() {
					// _, _, _ = filter3.Get()
				}
			}
		})
	}
}

func BenchmarkFilter4Iterate(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder4 := NewBuilder4[Position, Velocity, Health, WithPointer](&w)
			builder4.NewEntities(size)
			filter4 := NewFilter4[Position, Velocity, Health, WithPointer](&w)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filter4.Reset()
				for filter4.Next() {
					// _, _, _, _ = filter4.Get()
				}
			}
		})
	}
}

func BenchmarkFilter5Iterate(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder5 := NewBuilder5[Position, Velocity, Health, WithPointer, Dummy1](&w)
			builder5.NewEntities(size)
			filter5 := NewFilter5[Position, Velocity, Health, WithPointer, Dummy1](&w)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filter5.Reset()
				for filter5.Next() {
					// _, _, _, _, _ = filter5.Get()
				}
			}
		})
	}
}

func BenchmarkFilter6Iterate(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder6 := NewBuilder6[Position, Velocity, Health, WithPointer, Dummy1, Dummy2](&w)
			builder6.NewEntities(size)
			filter6 := NewFilter6[Position, Velocity, Health, WithPointer, Dummy1, Dummy2](&w)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filter6.Reset()
				for filter6.Next() {
					// _, _, _, _, _, _ = filter6.Get()
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
