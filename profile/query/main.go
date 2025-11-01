// Profiling:
// go build ./profile/query
// go tool pprof -http=":8000" -nodefraction=0.001 ./query mem.pprof

package main

import (
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/edwinsyarief/teishoku"
)

type comp1 struct {
	V int64
	W int64
}

type comp2 struct {
	V int64
	W int64
}

type comp3 struct {
	V int64
	W int64
}

type comp4 struct {
	V int64
	W int64
}

type comp5 struct {
	V int64
	W int64
}

type comp6 struct {
	V int64
	W int64
}

func main() {
	// CPU Profiling
	f, _ := os.Create("cpu.prof")
	_ = pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	count := 50
	iters := 10000
	entities := 100000
	//p := profile.Start(profile.MemProfileAllocs, profile.ProfilePath("."), profile.NoShutdownHook)
	run(count, iters, entities)
	//p.Stop()

	// Memory Profiling
	memFile, _ := os.Create("mem.prof")
	defer memFile.Close()
	runtime.GC() // Trigger garbage collection
	_ = pprof.WriteHeapProfile(memFile)
}

func run(rounds, iters, numEntities int) {
	for range rounds {
		w := teishoku.NewWorld(numEntities)
		query := teishoku.NewFilter6[comp1, comp2, comp3, comp4, comp5, comp6](&w)
		batch := teishoku.NewBuilder6[comp1, comp2, comp3, comp4, comp5, comp6](&w)
		batch.NewEntities(numEntities)

		for range iters {
			query.Reset()
			for query.Next() {
				comp1, comp2, _, _, _, _ := query.Get()
				comp1.V += comp2.V
				comp1.W += comp2.W
			}
		}
	}
}
