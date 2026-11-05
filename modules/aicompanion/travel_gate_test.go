package aicompanion

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// A companion's trip walks `go`, which a mob takes straight through a room_gate,
// so an errand could carry her into a room its owner cannot enter and let her
// read who is there. Her routes and explores stay out of a guarded room. Her
// owner walking in still brings her along: the follow does not plan a route.

func gateCompanionRoom(t *testing.T, guarded int) {
	t.Helper()
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: 999651,
		Name:    "Companion Gate",
		Steps:   []questengine.QuestStep{{Id: "start"}},
		Triggers: []questengine.TriggerDef{{
			Event:      "room_gate",
			Room:       guarded,
			Conditions: questengine.Conditions{Missing: []string{"999651-start"}},
			Actions:    []questengine.ActionDef{{SendText: "You have no reason to go there."}},
		}},
	})
}

func TestFindPath_NeverEntersAGuardedRoom(t *testing.T) {
	gateCompanionRoom(t, 2)
	mp := testMap()

	path, ok := findPath(mp, 1, 3, 10)
	if !ok || len(path) != 3 || path[1].To != 5 {
		t.Fatalf("the lane is guarded, so the route should go through the bog: %+v %v", path, ok)
	}
	if _, ok := findPath(mp, 1, 2, 10); ok {
		t.Fatal("a guarded room is no destination")
	}
	// Inside already, she can still walk out.
	if _, ok := findPath(mp, 2, 3, 10); !ok {
		t.Fatal("the way out of a guarded room stays open")
	}
}

func TestStartExplore_RefusesAGuardedRoom(t *testing.T) {
	gateCompanionRoom(t, 9702)
	here := &rooms.Room{RoomId: 9701, Exits: map[string]exit.RoomExit{
		`up`:   {RoomId: 9702},
		`down`: {RoomId: 9703},
	}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9701: here}, nil))

	m := &AICompanionModule{}
	mob := &mobs.Mob{Character: characters.Character{RoomId: 9701}}
	c := &controller{mind: &Mind{Map: map[int]*RoomRecord{
		9701: {Exits: map[string]*ExitRecord{`up`: {}, `down`: {}}},
	}}}

	if reason := m.startExplore(c, mob, `up`, false); reason == `` || c.travel != nil {
		t.Fatalf("explored into a guarded room: reason %q, travel %+v", reason, c.travel)
	}
	if reason := m.startExplore(c, mob, `down`, false); reason != `` || c.travel == nil {
		t.Fatalf("the open way was refused: %q", reason)
	}
}
