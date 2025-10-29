package teishoku

import (
	"fmt"
	"testing"
)

func BenchmarkResourcesAdd(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			_, reses := generateDistinctTypesAndRes(size)
			r := &Resources{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < size; i++ {
				r.Add(reses[i])
			}
		})
	}
}

func BenchmarkResourcesHas(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			_, reses := generateDistinctTypesAndRes(size)
			r := &Resources{}
			for i := 0; i < size; i++ {
				r.Add(reses[i])
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < size; i++ {
				r.Has(i)
			}
		})
	}
}

func BenchmarkResourcesGet(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			_, reses := generateDistinctTypesAndRes(size)
			r := &Resources{}
			for i := 0; i < size; i++ {
				r.Add(reses[i])
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < size; i++ {
				r.Get(i)
			}
		})
	}
}

func BenchmarkResourcesRemove(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			_, reses := generateDistinctTypesAndRes(size)
			r := &Resources{}
			for i := 0; i < size; i++ {
				r.Add(reses[i])
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < size; i++ {
				r.Remove(i)
			}
		})
	}
}

func BenchmarkResourcesClear(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			_, reses := generateDistinctTypesAndRes(size)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				b.StopTimer()
				r := &Resources{}
				for j := 0; j < size; j++ {
					r.Add(reses[j])
				}
				b.StartTimer()
				r.Clear()
			}
		})
	}
}
