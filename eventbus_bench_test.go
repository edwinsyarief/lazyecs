package teishoku

import (
	"fmt"
	"testing"
)

func BenchmarkEventBusSubscribe(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			bus := &EventBus{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < size; i++ {
				Subscribe(bus, func(e TestEvent) {})
			}
		})
	}
}

func BenchmarkEventBusPublishNoHandlers(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			bus := &EventBus{}
			event := TestEvent{Value: 42}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < size; i++ {
				Publish(bus, event)
			}
		})
	}
}

func BenchmarkEventBusPublishOneHandler(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			bus := &EventBus{}
			Subscribe(bus, func(e TestEvent) {})
			event := TestEvent{Value: 42}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < size; i++ {
				Publish(bus, event)
			}
		})
	}
}

func BenchmarkEventBusPublishManyHandlers(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}
	for _, size := range sizes {
		name := fmt.Sprintf("%dK", size/1000)
		if size == 1000000 {
			name = "1M"
		}
		b.Run(name, func(b *testing.B) {
			bus := &EventBus{}
			for i := 0; i < size; i++ {
				Subscribe(bus, func(e TestEvent) {})
			}
			event := TestEvent{Value: 42}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				Publish(bus, event)
			}
		})
	}
}
