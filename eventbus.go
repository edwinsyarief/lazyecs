package lazyecs

import "reflect"

// MaxEventTypes defines the maximum number of unique event types that can be
// registered in the EventBus. This value is fixed at 256.
const MaxEventTypes = 256

// EventBus provides a powerful, robust, blazing-fast event bus for publishing and subscribing to events.
type EventBus struct {
	eventTypeMap    map[reflect.Type]uint8
	handlers        [MaxEventTypes][]interface{}
	nextEventTypeID uint8
}

// Subscribe registers a handler for events of type T. The handler will be called whenever an event of type T is published.
// This operation may allocate if the handler list grows or if it's a new event type.
func Subscribe[T any](bus *EventBus, handler func(T)) {
	t := reflect.TypeFor[T]()
	id := bus.getEventTypeID(t)
	if cap(bus.handlers[id]) == 0 {
		bus.handlers[id] = make([]interface{}, 0, 4) // Preallocate small capacity to reduce reallocs
	}
	bus.handlers[id] = append(bus.handlers[id], handler)
}

// Publish sends an event of type T to all subscribed handlers. This operation is zero-allocation and zero bytes/op.
func Publish[T any](bus *EventBus, event T) {
	t := reflect.TypeFor[T]()
	if id, ok := bus.eventTypeMap[t]; ok {
		hs := bus.handlers[id]
		for _, h := range hs {
			h.(func(T))(event)
		}
	}
}

// getEventTypeID retrieves or assigns an ID for the event type.
func (bus *EventBus) getEventTypeID(t reflect.Type) uint8 {
	if bus.eventTypeMap == nil {
		bus.eventTypeMap = make(map[reflect.Type]uint8)
	}
	if id, ok := bus.eventTypeMap[t]; ok {
		return id
	}
	id := bus.nextEventTypeID
	bus.nextEventTypeID++
	if int(id) >= MaxEventTypes {
		panic("ecs: too many event types")
	}
	bus.eventTypeMap[t] = id
	return id
}
