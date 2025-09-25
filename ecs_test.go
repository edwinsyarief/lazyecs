package lazyecs_test

import (
	"testing"

	"github.com/edwinsyarief/lazyecs"
)

// --- Test Components ---
type Position struct{ X, Y float32 }
type Velocity struct{ VX, VY float32 }
type Health struct{ Current, Max int }
type Tag struct{}
type UnregisteredComponent struct{}

// --- Test Suite Setup ---
func setupWorld(_ *testing.T) (*lazyecs.World, lazyecs.ComponentID, lazyecs.ComponentID, lazyecs.ComponentID) {
	lazyecs.ResetGlobalRegistry()
	posID := lazyecs.RegisterComponent[Position]()
	velID := lazyecs.RegisterComponent[Velocity]()
	healthID := lazyecs.RegisterComponent[Health]()
	lazyecs.RegisterComponent[Tag]()
	return lazyecs.NewWorld(), posID, velID, healthID
}

// --- Tests ---

// go test -run ^TestCreateEntity$ . -count 1
func TestCreateEntity(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	e1 := world.CreateEntity()
	e2 := world.CreateEntity()

	if e1.ID != 0 {
		t.Errorf("Expected first entity ID to be 0, got %d", e1.ID)
	}
	if e1.Version != 1 {
		t.Errorf("Expected first entity version to be 1, got %d", e1.Version)
	}
	if e2.ID != 1 {
		t.Errorf("Expected second entity ID to be 1, got %d", e2.ID)
	}
}

// go test -run ^TestAddComponent$ . -count 1
func TestAddComponent(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	e := world.CreateEntity()

	p, ok := lazyecs.AddComponent[Position](world, e)
	if !ok {
		t.Fatal("Failed to add component")
	}
	if p == nil {
		t.Fatal("AddComponent returned a nil pointer")
	}

	p.X = 10
	p.Y = 20

	retrievedP, ok := lazyecs.GetComponent[Position](world, e)
	if !ok {
		t.Fatal("GetComponent failed to find the component")
	}
	if retrievedP.X != 10 || retrievedP.Y != 20 {
		t.Errorf("Component data is incorrect after adding. Got %+v", retrievedP)
	}
}

// go test -run ^TestSetComponent$ . -count 1
func TestSetComponent(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	e := world.CreateEntity()

	// --- SCENARIO 1: ADD A NEW COMPONENT ---
	t.Run("AddNewComponent", func(t *testing.T) {
		ok := lazyecs.SetComponent(world, e, Position{X: 100, Y: 200})
		if !ok {
			t.Fatal("SetComponent failed to add a new component")
		}

		p, ok := lazyecs.GetComponent[Position](world, e)
		if !ok {
			t.Fatal("GetComponent failed after SetComponent added a component")
		}
		if p.X != 100 || p.Y != 200 {
			t.Errorf("Component data incorrect after SetComponent add. Expected {100, 200}, got %+v", p)
		}
	})

	// --- SCENARIO 2: UPDATE AN EXISTING COMPONENT ---
	t.Run("UpdateExistingComponent", func(t *testing.T) {
		// Add a velocity component to ensure it's not affected by the update.
		lazyecs.SetComponent(world, e, Velocity{VX: 1, VY: 2})

		ok := lazyecs.SetComponent(world, e, Position{X: 555, Y: 777})
		if !ok {
			t.Fatal("SetComponent failed to update an existing component")
		}

		p, ok := lazyecs.GetComponent[Position](world, e)
		if !ok {
			t.Fatal("GetComponent failed after SetComponent updated a component")
		}
		if p.X != 555 || p.Y != 777 {
			t.Errorf("Component data incorrect after SetComponent update. Expected {555, 777}, got %+v", p)
		}

		// Verify other components are untouched
		v, ok := lazyecs.GetComponent[Velocity](world, e)
		if !ok {
			t.Fatal("Velocity component was lost after updating Position")
		}
		if v.VX != 1 || v.VY != 2 {
			t.Errorf("Velocity component data was corrupted. Got %+v", v)
		}
	})

	// --- SCENARIO 3: ATTEMPT TO SET UNREGISTERED COMPONENT ---
	t.Run("SetUnregisteredComponent", func(t *testing.T) {
		ok := lazyecs.SetComponent(world, e, UnregisteredComponent{})
		if ok {
			t.Fatal("SetComponent should return false for an unregistered component, but it returned true")
		}
	})
}

// go test -run ^TestRemoveComponent$ . -count 1
func TestRemoveComponent(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	e := world.CreateEntity()
	lazyecs.AddComponent[Position](world, e)
	lazyecs.AddComponent[Velocity](world, e)

	removed := lazyecs.RemoveComponent[Position](world, e)
	if !removed {
		t.Fatal("RemoveComponent returned false")
	}

	_, ok := lazyecs.GetComponent[Position](world, e)
	if ok {
		t.Fatal("Component was not actually removed")
	}

	_, ok = lazyecs.GetComponent[Velocity](world, e)
	if !ok {
		t.Fatal("There is a component that not removed but removed")
	}
}

// go test -run ^TestEntityRemoval$ . -count 1
func TestEntityRemoval(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	e1 := world.CreateEntity()
	lazyecs.AddComponent[Position](world, e1)
	e2 := world.CreateEntity()
	p2, _ := lazyecs.AddComponent[Position](world, e2)
	p2.X = 100

	world.RemoveEntity(e1)
	world.ProcessRemovals()

	// Check if e1 is gone
	_, ok := lazyecs.GetComponent[Position](world, e1)
	if ok {
		t.Fatal("GetComponent should fail for a removed entity")
	}

	// Check if e2 is still there and correct
	p2Retrieved, ok := lazyecs.GetComponent[Position](world, e2)
	if !ok {
		t.Fatal("Entity e2 was removed incorrectly")
	}
	if p2Retrieved.X != 100 {
		t.Errorf("Data for entity e2 was corrupted. Got %+v", p2Retrieved)
	}

	// Check if query is correct
	query := lazyecs.CreateQuery[Position](world)
	count := 0
	for query.Next() {
		count++
	}
	if count != 1 {
		t.Errorf("Query returned %d entities, expected 1", count)
	}
}

// go test -run ^TestQuery$ . -count 1
func TestQuery(t *testing.T) {
	world, _, _, _ := setupWorld(t)

	// Entity with Position and Velocity
	e1 := world.CreateEntity()
	p1, _ := lazyecs.AddComponent[Position](world, e1)
	p1.X = 10
	lazyecs.AddComponent[Velocity](world, e1)

	// Entity with only Position
	e2 := world.CreateEntity()
	p2, _ := lazyecs.AddComponent[Position](world, e2)
	p2.X = 20

	// Query for entities with both Position and Velocity
	queryBoth := lazyecs.CreateQuery2[Position, Velocity](world)
	countBoth := 0
	foundE1 := false
	for queryBoth.Next() {
		pos, _ := queryBoth.Get()
		e := queryBoth.Entity()
		countBoth++
		if e.ID == e1.ID {
			foundE1 = true
			if pos.X != 10 {
				t.Errorf("Incorrect component data in query slice for e1")
			}
		}
	}
	if countBoth != 1 {
		t.Errorf("Expected query for Pos+Vel to find 1 entity, found %d", countBoth)
	}
	if !foundE1 {
		t.Errorf("Query for Pos+Vel did not find entity e1")
	}

	// Query for entities with at least Position
	queryPos := lazyecs.CreateQuery[Position](world)
	countPos := 0
	for queryPos.Next() {
		countPos++
	}
	if countPos != 2 {
		t.Errorf("Expected query for Pos to find 2 entities, found %d", countPos)
	}
}

// go test -run ^TestComponentDataIntegrityAfterSwapAndPop$ . -count 1
func TestComponentDataIntegrityAfterSwapAndPop(t *testing.T) {
	world, _, _, _ := setupWorld(t)

	entities := make([]lazyecs.Entity, 11)
	for i := 0; i < 10; i++ {
		entities[i] = world.CreateEntity()
		p, _ := lazyecs.AddComponent[Position](world, entities[i])
		p.X = float32(i)
	}

	// Remove an entity from the middle
	entityToRemove := entities[5]
	world.RemoveEntity(entityToRemove)

	lastEnt := world.CreateEntity()
	p, _ := lazyecs.AddComponent[Position](world, lastEnt)
	p.X = 10

	entities[10] = lastEnt

	world.ProcessRemovals()

	// Check that the removed entity is gone
	_, ok := lazyecs.GetComponent[Position](world, entityToRemove)
	if ok {
		t.Fatalf("Entity %d was not removed", entityToRemove.ID)
	}

	// Check that all other entities have their correct data
	for i, e := range entities {
		if i == 5 { // Skip the removed one
			continue
		}
		p, ok := lazyecs.GetComponent[Position](world, e)
		if !ok {
			t.Errorf("Entity %d lost its component after removal", e.ID)
			continue
		}
		if p.X != float32(i) {
			t.Errorf("Data for entity %d is incorrect. Expected X=%d, got X=%.f", e.ID, i, p.X)
		}
	}

	// Check query count
	query := lazyecs.CreateQuery[Position](world)
	count := 0
	for query.Next() {
		count++
	}
	if count != 10 {
		t.Errorf("Query returned %d entities after removal, expected 10", count)
	}
}

const numEntities = 100000

// go test -benchmem -run=^$ -bench ^BenchmarkAddComponent$ . -count 1
func BenchmarkAddComponent(b *testing.B) {
	world := lazyecs.NewWorld()
	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()

	entities := world.CreateEntities(numEntities)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, e := range entities {
			lazyecs.AddComponent[Position](world, e)
		}
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkRemoveComponent$ . -count 1
func BenchmarkRemoveComponent(b *testing.B) {
	world := lazyecs.NewWorld()
	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()

	entities := world.CreateEntities(numEntities)
	for _, e := range entities {
		lazyecs.AddComponent[Position](world, e)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, e := range entities {
			lazyecs.RemoveComponent[Position](world, e)
		}
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkAddRemoveEntities$ . -count 1
func BenchmarkAddRemoveEntities(b *testing.B) {
	world := lazyecs.NewWorld()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entities := world.CreateEntities(numEntities)
		for _, e := range entities {
			world.RemoveEntity(e)
		}
		world.ProcessRemovals()
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkQuery$ . -count 1
func BenchmarkQuery(b *testing.B) {
	world := lazyecs.NewWorld()
	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	lazyecs.RegisterComponent[Velocity]()

	entities := world.CreateEntities(numEntities)
	for _, e := range entities {
		lazyecs.AddComponent[Position](world, e)
		lazyecs.AddComponent[Velocity](world, e)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := lazyecs.CreateQuery2[Position, Velocity](world)
		for query.Next() {
			pos, vel := query.Get()
			pos.X += vel.VX
		}
	}
}
