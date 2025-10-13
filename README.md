# lazyecs

A high-performance, archetype-based, and easy-to-use Entity Component System (ECS) library for Go.

`lazyecs` is designed for performance-critical applications like games and simulations, offering a simple, generic API that minimizes garbage collection overhead. It uses archetypes to store entities with the same component layout in contiguous memory blocks, enabling extremely fast iteration.

## Features

- **Archetype-Based**: Stores entities with the same components together in contiguous memory for maximum cache efficiency.
- **Generic API**: Leverages Go generics for a type-safe and intuitive developer experience.
- **Zero-Allocation Hot Path**: Optimized for speed with zero GC overhead on the hot path (entity creation, iteration, and component access).
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

## Code Generation

`lazyecs` uses Go's `go generate` tool to create boilerplate code for multi-component `Builders`, `Filters`, and `World` API functions (e.g., `NewBuilder3`, `Filter4`, `GetComponent2`). This approach keeps the library's public API clean and consistent without requiring developers to write repetitive code manually.

To run the code generator, simply execute the following command from the root of the repository:

```bash
go generate ./...
```

This will regenerate the `*_generated.go` files. You should run this command whenever you change the templates in the `templates/` directory.

## API Reference

The following tables provide a summary of the core API. For complete and up-to-date documentation, please refer to the GoDoc comments in the source code.

### World

| Function | Description |
| --- | --- |
| `NewWorld(capacity int) *World` | Creates a new `World` with a pre-allocated entity capacity. |
| `(w *World) RemoveEntity(e Entity)` | Deactivates an entity and recycles its ID for future use. |
| `(w *World) IsValid(e Entity) bool` | Checks if an entity reference is still valid (i.e., not deleted). |
| `(w *World).Resources() *Resources` | Retrieves the `Resources` manager for storing global data. |

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

### EventBus

| Function | Description |
| --- | --- |
| `Subscribe[T any](bus *EventBus, handler func(T))` | Registers a handler for events of type T. |
| `Publish[T any](bus *EventBus, event T)` | Sends an event of type T to all subscribed handlers. |

### Resources

| Function | Description |
| --- | --- |
| `(r *Resources) Add(res any) int` | Adds a resource and returns its ID. |
| `(r *Resources) Has(id int) bool` | Checks if a resource with the given ID exists. |
| `(r *Resources) Get(id int) any` | Retrieves the resource by ID. |
| `(r *Resources) Remove(id int)` | Removes the resource by ID. |
| `(r *Resources) Clear()` | Removes all resources. |
| `HasResource[T any](r *Resources) (bool, int)` | Checks if a resource of type T exists. |
| `GetResource[T any](r *Resources) (*T, int)` | Retrieves the resource of type T. |

## Concurrency

The `World` object and the `Resources` manager are **not** thread-safe. All operations that modify the world state (e.g., creating/removing entities, adding/removing components, or modifying resources) should be performed from a single goroutine.

## Benchmark Results

The following tables summarize the performance of `lazyecs` across a range of common operations. The results are presented in nanoseconds (ns) per unit (e.g., per entity) and were run on an AMD EPYC 7763 64-Core Processor.

Notably, many core operations like creating entities, accessing components, and iterating with filters show **zero memory allocations** (`0 allocs/op`), making `lazyecs` ideal for performance-critical applications where garbage collection pressure is a concern.

### Entity Benchmark

| Action Name | 1K (ns) | 10K (ns) | 100K (ns) | 1M (ns) |
| :--- | :--- | :--- | :--- | :--- |
| **Create World** | 5.87 | 5.50 | 4.16 | 3.34 |
| **Auto Expand** | 25.67 | 27.68 | 39.28 | N/A |
| **Create Entity** | 6.67 | 7.36 | 5.60 | 5.46 |
| **New Entities (Batch)** | 2.70 | 2.60 | 2.37 | 2.36 |
| **New Entities With Value Set (Batch)** | 4.58 | 4.76 | 4.31 | 4.24 |
| **Get Component** | 4.59 | 4.59 | 4.62 | 4.67 |
| **Set Component Existing**| 25.49 | 25.39 | 25.50 | 25.54 |
| **Set Component New** | 78.85 | 75.55 | 73.69 | 73.78 |
| **Remove Component** | 77.73 | 73.80 | 72.28 | 72.06 |
| **Remove Entity** | 12.49 | 12.98 | 11.37 | 11.11 |
| **Filter & Remove (Batch)** | 4.75 | 4.75 | 4.20 | 4.15 |
| **Filter & Iterate** | 2.36 | 2.35 | 2.34 | 2.33 |
| **Clear Entities** | 3.25 | 3.31 | 2.65 | 2.62 |

### Event Bus Benchmark

| Action Name | 1K (ns) | 10K (ns) | 100K (ns) | 1M (ns) |
| :--- | :--- | :--- | :--- | :--- |
| **Subscribe** | 0.000044 | 0.000253 | 0.006287 | 0.06433 |
| **Publish (No Handlers)** | 0.000018 | 0.000090 | 0.000921 | 0.009018 |
| **Publish (One Handler)** | 0.000019 | 0.000183 | 0.001852 | 0.01839 |
| **Publish (Many Handlers)**| 1.89 | 1.88 | 1.88 | 1.88 |

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

```
goos: linux
goarch: amd64
pkg: github.com/edwinsyarief/lazyecs
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkCreateWorld/1K-4                	  229825	      5867 ns/op	   29608 B/op	       5 allocs/op
BenchmarkCreateWorld/10K-4               	   25072	     55012 ns/op	  287657 B/op	       5 allocs/op
BenchmarkCreateWorld/100K-4              	    2828	    415974 ns/op	 2802609 B/op	       5 allocs/op
BenchmarkCreateWorld/1M-4                	     348	   3344778 ns/op	28009395 B/op	       5 allocs/op
BenchmarkAutoExpand/1K_init_x2-4         	   46251	     25673 ns/op	  127001 B/op	       6 allocs/op
BenchmarkAutoExpand/10K_init_x2-4        	    4754	    276777 ns/op	 1105944 B/op	       6 allocs/op
BenchmarkAutoExpand/100K_init_x2-4       	     301	   3928268 ns/op	11903016 B/op	       6 allocs/op
BenchmarkCreateEntity/1K-4               	  180673	      6665 ns/op	       0 B/op	       0 allocs/op
BenchmarkCreateEntity/10K-4              	   16522	     73585 ns/op	       0 B/op	       0 allocs/op
BenchmarkCreateEntity/100K-4             	    1838	    560497 ns/op	       0 B/op	       0 allocs/op
BenchmarkCreateEntity/1M-4               	     218	   5458086 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesBatch/1K-4           	  451422	      2703 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesBatch/10K-4          	   46188	     25999 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesBatch/100K-4         	    5149	    237179 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesBatch/1M-4           	     505	   2364396 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/1K-4               	261826854	         4.589 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/10K-4              	261219073	         4.586 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/100K-4             	259739383	         4.624 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/1M-4               	256528809	         4.667 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/1K-4       	46266636	        25.49 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/10K-4      	47357782	        25.39 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/100K-4     	47116774	        25.50 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/1M-4       	46637628	        25.54 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentNew/1K-4            	   15271	     78849 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentNew/10K-4           	    1569	    755457 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentNew/100K-4          	     162	   7368665 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentNew/1M-4            	      15	  73784377 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/1K-4    	  262287	      4580 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/10K-4   	   25424	     47610 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/100K-4  	    2784	    431286 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/1M-4    	     282	   4241878 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/1K-4   	  193932	      6198 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/10K-4  	   19644	     60820 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/100K-4 	    2102	    605190 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/1M-4   	     214	   5600959 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveComponent/1K-4            	   15578	     77734 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveComponent/10K-4           	    1590	    738000 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveComponent/100K-4          	     165	   7228051 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveComponent/1M-4            	      15	  72056258 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveEntity/1K-4               	   96518	     12490 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveEntity/10K-4              	    8998	    129802 ns/op	       1 B/op	       0 allocs/op
BenchmarkRemoveEntity/100K-4             	    1052	   1137397 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveEntity/1M-4               	     100	  11109441 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/1K-4       	  252822	      4752 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/10K-4      	   25213	     47484 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/100K-4     	    2839	    419632 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterRemoveEntities/1M-4       	     289	   4150578 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/1K-4      	  254942	      4739 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/10K-4     	   25632	     46980 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/100K-4    	    2853	    423693 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2RemoveEntities/1M-4      	     288	   4169580 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveEntities/1K-4             	   87817	     13532 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveEntities/10K-4            	    8610	    119523 ns/op	       0 B/op	       0 allocs/op
BenchmarkRemoveEntities/100K-4           	     990	   1264480 ns/op	       8 B/op	       0 allocs/op
BenchmarkRemoveEntities/1M-4             	     100	  11200585 ns/op	       0 B/op	       0 allocs/op
BenchmarkClearEntities/1K-4              	  374592	      3254 ns/op	       0 B/op	       0 allocs/op
BenchmarkClearEntities/10K-4             	   36470	     33142 ns/op	       0 B/op	       0 allocs/op
BenchmarkClearEntities/100K-4            	    4518	    264554 ns/op	       0 B/op	       0 allocs/op
BenchmarkClearEntities/1M-4              	     457	   2624465 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/1K-4              	  510142	      2356 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/10K-4             	   51160	     23505 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/100K-4            	    5058	    233560 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilterIterate/1M-4              	     512	   2331720 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/1K-4             	  522560	      2288 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/10K-4            	   53017	     22600 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/100K-4           	    5277	    227006 ns/op	       0 B/op	       0 allocs/op
BenchmarkFilter2Iterate/1M-4             	     528	   2268082 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/1K-4          	1000000000	         0.0000440 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/10K-4         	1000000000	         0.0002530 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/100K-4        	1000000000	         0.006287 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusSubscribe/1M-4          	1000000000	         0.06433 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/1K-4  	1000000000	         0.0000178 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/10K-4 	1000000000	         0.0000899 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/100K-4         	1000000000	         0.0009209 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishNoHandlers/1M-4           	1000000000	         0.009018 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/1K-4           	1000000000	         0.0000186 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/10K-4          	1000000000	         0.0001825 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/100K-4         	1000000000	         0.001852 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/1M-4           	1000000000	         0.01839 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/1K-4         	  635767	      1889 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/10K-4        	   63837	     18780 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/100K-4       	    6360	    187900 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/1M-4         	     637	   1880374 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/1K-4                        	1000000000	         0.0001164 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/10K-4                       	1000000000	         0.0009778 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/100K-4                      	1000000000	         0.01219 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/1M-4                        	1000000000	         0.1637 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/1K-4                        	1000000000	         0.0000005 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/10K-4                       	1000000000	         0.0000042 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/100K-4                      	1000000000	         0.0000312 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/1M-4                        	1000000000	         0.0003087 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/1K-4                        	1000000000	         0.0000009 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/10K-4                       	1000000000	         0.0000032 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/100K-4                      	1000000000	         0.0000312 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/1M-4                        	1000000000	         0.0003087 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/1K-4                     	1000000000	         0.0000534 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/10K-4                    	1000000000	         0.0004641 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/100K-4                   	1000000000	         0.005955 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/1M-4                     	1000000000	         0.1185 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/1K-4                      	   77750	     15924 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/10K-4                     	   10000	    140288 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/100K-4                    	     928	   1354513 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/1M-4                      	     100	  22719380 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/edwinsyarief/lazyecs	475.819s
```

</details>

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
