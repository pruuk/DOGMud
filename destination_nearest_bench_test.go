package main

import (
	"math/rand"
	"os"
	"sort"
	"sync"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Benchmarks for the quest destination search on shipped zones (#375 phase
// 4), against master's per-move NextStep:
//
//	DOGMUD_BOOT_SMOKE=1 go test . -run x -bench DestinationNearest -count=5
//
// Each case cycles through a fixed seeded set of 200 (from, target) room
// pairs inside one zone. NextStep/cold uses those 200 unique pairs, which is
// more than the 128-entry path cache holds, so every call misses it (a moving
// player changes "from" every move). NextStep/warm repeats one pair.
// NearestOf/single is a one-room set; NearestOf/set40 is 40 rooms of the zone.
// The cases are the table destBenchCases, so a new resolver is one more row.
//
// Every zone's map crawls the whole connected world (getMapNode does not stop
// at a zone border), so the zone choice changes where the pairs lie, not how
// big the graph is.

const destBenchPairs = 200

var (
	destBenchOnce sync.Once
	destBenchErr  error
)

// loadShippedWorldTB is loadShippedQuestWorld for a benchmark: it loads the
// shipped data once, skipping unless DOGMUD_BOOT_SMOKE is set.
func loadShippedWorldTB(tb testing.TB) {
	tb.Helper()
	if os.Getenv(bootSmokeEnvVar) == `` {
		tb.Skipf("set %s=1 to run the shipped-data benchmarks", bootSmokeEnvVar)
	}
	destBenchOnce.Do(func() {
		mudlog.SetupLogger(nil, `LOW`, ``, false)
		if err := configs.ReloadConfig(); err != nil {
			destBenchErr = err
			return
		}
		func() {
			defer func() { _ = recover() }() // boot failures are the boot test's job
			loadAllDataFiles(false)
		}()
	})
	if destBenchErr != nil {
		tb.Fatalf("ReloadConfig failed: %v", destBenchErr)
	}
	if len(rooms.GetAllRoomIds()) < 500 {
		tb.Fatalf("only %d rooms loaded", len(rooms.GetAllRoomIds()))
	}
}

// zoneMapRooms is the sorted ids of the zone's rooms its root's map crawls.
func zoneMapRooms(zone string) []int {
	root, err := rooms.GetZoneRoot(zone)
	if err != nil {
		return nil
	}
	m := mapper.GetMapper(root)
	if m == nil {
		return nil
	}
	var ids []int
	for _, id := range m.CrawledRoomIds() {
		if r := rooms.LoadRoom(id); r != nil && r.Zone == zone {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

// largestZone is the zone whose root map crawls the most rooms of it.
func largestZone() (string, []int) {
	best, bestIds := ``, []int(nil)
	names := rooms.GetAllZoneNames()
	sort.Strings(names)
	for _, z := range names {
		if ids := zoneMapRooms(z); len(ids) > len(bestIds) {
			best, bestIds = z, ids
		}
	}
	return best, bestIds
}

// destBenchTown is the typical town zone measured beside the largest one.
const destBenchTown = `Greenford`

type destPair struct{ from, to int }

func destPairs(ids []int, seed int64) []destPair {
	rng := rand.New(rand.NewSource(seed))
	seen := map[destPair]bool{}
	var out []destPair
	for len(out) < destBenchPairs {
		p := destPair{ids[rng.Intn(len(ids))], ids[rng.Intn(len(ids))]}
		if p.from == p.to || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func destSet(ids []int, n int, seed int64) map[int]bool {
	rng := rand.New(rand.NewSource(seed))
	s := map[int]bool{}
	for len(s) < n && len(s) < len(ids) {
		s[ids[rng.Intn(len(ids))]] = true
	}
	return s
}

func BenchmarkDestinationNearest(b *testing.B) {
	loadShippedWorldTB(b)
	big, bigIds := largestZone()
	town := zoneMapRooms(destBenchTown)
	if len(town) == 0 {
		b.Fatalf("zone %s has no mapped rooms", destBenchTown)
	}
	b.Logf("largest zone %s: %d rooms (its map crawls %d); town %s: %d rooms (its map crawls %d)",
		big, len(bigIds), len(mapper.GetMapper(bigIds[0]).CrawledRoomIds()),
		destBenchTown, len(town), len(mapper.GetMapper(town[0]).CrawledRoomIds()))

	for _, zc := range []struct {
		name string
		zone string
		ids  []int
	}{{`large`, big, bigIds}, {`town`, destBenchTown, town}} {
		z := destBenchZone{
			zone:  zc.zone,
			zones: []string{zc.zone},
			pairs: destPairs(zc.ids, 375),
			set40: destSet(zc.ids, 40, 4),
		}
		for _, c := range destBenchCases {
			b.Run(zc.name+`/`+c.name, func(b *testing.B) {
				c.setup(b, z)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					c.step(z, i)
				}
			})
		}
	}
}

// destBenchZone is one measured zone: its name (alone, as the zones list a
// set touching only it passes), the 200 seeded pairs and the seeded 40-room
// set.
type destBenchZone struct {
	zone  string
	zones []string
	pairs []destPair
	set40 map[int]bool
}

// destBenchCase is one way of answering "which way from here": setup runs
// untimed before the loop (nil for none), step is iteration i. A new
// resolver (a direction table, say) is measured by adding cases here.
type destBenchCase struct {
	name  string
	setup func(b *testing.B, z destBenchZone)
	step  func(z destBenchZone, i int)
}

var destBenchCases = []destBenchCase{
	{
		// master's per-move call; 200 unique pairs overflow the 128-entry
		// path cache, so every call misses it.
		name:  `NextStep/cold`,
		setup: func(*testing.B, destBenchZone) {},
		step: func(z destBenchZone, i int) {
			p := z.pairs[i%len(z.pairs)]
			mapper.NextStep(p.from, p.to)
		},
	},
	{
		name: `NextStep/warm`,
		setup: func(_ *testing.B, z destBenchZone) {
			mapper.NextStep(z.pairs[0].from, z.pairs[0].to)
		},
		step: func(z destBenchZone, _ int) {
			mapper.NextStep(z.pairs[0].from, z.pairs[0].to)
		},
	},
	{
		name:  `NearestOf/single`,
		setup: func(*testing.B, destBenchZone) {},
		step: func(z destBenchZone, i int) {
			p := z.pairs[i%len(z.pairs)]
			mapper.NearestOf(p.from, func(id int) bool { return id == p.to }, z.zones)
		},
	},
	{
		name:  `NearestOf/set40`,
		setup: func(*testing.B, destBenchZone) {},
		step: func(z destBenchZone, i int) {
			mapper.NearestOf(z.pairs[i%len(z.pairs)].from, func(id int) bool { return z.set40[id] }, z.zones)
		},
	},
}
