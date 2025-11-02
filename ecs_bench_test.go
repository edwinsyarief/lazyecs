package teishoku

import (
	"fmt"
	"testing"
)

/* // World Creation Benchmarks
func BenchmarkCreateWorld(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				_ = NewWorld(size)
			}
			b.ReportAllocs()
		})
	}
}

// Expansion Benchmarks
func BenchmarkAutoExpand(b *testing.B) {
	initialSizes := []int{1000, 10000, 100000, 1000000}
	expandMultiplier := 2
	for _, initSize := range initialSizes {
		name := fmt.Sprintf("%dK_init_x%d", initSize/1000, expandMultiplier)
		b.Run(name, func(b *testing.B) {
			targetEntities := initSize * expandMultiplier
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(initSize)
				builder := NewBuilder[Position](w)
				b.StartTimer()
				for j := range targetEntities {
					_ = j
					builder.NewEntity()
				}
			}
			b.ReportAllocs()
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
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				b.StartTimer()
				for j := range size {
					_ = j
					w.CreateEntity()
				}
			}
			b.ReportAllocs()
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
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				b.StartTimer()
				w.CreateEntities(size)
			}
			b.ReportAllocs()
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
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				b.StartTimer()
				for j := range size {
					_ = j
					builder.NewEntity()
				}
			}
			b.ReportAllocs()
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
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				b.StartTimer()
				builder.NewEntities(size)
			}
			b.ReportAllocs()
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
			val := Position{1, 2}
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				b.StartTimer()
				builder.NewEntitiesWithValueSet(size, val)
			}
			b.ReportAllocs()
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
			pos := Position{1, 2}
			vel := Velocity{3, 4}
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder2 := NewBuilder2[Position, Velocity](w)
				b.StartTimer()
				builder2.NewEntitiesWithValueSet(size, pos, vel)
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkBuilderSetComponent(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				ents := make([]Entity, size)
				for j := range size {
					ents[j] = w.CreateEntity()
				}
				builder := NewBuilder[Position](w)
				b.StartTimer()
				for j := range size {
					builder.Set(ents[j], Position{})
				}
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkBuilderSetComponent2(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				ents := make([]Entity, size)
				for j := range size {
					ents[j] = w.CreateEntity()
				}
				builder := NewBuilder2[Position, Velocity](w)
				b.StartTimer()
				for j := range size {
					builder.Set(ents[j], Position{}, Velocity{})
				}
			}
			b.ReportAllocs()
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
			builder := NewBuilder[Position](w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			for b.Loop() {
				for j := range size {
					builder.Get(ents[j%size])
				}
			}
			b.ReportAllocs()
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
			builder := NewBuilder2[Position, Velocity](w)
			builder.NewEntities(size)
			ents := builder.arch.entityIDs[:size]
			for b.Loop() {
				for j := range size {
					builder.Get(ents[j%size])
				}
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkFunctionsGetComponent(b *testing.B) {
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
			for b.Loop() {
				for j := range size {
					GetComponent[Position](w, ents[j%size])
				}
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkFunctionsGetComponent2(b *testing.B) {
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
			for b.Loop() {
				for j := range size {
					GetComponent2[Position, Velocity](w, ents[j%size])
				}
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkFunctionsSetComponentExisting(b *testing.B) {
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
			for b.Loop() {
				for j := range size {
					SetComponent(w, ents[j%size], val)
				}
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkFunctionsSetComponentNew(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			val := Velocity{3, 4}
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				dummyBuilder := NewBuilder2[Position, Velocity](w)
				dummy := dummyBuilder.NewEntity()
				w.RemoveEntity(dummy)
				builder.NewEntities(size)
				ents := builder.arch.entityIDs[:size]
				b.StartTimer()
				for j := range size {
					SetComponent(w, ents[j], val)
				}
			}
			b.ReportAllocs()
		})
	}
}

// Removal Benchmarks
func BenchmarkFunctionsRemoveComponent(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder2 := NewBuilder2[Position, Velocity](w)
				dummyBuilder := NewBuilder[Position](w)
				dummy := dummyBuilder.NewEntity()
				w.RemoveEntity(dummy)
				builder2.NewEntities(size)
				ents := builder2.arch.entityIDs[:size]
				b.StartTimer()
				for j := range size {
					RemoveComponent[Velocity](w, ents[j])
				}
			}
			b.ReportAllocs()
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
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				builder.NewEntities(size)
				ents := make([]Entity, size)
				copy(ents, builder.arch.entityIDs[:size])
				b.StartTimer()
				for j := range size {
					w.RemoveEntity(ents[j])
				}
			}
			b.ReportAllocs()
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
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				builder.NewEntities(size)
				ents := make([]Entity, size)
				copy(ents, builder.arch.entityIDs[:size])
				b.StartTimer()
				for j := range size {
					w.RemoveEntity(ents[j])
				}
			}
			b.ReportAllocs()
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
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				builder.NewEntities(size)
				b.StartTimer()
				w.ClearEntities()
			}
			b.ReportAllocs()
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
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder := NewBuilder[Position](w)
				builder.NewEntities(size)
				filter := NewFilter[Position](w)
				b.StartTimer()
				filter.RemoveEntities()
			}
			b.ReportAllocs()
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
			for b.Loop() {
				b.StopTimer()
				w := NewWorld(size)
				builder2 := NewBuilder2[Position, Velocity](w)
				builder2.NewEntities(size)
				filter2 := NewFilter2[Position, Velocity](w)
				b.StartTimer()
				filter2.RemoveEntities()
			}
			b.ReportAllocs()
		})
	}
} */

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
			builder := NewBuilder[Position](w)
			builder.NewEntities(size)
			filter := NewFilter[Position](w)
			for b.Loop() {
				query := filter.Query()
				for query.Next() {
					_ = query.Get()
				}
			}
			b.ReportAllocs()
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
			for b.Loop() {
				query := filter2.Query()
				for query.Next() {
					_, _ = query.Get()
				}
			}
			b.ReportAllocs()
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
			builder3 := NewBuilder3[Position, Velocity, Health](w)
			builder3.NewEntities(size)
			filter3 := NewFilter3[Position, Velocity, Health](w)
			for b.Loop() {
				query := filter3.Query()
				for query.Next() {
					_, _, _ = query.Get()
				}
			}
			b.ReportAllocs()
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
			builder4 := NewBuilder4[Position, Velocity, Health, WithPointer](w)
			builder4.NewEntities(size)
			filter4 := NewFilter4[Position, Velocity, Health, WithPointer](w)
			for b.Loop() {
				query := filter4.Query()
				for query.Next() {
					_, _, _, _ = query.Get()
				}
			}
			b.ReportAllocs()
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
			builder5 := NewBuilder5[Position, Velocity, Health, WithPointer, Dummy1](w)
			builder5.NewEntities(size)
			filter5 := NewFilter5[Position, Velocity, Health, WithPointer, Dummy1](w)
			for b.Loop() {
				query := filter5.Query()
				for query.Next() {
					_, _, _, _, _ = query.Get()
				}
			}
			b.ReportAllocs()
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
			builder6 := NewBuilder6[Position, Velocity, Health, WithPointer, Dummy1, Dummy2](w)
			builder6.NewEntities(size)
			filter6 := NewFilter6[Position, Velocity, Health, WithPointer, Dummy1, Dummy2](w)
			for b.Loop() {
				query := filter6.Query()
				for query.Next() {
					_, _, _, _, _, _ = query.Get()
				}
			}
			b.ReportAllocs()
		})
	}
}

/* func BenchmarkFilterGetEntitiesCached(b *testing.B) {
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
			for b.Loop() {
				filter.Entities()
			}
			b.ReportAllocs()
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
			builder := NewBuilder[Position](w)
			filter := NewFilter[Position](w)
			for b.Loop() {
				b.StopTimer()
				w.ClearEntities()
				builder.NewEntities(size)
				b.StartTimer()
				filter.Entities()
			}
			b.ReportAllocs()
		})
	}
} */
