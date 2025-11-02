// Profiling:
// go build ./profile/entities
// go tool pprof -http=":8000" -nodefraction=0.001 ./query mem.pprof

package main

import (
	"github.com/edwinsyarief/teishoku"
	"github.com/pkg/profile"
)

type comp1 struct {
	V int64
	W int64
}

type comp2 struct {
	V int64
	W int64
}

func main() {
	count := 50
	iters := 10000
	entities := 1000
	p := profile.Start(profile.MemProfileAllocs, profile.ProfilePath("."), profile.NoShutdownHook)
	run(count, iters, entities)
	p.Stop()
}

func run(rounds, iters, numEntities int) {
	for range rounds {
		w := teishoku.NewWorld(numEntities)
		query := teishoku.NewFilter2[comp1, comp2](w)
		batch := teishoku.NewBuilder2[comp1, comp2](w)

		for range iters {
			batch.NewEntities(numEntities)
			entities := []teishoku.Entity{}
			query.Reset()
			for query.Next() {
				entities = append(entities, query.Entity())
				comp1, comp2 := query.Get()
				comp1.V += comp2.V
				comp1.W += comp2.W
			}
			for _, e := range entities {
				w.RemoveEntity(e)
			}
		}
	}
}
