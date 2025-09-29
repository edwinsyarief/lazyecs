package lazyecs

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

func TestNewWorld(t *testing.T) {
	w := NewWorld(TestCap)
	if w.capacity != TestCap {
		t.Errorf("expected capacity %d, got %d", TestCap, w.capacity)
	}
	if len(w.freeIDs) != TestCap {
		t.Errorf("expected %d free IDs, got %d", TestCap, len(w.freeIDs))
	}
	if len(w.metas) != TestCap {
		t.Errorf("expected %d metas, got %d", TestCap, len(w.metas))
	}
	if len(w.archetypes) != 0 {
		t.Errorf("expected 0 archetypes, got %d", len(w.archetypes))
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
	if w.nextCompTypeID != 2 {
		t.Errorf("expected nextCompTypeID 2, got %d", w.nextCompTypeID)
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
	if len(w.archetypes) != 1 {
		t.Errorf("expected 1 archetype, got %d", len(w.archetypes))
	}
	a2 := w.getOrCreateArchetype(mask, specs)
	if a1 != a2 {
		t.Errorf("expected same archetype, got different")
	}
}

func TestCreateEntity(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](w)
	ent := builder.NewEntity()
	if !w.IsValid(ent) {
		t.Error("entity should be valid")
	}
	if w.metas[ent.ID].archetypeIndex == -1 {
		t.Error("archetypeIndex not set")
	}
	if w.metas[ent.ID].index != 0 {
		t.Errorf("expected index 0, got %d", w.metas[ent.ID].index)
	}
	if w.metas[ent.ID].version != ent.Version {
		t.Error("version mismatch")
	}
	a := w.archetypes[w.metas[ent.ID].archetypeIndex]
	if a.size != 1 {
		t.Errorf("expected size 1, got %d", a.size)
	}
	if a.entityIDs[0] != ent {
		t.Error("entity not in archetype")
	}
}

func TestNewEntitiesBatch(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](w)
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

func TestGetComponent(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](w)
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
	velBuilder := NewBuilder[Velocity](w)
	if velBuilder.Get(ent) != nil {
		t.Error("expected nil for missing component")
	}
}

func TestSetComponent(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](w)
	ent := builder.NewEntity()
	SetComponent(w, ent, Velocity{DX: 3, DY: 4})
	if !w.IsValid(ent) {
		t.Error("entity invalid after set")
	}
	vel := GetComponent[Velocity](w, ent)
	if vel == nil {
		t.Fatal("velocity not set")
	}
	if vel.DX != 3 || vel.DY != 4 {
		t.Error("velocity values incorrect")
	}
	pos := GetComponent[Position](w, ent)
	if pos == nil {
		t.Error("position lost after set")
	}
	// set existing
	SetComponent(w, ent, Position{X: 5, Y: 6})
	pos = GetComponent[Position](w, ent)
	if pos.X != 5 || pos.Y != 6 {
		t.Error("position not updated")
	}
}

func TestRemoveComponent(t *testing.T) {
	w := NewWorld(TestCap)
	builder2 := NewBuilder2[Position, Velocity](w)
	ent := builder2.NewEntity()
	RemoveComponent[Velocity](w, ent)
	if GetComponent[Velocity](w, ent) != nil {
		t.Error("velocity not removed")
	}
	if GetComponent[Position](w, ent) == nil {
		t.Error("position lost")
	}
	// remove non-existing
	RemoveComponent[Health](w, ent)
	if !w.IsValid(ent) {
		t.Error("entity invalid after removing non-existing")
	}
}

func TestRemoveEntity(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](w)
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
	if w.metas[ent2.ID].index != 0 {
		t.Error("ent2 index not updated")
	}
	// remove stale
	w.RemoveEntity(ent1)
	if a.size != 1 {
		t.Error("size changed on stale remove")
	}
}

func TestFilter(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](w)
	builder.NewEntities(3)
	builder2 := NewBuilder2[Position, Velocity](w)
	builder2.NewEntities(2)
	filter := NewFilter[Position](w)
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
}

func TestFilter2(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](w)
	builder.NewEntities(3)
	builder2 := NewBuilder2[Position, Velocity](w)
	builder2.NewEntities(2)
	filter2 := NewFilter2[Position, Velocity](w)
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

func TestDataIntegrityAfterRemoveEntity(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](w)
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
		pos := GetComponent[Position](w, ents[i])
		if pos == nil {
			t.Errorf("position nil for entity %d", i)
		} else if pos.X != float32(i) || pos.Y != float32(i*2) {
			t.Errorf("data corrupted for entity %d: got (%f,%f), expected (%f,%f)", i, pos.X, pos.Y, float32(i), float32(i*2))
		}
	}
}

func TestDataIntegrityAfterSetComponentNew(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[Position](w)
	ents := make([]Entity, TestEntities)
	for i := 0; i < TestEntities; i++ {
		ents[i] = builder.NewEntity()
		pos := builder.Get(ents[i])
		*pos = Position{X: float32(i), Y: float32(i * 2)}
	}
	// Add velocity to every entity
	for i := 0; i < TestEntities; i++ {
		SetComponent(w, ents[i], Velocity{DX: float32(i * 3), DY: float32(i * 4)})
	}
	// Check data
	for i := 0; i < TestEntities; i++ {
		pos := GetComponent[Position](w, ents[i])
		if pos == nil || pos.X != float32(i) || pos.Y != float32(i*2) {
			t.Errorf("position corrupted for entity %d", i)
		}
		vel := GetComponent[Velocity](w, ents[i])
		if vel == nil || vel.DX != float32(i*3) || vel.DY != float32(i*4) {
			t.Errorf("velocity incorrect for entity %d", i)
		}
	}
}

func TestDataIntegrityAfterRemoveComponent(t *testing.T) {
	w := NewWorld(TestCap)
	builder2 := NewBuilder2[Position, Velocity](w)
	ents := make([]Entity, TestEntities)
	for i := 0; i < TestEntities; i++ {
		ents[i] = builder2.NewEntity()
		pos, vel := builder2.Get(ents[i])
		*pos = Position{X: float32(i), Y: float32(i * 2)}
		*vel = Velocity{DX: float32(i * 3), DY: float32(i * 4)}
	}
	// Remove velocity from every entity
	for i := 0; i < TestEntities; i++ {
		RemoveComponent[Velocity](w, ents[i])
	}
	// Check data
	for i := 0; i < TestEntities; i++ {
		pos := GetComponent[Position](w, ents[i])
		if pos == nil || pos.X != float32(i) || pos.Y != float32(i*2) {
			t.Errorf("position corrupted for entity %d", i)
		}
		vel := GetComponent[Velocity](w, ents[i])
		if vel != nil {
			t.Errorf("velocity not removed for entity %d", i)
		}
	}
}

func TestComponentWithPointer(t *testing.T) {
	w := NewWorld(TestCap)
	builder := NewBuilder[WithPointer](w)
	ent := builder.NewEntity()
	data := 42
	comp := builder.Get(ent)
	comp.Data = &data
	got := builder.Get(ent)
	if *got.Data != 42 {
		t.Error("pointer data not preserved")
	}
	// Add another component
	SetComponent(w, ent, Position{X: 1, Y: 2})
	got = GetComponent[WithPointer](w, ent)
	if got == nil || *got.Data != 42 {
		t.Error("pointer data lost after archetype move")
	}
}

func TestGeneratedAPIs(t *testing.T) {
	t.Run("GetComponents2", func(t *testing.T) {
		w := NewWorld(TestCap)
		builder := NewBuilder2[Position, Velocity](w)
		ent := builder.NewEntity()
		pos, vel, ok := GetComponents2[Position, Velocity](w, ent)
		if !ok {
			t.Fatal("GetComponents2 returned not ok")
		}
		if pos == nil || vel == nil {
			t.Fatal("GetComponents2 returned nil components")
		}
		*pos = Position{1, 2}
		*vel = Velocity{3, 4}
		gotPos, gotVel, ok := GetComponents2[Position, Velocity](w, ent)
		if !ok {
			t.Fatal("GetComponents2 returned not ok on second call")
		}
		if gotPos.X != 1 || gotPos.Y != 2 {
			t.Error("Position not set correctly")
		}
		if gotVel.DX != 3 || gotVel.DY != 4 {
			t.Error("Velocity not set correctly")
		}
	})

	t.Run("SetComponents2", func(t *testing.T) {
		w := NewWorld(TestCap)
		builder := NewBuilder[Position](w)
		ent := builder.NewEntity()
		SetComponents2(w, ent, Velocity{DX: 3, DY: 4}, Health{HP: 100})
		if !w.IsValid(ent) {
			t.Error("entity invalid after set")
		}
		vel := GetComponent[Velocity](w, ent)
		if vel == nil || vel.DX != 3 || vel.DY != 4 {
			t.Error("velocity values incorrect")
		}
		health := GetComponent[Health](w, ent)
		if health == nil || health.HP != 100 {
			t.Error("health values incorrect")
		}
		pos := GetComponent[Position](w, ent)
		if pos == nil {
			t.Error("position lost after set")
		}
	})

	t.Run("RemoveComponents2", func(t *testing.T) {
		w := NewWorld(TestCap)
		builder3 := NewBuilder3[Position, Velocity, Health](w)
		ent := builder3.NewEntity()
		RemoveComponents2[Velocity, Health](w, ent)
		if GetComponent[Velocity](w, ent) != nil {
			t.Error("velocity not removed")
		}
		if GetComponent[Health](w, ent) != nil {
			t.Error("health not removed")
		}
		if GetComponent[Position](w, ent) == nil {
			t.Error("position lost")
		}
	})

	t.Run("GetComponents3", func(t *testing.T) {
		w := NewWorld(TestCap)
		builder := NewBuilder3[Position, Velocity, Health](w)
		ent := builder.NewEntity()
		pos, vel, health, ok := GetComponents3[Position, Velocity, Health](w, ent)
		if !ok {
			t.Fatal("GetComponents3 returned not ok")
		}
		if pos == nil || vel == nil || health == nil {
			t.Fatal("GetComponents3 returned nil components")
		}
		*pos = Position{1, 2}
		*vel = Velocity{3, 4}
		*health = Health{100}
		gotPos, gotVel, gotHealth, ok := GetComponents3[Position, Velocity, Health](w, ent)
		if !ok {
			t.Fatal("GetComponents3 returned not ok on second call")
		}
		if gotPos.X != 1 || gotPos.Y != 2 {
			t.Error("Position not set correctly")
		}
		if gotVel.DX != 3 || gotVel.DY != 4 {
			t.Error("Velocity not set correctly")
		}
		if gotHealth.HP != 100 {
			t.Error("Health not set correctly")
		}
	})

	t.Run("SetComponents3", func(t *testing.T) {
		w := NewWorld(TestCap)
		builder := NewBuilder[Position](w)
		ent := builder.NewEntity()
		SetComponents3(w, ent, Velocity{DX: 3, DY: 4}, Health{HP: 100}, WithPointer{Data: new(int)})
		if !w.IsValid(ent) {
			t.Error("entity invalid after set")
		}
		vel := GetComponent[Velocity](w, ent)
		if vel == nil || vel.DX != 3 || vel.DY != 4 {
			t.Error("velocity values incorrect")
		}
		health := GetComponent[Health](w, ent)
		if health == nil || health.HP != 100 {
			t.Error("health values incorrect")
		}
		ptr := GetComponent[WithPointer](w, ent)
		if ptr == nil || ptr.Data == nil {
			t.Error("pointer values incorrect")
		}
		pos := GetComponent[Position](w, ent)
		if pos == nil {
			t.Error("position lost after set")
		}
	})

	t.Run("RemoveComponents3", func(t *testing.T) {
		w := NewWorld(TestCap)
		builder := NewBuilder[Position](w)
		e := builder.NewEntity()
		SetComponents3(w, e, Velocity{3, 4}, Health{100}, WithPointer{new(int)})
		RemoveComponents3[Velocity, Health, WithPointer](w, e)
		if GetComponent[Velocity](w, e) != nil {
			t.Error("velocity not removed")
		}
		if GetComponent[Health](w, e) != nil {
			t.Error("health not removed")
		}
		if GetComponent[WithPointer](w, e) != nil {
			t.Error("withpointer not removed")
		}
		if GetComponent[Position](w, e) == nil {
			t.Error("position lost")
		}
	})
}

// Assuming Position, Velocity, Entity, NewWorld, NewBuilder, NewBuilder2, NewFilter, NewFilter2, SetComponent, RemoveComponent are defined elsewhere.

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

func BenchmarkGetComponents2(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder2[Position, Velocity](w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				GetComponents2[Position, Velocity](w, ents[i%size])
			}
		})
	}
}

func BenchmarkSetComponents2_New(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			v1 := Velocity{3, 4}
			v2 := Health{100}
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				builder.NewEntities(size)
				ents := builder.arch.entityIDs[:size]
				b.StartTimer()
				for j := 0; j < size; j++ {
					SetComponents2(w, ents[j], v1, v2)
				}
			}
		})
	}
}

func BenchmarkSetComponents2_Existing(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder2[Position, Velocity](w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			v1 := Position{1, 2}
			v2 := Velocity{3, 4}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SetComponents2(w, ents[i%size], v1, v2)
			}
		})
	}
}

func BenchmarkRemoveComponents2(b *testing.B) {
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
				builder3 := NewBuilder3[Position, Velocity, Health](w)
				builder3.NewEntities(size)
				ents := builder3.arch.entityIDs[:size]
				b.StartTimer()
				for j := 0; j < size; j++ {
					RemoveComponents2[Velocity, Health](w, ents[j])
				}
			}
		})
	}
}

func BenchmarkGetComponents3(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder3[Position, Velocity, Health](w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				GetComponents3[Position, Velocity, Health](w, ents[i%size])
			}
		})
	}
}

func BenchmarkSetComponents3_New(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			v1 := Velocity{3, 4}
			v2 := Health{100}
			v3 := WithPointer{new(int)}
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				builder.NewEntities(size)
				ents := builder.arch.entityIDs[:size]
				b.StartTimer()
				for j := 0; j < size; j++ {
					SetComponents3(w, ents[j], v1, v2, v3)
				}
			}
		})
	}
}

func BenchmarkSetComponents3_Existing(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder3[Position, Velocity, Health](w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			v1 := Position{1, 2}
			v2 := Velocity{3, 4}
			v3 := Health{100}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SetComponents3(w, ents[i%size], v1, v2, v3)
			}
		})
	}
}

func BenchmarkRemoveComponents3(b *testing.B) {
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
				builder := NewBuilder[Position](w)
				builder.NewEntities(size)
				ents := builder.arch.entityIDs[:size]
				SetComponents3(w, ents[0], Velocity{3, 4}, Health{100}, WithPointer{new(int)})
				b.StartTimer()
				for j := 0; j < size; j++ {
					RemoveComponents3[Velocity, Health, WithPointer](w, ents[j])
				}
			}
		})
	}
}

func BenchmarkCreateEntity(b *testing.B) {
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
				builder := NewBuilder[Position](w)
				b.StartTimer()
				for j := 0; j < size; j++ {
					builder.NewEntity()
				}
			}
		})
	}
}

func BenchmarkNewEntitiesBatch(b *testing.B) {
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
				builder := NewBuilder[Position](w)
				b.StartTimer()
				builder.NewEntities(size)
			}
		})
	}
}

func BenchmarkGetComponent(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](w)
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

func BenchmarkSetComponentExisting(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			val := Position{1, 2}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				SetComponent(w, ents[i%size], val)
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
				builder := NewBuilder[Position](w)
				builder.NewEntities(size)
				ents := builder.arch.entityIDs[:size]
				b.StartTimer()
				for j := 0; j < size; j++ {
					SetComponent(w, ents[j], val)
				}
			}
		})
	}
}

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
				builder2 := NewBuilder2[Position, Velocity](w)
				builder2.NewEntities(size)
				ents := builder2.arch.entityIDs[:size]
				b.StartTimer()
				for j := 0; j < size; j++ {
					RemoveComponent[Velocity](w, ents[j])
				}
			}
		})
	}
}

func BenchmarkRemoveEntity(b *testing.B) {
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
				builder := NewBuilder[Position](w)
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

func BenchmarkFilterIterate(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			w := NewWorld(size)
			builder := NewBuilder[Position](w)
			builder.NewEntities(size)
			filter := NewFilter[Position](w)
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
			builder2 := NewBuilder2[Position, Velocity](w)
			builder2.NewEntities(size)
			filter2 := NewFilter2[Position, Velocity](w)
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
