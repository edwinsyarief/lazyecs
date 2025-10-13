package lazyecs

import (
	"fmt"
	"testing"
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

func BenchmarkNewEntitiesWithValueSet(b *testing.B) {
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

func BenchmarkNewEntitiesWithValueSet2(b *testing.B) {
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

// Component Operation Benchmarks
func BenchmarkGetComponent(b *testing.B) {
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
