# lazyecs

A high-performance, archetype-based, and easy-to-use Entity Component System (ECS) library for Go.

`lazyecs` is designed for performance-critical applications like games and simulations, offering a simple, generic API that minimizes garbage collection overhead. It uses archetypes to store entities with the same component layout in contiguous memory blocks, enabling extremely fast iteration.

## Features

- **Archetype-Based**: Stores entities with the same components together for maximum cache efficiency.
- **Generic API**: Leverages Go generics for a type-safe and intuitive developer experience.
- **High Performance**: Optimized for speed with zero GC overhead during entity creation and querying.
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

The `World` is the container for all your entities and components. You must specify its entity capacity upon creation.

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

## Core Concepts

- **World**: The central container that manages all entities, components, and archetypes. All ECS operations happen within a `World`.
- **Entity**: A simple identifier for an object in your application. It doesn't hold any data itself but serves as a key to associate components.
- **Component**: A plain Go struct that stores data. Components should contain only data and no logic.
- **Archetype**: A specific combination of component types. All entities with the exact same set of components are stored together in the same archetype, allowing for highly efficient, cache-friendly iteration.

## Cookbook

This section provides quick solutions to common problems.

### 1. Creating and Destroying Entities

The most direct way to create an entity is with a `Builder`. A `Builder` is configured for a specific set of components and is highly efficient at creating entities with that exact layout.

#### Creating a Single Entity

```go
// Create a builder for entities with Position and Velocity.
builder := lazyecs.NewBuilder2[Position, Velocity](world)

// Create a single entity.
entity := builder.NewEntity()

// You can get the components right away.
pos, vel := builder.Get(entity)
pos.X = 10
vel.VX = 1

// To destroy an entity, use world.RemoveEntity().
world.RemoveEntity(entity)
```

#### Creating Entities in Batches

For better performance, you can create multiple entities at once using `NewEntities`. This is useful for spawning large groups of objects. Note that `NewEntities` does not return the created entities, so you will need to use a `Filter` to access them afterward.

```go
// Create a builder for entities with Position.
builder := lazyecs.NewBuilder[Position](world)

// Create 1000 entities at once.
builder.NewEntities(1000)

// You can then use a filter to find and initialize them.
query := lazyecs.NewFilter[Position](world)
for query.Next() {
    pos := query.Get()
    pos.X = rand.Float32() * 1024
    pos.Y = rand.Float32() * 768
}
```

### 2. Adding and Removing Components

You can dynamically add or remove components from an entity. This operation is less performant than creating entities with a builder because it may involve moving the entity between archetypes.

```go
// Create an entity with only a Position component.
builder := lazyecs.NewBuilder[Position](world)
entity := builder.NewEntity()

// Add a Velocity component later.
lazyecs.SetComponent(world, entity, Velocity{VX: 5, VY: 5})

// Remove the Position component.
lazyecs.RemoveComponent[Position](world, entity)
```

### 3. Querying for Entities

Use a `Filter` to iterate over all entities that have a specific set of components.

```go
// Create a filter for all entities with Position and Velocity.
query := lazyecs.NewFilter2[Position, Velocity](world)

// Loop through the results.
for query.Next() {
    // Get the entity and its components.
    entity := query.Entity()
    pos, vel := query.Get()

    // Apply logic.
    pos.X += vel.VX
    fmt.Printf("Entity %d updated to X=%.2f\n", entity.ID, pos.X)
}
```

## API Reference

### World

| Function                               | Description                                             |
| -------------------------------------- | ------------------------------------------------------- |
| `NewWorld(capacity int) *World`        | Creates a new `World` with a pre-allocated entity capacity. |
| `(w *World) RemoveEntity(e Entity)`    | Deletes an entity and recycles its ID.                  |
| `(w *World) IsValid(e Entity) bool`    | Checks if an entity ID is still valid and has not been deleted. |

### Component Management

Functions are provided for up to 5 components (`GetComponent`, `GetComponents2`, etc.).

| Function                               | Description                                             |
| -------------------------------------- | ------------------------------------------------------- |
| `GetComponent[T](w *World, e Entity) *T` | Retrieves a pointer to a single component `T` for an entity. |
| `SetComponent[T](w *World, e Entity, val T)` | Adds or updates a single component `T` for an entity. |
| `RemoveComponent[T](w *World, e Entity)` | Removes a single component `T` from an entity.         |

### Builders (Entity Creation)

Builders are available for creating entities with 1 to 5 components (`NewBuilder`, `NewBuilder2`, etc.).

| Function                               | Description                                             |
| -------------------------------------- | ------------------------------------------------------- |
| `NewBuilder[T](w *World) *Builder[T]`  | Creates a `Builder` for entities with component `T`.      |
| `(b *Builder[T]) NewEntity() Entity`   | Creates a new entity with the pre-configured components. |
| `(b *Builder[T]) NewEntities(count int)` | Creates a batch of `count` entities with the pre-configured components. |
| `(b *Builder[T]) Get(e Entity) *T`     | Gets the component `T` for an entity created by this builder. |

### Filters (Querying)

Filters are available for iterating over entities with 1 to 5 components (`NewFilter`, `NewFilter2`, etc.).

| Function                               | Description                                             |
| -------------------------------------- | ------------------------------------------------------- |
| `NewFilter[T](w *World) *Filter[T]`    | Creates a `Filter` to iterate over entities with component `T`. |
| `(f *Filter[T]) Next() bool`           | Advances the iterator to the next entity. Returns `false` if none are left. |
| `(f *Filter[T]) Entity() Entity`       | Returns the current `Entity` in the iteration.            |
| `(f *Filter[T]) Get() *T`              | Returns the component `T` for the current entity.         |
| `(f *Filter[T]) Reset()`               | Resets the iterator to the beginning.                   |
| `(f *Filter[T]) RemoveEntities()`      | Removes entities.                   |

## Concurrency

The `World` object is **not** thread-safe. All operations that modify the world state (e.g., creating/removing entities, adding/removing components) should be performed from a single goroutine.

## Benchmark Results

```
goos: linux
goarch: amd64
pkg: github.com/edwinsyarief/lazyecs
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkCreateWorld/1K-4             	  176977	      6036 ns/op	   29608 B/op	       5 allocs/op
BenchmarkCreateWorld/10K-4            	   24045	     48726 ns/op	  287658 B/op	       5 allocs/op
BenchmarkCreateWorld/100K-4           	    3114	    468926 ns/op	 2802609 B/op	       5 allocs/op
BenchmarkCreateWorld/1M-4             	     308	   3847333 ns/op	28009395 B/op	       5 allocs/op
BenchmarkAutoExpand/1K_init_x2-4      	   44229	     26721 ns/op	  127001 B/op	       6 allocs/op
BenchmarkAutoExpand/10K_init_x2-4     	    3807	    274281 ns/op	 1105945 B/op	       6 allocs/op
BenchmarkAutoExpand/100K_init_x2-4    	     302	   4034151 ns/op	11903024 B/op	       6 allocs/op
BenchmarkCreateEntity/1K-4            	  186903	      6548 ns/op	       0 B/op	       0 allocs/op
BenchmarkCreateEntity/10K-4           	   16485	     73208 ns/op	       0 B/op	       0 allocs/op
BenchmarkCreateEntity/100K-4          	    2059	    615932 ns/op	       4 B/op	       0 allocs/op
BenchmarkCreateEntity/1M-4            	     217	   5518963 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesBatch/1K-4        	  441910	      2729 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesBatch/10K-4       	   44210	     26990 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesBatch/100K-4      	    4659	    282403 ns/op	       2 B/op	       0 allocs/op
BenchmarkNewEntitiesBatch/1M-4        	     490	   2469399 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/1K-4            	94178534	        12.58 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/10K-4           	93488174	        12.64 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/100K-4          	93599734	        12.72 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/1M-4            	94421305	        12.75 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/1K-4    	39301866	        30.59 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/10K-4   	39114544	        30.67 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/100K-4  	38847579	        30.79 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/1M-4    	38958819	        30.79 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentNew/1K-4         	   13791	     86559 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentNew/10K-4        	    1402	    851996 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentNew/100K-4       	     141	   8421829 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentNew/1M-4         	      13	  84154257 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/1K-4 	  261081	      4585 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/10K-4         	   25215	     47748 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/100K-4        	    2779	    439465 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/1M-4          	     282	   4249562 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/1K-4         	  194618	      6195 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/10K-4        	   18286	     65748 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/100K-4       	    1676	    659006 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/1M-4         	     205	   6026165 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveComponent/1K-4                  	   13387	     89385 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveComponent/10K-4                 	    1371	    869315 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveComponent/100K-4                	     139	   8569709 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveComponent/1M-4                  	      13	  85785122 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveEntity/1K-4                     	   89785	     13196 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveEntity/10K-4                    	    9222	    133189 ns/op	       1 B/op	       0 allocs/op
BenchmarkRemoveEntity/100K-4                   	    1052	   1125884 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveEntity/1M-4                     	      94	  10971168 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/1K-4             	  241581	      4923 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/10K-4            	   25821	     46602 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/100K-4           	    2862	    420144 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/1M-4             	     285	   4188014 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/1K-4            	  245383	      4964 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/10K-4           	   26204	     45360 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/100K-4          	    2846	    422381 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/1M-4            	     285	   4196001 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/1K-4                    	  499658	      2344 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/10K-4                   	   51602	     23357 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/100K-4                  	    5136	    235341 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/1M-4                    	     511	   2351143 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/1K-4                   	  503848	      2353 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/10K-4                  	   51441	     23392 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/100K-4                 	    5046	    234303 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/1M-4                   	     511	   2350547 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/edwinsyarief/lazyecs	235.565s
```

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
