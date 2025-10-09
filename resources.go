package lazyecs

import "reflect"

// Resources provides a type-safe, high-performance container for managing
// global, singleton-like objects. It ensures that only one resource of each
// type exists at any given time.
//
// Operations like Add, Get, Has, and Remove are designed to be O(1) and
// minimize allocations, making it suitable for performance-sensitive
// applications. It is not thread-safe.
type Resources struct {
	items   []any
	types   map[reflect.Type]int
	freeIds []int
}

// Add stores a new resource. It panics if a resource of the same type has
// already been added or if the provided resource is nil.
//
// This operation is highly efficient, reusing memory from previously removed
// resources to prevent unnecessary allocations.
//
// Parameters:
//   - res: The resource object to add. Must be a non-nil pointer.
//
// Returns:
//   - The unique integer ID assigned to this resource.
func (r *Resources) Add(res any) int {
	if res == nil {
		panic("cannot add nil resource")
	}
	t := reflect.TypeOf(res)
	if r.types == nil {
		r.types = make(map[reflect.Type]int)
	}
	if _, ok := r.types[t]; ok {
		panic("resource of the same type already exists")
	}
	var id int
	if len(r.freeIds) > 0 {
		id = r.freeIds[len(r.freeIds)-1]
		r.freeIds = r.freeIds[:len(r.freeIds)-1]
		r.items[id] = res
	} else {
		r.items = append(r.items, res)
		id = len(r.items) - 1
	}
	r.types[t] = id
	return id
}

// Has checks if a resource with the given ID is currently stored.
//
// Parameters:
//   - id: The ID of the resource to check.
//
// Returns:
//   - true if the resource exists, false otherwise.
func (r *Resources) Has(id int) bool {
	return id >= 0 && id < len(r.items) && r.items[id] != nil
}

// Get retrieves a resource by its ID. It returns nil if no resource is found
// with the specified ID.
//
// Parameters:
//   - id: The ID of the resource to retrieve.
//
// Returns:
//   - The resource as an `any` type, or nil if not found.
func (r *Resources) Get(id int) any {
	if !r.Has(id) {
		return nil
	}
	return r.items[id]
}

// Remove deletes a resource by its ID. If the resource exists, its ID is
// recycled for future use. If the ID is invalid, the operation does nothing.
//
// Parameters:
//   - id: The ID of the resource to remove.
func (r *Resources) Remove(id int) {
	if !r.Has(id) {
		return
	}
	res := r.items[id]
	t := reflect.TypeOf(res)
	delete(r.types, t)
	r.items[id] = nil
	r.freeIds = append(r.freeIds, id)
}

// Clear removes all resources and resets the container to its initial state.
// This is a fast operation that avoids re-allocating the internal maps and slices.
func (r *Resources) Clear() {
	for i := range r.items {
		r.items[i] = nil
	}
	r.items = r.items[:0]
	clear(r.types)
	r.freeIds = r.freeIds[:0]
}

// HasResource is a generic helper function that checks if a resource of type `T`
// exists in the container.
//
// Parameters:
//   - r: The Resources container to check.
//
// Returns:
//   - A boolean indicating if the resource was found, and its integer ID. If
//     not found, returns (false, -1).
func HasResource[T any](r *Resources) (bool, int) {
	t := reflect.TypeOf((*T)(nil))
	if id, ok := r.types[t]; ok {
		return true, id
	}
	return false, -1
}

// GetResource is a generic helper function that retrieves a resource of type `T`.
//
// Parameters:
//   - r: The Resources container to query.
//
// Returns:
//   - A pointer to the resource of type `T` and its integer ID. If not found,
//     returns (nil, -1).
func GetResource[T any](r *Resources) (*T, int) {
	t := reflect.TypeOf((*T)(nil))
	if id, ok := r.types[t]; ok {
		res := r.items[id].(*T)
		return res, id
	}
	return nil, -1
}
