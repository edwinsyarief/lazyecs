# Teishoku

![Go Version](https://img.shields.io/badge/Go-1.25.1-blue?logo=go&style=flat&logoColor=white)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/edwinsyarief/teishoku/test.yml)
<!--![GitHub Repo stars](https://img.shields.io/github/stars/edwinsyarief/teishoku)-->

A high-performance, archetype-based, and easy-to-use Entity Component System (ECS) library for Go.

`Teishoku` is designed for performance-critical applications like games and simulations, offering a simple, generic API that minimizes garbage collection overhead. It uses archetypes to store entities with the same component layout in contiguous memory blocks, enabling extremely fast iteration.

## Features

- **Archetype-Based**: Stores entities with the same components together in contiguous memory for maximum cache efficiency.
- **Generic API**: Leverages Go generics for a type-safe and intuitive developer experience.
- **Zero-Allocation Hot Path**: Optimized for speed with zero GC overhead on the hot path (entity creation, iteration, and component access).
- **Simple and Clean**: Designed to be easy to learn and integrate into any project.

## Getting Started

This guide covers the primary workflow for setting up and using `Teishoku`.

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
builder := teishoku.NewBuilder2[Position, Velocity](world)

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
query := teishoku.NewFilter2[Position, Velocity](world)

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

`teishoku` is built around a few core concepts that work together to provide a high-performance and ergonomic experience.

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

`teishoku` uses Go's `go generate` tool to create boilerplate code for multi-component `Builders`, `Filters`, and `World` API functions (e.g., `NewBuilder3`, `Filter4`, `GetComponent2`). This approach keeps the library's public API clean and consistent without requiring developers to write repetitive code manually.

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

| Action Name                             | 1K (ns) | 10K (ns) | 100K (ns) | 1M (ns) |
| :-------------------------------------- | :------ | :------- | :-------- | :------ |
| **Create World**                        | 5.87    | 5.50     | 4.16      | 3.34    |
| **Auto Expand**                         | 25.67   | 27.68    | 39.28     | N/A     |
| **Create Entity**                       | 6.67    | 7.36     | 5.60      | 5.46    |
| **New Entities (Batch)**                | 2.70    | 2.60     | 2.37      | 2.36    |
| **New Entities With Value Set (Batch)** | 4.58    | 4.76     | 4.31      | 4.24    |
| **Get Component**                       | 4.59    | 4.59     | 4.62      | 4.67    |
| **Set Component Existing**              | 25.49   | 25.39    | 25.50     | 25.54   |
| **Set Component New**                   | 78.85   | 75.55    | 73.69     | 73.78   |
| **Remove Component**                    | 77.73   | 73.80    | 72.28     | 72.06   |
| **Remove Entity**                       | 12.49   | 12.98    | 11.37     | 11.11   |
| **Filter & Remove (Batch)**             | 4.75    | 4.75     | 4.20      | 4.15    |
| **Filter & Iterate**                    | 2.36    | 2.35     | 2.34      | 2.33    |
| **Clear Entities**                      | 3.25    | 3.31     | 2.65      | 2.62    |

### Event Bus Benchmark

| Action Name                 | 1K (ns)  | 10K (ns) | 100K (ns) | 1M (ns)  |
| :-------------------------- | :------- | :------- | :-------- | :------- |
| **Subscribe**               | 0.000044 | 0.000253 | 0.006287  | 0.06433  |
| **Publish (No Handlers)**   | 0.000018 | 0.000090 | 0.000921  | 0.009018 |
| **Publish (One Handler)**   | 0.000019 | 0.000183 | 0.001852  | 0.01839  |
| **Publish (Many Handlers)** | 1.89     | 1.88     | 1.88      | 1.88     |

### Resources Benchmark

| Action Name | 1K (ns)  | 10K (ns) | 100K (ns) | 1M (ns)  |
| :---------- | :------- | :------- | :-------- | :------- |
| **Add**     | 0.000116 | 0.000978 | 0.01219   | 0.1637   |
| **Has**     | 0.000001 | 0.000004 | 0.000031  | 0.000309 |
| **Get**     | 0.000001 | 0.000003 | 0.000031  | 0.000309 |
| **Remove**  | 0.000053 | 0.000464 | 0.005955  | 0.1185   |
| **Clear**   | 15.92    | 14.03    | 13.55     | 22.72    |

<details>

<summary>Click to view raw benchmark output</summary>

```plaintext
goos: linux
goarch: amd64
pkg: github.com/edwinsyarief/teishoku
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkCreateWorld/1K-4                	          140716	          8887 ns/op	   43184 B/op	      12 allocs/op
BenchmarkCreateWorld/10K-4               	           20086	         62327 ns/op	  374962 B/op	      12 allocs/op
BenchmarkCreateWorld/100K-4              	            2406	       1267950 ns/op	 3610810 B/op	      12 allocs/op
BenchmarkCreateWorld/1M-4                	             291	       4125979 ns/op	36018360 B/op	      12 allocs/op
BenchmarkAutoExpand/1K_init_x2-4         	           44095	         27007 ns/op	  143385 B/op	       7 allocs/op
BenchmarkAutoExpand/10K_init_x2-4        	            4136	        275411 ns/op	 1269786 B/op	       7 allocs/op
BenchmarkAutoExpand/100K_init_x2-4       	             452	       3115612 ns/op	13508650 B/op	       7 allocs/op
BenchmarkWorldCreateEntity/1K-4          	           45007	         26238 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntity/10K-4         	            5431	        229213 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntity/100K-4        	             548	       2219992 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntity/1M-4          	              56	      21124371 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/1K-4        	          435333	          2751 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/10K-4       	           47692	         25647 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/100K-4      	            5294	        228120 ns/op	       0 B/op	       0 allocs/op
BenchmarkWorldCreateEntities/1M-4        	             528	       2277886 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/1K-4           	          173992	          6854 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/10K-4          	           16599	         72558 ns/op	       2 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/100K-4         	            2140	        580388 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntity/1M-4           	             218	       5484554 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/1K-4         	          424062	          2818 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/10K-4        	           44530	         26943 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/100K-4       	            5230	        265738 ns/op	       1 B/op	       0 allocs/op
BenchmarkBuilderNewEntities/1M-4         	             501	       2367360 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/1K-4    	          256276	          4680 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/10K-4   	           24685	         48438 ns/op	       1 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/100K-4  	            2349	        450988 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet/1M-4    	             283	       4233496 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/1K-4   	          189168	          6341 ns/op	       0 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/10K-4  	           17635	         68803 ns/op	       1 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/100K-4 	            1825	        752577 ns/op	      16 B/op	       0 allocs/op
BenchmarkNewEntitiesWithValueSet2/1M-4   	             214	       5540356 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSet/1K-4                 	           25635	         47603 ns/op	       0 B/op	       0 allocs/op
BenchmarkBuilderSet/10K-4                	            2560	        472272 ns/op	       2 B/op	       0 allocs/op
BenchmarkBuilderSet/100K-4               	             258	       4652498 ns/op	      10 B/op	       0 allocs/op
BenchmarkBuilderSet/1M-4                 	              26	      44938009 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/1K-4               	       253979222	         4.723 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/10K-4              	       252842235	         4.731 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/100K-4             	       252429990	         4.754 ns/op	       0 B/op	       0 allocs/op
BenchmarkGetComponent/1M-4               	       249598573	         4.782 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/1K-4       	        46708654	         25.31 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/10K-4      	        47373886	         25.35 ns/op	       0 B/op	       0 allocs/op
BenchmarkSetComponentExisting/100K-4     	        47299298	         25.40 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/10K-4          1000000000	     0.0002000 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/100K-4         1000000000	      0.001849 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishOneHandler/1M-4           1000000000	       0.01882 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/1K-4         	  630625	          1890 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/10K-4        	   63984	         18722 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/100K-4       	    6388	        187971 ns/op	       0 B/op	       0 allocs/op
BenchmarkEventBusPublishManyHandlers/1M-4         	     637	       1883845 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/1K-4                        	1000000000	     0.0001165 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/10K-4                       	1000000000	     0.0009914 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/100K-4                      	1000000000	       0.01085 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesAdd/1M-4                        	1000000000	        0.1615 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/1K-4                        	1000000000	     0.0000009 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/10K-4                       	1000000000	     0.0000081 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/100K-4                      	1000000000	     0.0000317 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesHas/1M-4                        	1000000000	     0.0003173 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/1K-4                        	1000000000	     0.0000005 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/10K-4                       	1000000000	     0.0000034 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/100K-4                      	1000000000	     0.0000467 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesGet/1M-4                        	1000000000	     0.0003088 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/1K-4                     	1000000000	     0.0000450 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/10K-4                    	1000000000	     0.0005381 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/100K-4                   	1000000000	      0.005864 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesRemove/1M-4                     	1000000000	        0.1144 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/1K-4                      	     76438	         16029 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/10K-4                     	     10000	        137853 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/100K-4                    	       848	       1408777 ns/op	       0 B/op	       0 allocs/op
BenchmarkResourcesClear/1M-4                      	       100	      20839845 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/edwinsyarief/teishoku	534.130s
```

</details>

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
