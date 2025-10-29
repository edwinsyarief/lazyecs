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
BenchmarkCreateWorld/1K-4                    	  153994	      8061 ns/op	   43304 B/op	      12 allocs/op
BenchmarkCreateWorld/10K-4                   	   19089	     72171 ns/op	  375082 B/op	      12 allocs/op
BenchmarkCreateWorld/100K-4                  	    3139	   1207975 ns/op	 3610930 B/op	      12 allocs/op
BenchmarkCreateWorld/1M-4                    	     290	   4233038 ns/op	36018483 B/op	      12 allocs/op
BenchmarkAutoExpand/1K_init_x2-4             	   46105	     26286 ns/op	  143385 B/op	       7 allocs/op
BenchmarkAutoExpand/10K_init_x2-4            	    4306	    272724 ns/op	 1269786 B/op	       7 allocs/op
BenchmarkAutoExpand/100K_init_x2-4           	     535	   2403011 ns/op	13508650 B/op	       7 allocs/op
BenchmarkAutoExpand/1000K_init_x2-4          	      54	  22231545 ns/op	134651942 B/op	       7 allocs/op
BenchmarkWorldCreateEntity/1K-4              	  173991	      6825 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntity/10K-4             	   20194	     59928 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntity/100K-4            	    2124	    567732 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntity/1M-4              	     213	   5613423 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/1K-4            	  453991	      2635 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/10K-4           	   47265	     25611 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/100K-4          	    5268	    227892 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/1M-4            	     512	   2338246 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/1K-4               	  172173	      6923 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/10K-4              	   17458	     71956 ns/op	       1 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/100K-4             	    2162	    598355 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/1M-4               	     219	   5467634 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/1K-4             	  439428	      2758 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/10K-4            	   45914	     26073 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/100K-4           	    4702	    275458 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/1M-4             	     504	   2434693 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/1K-4 	  266618	      4516 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/10K-4         	   26374	     45434 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/100K-4        	    2761	    488380 ns/op	       7 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/1M-4          	     292	   4105112 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/1K-4         	  192235	      6294 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/10K-4        	   18136	     66009 ns/op	       1 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/100K-4       	    2094	    673437 ns/op	      18 B/op	       0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/1M-4         	     216	   5533991 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSetComponent/1K-4                     	   24487	     48895 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSetComponent/10K-4                    	    2467	    490957 ns/op	       3 B/op	       0 allocs/op
BenchmarkBuilderSetComponent/100K-4                   	     249	   4787525 ns/op	      16 B/op	       0 allocs/op
BenchmarkBuilderSetComponent/1M-4                     	      25	  46691291 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSetComponent2/1K-4                    	   21240	     56464 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSetComponent2/10K-4                   	    2120	    570525 ns/op	       2 B/op	       0 allocs/op
BenchmarkBuilderSetComponent2/100K-4                  	     217	   5527015 ns/op	      18 B/op	       0 allocs/op
BenchmarkBuilderSetComponent2/1M-4                    	      21	  54132131 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent/1K-4                     	272134158	         4.409 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderGetComponent/10K-4                    	271559527	         4.415 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/1K-4                          	 1268396	       945.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/10K-4                         	  128301	      9360 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/100K-4                        	   12831	     93676 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/1M-4                          	    1285	    934151 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter3Iterate/1K-4                          	 1265319	       948.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter3Iterate/10K-4                         	  128187	      9345 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter3Iterate/100K-4                        	   12858	     93343 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter3Iterate/1M-4                          	    1285	    933863 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter4Iterate/1K-4                          	 1262593	       949.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter4Iterate/10K-4                         	  128128	      9364 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter4Iterate/100K-4                        	   12728	     93394 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter4Iterate/1M-4                          	    1285	    933302 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter5Iterate/1K-4                          	 1266921	       948.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter5Iterate/10K-4                         	  127908	      9349 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter5Iterate/100K-4                        	   12844	     93516 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter5Iterate/1M-4                          	    1276	    933615 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter6Iterate/1K-4                          	 1264392	       949.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter6Iterate/10K-4                         	  128078	      9355 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter6Iterate/100K-4                        	   12849	     93654 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter6Iterate/1M-4                          	    1285	    933404 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesCached/1K-4                 	484727713	         2.460 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesCached/10K-4                	490976347	         2.455 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesCached/100K-4               	488559230	         2.455 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesCached/1M-4                 	488095389	         2.456 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesUncached/1K-4               	 3468256	       346.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesUncached/10K-4              	  463063	      2526 ns/op	       1 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesUncached/100K-4             	   42970	     26791 ns/op	     140 B/op	       0 allocs/op
BenchmarkFilterGetEntitiesUncached/1M-4               	    3018	    390284 ns/op	   19894 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/1K-4                       	1000000000	         0.0000458 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/10K-4                      	1000000000	         0.0002380 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/100K-4                     	1000000000	         0.007180 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/1M-4                       	1000000000	         0.06509 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/1K-4               	1000000000	         0.0000090 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/10K-4              	1000000000	         0.0001005 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/100K-4             	1000000000	         0.0008958 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/1M-4               	1000000000	         0.008886 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/1K-4               	1000000000	         0.0000186 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/10K-4              	1000000000	         0.0002102 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/100K-4             	1000000000	         0.001841 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/1M-4               	1000000000	         0.01869 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/1K-4             	  635790	      1887 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/10K-4            	   64058	     18767 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/100K-4           	    6325	    188027 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/1M-4             	     638	   1881127 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/1K-4                            	1000000000	         0.0001196 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/10K-4                           	1000000000	         0.001422 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/100K-4                          	1000000000	         0.01230 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/1M-4                            	1000000000	         0.1715 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/1K-4                            	1000000000	         0.0000007 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/10K-4                           	1000000000	         0.0000033 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/100K-4                          	1000000000	         0.0000471 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/1M-4                            	1000000000	         0.0003223 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/1K-4                            	1000000000	         0.0000005 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/10K-4                           	1000000000	         0.0000034 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/100K-4                          	1000000000	         0.0000321 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/1M-4                            	1000000000	         0.0003088 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/1K-4                         	1000000000	         0.0000539 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/10K-4                        	1000000000	         0.0005125 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/100K-4                       	1000000000	         0.005821 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/1M-4                         	1000000000	         0.1377 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/1K-4                          	  276075	     15707 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/10K-4                         	   10000	    139006 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/100K-4                        	     861	   1429659 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/1M-4                          	     100	  22707248 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/edwinsyarief/teishoku	998.380s
```

</details>

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
