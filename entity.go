// Package lazyecs provides a simple and efficient Entity-Component-System (ECS) library.
package lazyecs

// Entity represents a unique entity in the ECS world.
type Entity struct {
	ID      uint32 // The unique ID of the entity.
	Version uint32 // The version of the entity, used to check for validity.
}

// entityMeta stores metadata about an entity.
type entityMeta struct {
	Archetype *Archetype // A pointer to the entity's archetype.
	Index     int        // The entity's index within the archetype.
	Version   uint32     // The current version of the entity.
}
