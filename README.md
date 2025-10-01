# lazyecs

A high-performance, archetype-based, and easy-to-use Entity Component System (ECS) library for Go.

`lazyecs` is designed for performance-critical applications like games and simulations, offering a simple, generic API that minimizes garbage collection overhead. It uses archetypes to store entities with the same component layout in contiguous memory blocks, enabling extremely fast iteration.

## Features

- **Archetype-Based**: Stores entities with the same components together in contiguous memory for maximum cache efficiency.
- **Generic API**: Leverages Go generics for a type-safe and intuitive developer experience.
- **High Performance**: Optimized for speed with zero GC overhead on the hot path (entity creation, iteration, and component access).
- **Simple and Clean**: Designed to be easy to learn and integrate into any project.

## Getting Started

This guide covers the primary workflow for setting up and using `lazyecs`.

### 1. Define Components

Components are simple Go structs that hold the data for your entities. They should contain only data, not logic.

```go
// Position represents a 2D position.
type Position struct {
    X, Y float32
}

// Velocity represents a 2D velocity.
type Velocity struct {
    VX, VY float32
}
```

### 2. Create a World

The `World` is the container for all your entities and components. You must specify its initial entity capacity.

```go
import "github.com/edwinsyarief/lazyecs"

func main() {
    // Create a new world with a capacity for 10,000 entities.
    world := lazyecs.NewWorld(10000)
}
```

### 3. Create Entities with a Builder

A `Builder` is the most efficient way to create entities with a predefined set of components.

```go
// Create a builder for entities that have both Position and Velocity.
builder := lazyecs.NewBuilder2[Position, Velocity](world)

// Create 100 entities with these components.
for i := 0; i < 100; i++ {
    entity := builder.NewEntity()
    
    // Get the components and initialize their data.
    pos, vel := builder.Get(entity)
    pos.X = float32(i) * 2.0
    vel.VX = 1.0
}
```

### 4. Create a System with a Filter

A `Filter` (or "query") allows you to iterate over all entities that have a specific set of components. This is how you implement your application's logic.

```go
// Create a filter to find all entities with Position and Velocity.
query := lazyecs.NewFilter2[Position, Velocity](world)

// This is your system's main loop.
for query.Next() {
    // Get the components for the current entity.
    pos, vel := query.Get()

    // Update the position based on the velocity.
    pos.X += vel.VX
    pos.Y += vel.VY
}
```

## How It Works

`lazyecs` is built around a few core concepts that work together to provide a high-performance and ergonomic experience.

### World, Entity, and Component

- **World**: The central container that manages all entities, components, and game state. All ECS operations happen within a `World`. It also includes a `Resources` map for storing global, singleton-like data.
- **Entity**: A simple integer that uniquely identifies an object in your application. It doesn't hold any data itself but serves as a key to associate a group of components.
- **Component**: A plain Go struct that stores data (e.g., `Position`, `Velocity`). Components should contain only data and no logic.

### Archetypes and Performance

The key to `lazyecs`'s performance is its **archetype-based architecture**.

An **archetype** is a unique combination of component types. For example, all entities that have *only* a `Position` and a `Velocity` component belong to the same archetype.

Inside an archetype, all components are stored in tightly packed, contiguous arrays. This means all `Position` components are next to each other in memory, and all `Velocity` components are also next to each other. When a `Filter` iterates over entities, it can access their component data in a linear, cache-friendly manner, which is extremely fast.

When you add or remove a component from an entity, the entity is moved from its old archetype to a new one that matches its new set of components. While this operation is efficient, it is slower than creating entities with a fixed layout using a `Builder`.

## API Reference

The following tables provide a summary of the core API. For more details, please refer to the GoDoc comments in the source code.

### World

| Function | Description |
| --- | --- |
| `NewWorld(capacity int) *World` | Creates a new `World` with a pre-allocated entity capacity. |
| `(w *World) RemoveEntity(e Entity)` | Deactivates an entity and recycles its ID for future use. |
| `(w *World) IsValid(e Entity) bool` | Checks if an entity reference is still valid (i.e., not deleted). |
| `(w *World).Resources` | A `sync.Map` for storing global, thread-safe key-value data. |

### Component Management

Functions are provided for up to 6 components (`GetComponent`, `GetComponent2`, etc.).

| Function | Description |
| --- | --- |
| `GetComponent[T](w *World, e Entity) *T` | Retrieves a pointer to a single component `T` for an entity. |
| `SetComponent[T](w *World, e Entity, val T)` | Adds or updates a component. May move the entity to a new archetype. |
| `RemoveComponent[T](w *World, e Entity)` | Removes a component. May move the entity to a new archetype. |

### Builders (Entity Creation)

Builders are available for creating entities with 1 to 6 components (`NewBuilder`, `NewBuilder2`, etc.).

| Function | Description |
| --- | --- |
| `NewBuilder[T](w *World) *Builder[T]` | Creates a `Builder` for entities with a specific set of components. |
| `(b *Builder[T]) NewEntity() Entity` | Creates a single new entity with the pre-configured components. |
| `(b *Builder[T]) NewEntities(count int)` | Creates a batch of `count` entities with the pre-configured components. |
| `(b *Builder[T]) Get(e Entity) *T` | Gets the component(s) for an entity created by this builder. |

### Filters (Querying)

Filters are available for iterating over entities with 1 to 6 components (`NewFilter`, `NewFilter2`, etc.).

| Function | Description |
| --- | --- |
| `NewFilter[T](w *World) *Filter[T]` | Creates a `Filter` to iterate over entities with a set of components. |
| `(f *Filter[T]) Next() bool` | Advances the iterator to the next entity. Returns `false` if none are left. |
| `(f *Filter[T]) Entity() Entity` | Returns the current `Entity` in the iteration. |
| `(f *Filter[T]) Get() *T` | Returns the component(s) for the current entity. |
| `(f *Filter[T]) Reset()` | Resets the iterator to the beginning. |
| `(f *Filter[T]) RemoveEntities()` | Efficiently removes all entities matching the filter. |

## Concurrency

The `World` object is **not** thread-safe. All operations that modify the world state (e.g., creating/removing entities, adding/removing components) should be performed from a single goroutine. The `World.Resources` map, however, is a `sync.Map` and can be safely accessed from multiple goroutines.

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
