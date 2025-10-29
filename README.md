# Teishoku

![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/edwinsyarief/teishoku/test.yml)
![Go Version](https://img.shields.io/badge/Go-1.25.1-blue?logo=go&style=flat&logoColor=white)
![Go Reference](https://pkg.go.dev/badge/github.com/edwinsyarief/teishoku.svg)

A high-performance, archetype-based, and easy-to-use Entity Component System (ECS) library for Go.

`Teishoku` is designed for performance-critical applications like games and simulations, offering a simple, generic API that minimizes garbage collection overhead. It uses archetypes to store entities with the same component layout in contiguous memory blocks, enabling extremely fast iteration.

## Features

- **Archetype-Based**: Stores entities with the same components together in contiguous memory for maximum cache efficiency.
- **Generic API**: Leverages Go generics for a type-safe and intuitive developer experience.
- **Zero-Allocation Hot Path**: Optimized for speed with zero GC overhead on the hot path (entity creation, iteration, and component access).
- **Simple and Clean**: Designed to be easy to learn and integrate into any project.

## Getting Started

This guide covers the primary workflow for setting up and using `Teishoku`.

### 1. Set up Go Modules

First, get the teishoku.

```bash
go get github.com/edwinsyarief/teishoku
```

### 2. Define Components

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
import "github.com/edwinsyarief/teishoku"

func main() {
    // Create a new world with a capacity for 10,000 entities.
    world := teishoku.NewWorld(10000)
}
```

### 3. Create Entities with a Builder

A `Builder` is the most efficient way to create entities with a predefined set of components.

```go
// Create a builder for entities that have both Position and Velocity.
builder := teishoku.NewBuilder2[Position, Velocity](&world)

// Create 100 entities with these components.
for i := 0; i < 100; i++ {
    entity := builder.NewEntity()

    // Get the components and initialize their data.
    pos, vel := builder.Get(entity)
    pos.X = float32(i) * 2.0
    vel.VX = 1.0
}

// OR

builder.NewEntities(100)

// Use filter to iterate and set value

```

### 4. Create a System with a Filter

A `Filter` (or "query") allows you to iterate over all entities that have a specific set of components. This is how you implement your application's logic.

```go
// Create a filter to find all entities with Position and Velocity.
query := teishoku.NewFilter2[Position, Velocity](&world)

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

`Teishoku` is built around a few core concepts that work together to provide a high-performance and ergonomic experience.

### World, Entity, and Component

- **World**: The central container that manages all entities, components, and game state. All ECS operations happen within a `World`. It also includes a `Resources` map for storing global, singleton-like data.
- **Entity**: A simple integer that uniquely identifies an object in your application. It doesn't hold any data itself but serves as a key to associate a group of components.
- **Component**: A plain Go struct that stores data (e.g., `Position`, `Velocity`). Components should contain only data and no logic.

### Archetypes and Performance

The key to `teishoku`'s performance is its **archetype-based architecture**.

An **archetype** is a unique combination of component types. For example, all entities that have _only_ a `Position` and a `Velocity` component belong to the same archetype.

Inside an archetype, all components are stored in tightly packed, contiguous arrays. This means all `Position` components are next to each other in memory, and all `Velocity` components are also next to each other. When a `Filter` iterates over entities, it can access their component data in a linear, cache-friendly manner, which is extremely fast.

When you add or remove a component from an entity, the entity is moved from its old archetype to a new one that matches its new set of components. While this operation is efficient, it is slower than creating entities with a fixed layout using a `Builder`.

## Code Generation

`Teishoku` uses Go's `go generate` tool to create boilerplate code for multi-component `Builders`, `Filters`, and `World` API functions (e.g., `NewBuilder3`, `Filter4`, `GetComponent2`). This approach keeps the library's public API clean and consistent without requiring developers to write repetitive code manually.

To run the code generator, simply execute the following command from the root of the repository:

```bash
go generate ./...
```

This will regenerate the `*_generated.go` files. You should run this command whenever you change the templates in the `templates/` directory.

## API Reference

The following tables provide a summary of the core API. For complete and up-to-date documentation, please refer to the GoDoc comments in the source code.

### World

| Function                            | Description                                                       |
| ----------------------------------- | ----------------------------------------------------------------- |
| `NewWorld(capacity int) *World`     | Creates a new `World` with a pre-allocated entity capacity.       |
| `(w *World) RemoveEntity(e Entity)` | Deactivates an entity and recycles its ID for future use.         |
| `(w *World) IsValid(e Entity) bool` | Checks if an entity reference is still valid (i.e., not deleted). |
| `(w *World).Resources() *Resources` | Retrieves the `Resources` manager for storing global data.        |

### Component Management

Functions are provided for up to 6 components (`GetComponent`, `GetComponent2`, etc.).

| Function                                     | Description                                                          |
| -------------------------------------------- | -------------------------------------------------------------------- |
| `GetComponent[T](w *World, e Entity) *T`     | Retrieves a pointer to a single component `T` for an entity.         |
| `SetComponent[T](w *World, e Entity, val T)` | Adds or updates a component. May move the entity to a new archetype. |
| `RemoveComponent[T](w *World, e Entity)`     | Removes a component. May move the entity to a new archetype.         |

### Builders (Entity Creation)

Builders are available for creating entities with 1 to 6 components (`NewBuilder`, `NewBuilder2`, etc.).

| Function                                       | Description                                                             |
| ---------------------------------------------- | ----------------------------------------------------------------------- |
| `NewBuilder[T](w *World) *Builder[T]`          | Creates a `Builder` for entities with a specific set of components.     |
| `(b *Builder[T]) NewEntity() Entity`           | Creates a single new entity with the pre-configured components.         |
| `(b *Builder[T]) NewEntities(count int)`       | Creates a batch of `count` entities with the pre-configured components. |
| `(b *Builder[T]) Get(e Entity) *T`             | Gets the component(s) for an entity created by this builder.            |
| `(b *Builder[T]) Set(e Entity, comp T)`        | Sets the component(s) for an entity.                                    |
| `(b *Builder[T]) SetBatch(e []Entity, comp T)` | Sets the component(s) for entities.                                     |

### Filters (Querying)

Filters are available for iterating over entities with 1 to 6 components (`NewFilter`, `NewFilter2`, etc.).

| Function                             | Description                                                                 |
| ------------------------------------ | --------------------------------------------------------------------------- |
| `NewFilter[T](w *World) *Filter[T]`  | Creates a `Filter` to iterate over entities with a set of components.       |
| `(f *Filter[T]) Next() bool`         | Advances the iterator to the next entity. Returns `false` if none are left. |
| `(f *Filter[T]) Entity() Entity`     | Returns the current `Entity` in the iteration.                              |
| `(f *Filter[T]) Get() *T`            | Returns the component(s) for the current entity.                            |
| `(f *Filter[T]) Reset()`             | Resets the iterator to the beginning.                                       |
| `(f *Filter[T]) RemoveEntities()`    | Efficiently removes all entities matching the filter.                       |
| `(f *Filter[T]) Entities() []Entity` | Returns the entities matching the filter.                                   |

### EventBus

| Function                                           | Description                                          |
| -------------------------------------------------- | ---------------------------------------------------- |
| `Subscribe[T any](bus *EventBus, handler func(T))` | Registers a handler for events of type T.            |
| `Publish[T any](bus *EventBus, event T)`           | Sends an event of type T to all subscribed handlers. |

### Resources

| Function                                       | Description                                    |
| ---------------------------------------------- | ---------------------------------------------- |
| `(r *Resources) Add(res any) int`              | Adds a resource and returns its ID.            |
| `(r *Resources) Has(id int) bool`              | Checks if a resource with the given ID exists. |
| `(r *Resources) Get(id int) any`               | Retrieves the resource by ID.                  |
| `(r *Resources) Remove(id int)`                | Removes the resource by ID.                    |
| `(r *Resources) Clear()`                       | Removes all resources.                         |
| `HasResource[T any](r *Resources) (bool, int)` | Checks if a resource of type T exists.         |
| `GetResource[T any](r *Resources) (*T, int)`   | Retrieves the resource of type T.              |

## Concurrency

The `World` object and the `Resources` manager are **not** thread-safe. All operations that modify the world state (e.g., creating/removing entities, adding/removing components, or modifying resources) should be performed from a single goroutine.

## Benchmark Results

The following tables summarize the performance of `teishoku` across a range of common operations. The results are presented in nanoseconds (ns) per unit (e.g., per entity) and were run on an AMD EPYC 7763 64-Core Processor.

Notably, many core operations like creating entities, accessing components, and iterating with filters show **zero memory allocations** (`0 allocs/op`), making `teishoku` ideal for performance-critical applications where garbage collection pressure is a concern.

### Entity Benchmark

| Action Name | 1K (ns) | 10K (ns) | 100K (ns) | 1M (ns) |
| :--- | :--- | :--- | :--- | :--- |
| **Create World** | 9.66 | 6.72 | 11.97 | 4.72 |
| **Auto Expand** | 27.60 | 27.12 | 30.27 | N/A |
| **Create Entity** | 6.98 | 6.20 | 5.67 | 5.61 |
| **New Entities (Batch)** | 2.62 | 2.55 | 2.27 | 2.43 |
| **New Entities With Value Set (Batch)** | 4.52 | 4.51 | 4.54 | 4.12 |
| **Get Component** | 4.49 | 4.46 | 4.59 | 4.58 |
| **Set Component Existing** | 25.31 | 25.35 | 25.40 | 25.54 |
| **Set Component New** | 78.85 | 75.55 | 73.69 | 73.78 |
| **Remove Component** | 77.73 | 73.80 | 72.28 | 72.06 |
| **Remove Entity** | 12.49 | 12.98 | 11.37 | 11.11 |
| **Filter & Remove (Batch)** | 4.75 | 4.75 | 4.20 | 4.15 |
| **Filter & Iterate** | 0.95 | 0.94 | 0.93 | 0.93 |
| **Clear Entities** | 3.25 | 3.31 | 2.65 | 2.62 |

### Event Bus Benchmark

| Action Name | 1K (ns) | 10K (ns) | 100K (ns) | 1M (ns) |
| :--- | :--- | :--- | :--- | :--- |
| **Subscribe** | 0.000044 | 0.000253 | 0.006287 | 0.06433 |
| **Publish (No Handlers)** | 0.000018 | 0.000090 | 0.000921 | 0.009018 |
| **Publish (One Handler)** | 0.000019 | 0.000183 | 0.001852 | 0.01839 |
| **Publish (Many Handlers)** | 1.89 | 1.88 | 1.88 | 1.88 |

### Resources Benchmark

| Action Name | 1K (ns) | 10K (ns) | 100K (ns) | 1M (ns) |
| :--- | :--- | :--- | :--- | :--- |
| **Add** | 0.000116 | 0.000978 | 0.01219 | 0.1637 |
| **Has** | 0.000001 | 0.000004 | 0.000031 | 0.000309 |
| **Get** | 0.000001 | 0.000003 | 0.000031 | 0.000309 |
| **Remove** | 0.000053 | 0.000464 | 0.005955 | 0.1185 |
| **Clear** | 15.92 | 14.03 | 13.55 | 22.72 |

<details>

<summary>Click to view raw benchmark output</summary>

```plaintext
goos: linux
goarch: amd64
pkg: github.com/edwinsyarief/teishoku
cpu: AMD EPYC 7763 64-Core Processor
BenchmarkCreateWorld/1K-4                           	   114622    	      9658 ns/op	   43304 B/op	      12 allocs/op
BenchmarkCreateWorld/10K-4                          	    16155    	     67151 ns/op	  375082 B/op	      12 allocs/op
BenchmarkCreateWorld/100K-4                         	     2937    	   1197027 ns/op	 3610934 B/op	      12 allocs/op
BenchmarkCreateWorld/1M-4                           	      266    	   4717612 ns/op	36018481 B/op	      12 allocs/op
BenchmarkAutoExpand/1K_init_x2-4                    	    43407    	     27601 ns/op	  143385 B/op	       7 allocs/op
BenchmarkAutoExpand/10K_init_x2-4                   	     4327    	    271219 ns/op	 1269785 B/op	       7 allocs/op
BenchmarkAutoExpand/100K_init_x2-4                  	      334    	   3027116 ns/op	13508662 B/op	       7 allocs/op
BenchmarkWorldCreateEntity/1K-4                     	   170476    	      6983 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntity/10K-4                    	    19592    	     61972 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntity/100K-4                   	     2125    	    566800 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntity/1M-4                     	      213    	   5611298 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/1K-4                   	   452203    	      2623 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/10K-4                  	    46761    	     25508 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/100K-4                 	     5259    	    227454 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/1M-4                   	      487    	   2432403 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/1K-4                      	   173772    	      6867 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/10K-4                     	    18586    	     63954 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/100K-4                    	     2154    	    605095 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/1M-4                      	      219    	   5439640 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/1K-4                    	   437331    	      2775 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/10K-4                   	    44544    	     26916 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/100K-4                  	     4278    	    279544 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/1M-4                    	      483    	   2465351 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/1K-4        	   266686    	      4517 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/10K-4         	    26703    	     45098 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/100K-4        	     2806    	    453834 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/1M-4          	      291    	   4123506 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/1K-4         	   191287    	      6218 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/10K-4        	    19129    	     62668 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/100K-4       	     1861    	    669168 ns/op	       9 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/1M-4         	      216    	   5514310 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSetComponent/1K-4                     	    24388    	     49221 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSetComponent/10K-4                    	     2438    	    496310 ns/op	       1 B/op	       0 allocs/op
BenchmarkBuilderSetComponent/100K-4                   	      248    	   4785353 ns/op	       6 B/op	       0 allocs/op
BenchmarkBuilderSetComponent/1M-4                     	       25    	  46832357 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSetComponent2/1K-4                    	    21103    	     56590 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSetComponent2/10K-4                   	     2120    	    575084 ns/op	       2 B/op	       0 allocs/op
BenchmarkBuilderSetComponent2/100K-4                  	      217    	   5454010 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSetComponent2/1M-4                    	       21    	  54210554 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent/1K-4                   	272050629	         4.485 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent/10K-4                    	270414163	         4.456 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent/100K-4                   	269171257	         4.594 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent/1M-4                     	262302666	         4.515 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent2/1K-4                    	219502479	         5.480 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent2/10K-4                   	217562380	         5.482 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent2/100K-4                  	219184404	         5.498 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent2/1M-4                    	219834985	         5.476 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIGetComponent/1K-4                         	 59609725	         20.07 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIGetComponent/10K-4                        	 59502032	         20.08 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIGetComponent/100K-4                       	 58154128	         20.15 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIGetComponent/1M-4                         	 59389701	         20.20 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIGetComponent2/1K-4                        	 31508949	         38.04 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIGetComponent2/10K-4                       	 31260826	         38.07 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIGetComponent2/100K-4                      	 31042326	         38.22 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIGetComponent2/1M-4                        	 30975897	         38.20 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPISetComponentExisting/1K-4                 	 49117706	         24.45 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPISetComponentExisting/10K-4                	 48387937	         24.69 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPISetComponentExisting/100K-4               	 48414714	         24.87 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPISetComponentExisting/1M-4                 	 48078220	         24.91 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPISetComponentNew/1K-4                      	    15642	         76999 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPISetComponentNew/10K-4                     	     1654	        736261 ns/op	       1 B/op	       0 allocs/op
BenchmarkAPISetComponentNew/100K-4                    	      169	       7072539 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPISetComponentNew/1M-4                      	       16	      70091263 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIRemoveComponent/1K-4                      	    15195	         77890 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIRemoveComponent/10K-4                     	     1628	        746177 ns/op	       1 B/op	       0 allocs/op
BenchmarkAPIRemoveComponent/100K-4                    	      168	       7096780 ns/op	       0 B/op	       0 allocs/op
BenchmarkAPIRemoveComponent/1M-4                      	       16	      70699278 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldRemoveEntity/1K-4                       	    83506	         14224 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldRemoveEntity/10K-4                      	     7264	        156207 ns/op	       3 B/op	       0 allocs/op
BenchmarkWorldRemoveEntity/100K-4                     	      699	       1699303 ns/op	       4 B/op	       0 allocs/op
BenchmarkWorldRemoveEntity/1M-4                       	       70	      16561800 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldRemoveEntities/1K-4                     	    80126	         14921 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldRemoveEntities/10K-4                    	     7869	        158211 ns/op	       1 B/op	       0 allocs/op
BenchmarkWorldRemoveEntities/100K-4                   	      692	       1719255 ns/op	      15 B/op	       0 allocs/op
BenchmarkWorldRemoveEntities/1M-4                     	       70	      16407098 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldClearEntities/1K-4                      	   826634	          1469 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldClearEntities/10K-4                     	    93787	         12948 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldClearEntities/100K-4                    	    12739	         95643 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldClearEntities/1M-4                      	     1342	        895446 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/1K-4                    	   248090	          4815 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/10K-4                   	    26745	         45017 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/100K-4                  	     2599	        460801 ns/op	       6 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/1M-4                    	      294	       4068298 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/1K-4                   	   250203	          4813 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/10K-4                  	    25930	         46543 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/100K-4                 	     2880	        450651 ns/op	       3 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/1M-4                   	      292	       4090299 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/1K-4                           	  1270090	         949.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/10K-4                          	   128017	          9363 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/100K-4                         	    12843	         93388 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/1M-4                           	     1284	        935041 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/1K-4                          	  1269235	         945.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/10K-4                         	   127970	          9352 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/100K-4                        	    12846	         93530 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/1M-4                          	     1282	        935320 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter3Iterate/1K-4                          	  1262577	         946.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter3Iterate/10K-4                         	   127974	          9354 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter3Iterate/100K-4                        	    12843	         93498 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter3Iterate/1M-4                          	     1274	        934757 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter4Iterate/1K-4                          	   545881	          2194 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter4Iterate/10K-4                         	    55075	         21800 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter4Iterate/100K-4                        	     5493	        217879 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter4Iterate/1M-4                          	      550	       2181720 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter5Iterate/1K-4                          	   541797	          2196 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter5Iterate/10K-4                         	    54843	         21812 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter5Iterate/100K-4                        	     5505	        217763 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter5Iterate/1M-4                          	      549	       2183428 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter6Iterate/1K-4                          	   479412	          2195 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter6Iterate/10K-4                         	    55032	         21779 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter6Iterate/100K-4                        	     5467	        217714 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter6Iterate/1M-4                          	      549	       2196629 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesCached/1K-4                 	486276482	         2.454 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesCached/10K-4                	487772611	         2.462 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesCached/100K-4               	489392029	         2.460 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesCached/1M-4                 	484420545	         2.454 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesUncached/1K-4               	  3285620	         360.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesUncached/10K-4              	   376084	          3040 ns/op	       1 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesUncached/100K-4             	    38862	         32212 ns/op	     155 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesUncached/1M-4               	     3192	        375796 ns/op	   18809 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/1K-4                       	1000000000	     0.0000520 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/10K-4                      	1000000000	     0.0002770 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/100K-4                     	1000000000	      0.007043 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/1M-4                       	1000000000	       0.06678 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/1K-4               	1000000000	     0.0000162 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/10K-4              	1000000000	     0.0000935 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/100K-4             	1000000000	     0.0008634 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/1M-4               	1000000000	      0.008419 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/1K-4               	1000000000	     0.0000187 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/10K-4              	1000000000	     0.0001926 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/100K-4             	1000000000	      0.001853 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/1M-4               	1000000000	       0.01842 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/1K-4             	    633637	          1891 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/10K-4            	     64172	         18703 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/100K-4           	      6356	        188291 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/1M-4             	       636	       1883910 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/1K-4                            	1000000000	     0.0001207 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/10K-4                           	1000000000	      0.001028 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/100K-4                          	1000000000	       0.01261 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/1M-4                            	1000000000	        0.1693 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/1K-4                            	1000000000	     0.0000009 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/10K-4                           	1000000000	     0.0000081 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/100K-4                          	1000000000	     0.0000313 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/1M-4                            	1000000000	     0.0003230 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/1K-4                            	1000000000	     0.0000010 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/10K-4                           	1000000000	     0.0000081 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/100K-4                          	1000000000	     0.0000314 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/1M-4                            	1000000000	     0.0003091 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/1K-4                         	1000000000	     0.0000455 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/10K-4                        	1000000000	     0.0004957 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/100K-4                       	1000000000	      0.006140 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/1M-4                         	1000000000	        0.1137 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/1K-4                          	     76160	         15426 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/10K-4                         	     10000	        140566 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/100K-4                        	       807	       1450460 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/1M-4                          	       100	      24029756 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/edwinsyarief/teishoku	933.305s
```

</details>

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
