package lazyecs

import (
	"fmt"
	"reflect"
	"testing"
)

func TestResources(t *testing.T) {
	type testStruct1 struct{}
	type testStruct2 struct{}

	t.Run("Add and Get", func(t *testing.T) {
		r := &Resources{}
		res1 := &testStruct1{}
		id := r.Add(res1)
		if id != 0 {
			t.Errorf("expected id 0, got %d", id)
		}
		if got := r.Get(0); got != res1 {
			t.Errorf("expected %v, got %v", res1, got)
		}
	})

	t.Run("Has", func(t *testing.T) {
		r := &Resources{}
		r.Add(&testStruct1{})
		if !r.Has(0) {
			t.Error("expected true")
		}
		if r.Has(1) {
			t.Error("expected false")
		}
		if r.Has(-1) {
			t.Error("expected false")
		}
	})

	t.Run("Add same type panics", func(t *testing.T) {
		r := &Resources{}
		r.Add(&testStruct1{})
		defer func() {
			if recover() == nil {
				t.Error("expected panic")
			}
		}()
		r.Add(&testStruct1{})
	})

	t.Run("Add different types", func(t *testing.T) {
		r := &Resources{}
		r.Add(&testStruct1{})
		id := r.Add(&testStruct2{})
		if id != 1 {
			t.Errorf("expected id 1, got %d", id)
		}
	})

	t.Run("Remove", func(t *testing.T) {
		r := &Resources{}
		id := r.Add(&testStruct1{})
		r.Remove(id)
		if r.Has(id) {
			t.Error("expected false")
		}
		if r.Get(id) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("Add after Remove same type", func(t *testing.T) {
		r := &Resources{}
		id1 := r.Add(&testStruct1{})
		r.Remove(id1)
		id2 := r.Add(&testStruct1{})
		if id2 != id1 {
			t.Errorf("expected reused id %d, got %d", id1, id2)
		}
		if !r.Has(id2) {
			t.Error("expected true")
		}
	})

	t.Run("Add after multiple Removes", func(t *testing.T) {
		r := &Resources{}
		id0 := r.Add(&testStruct1{})
		id1 := r.Add(&testStruct2{})
		r.Remove(id0)
		r.Remove(id1)
		id2 := r.Add(&testStruct1{})
		if id2 != 1 {
			t.Errorf("expected reused id 1, got %d", id2)
		}
		id3 := r.Add(&testStruct2{})
		if id3 != 0 {
			t.Errorf("expected reused id 0, got %d", id3)
		}
	})

	t.Run("Clear", func(t *testing.T) {
		r := &Resources{}
		r.Add(&testStruct1{})
		r.Add(&testStruct2{})
		r.Clear()
		if len(r.items) != 0 {
			t.Error("expected empty")
		}
		if len(r.types) != 0 {
			t.Error("expected empty types")
		}
		if len(r.freeIds) != 0 {
			t.Error("expected empty freeIds")
		}
		if r.Has(0) {
			t.Error("expected false")
		}
	})

	t.Run("Add nil panics", func(t *testing.T) {
		r := &Resources{}
		defer func() {
			if recover() == nil {
				t.Error("expected panic")
			}
		}()
		r.Add(nil)
	})

	t.Run("Remove non-existent", func(t *testing.T) {
		r := &Resources{}
		r.Remove(0) // no panic
	})

	t.Run("Get non-existent", func(t *testing.T) {
		r := &Resources{}
		if r.Get(0) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("Pointers preserved", func(t *testing.T) {
		r := &Resources{}
		res := &testStruct1{}
		id := r.Add(res)
		if got := r.Get(id); got != res {
			t.Errorf("expected same pointer %p, got %p", res, got)
		}
	})
}

func generateDistinctTypesAndRes(n int) ([]reflect.Type, []any) {
	types := make([]reflect.Type, n)
	res := make([]any, n)
	for i := 0; i < n; i++ {
		fields := []reflect.StructField{
			{Name: fmt.Sprintf("F%d", i), Type: reflect.TypeOf(0)},
		}
		types[i] = reflect.StructOf(fields)
		res[i] = reflect.New(types[i]).Interface()
	}
	return types, res
}

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
