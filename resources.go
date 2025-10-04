package lazyecs

import "reflect"

// Resources manages a collection of resources, ensuring no duplicate types are present at the same time.
// It uses a slice for storage, a map for quick type to ID mapping, and a free list for ID reuse.
// Designed for high performance with O(1) operations and minimal allocations when preallocated.
type Resources struct {
	items   []any
	types   map[reflect.Type]int
	freeIds []int
}

// Add adds a resource and returns its ID. Panics if a resource of the same type already exists.
// Reuses free IDs if available to avoid growing the slice unnecessarily.
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

// Has checks if a resource with the given ID exists.
func (r *Resources) Has(id int) bool {
	return id >= 0 && id < len(r.items) && r.items[id] != nil
}

// Get retrieves the resource by ID, or nil if it doesn't exist.
func (r *Resources) Get(id int) any {
	if !r.Has(id) {
		return nil
	}
	return r.items[id]
}

// Remove removes the resource by ID if it exists, marking the ID as free for reuse.
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

// Clear removes all resources, resetting the free list.
func (r *Resources) Clear() {
	for i := range r.items {
		r.items[i] = nil
	}
	r.items = r.items[:0]
	clear(r.types)
	r.freeIds = r.freeIds[:0]
}

// HasResource checks if a resource of type T exists, returning true and its ID, or false and -1.
func HasResource[T any](r *Resources) (bool, int) {
	t := reflect.TypeOf((*T)(nil))
	if id, ok := r.types[t]; ok {
		return true, id
	}
	return false, -1
}

// GetResource retrieves the resource of type T if it exists, returning it as *T and its ID, or nil and -1.
func GetResource[T any](r *Resources) (*T, int) {
	t := reflect.TypeOf((*T)(nil))
	if id, ok := r.types[t]; ok {
		res := r.items[id].(*T)
		return res, id
	}
	return nil, -1
}
