# Lazy ECS

A simple, efficient, and easy-to-use Entity-Component-System (ECS) library for Go, designed with a focus on performance and a clean API.

Lazy ECS is built around the concept of archetypes. All entities with the same set of components are stored together in a contiguous block of memory, which allows for highly efficient iteration and processing.

## Features

- **Archetype-based ECS**: Efficiently stores and manages entities and components.
- **Generic API**: Utilizes Go generics for a type-safe and convenient API.
- **High Performance**: Designed for performance-critical applications like game development.
- **Simple and Clean API**: Easy to learn and use.

## Getting Started

### Installation

To install Lazy ECS, use `go get`:

```shell
go get github.com/edwinsyarief/lazyecs
```

### Example Usage

Here's a complete example of how to use Lazy ECS:

#### 1. Define Your Components

Components are simple Go structs that hold data.

```go
package main

// Position represents a 2D position.
type Position struct {
    X, Y float32
}

// Velocity represents a 2D velocity.
type Velocity struct {
    VX, VY float32
}
```

#### 2. Register Components

Before you can use your components, you need to register them. This is typically done in an `init` function.

```go
import "github.com/edwinsyarief/lazyecs"

func init() {
    // Register components to get their unique IDs.
    lazyecs.RegisterComponent[Position]()
    lazyecs.RegisterComponent[Velocity]()
}
```

#### 3. Create a World and Entities

The `World` is the container for all your entities and components.

```go
import "fmt"

func main() {
    // Create a new ECS world.
    world := lazyecs.NewWorld()

    // Create a new entity.
    entity1 := world.CreateEntity()

    // Add and set component data.
    lazyecs.SetComponent(world, entity1, Position{X: 10, Y: 20})
    lazyecs.SetComponent(world, entity1, Velocity{VX: 1, VY: 0})

    // Create another entity.
    entity2 := world.CreateEntity()
    lazyecs.SetComponent(world, entity2, Position{X: 5, Y: 5})

    // You can also add a component and get a pointer to it.
    p, ok := lazyecs.AddComponent[Position](world, entity2)
    if ok {
        p.X = -5
    }
}
```

#### 4. Query for Entities

You can query for entities that have a specific set of components.

```go
// Query for all entities with both Position and Velocity components.
query := lazyecs.CreateQuery2[Position, Velocity](world)

// Iterate over the query results.
for query.Next() {
    // Get the components for the current entity.
    pos, vel := query.Get()

    // Update the position based on the velocity.
    pos.X += vel.VX
    pos.Y += vel.VY

    fmt.Printf("Updated position: %+v\n", *pos)
}
```

You can also get the entity itself during iteration:

```go
query := lazyecs.CreateQuery2[Position, Velocity](world)
for query.Next() {
    entity := query.Entity()
    fmt.Printf("Processing entity: %d\n", entity.ID)
}
```

#### 5. Remove Components and Entities

You can remove components from entities or remove entities from the world entirely.

```go
// Remove the Velocity component from an entity.
lazyecs.RemoveComponent[Velocity](world, entity1)

// Remove an entity from the world.
world.RemoveEntity(entity2)

// Process all pending entity removals.
// This should be called once per frame, typically at the end of the game loop.
world.ProcessRemovals()
```

## API Overview

- `NewWorld()`: Creates a new ECS world.
- `CreateEntity()`: Creates a new entity.
- `AddComponent[T](world, entity)`: Adds a component of type `T` to an entity.
- `SetComponent[T](world, entity, component)`: Sets the component data for an entity.
- `GetComponent[T](world, entity)`: Retrieves a component of type `T` from an entity.
- `RemoveComponent[T](world, entity)`: Removes a component of type `T` from an entity.
- `RemoveEntity(entity)`: Marks an entity for removal.
- `ProcessRemovals()`: Processes all pending entity removals.
- `CreateQuery[T](world, ...)`: Creates a query for entities with a specific component.
- `CreateQuery2[T1, T2](world, ...)`: Creates a query for entities with two components.
- ...and so on for up to 5 components.

## Concurrency

The `World` object is **not** thread-safe. All operations that modify the world state (e.g., creating/removing entities, adding/removing components) should be performed from a single goroutine. The `Resources` map within the `World` uses a `sync.Map` and is safe for concurrent access.

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
