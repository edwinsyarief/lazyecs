package teishoku

import (
	"testing"
)

// EventBus test components
type TestEvent struct {
	Value int
}

func TestEventBusSubscribeAndPublish(t *testing.T) {
	bus := &EventBus{}
	received := 0
	Subscribe(bus, func(e TestEvent) {
		received += e.Value
	})
	Subscribe(bus, func(e TestEvent) {
		received += e.Value * 2
	})
	Publish(bus, TestEvent{Value: 1})
	if received != 3 {
		t.Errorf("expected received 3, got %d", received)
	}
	Publish(bus, TestEvent{Value: 2})
	if received != 3+6 {
		t.Errorf("expected received 9, got %d", received)
	}
}

func TestEventBusMultipleTypes(t *testing.T) {
	bus := &EventBus{}
	received1 := 0
	received2 := 0
	Subscribe(bus, func(e TestEvent) {
		received1 += e.Value
	})
	Subscribe(bus, func(p Position) {
		received2 += int(p.X)
	})
	Publish(bus, TestEvent{Value: 42})
	Publish(bus, Position{X: 10})
	if received1 != 42 {
		t.Errorf("expected received1 42, got %d", received1)
	}
	if received2 != 10 {
		t.Errorf("expected received2 10, got %d", received2)
	}
}

func TestEventBusNoHandlers(t *testing.T) {
	bus := &EventBus{}
	// No panic expected
	Publish(bus, TestEvent{Value: 42})
}

func TestEventBusManySubscribers(t *testing.T) {
	bus := &EventBus{}
	const numSubs = 100
	received := 0
	for i := 0; i < numSubs; i++ {
		Subscribe(bus, func(e TestEvent) {
			received += e.Value
		})
	}
	Publish(bus, TestEvent{Value: 1})
	if received != numSubs {
		t.Errorf("expected %d, got %d", numSubs, received)
	}
}
