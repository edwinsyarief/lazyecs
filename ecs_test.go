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

// Add more builder tests if necessary...

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
					//_, _ = filter2.Get()
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