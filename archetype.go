// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

// Archetype represents a unique combination of component types.
// Entities with the same set of components are stored in the same archetype.
type Archetype struct {
	mask          maskType               // The component mask for this archetype.
	componentData [][]byte               // Byte slices of component data.
	componentIDs  []ComponentID          // A sorted list of component IDs in this archetype.
	entities      []Entity               // The list of entities in this archetype.
	slots         [maxComponentTypes]int // Slot lookup for component IDs; -1 if not present.
}

// getSlot finds the index of a component ID in the archetype's componentID list.
// It uses a lookup array for constant time access.
func (self *Archetype) getSlot(id ComponentID) int {
	return self.slots[id]
}
