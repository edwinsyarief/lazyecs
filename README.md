# Teishoku ECS

![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/edwinsyarief/teishoku/test.yml)
![Go Version](https://img.shields.io/badge/Go-1.25.1-blue?logo=go&style=flat&logoColor=white)
![Go Reference](https://pkg.go.dev/badge/github.com/edwinsyarief/teishoku.svg)

`Teishoku` is a high-performance, archetype-based, and easy-to-use Entity Component System (ECS) library for Go.

It is designed for performance-critical applications like games and simulations, offering a simple, generic API that emphasizes **zero-allocation performance** on the hot path. By storing entities with the same component layout in contiguous memory blocks (archetypes), `Teishoku` enables extremely fast, cache-friendly iteration.

## Features

- **Archetype-Based**: Maximizes cache efficiency by grouping entities with identical component sets.
- **Generic API**: Leverages Go generics for a clean, type-safe, and intuitive developer experience.
- **Zero-Allocation Hot Path**: Entity creation, querying, and component access are optimized for speed with minimal GC overhead.
- **Simple and Clean**: Designed to be easy to learn and integrate into any project.

## When to use Teishoku

`Teishoku` is ideal for projects where performance is critical and you need to manage a large number of objects with varying properties. Common use cases include:

-   Game development (e.g., managing game objects, characters, and particles).
-   High-performance simulations.
-   Real-time systems that require predictable performance and low latency.

## How It Works

`Teishoku` is built around a few core concepts that work together to provide a high-performance and ergonomic experience.

### World, Entity, and Component

-   **World**: The central container that manages all entities, components, and game state. All ECS operations happen within a `World`. It also includes a `Resources` map for storing global, singleton-like data.
-   **Entity**: A simple integer that uniquely identifies an object in your application. It doesn't hold any data itself but serves as a key to associate a group of components.
-   **Component**: A plain Go struct that stores data (e.g., `Position`, `Velocity`). Components should contain only data and no logic.

### Archetypes and Performance

The key to `teishoku`'s performance is its **archetype-based architecture**.

An **archetype** is a unique combination of component types. For example, all entities that have _only_ a `Position` and a `Velocity` component belong to the same archetype.

Inside an archetype, all components are stored in tightly packed, contiguous arrays. This means all `Position` components are next to each other in memory, and all `Velocity` components are also next to each other. When a `Filter` iterates over entities, it can access their component data in a linear, cache-friendly manner, which is extremely fast.

When you add or remove a component from an entity, the entity is moved from its old archetype to a new one that matches its new set of components. While this operation is efficient, it is slower than creating entities with a fixed layout using a `Builder`.

## Getting Started

This guide covers the primary workflow for setting up and using `Teishoku`.

### 1. Installation

Add `Teishoku` as a dependency to your project:

```bash
go get github.com/edwinsyarief/teishoku
```

### 2. A Complete Example

Here is a complete, runnable example that demonstrates the core features:

```go
package main

import (
	"fmt"
	"github.com/edwinsyarief/teishoku"
)

// 1. Define Components
// Components are simple Go structs that hold data.

type Position struct {
	X, Y float32
}

type Velocity struct {
	VX, VY float32
}

func main() {
	// 2. Create a World
	// The World is the container for all your entities and components.
	world := teishoku.NewWorld(10000)

	// 3. Create Entities with a Builder
	// A Builder is the most efficient way to create entities with a
	// predefined set of components.
	builder := teishoku.NewBuilder2[Position, Velocity](&world)

	// Create 100 entities in a batch.
	builder.NewEntities(100)

	// 4. Initialize Components with a Filter
	// Use a Filter (or "query") to iterate over entities and initialize them.
	initQuery := teishoku.NewFilter2[Position, Velocity](&world)
	i := 0
	for initQuery.Next() {
		pos, vel := initQuery.Get()
		pos.X = float32(i) * 2.0
		pos.Y = 0
		vel.VX = 1.0
		vel.VY = 0.5
		i++
	}

	// 5. Create a System to Update Entities
	// A system is a loop that uses a Filter to update component data.
	movementSystem := teishoku.NewFilter2[Position, Velocity](&world)
	for i := 0; i < 5; i++ { // Simulate 5 frames
		// When reusing a filter, you must call Reset() before each iteration.
		movementSystem.Reset()
		for movementSystem.Next() {
			pos, vel := movementSystem.Get()
			pos.X += vel.VX
			pos.Y += vel.VY
		}
	}
    
    // 6. Verify the result (optional)
    resultQuery := teishoku.NewFilter2[Position, Velocity](&world)
    fmt.Printf("Entity 10's final position: %+v\n", *resultQuery.Entities()[10])
}
```

## Code Generation

`Teishoku` uses Go's `go generate` tool to create boilerplate code for multi-component `Builders`, `Filters`, and `World` API functions (e.g., `NewBuilder3`, `Filter4`, `GetComponent2`). This approach keeps the library's public API clean and consistent without requiring developers to write repetitive code manually.

To run the code generator, simply execute the following command from the root of the repository:

```bash
go generate ./...
```

This will regenerate the `*_generated.go` files. You should run this command whenever you change the templates in the `templates/` directory.

## API Reference

The following tables provide a summary of the core API. For complete and up-to-date documentation, please refer to the **GoDoc comments in the source code**, which serve as the official API reference.

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

The `World` object and the `Resources` manager are thread-safe. However, the `EventBus` is **not** thread-safe and should not be accessed from multiple goroutines concurrently without external synchronization.

## Benchmark Results

The following tables summarize the performance of `teishoku` across a range of common operations. The results are presented in nanoseconds (ns) per unit (e.g., per entity) and were run on an AMD EPYC 7763 64-Core Processor.

Notably, many core operations like creating entities, accessing components, and iterating with filters show **zero memory allocations** (`0 allocs/op`), making `teishoku` ideal for performance-critical applications where garbage collection pressure is a concern.

### Entity Benchmark

| Action Name | Entities | ns/op | B/op | allocs/op |
| :--- | :--- | :--- | :--- | :--- |
| **Create World** | 1K | 9917 | 49848 | 13 |
| | 10K | 70488 | 381626 | 13 |
| | 100K | 629840 | 3617473 | 13 |
| | 1M | 4369555 | 36025028 | 13 |
| **Auto Expand** | 1K | 54435 | 143384 | 7 |
| | 10K | 582947 | 1269786 | 7 |
| | 100K | 4525902 | 13508661 | 7 |
| | 1M | 40643101 | 134651935 | 7 |
| **World: Create Entity** | 1K | 40366 | 0 | 0 |
| | 10K | 352969 | 0 | 0 |
| | 100K | 3536612 | 0 | 0 |
| | 1M | 33623122 | 0 | 0 |
| **World: Create Entities (Batch)** | 1K | 3009 | 0 | 0 |
| | 10K | 26321 | 0 | 0 |
| | 100K | 234125 | 0 | 0 |
| | 1M | 2462145 | 0 | 0 |
| **Builder: New Entity** | 1K | 18620 | 0 | 0 |
| | 10K | 178736 | 2 | 0 |
| | 100K | 1558767 | 12 | 0 |
| | 1M | 14534337 | 0 | 0 |
| **Builder: New Entities (Batch)** | 1K | 2889 | 0 | 0 |
| | 10K | 27209 | 0 | 0 |
| | 100K | 283295 | 0 | 0 |
| | 1M | 2449114 | 0 | 0 |
| **Builder: New Entities w/ Value Set (Batch)** | 1K | 4724 | 0 | 0 |
| | 10K | 48071 | 1 | 0 |
| | 100K | 442229 | 0 | 0 |
| | 1M | 4117529 | 0 | 0 |
| **Builder: New Entities w/ Value Set 2 (Batch)** | 1K | 6408 | 0 | 0 |
| | 10K | 65159 | 1 | 0 |
| | 100K | 670657 | 12 | 0 |
| | 1M | 5526794 | 0 | 0 |
| **Builder: Set Component** | 1K | 57795 | 0 | 0 |
| | 10K | 566780 | 0 | 0 |
| | 100K | 5555934 | 10 | 0 |
| | 1M | 53948158 | 0 | 0 |
| **Builder: Set Component 2** | 1K | 67170 | 0 | 0 |
| | 10K | 666647 | 3 | 0 |
| | 100K | 6392592 | 0 | 0 |
| | 1M | 63424222 | 0 | 0 |
| **Builder: Get Component** | 1K | 8.424 | 0 | 0 |
| | 10K | 8.543 | 0 | 0 |
| **Filter & Iterate** | 1K | 952.8 | 0 | 0 |
| | 10K | 9356 | 0 | 0 |
| | 100K | 93304 | 0 | 0 |
| | 1M | 935081 | 0 | 0 |
| **Filter & Iterate (2 components)** | 1K | 954.5 | 0 | 0 |
| | 10K | 9358 | 0 | 0 |
| | 100K | 93322 | 0 | 0 |
| | 1M | 934063 | 0 | 0 |
| **Filter & Iterate (3 components)** | 1K | 975.1 | 0 | 0 |
| | 10K | 9387 | 0 | 0 |
| | 100K | 93513 | 0 | 0 |
| | 1M | 935502 | 0 | 0 |
| **Filter & Iterate (4 components)** | 1K | 953.9 | 0 | 0 |
| | 10K | 9355 | 0 | 0 |
| | 100K | 93623 | 0 | 0 |
| | 1M | 934623 | 0 | 0 |
| **Filter & Iterate (5 components)** | 1K | 952.7 | 0 | 0 |
| | 10K | 9362 | 0 | 0 |
| | 100K | 93507 | 0 | 0 |
| | 1M | 933519 | 0 | 0 |
| **Filter & Iterate (6 components)** | 1K | 952.7 | 0 | 0 |
| | 10K | 9362 | 0 | 0 |
| | 100K | 93507 | 0 | 0 |
| | 1M | 933519 | 0 | 0 |
| **Filter: Get Entities (Cached)** | 1K | 7.512 | 0 | 0 |
| | 10K | 7.507 | 0 | 0 |
| | 100K | 7.514 | 0 | 0 |
| | 1M | 7.501 | 0 | 0 |
| **Filter: Get Entities (Uncached)** | 1K | 411.3 | 0 | 0 |
| | 10K | 2416 | 0 | 0 |
| | 100K | 34316 | 0 | 0 |
| | 1M | 384441 | 0 | 0 |

### Event Bus Benchmark

| Action Name | Entities | ns/op | B/op | allocs/op |
| :--- | :--- | :--- | :--- | :--- |
| **Subscribe** | 1K | 0.0000533 | 0 | 0 |
| | 10K | 0.0002618 | 0 | 0 |
| | 100K | 0.005772 | 0 | 0 |
| | 1M | 0.06385 | 0 | 0 |
| **Publish (No Handlers)** | 1K | 0.0000088 | 0 | 0 |
| | 10K | 0.0000961 | 0 | 0 |
| | 100K | 0.0009734 | 0 | 0 |
| | 1M | 0.008387 | 0 | 0 |
| **Publish (One Handler)** | 1K | 0.0000271 | 0 | 0 |
| | 10K | 0.0001824 | 0 | 0 |
| | 100K | 0.001911 | 0 | 0 |
| | 1M | 0.01867 | 0 | 0 |
| **Publish (Many Handlers)** | 1K | 1890 | 0 | 0 |
| | 10K | 18697 | 0 | 0 |
| | 100K | 188506 | 0 | 0 |
| | 1M | 1897631 | 0 | 0 |

### Resources Benchmark

| Action Name | Entities | ns/op | B/op | allocs/op |
| :--- | :--- | :--- | :--- | :--- |
| **Add** | 1K | 0.0001176 | 0 | 0 |
| | 10K | 0.001122 | 0 | 0 |
| | 100K | 0.01192 | 0 | 0 |
| | 1M | 0.2008 | 0 | 0 |
| **Has** | 1K | 0.0000100 | 0 | 0 |
| | 10K | 0.0000609 | 0 | 0 |
| | 100K | 0.0005573 | 0 | 0 |
| | 1M | 0.005779 | 0 | 0 |
| **Get** | 1K | 0.0000065 | 0 | 0 |
| | 10K | 0.0000739 | 0 | 0 |
| | 100K | 0.0006336 | 0 | 0 |
| | 1M | 0.006237 | 0 | 0 |
| **Remove** | 1K | 0.0000697 | 0 | 0 |
| | 10K | 0.0005709 | 0 | 0 |
| | 100K | 0.006688 | 0 | 0 |
| | 1M | 0.1681 | 0 | 0 |
| **Clear** | 1K | 15769 | 0 | 0 |
| | 10K | 141010 | 0 | 0 |
| | 100K | 1386929 | 0 | 0 |
| | 1M | 23580516 | 0 | 0 |

<details>
<summary>Click to view raw benchmark output</summary>

```plaintext
goos: linux
goarch: amd64
pkg: github.com/edwinsyarief/teishoku
cpu: AMD EPYC 7763 64-Core Processor
BenchmarkCreateWorld/1K-4                      113096          9917 ns/op        49848 B/op          13 allocs/op
BenchmarkCreateWorld/10K-4                      16908         70488 ns/op       381626 B/op          13 allocs/op
BenchmarkCreateWorld/100K-4                      2374        629840 ns/op      3617473 B/op          13 allocs/op
BenchmarkCreateWorld/1M-4                         272       4369555 ns/op     36025028 B/op          13 allocs/op
BenchmarkAutoExpand/1K_init_x2-4                22147         54435 ns/op       143384 B/op           7 allocs/op
BenchmarkAutoExpand/10K_init_x2-4                2034        582947 ns/op      1269786 B/op           7 allocs/op
BenchmarkAutoExpand/100K_init_x2-4                241       4525902 ns/op     13508661 B/op           7 allocs/op
BenchmarkAutoExpand/1000K_init_x2-4                25      40643101 ns/op    134651935 B/op           7 allocs/op
BenchmarkWorldCreateEntity/1K-4                 29593         40366 ns/op            0 B/op           0 allocs/op
BenchmarkWorldCreateEntity/10K-4                 3470        352969 ns/op            0 B/op           0 allocs/op
BenchmarkWorldCreateEntity/100K-4                 352       3536612 ns/op            0 B/op           0 allocs/op
BenchmarkWorldCreateEntity/1M-4                    36      33623122 ns/op            0 B/op           0 allocs/op
BenchmarkWorldCreateEntities/1K-4              397861          3009 ns/op            0 B/op           0 allocs/op
BenchmarkWorldCreateEntities/10K-4              45781         26321 ns/op            0 B/op           0 allocs/op
BenchmarkWorldCreateEntities/100K-4              5116        234125 ns/op            0 B/op           0 allocs/op
BenchmarkWorldCreateEntities/1M-4                 489       2462145 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntity/1K-4                  63282         18620 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntity/10K-4                  7418        178736 ns/op            2 B/op           0 allocs/op
BenchmarkBuilderNewEntity/100K-4                  764       1558767 ns/op           12 B/op           0 allocs/op
BenchmarkBuilderNewEntity/1M-4                     81      14534337 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntities/1K-4               410301          2889 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntities/10K-4               44179         27209 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntities/100K-4               3710        283295 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntities/1M-4                  498       2449114 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/1K-4   252379          4724 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/10K-4   25288         48071 ns/op            1 B/op           0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/100K-4   2820        442229 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet/1M-4      291       4117529 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/1K-4  188880          6408 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/10K-4  18366         65159 ns/op            1 B/op           0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/100K-4  1785        670657 ns/op           12 B/op           0 allocs/op
BenchmarkBuilderNewEntitiesWithValueSet2/1M-4     216       5526794 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderSetComponent/1K-4               20775         57795 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderSetComponent/10K-4               2108        566780 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderSetComponent/100K-4               210       5555934 ns/op           10 B/op           0 allocs/op
BenchmarkBuilderSetComponent/1M-4                  21      53948158 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderSetComponent2/1K-4              17824         67170 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderSetComponent2/10K-4              1821        666647 ns/op            3 B/op           0 allocs/op
BenchmarkBuilderSetComponent2/100K-4              187       6392592 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderSetComponent2/1M-4                 18      63424222 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderGetComponent/1K-4              142381077         8.424 ns/op            0 B/op           0 allocs/op
BenchmarkBuilderGetComponent/10K-4             140378828         8.543 ns/op            0 B/op           0 allocs/op
BenchmarkFilter2Iterate/1K-4                   1260806         952.8 ns/op            0 B/op           0 allocs/op
BenchmarkFilter2Iterate/10K-4                   127950          9356 ns/op            0 B/op           0 allocs/op
BenchmarkFilter2Iterate/100K-4                  12830         93304 ns/op            0 B/op           0 allocs/op
BenchmarkFilter2Iterate/1M-4                     1284        935081 ns/op            0 B/op           0 allocs/op
BenchmarkFilter3Iterate/1K-4                   1259912         954.5 ns/op            0 B/op           0 allocs/op
BenchmarkFilter3Iterate/10K-4                   128036          9358 ns/op            0 B/op           0 allocs/op
BenchmarkFilter3Iterate/100K-4                  12849         93322 ns/op            0 B/op           0 allocs/op
BenchmarkFilter3Iterate/1M-4                     1280        934063 ns/op            0 B/op           0 allocs/op
BenchmarkFilter4Iterate/1K-4                   1230201         975.1 ns/op            0 B/op           0 allocs/op
BenchmarkFilter4Iterate/10K-4                   127768          9387 ns/op            0 B/op           0 allocs/op
BenchmarkFilter4Iterate/100K-4                  12852         93513 ns/op            0 B/op           0 allocs/op
BenchmarkFilter4Iterate/1M-4                     1282        935502 ns/op            0 B/op           0 allocs/op
BenchmarkFilter5Iterate/1K-4                   1258552         953.9 ns/op            0 B/op           0 allocs/op
BenchmarkFilter5Iterate/10K-4                   128028          9355 ns/op            0 B/op           0 allocs/op
BenchmarkFilter5Iterate/100K-4                  12862         93623 ns/op            0 B/op           0 allocs/op
BenchmarkFilter5Iterate/1M-4                     1286        934623 ns/op            0 B/op           0 allocs/op
BenchmarkFilter6Iterate/1K-4                   1261116         952.7 ns/op            0 B/op           0 allocs/op
BenchmarkFilter6Iterate/10K-4                   128180          9362 ns/op            0 B/op           0 allocs/op
BenchmarkFilter6Iterate/100K-4                  12831         93507 ns/op            0 B/op           0 allocs/op
BenchmarkFilter6Iterate/1M-4                     1281        933519 ns/op            0 B/op           0 allocs/op
BenchmarkFilterGetEntitiesCached/1K-4          159448855         7.512 ns/op            0 B/op           0 allocs/op
BenchmarkFilterGetEntitiesCached/10K-4         160041580         7.507 ns/op            0 B/op           0 allocs/op
BenchmarkFilterGetEntitiesCached/100K-4        159234398         7.514 ns/op            0 B/op           0 allocs/op
BenchmarkFilterGetEntitiesCached/1M-4          159619701         7.501 ns/op            0 B/op           0 allocs/op
BenchmarkFilterGetEntitiesUncached/1K-4        2840005         411.3 ns/op            0 B/op           0 allocs/op
BenchmarkFilterGetEntitiesUncached/10K-4        538636          2416 ns/op            0 B/op           0 allocs/op
BenchmarkFilterGetEntitiesUncached/100K-4        33975         34316 ns/op            0 B/op           0 allocs/op
BenchmarkFilterGetEntitiesUncached/1M-4           2859        384441 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusSubscribe/1K-4                1000000000         0.0000533 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusSubscribe/10K-4               1000000000         0.0002618 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusSubscribe/100K-4              1000000000         0.005772 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusSubscribe/1M-4                1000000000         0.06385 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishNoHandlers/1K-4        1000000000         0.0000088 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishNoHandlers/10K-4       1000000000         0.0000961 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishNoHandlers/100K-4      1000000000         0.0009734 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishNoHandlers/1M-4        1000000000         0.008387 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishOneHandler/1K-4        1000000000         0.0000271 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishOneHandler/10K-4       1000000000         0.0001824 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishOneHandler/100K-4      1000000000         0.001911 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishOneHandler/1M-4        1000000000         0.01867 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishManyHandlers/1K-4        634951          1890 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishManyHandlers/10K-4        64209         18697 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishManyHandlers/100K-4       6370        188506 ns/op            0 B/op           0 allocs/op
BenchmarkEventBusPublishManyHandlers/1M-4          633       1897631 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesAdd/1K-4                     1000000000         0.0001176 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesAdd/10K-4                    1000000000         0.001122 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesAdd/100K-4                   1000000000         0.01192 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesAdd/1M-4                     1000000000         0.2008 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesHas/1K-4                     1000000000         0.0000100 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesHas/10K-4                    1000000000         0.0000609 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesHas/100K-4                   1000000000         0.0005573 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesHas/1M-4                     1000000000         0.005779 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesGet/1K-4                     1000000000         0.0000065 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesGet/10K-4                    1000000000         0.0000739 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesGet/100K-4                   1000000000         0.0006336 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesGet/1M-4                     1000000000         0.006237 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesRemove/1K-4                  1000000000         0.0000697 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesRemove/10K-4                 1000000000         0.0005709 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesRemove/100K-4                1000000000         0.006688 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesRemove/1M-4                  1000000000         0.1681 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesClear/1K-4                       77509         15769 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesClear/10K-4                      10000        141010 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesClear/100K-4                       832       1386929 ns/op            0 B/op           0 allocs/op
BenchmarkResourcesClear/1M-4                         100      23580516 ns/op            0 B/op           0 allocs/op
PASS
ok      github.com/edwinsyarief/teishoku    881.890s
```

</details>

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
