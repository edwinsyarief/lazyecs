# Lazy ECS

Just another golang ECS with simple API.

## Features

- Easy component registration for later use.
- Fast & unique ECS architecture.

## Getting Started

### Installation

```shell
go get github.com/edwinsyarief/lazyecs
```

### Example Usage

#### Component Registration

For example you have component below:

```go
type Position struct{ X, Y float32 }
type Velocity struct{ VX, VY float32 }
```

Now you need to create `init` function and register your components:

```go
// in file mygame/components/components.go
package components

import (
    "github.com/edwinsyarief/lazyecs"
)

var (
    posID lazyecs.ComponentID
    velID lazyecs.ComponentID
)

func init() {
    // Register the component and store its ID.
    // The component type is passed as a type parameter.
    posID := lazyecs.RegisterComponent[Position]()
    velID := lazyecs.RegisterComponent[Velocity]()
}
```

### Creating Entity & Component

```go
// create a new ECS world.
world := lazyecs.NewWorld()

// create a new entity.
e := world.CreateEntity()

// add components
p, ok1 := lazyecs.AddComponent[Position](world, e)
v, ok2 := lazyecs.AddComponent[Velocity](world, e)

if ok1 && ok2 {
    p.X = 1
    p.Y = 2

    v.VX = 1
}

// or we can do this
ok1 := SetComponent(world, e, Position{X:1, Y:2})
ok2 := SetComponent(world, e, Velocity{VX:1, VY:0})

if !ok1 || !ok2 {
    // do something
}
```

### Querying Entities & Component

```go
query := world.Query(posID, velID)
for query.Next() {
    for _, entity := range query.Entities() {
        position, _ := lazyecs.GetComponent[Position](world, entity)
        velocity, _ := lazyecs.GetComponent[Velocity](world, entity)
        p.X += velocities[i].VX
        p.Y += velocities[i].VY
    }
}
```

We can also access entities from query:

```go
query := world.Query(posID, velID)
for query.Next() {
    entities := query.Entities() // entities that has position and velocity
    // do something
}
```

### Removing Component from Entity

```go
query := world.Query(posID, velID)
for query.Next() {
    entities := query.Entities()
    for i, e := range entities {
        removed := lazyecs.RemoveComponent[Position](world, e)
        if removed {
            // do something
        }
    } 
}
```

### Removing Entity

```go
world.RemoveEntity(entity)
world.ProcessRemovals()
```

To reduce overhead, `RemoveEntity` function only marks an entity for removal at the end of the frame.
So `ProcessRemovals` function need to be executed or should be called once per game loop
iteration (e.g., at the end of the frame).

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.
