// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

import (
	"fmt"
	"reflect"
	"unsafe"
)

// ComponentID is a unique identifier for a component type.
type ComponentID uint32

const (
	bitsPerWord            = 64
	maskWords              = 4
	maxComponentTypes      = maskWords * bitsPerWord
	defaultInitialCapacity = 65536
)

var (
	nextComponentID ComponentID
	typeToID        = make(map[reflect.Type]ComponentID, maxComponentTypes)
	idToType        = make(map[ComponentID]reflect.Type, maxComponentTypes)
	componentSizes  [maxComponentTypes]uintptr
)

// ResetGlobalRegistry resets the global component registry.
// This is useful for tests or applications that need to re-initialize the ECS state.
func ResetGlobalRegistry() {
	nextComponentID = 0
	typeToID = make(map[reflect.Type]ComponentID, maxComponentTypes)
	idToType = make(map[ComponentID]reflect.Type, maxComponentTypes)
	componentSizes = [maxComponentTypes]uintptr{}
}

// RegisterComponent registers a component type and returns its unique ID.
// If the component type is already registered, it returns the existing ID.
// It panics if the maximum number of component types is exceeded.
func RegisterComponent[T any]() ComponentID {
	var t T
	compType := reflect.TypeOf(t)

	if id, ok := typeToID[compType]; ok {
		return id
	}

	if int(nextComponentID) >= maxComponentTypes {
		panic(fmt.Sprintf("cannot register component %s: maximum number of component types (%d) reached", compType.Name(), maxComponentTypes))
	}

	id := nextComponentID
	typeToID[compType] = id
	idToType[id] = compType
	componentSizes[id] = unsafe.Sizeof(t)
	nextComponentID++
	return id
}

// GetID returns the ComponentID for a given component type.
// It panics if the component type has not been registered.
func GetID[T any]() ComponentID {
	var zero T
	typ := reflect.TypeOf(zero)
	id, ok := typeToID[typ]
	if !ok {
		panic(fmt.Sprintf("component type %s not registered", typ))
	}
	return id
}

// TryGetID returns the ComponentID for a given component type and a boolean indicating if it was found.
// It does not panic if the component type is not registered.
func TryGetID[T any]() (ComponentID, bool) {
	var zero T
	typ := reflect.TypeOf(zero)
	id, ok := typeToID[typ]
	return id, ok
}
