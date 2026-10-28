package questengine

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// Benchmarks for ResolveDestination (#375 phase 4):
//
//	go test ./internal/questengine/ -run x -bench ResolveDestination -count=5
//
// Mob30: a mob destination with 30 live copies over 30 rooms, half of them
// hidden; the sight check is the real messaging.ParticipantSight against a
// bright room. Area40: a 40-room area in the player's zone. In both,
// nearestOf returns at once, so the cost measured is the resolver's own
// (the map search has its own benchmarks in internal/mapper).

// benchBrightRoom is a lit room for ParticipantSight.
type benchBrightRoom struct{}

func (benchBrightRoom) LightLevel() int { return 70 }

// installDestBench stubs the world seams for a benchmark: every room in zone
// "z", the given copies, real sight in a bright room, and a nearestOf that
// answers at once with the first room past from.
func installDestBench(b *testing.B, copies map[int][]liveCopy) {
	b.Helper()
	pz, pc, ps, psp, pn, pnear, ptpl := roomZone, liveCopies, sightIn, spawnRoomsOf, npcName, nearestOf, templateRoom
	roomZone = func(int) string { return "z" }
	liveCopies = func(mobId int) []liveCopy { return copies[mobId] }
	sightIn = func(viewer *characters.Character, _ int) messaging.SightDecision {
		return messaging.ParticipantSight(viewer, benchBrightRoom{})
	}
	spawnRoomsOf = func(int) []int { return nil }
	npcName = func(int) string { return "a bench mob" }
	templateRoom = func(id int) int { return id }
	nearestOf = func(from int, _ func(int) bool, _ []string, _ mapper.SecretExitKnown) (int, int, string, bool) {
		return from + 1, from + 1, "north", true
	}
	b.Cleanup(func() {
		roomZone, liveCopies, sightIn, spawnRoomsOf, npcName, nearestOf, templateRoom = pz, pc, ps, psp, pn, pnear, ptpl
	})
}

// benchCopy is destCopy for a benchmark.
func benchCopy(b *testing.B, roomId int, hidden bool) liveCopy {
	b.Helper()
	c := characters.New()
	c.Awareness = awareness.NewMachine()
	if hidden {
		r := state.TransitionReason{Trigger: "destination_bench"}
		if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r); err != nil {
			b.Fatal(err)
		}
		c.Awareness.ResolveConcealment(true, r)
	}
	return liveCopy{roomId: roomId, char: c}
}

func BenchmarkResolveDestinationMob30(b *testing.B) {
	const mobId = 900
	var copies []liveCopy
	for i := 0; i < 30; i++ {
		copies = append(copies, benchCopy(b, 101+i, i%2 == 1))
	}
	installDestBench(b, map[int][]liveCopy{mobId: copies})
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Mobs: []int{mobId}}}})
	u := destUser(1)
	if d := e.ResolveDestination(u, 700, "start"); d.Kind != DestMob || len(d.ZoneRooms) != 15 {
		b.Fatalf("want a mob destination over 15 perceived rooms: %+v", d)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ResolveDestination(u, 700, "start")
	}
}

func BenchmarkResolveDestinationArea40(b *testing.B) {
	installDestBench(b, nil)
	zones := map[int]string{1: "z"}
	var ids []int
	for id := 101; id <= 140; id++ {
		ids = append(ids, id)
		zones[id] = "z"
	}
	b.Cleanup(areas.InstallForTest([]areas.Area{{Key: "bench", Name: "the bench", Rooms: ids}}, zones))
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Area: "bench"}}})
	u := destUser(1)
	if d := e.ResolveDestination(u, 700, "start"); d.Kind != DestArea || len(d.ZoneRooms) != 40 {
		b.Fatalf("want an area destination washing 40 rooms: %+v", d)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ResolveDestination(u, 700, "start")
	}
}
