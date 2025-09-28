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

// go test -run ^TestAddComponent2$ . -count 1
func TestAddComponent2(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	e := world.CreateEntity()

	p, v, ok := lazyecs.AddComponent2[Position, Velocity](world, e)
	if !ok {
		t.Fatal("Failed to add components")
	}
	if p == nil || v == nil {
		t.Fatal("AddComponent2 returned nil pointer")
	}

	p.X = 10
	v.VX = 5

	retrievedP, ok := lazyecs.GetComponent[Position](world, e)
	if !ok || retrievedP.X != 10 {
		t.Error("Position not added correctly")
	}
	retrievedV, ok := lazyecs.GetComponent[Velocity](world, e)
	if !ok || retrievedV.VX != 5 {
		t.Error("Velocity not added correctly")
	}
}

// go test -run ^TestAddComponentBatch$ . -count 1
func TestAddComponentBatch(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	entities := world.CreateEntities(3)

	ps := lazyecs.AddComponentBatch[Position](world, entities)
	for i, p := range ps {
		if p != nil {
			p.X = float32(i + 1)
		} else {
			t.Errorf("Nil pointer for entity %d", i)
		}
	}

	for i, e := range entities {
		p, ok := lazyecs.GetComponent[Position](world, e)
		if !ok || p.X != float32(i+1) {
			t.Errorf("Incorrect data for entity %d", i)
		}
	}
}

// go test -run ^TestGetComponent2$ . -count 1
func TestGetComponent2(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	e := world.CreateEntity()
	lazyecs.SetComponent(world, e, Position{X: 1, Y: 2})
	lazyecs.SetComponent(world, e, Velocity{VX: 3, VY: 4})

	p, v, ok := lazyecs.GetComponent2[Position, Velocity](world, e)
	if !ok {
		t.Fatal("GetComponent2 failed")
	}
	if p.X != 1 || p.Y != 2 {
		t.Errorf("Position data incorrect. Got %+v", p)
	}
	if v.VX != 3 || v.VY != 4 {
		t.Errorf("Velocity data incorrect. Got %+v", v)
	}

	// Test with missing component
	_, _, ok = lazyecs.GetComponent2[Position, Health](world, e)
	if ok {
		t.Error("GetComponent2 should have failed for missing component")
	}
}

// go test -run ^TestRemoveComponent2$ . -count 1
func TestRemoveComponent2(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	e := world.CreateEntity()
	lazyecs.AddComponent2[Position, Velocity](world, e)
	lazyecs.AddComponent[Health](world, e)

	ok := lazyecs.RemoveComponent2[Position, Velocity](world, e)
	if !ok {
		t.Fatal("Failed to remove components")
	}

	_, hasP := lazyecs.GetComponent[Position](world, e)
	_, hasV := lazyecs.GetComponent[Velocity](world, e)
	_, hasH := lazyecs.GetComponent[Health](world, e)
	if hasP || hasV {
		t.Error("Components not removed")
	}
	if !hasH {
		t.Error("Unrelated component removed")
	}
}

// go test -run ^TestRemoveComponentBatch$ . -count 1
func TestRemoveComponentBatch(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	entities := world.CreateEntities(3)
	lazyecs.AddComponentBatch[Position](world, entities)
	lazyecs.AddComponentBatch[Velocity](world, entities)

	lazyecs.RemoveComponentBatch[Position](world, entities)

	for _, e := range entities {
		_, hasP := lazyecs.GetComponent[Position](world, e)
		_, hasV := lazyecs.GetComponent[Velocity](world, e)
		if hasP {
			t.Error("Position not removed")
		}
		if !hasV {
			t.Error("Velocity removed incorrectly")
		}
	}
}

// go test -run ^TestSetComponent2$ . -count 1
func TestSetComponent2(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	e := world.CreateEntity()

	ok := lazyecs.SetComponent2(world, e, Position{X: 10}, Velocity{VX: 5})
	if !ok {
		t.Fatal("Failed to set components")
	}

	p, ok := lazyecs.GetComponent[Position](world, e)
	if !ok || p.X != 10 {
		t.Error("Position not set correctly")
	}
	v, ok := lazyecs.GetComponent[Velocity](world, e)
	if !ok || v.VX != 5 {
		t.Error("Velocity not set correctly")
	}

	// Update
	ok = lazyecs.SetComponent2(world, e, Position{X: 20}, Velocity{VX: 10})
	if !ok {
		t.Fatal("Failed to update components")
	}
	p, _ = lazyecs.GetComponent[Position](world, e)
	if p.X != 20 {
		t.Error("Position not updated")
	}
	v, _ = lazyecs.GetComponent[Velocity](world, e)
	if v.VX != 10 {
		t.Error("Velocity not updated")
	}
}

// go test -run ^TestSetComponentBatch$ . -count 1
func TestSetComponentBatch(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	entities := world.CreateEntities(3)

	lazyecs.SetComponentBatch(world, entities, Position{X: 100})

	for _, e := range entities {
		p, ok := lazyecs.GetComponent[Position](world, e)
		if !ok || p.X != 100 {
			t.Error("Position not set correctly")
		}
	}

	// Update
	lazyecs.SetComponentBatch(world, entities, Position{X: 200})
	for _, e := range entities {
		p, _ := lazyecs.GetComponent[Position](world, e)
		if p.X != 200 {
			t.Error("Position not updated")
		}
	}
}

// go test -run ^TestSetComponentBatch2$ . -count 1
func TestSetComponentBatch2(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	entities := world.CreateEntities(3)

	lazyecs.SetComponentBatch2(world, entities, Position{X: 10}, Velocity{VX: 5})

	for _, e := range entities {
		p, ok := lazyecs.GetComponent[Position](world, e)
		if !ok || p.X != 10 {
			t.Error("Position not set")
		}
		v, ok := lazyecs.GetComponent[Velocity](world, e)
		if !ok || v.VX != 5 {
			t.Error("Velocity not set")
		}
	}

	// Update
	lazyecs.SetComponentBatch2(world, entities, Position{X: 20}, Velocity{VX: 10})
	for _, e := range entities {
		p, _ := lazyecs.GetComponent[Position](world, e)
		if p.X != 20 {
			t.Error("Position not updated")
		}
		v, _ := lazyecs.GetComponent[Velocity](world, e)
		if v.VX != 10 {
			t.Error("Velocity not updated")
		}
	}
}

// go test -run ^TestBatchEntityCreation$ . -count 1
func TestBatchEntityCreation(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	batch := lazyecs.CreateBatch[Position](world)

	ok := batch.CreateEntities(5)
	if !ok {
		t.Fatalf("Expected true, got %v", ok)
	}

	query := lazyecs.CreateQuery[Position](world)
	count := 0
	for query.Next() {
		count++
	}

	if count != 5 {
		t.Fatalf("Expected 5 entities from nil slice, got %d", count)
	}
}

// go test -run ^TestBatchEntityCreationWithComponents$ . -count 1
func TestBatchEntityCreationWithComponents(t *testing.T) {
	world, _, _, _ := setupWorld(t)
	batch := lazyecs.CreateBatch[Position](world)

	ok := batch.CreateEntitiesWithComponents(3, Position{X: 10, Y: 20})
	if !ok {
		t.Fatalf("Expected true, got %v", ok)
	}

	query := lazyecs.CreateQuery[Position](world)
	count := 0
	for query.Next() {
		count++
		p, ok := lazyecs.GetComponent[Position](world, query.Entity())
		if !ok {
			t.Fatal("Failed to get components from batch-created entity")
		}
		if p.X != 10 || p.Y != 20 {
			t.Errorf("Position data incorrect. Got %+v", p)
		}
	}

	if count != 3 {
		t.Fatalf("Expected 3 entities, got %d", count)
	}
}

const numEntities = 100000
const initialCapacity = 100000

// go test -benchmem -run=^$ -bench ^BenchmarkAddComponent$ . -count 1
func BenchmarkAddComponent(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()

	entities := world.CreateEntities(numEntities)

	for b.Loop() {
		for _, e := range entities {
			lazyecs.AddComponent[Position](world, e)
		}
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkSetComponent$ . -count 1
func BenchmarkSetComponent(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()

	entities := world.CreateEntities(numEntities)

	for b.Loop() {
		for _, e := range entities {
			lazyecs.SetComponent(world, e, Position{X: 10, Y: 10})
		}
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkRemoveComponent$ . -count 1
func BenchmarkRemoveComponent(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()

	entities := world.CreateEntities(numEntities)
	for _, e := range entities {
		lazyecs.AddComponent[Position](world, e)
	}

	for b.Loop() {
		for _, e := range entities {
			lazyecs.RemoveComponent[Position](world, e)
		}
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkAddComponent2$ . -count 1
func BenchmarkAddComponent2(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	lazyecs.RegisterComponent[Velocity]()

	entities := world.CreateEntities(numEntities)
	for b.Loop() {
		for _, e := range entities {
			lazyecs.AddComponent2[Position, Velocity](world, e)
		}
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkSetComponent2$ . -count 1
func BenchmarkSetComponent2(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	lazyecs.RegisterComponent[Velocity]()

	entities := world.CreateEntities(numEntities)

	for b.Loop() {
		for _, e := range entities {
			lazyecs.SetComponent2(world, e, Position{X: 10}, Velocity{VX: 5})
		}
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkRemoveComponent2$ . -count 1
func BenchmarkRemoveComponent2(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	lazyecs.RegisterComponent[Velocity]()

	entities := world.CreateEntities(numEntities)
	for _, e := range entities {
		lazyecs.AddComponent2[Position, Velocity](world, e)
	}

	for b.Loop() {
		for _, e := range entities {
			lazyecs.RemoveComponent2[Position, Velocity](world, e)
		}
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkAddComponentBatch$ . -count 1
func BenchmarkAddComponentBatch(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()

	entities := world.CreateEntities(numEntities)

	for b.Loop() {
		lazyecs.AddComponentBatch[Position](world, entities)
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkSetComponentBatch$ . -count 1
func BenchmarkSetComponentBatch(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()

	entities := world.CreateEntities(numEntities)

	for b.Loop() {
		lazyecs.SetComponentBatch(world, entities, Position{X: 10})
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkRemoveComponentBatch$ . -count 1
func BenchmarkRemoveComponentBatch(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()

	entities := world.CreateEntities(numEntities)
	lazyecs.AddComponentBatch[Position](world, entities)

	for b.Loop() {
		lazyecs.RemoveComponentBatch[Position](world, entities)
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkAddComponentBatch2$ . -count 1
func BenchmarkAddComponentBatch2(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	lazyecs.RegisterComponent[Velocity]()

	entities := world.CreateEntities(numEntities)

	for b.Loop() {
		lazyecs.AddComponentBatch2[Position, Velocity](world, entities)
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkSetComponentBatch2$ . -count 1
func BenchmarkSetComponentBatch2(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	lazyecs.RegisterComponent[Velocity]()

	entities := world.CreateEntities(numEntities)

	for b.Loop() {
		lazyecs.SetComponentBatch2(world, entities, Position{X: 10}, Velocity{VX: 5})
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkRemoveComponentBatch2$ . -count 1
func BenchmarkRemoveComponentBatch2(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	lazyecs.RegisterComponent[Velocity]()

	entities := world.CreateEntities(numEntities)
	lazyecs.AddComponentBatch2[Position, Velocity](world, entities)

	for b.Loop() {
		lazyecs.RemoveComponentBatch2[Position, Velocity](world, entities)
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkAddEntities$ . -count 1
func BenchmarkAddEntities(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	for b.Loop() {
		world.CreateEntities(numEntities)
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkRemoveEntities$ . -count 1
func BenchmarkRemoveEntities(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	entities := world.CreateEntities(numEntities)
	for b.Loop() {
		for _, e := range entities {
			world.RemoveEntity(e)
		}
		world.ProcessRemovals()
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkBatchCreationTo_Preallocated$ . -count 1
func BenchmarkBatchCreationTo_Preallocated(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: numEntities,
	})
	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	batch := lazyecs.CreateBatch[Position](world)
	dst := make([]lazyecs.Entity, 0, numEntities)

	for b.Loop() {
		// In a real scenario, the slice would be cleared, not re-made.
		dst = dst[:0]
		_ = batch.CreateEntities(numEntities)
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkBatchCreationWithComponentsTo_Preallocated$ . -count 1
func BenchmarkBatchCreationWithComponentsTo_Preallocated(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: numEntities,
	})
	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	batch := lazyecs.CreateBatch[Position](world)
	dst := make([]lazyecs.Entity, 0, numEntities)
	pos := Position{X: 1, Y: 2}

	for b.Loop() {
		dst = dst[:0]
		_ = batch.CreateEntitiesWithComponents(numEntities, pos)
	}
}

// go test -benchmem -run=^$ -bench ^BenchmarkQuery$ . -count 1
func BenchmarkQuery(b *testing.B) {
	world := lazyecs.NewWorldWithOptions(lazyecs.WorldOptions{
		InitialCapacity: initialCapacity,
	})

	lazyecs.ResetGlobalRegistry()
	lazyecs.RegisterComponent[Position]()
	lazyecs.RegisterComponent[Velocity]()

	entities := world.CreateEntities(numEntities)
	lazyecs.AddComponentBatch2[Position, Velocity](world, entities)

	query := lazyecs.CreateQuery[Position](world)

	for b.Loop() {
		query.Reset()
		for query.Next() {
			pos := query.Get()
			pos.X += 1
		}
	}
}
