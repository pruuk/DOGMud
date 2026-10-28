package questengine

import (
	"reflect"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// destWorld stubs every seam ResolveDestination reads: room zones, live
// copies, the sight in each room, spawn rooms, names and the nearest search.
type destWorld struct {
	zones  map[int]string
	copies map[int][]liveCopy
	dark   map[int]messaging.SightDecision // room -> sight; absent = full
	spawns map[int][]int
	names  map[int]string
	near   func(from int, in func(int) bool, zones []string) (int, int, string, bool)
}

func installDestWorld(t *testing.T, w destWorld) {
	t.Helper()
	pz, pc, ps, psp, pn, pnear, ptpl := roomZone, liveCopies, sightIn, spawnRoomsOf, npcName, nearestOf, templateRoom
	roomZone = func(id int) string { return w.zones[id] }
	liveCopies = func(mobId int) []liveCopy { return w.copies[mobId] }
	sightIn = func(_ *characters.Character, roomId int) messaging.SightDecision {
		if s, ok := w.dark[roomId]; ok {
			return s
		}
		return messaging.SightFull
	}
	spawnRoomsOf = func(mobId int) []int { return w.spawns[mobId] }
	npcName = func(mobId int) string { return w.names[mobId] }
	templateRoom = func(id int) int { return id }
	nearestOf = w.near
	if nearestOf == nil {
		nearestOf = func(from int, in func(int) bool, zones []string) (int, int, string, bool) {
			for id := 1; id < 100000; id++ { // the lowest member, one hop "north"
				if id != from && in(id) {
					return id, id, "north", true
				}
			}
			return 0, 0, "", false
		}
	}
	t.Cleanup(func() {
		roomZone, liveCopies, sightIn, spawnRoomsOf, npcName, nearestOf, templateRoom = pz, pc, ps, psp, pn, pnear, ptpl
	})
}

func destCopy(t *testing.T, roomId int, hidden bool) liveCopy {
	t.Helper()
	c := characters.New()
	c.Awareness = awareness.NewMachine()
	if hidden {
		r := state.TransitionReason{Trigger: "destination_test"}
		if err := c.Awareness.TransitionToConcealing(awareness.ConcealingData{}, r); err != nil {
			t.Fatal(err)
		}
		c.Awareness.ResolveConcealment(true, r)
	}
	return liveCopy{roomId: roomId, char: c}
}

func destEngine(steps []quests.QuestStep, triggers ...quests.TriggerDef) *Engine {
	e := NewEngine()
	e.RegisterQuest(&QuestDef{QuestId: 700, Name: "Dest", Steps: append(steps, QuestStep{Id: "end"}), Triggers: triggers})
	return e
}

func destUser(roomId int) *users.UserRecord {
	u := users.NewTestUser(97001, "dest", "Dest", 97001)
	u.Character.RoomId = roomId
	return u
}

func gated(step string, t quests.TriggerDef) quests.TriggerDef {
	t.Conditions.Has = append(t.Conditions.Has, "700-"+step)
	t.Actions = []quests.ActionDef{{Grant: "700-end"}}
	return t
}

func TestResolveDestination_FinalStepAndUnknownQuest(t *testing.T) {
	installDestWorld(t, destWorld{})
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Room: 5}}})
	if d := e.ResolveDestination(destUser(1), 700, "end"); d.Kind != DestNone {
		t.Errorf("final step: %+v", d)
	}
	if d := e.ResolveDestination(destUser(1), 999, "start"); d.Kind != DestNone {
		t.Errorf("unknown quest: %+v", d)
	}
}

func TestResolveDestination_Anywhere(t *testing.T) {
	installDestWorld(t, destWorld{})
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Anywhere: true}}},
		gated("start", quests.TriggerDef{Event: "room_enter", Room: 5}))
	if d := e.ResolveDestination(destUser(1), 700, "start"); d.Kind != DestAnywhere || d.Pin != 0 {
		t.Errorf("anywhere must win over inference and mark nothing: %+v", d)
	}
}

func TestResolveDestination_Room(t *testing.T) {
	installDestWorld(t, destWorld{zones: map[int]string{1: "z", 5: "z"}})
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Room: 5}}})
	d := e.ResolveDestination(destUser(1), 700, "start")
	if d.Kind != DestRoom || d.Pin != 5 || d.NextDir != "north" || d.Inside || d.ZoneRooms != nil {
		t.Errorf("room: %+v", d)
	}
	if d := e.ResolveDestination(destUser(5), 700, "start"); !d.Inside || d.Pin != 0 || d.NextDir != "" {
		t.Errorf("standing in the room: %+v", d)
	}
}

func TestResolveDestination_AreaAndZone(t *testing.T) {
	zones := map[int]string{1: "town", 20: "grove", 21: "grove", 22: "grove", 30: "moor"}
	installDestWorld(t, destWorld{zones: zones})
	t.Cleanup(areas.InstallForTest([]areas.Area{{Key: "g", Name: "the grove", Rooms: []int{20, 21, 30}}}, zones))

	e := destEngine([]QuestStep{
		{Id: "start", Destination: quests.StepDestination{Area: "g"}},
		{Id: "zone", Destination: quests.StepDestination{Zone: "grove"}},
	})
	d := e.ResolveDestination(destUser(21), 700, "start")
	if d.Kind != DestArea || d.Label != "the grove" || !d.Inside || !reflect.DeepEqual(d.ZoneRooms, []int{20, 21}) {
		t.Errorf("inside the area: %+v", d)
	}
	// The stub nearestOf returns the lowest member wherever it lies, so Pin 20
	// checks membership routing only: the real NearestOf gives Pin 0 across
	// zones (the room is not on the drawn map) and only the direction.
	d = e.ResolveDestination(destUser(1), 700, "start")
	if d.Inside || d.Pin != 20 || d.ZoneRooms != nil {
		t.Errorf("outside the area (town has none of it): %+v", d)
	}
	d = e.ResolveDestination(destUser(22), 700, "zone")
	if d.Kind != DestZone || d.Label != "grove" || !d.Inside || d.ZoneRooms != nil {
		t.Errorf("inside the zone: %+v", d)
	}
}

func TestResolveDestination_NearestPerceivedCopy(t *testing.T) {
	w := destWorld{
		zones:  map[int]string{1: "z", 4: "z", 6: "z", 8: "z", 9: "z"},
		copies: map[int][]liveCopy{},
		dark:   map[int]messaging.SightDecision{6: messaging.SightNone, 8: messaging.SightShapes},
		names:  map[int]string{50: "a wolf"},
	}
	w.copies[50] = []liveCopy{destCopy(t, 4, true), destCopy(t, 6, false), destCopy(t, 8, false), destCopy(t, 9, false)}
	installDestWorld(t, w)
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Mob: 50}}})
	d := e.ResolveDestination(destUser(1), 700, "start")
	// 4 is hidden, 6 is dark, 8 is shapes only: only 9 counts.
	if d.Kind != DestMob || d.Pin != 9 || d.Npc != "a wolf" || d.Fallback {
		t.Errorf("want the one perceived copy in 9: %+v", d)
	}
}

func TestResolveDestination_MobFallsBack(t *testing.T) {
	zones := map[int]string{1: "z", 4: "z", 7: "z", 12: "z", 13: "z"}
	w := destWorld{zones: zones, copies: map[int][]liveCopy{}, names: map[int]string{50: "Velk"},
		spawns: map[int][]int{50: {12, 13}}}
	w.copies[50] = []liveCopy{destCopy(t, 4, true)} // one copy, hidden
	installDestWorld(t, w)

	e := destEngine([]QuestStep{
		{Id: "start", Destination: quests.StepDestination{Mob: 50, Room: 7}},
		{Id: "spawn", Destination: quests.StepDestination{Mob: 50}},
		{Id: "none", Destination: quests.StepDestination{Mob: 51}},
	})
	d := e.ResolveDestination(destUser(1), 700, "start")
	if d.Kind != DestRoom || d.Pin != 7 || d.Npc != "Velk" || !d.Fallback {
		t.Errorf("a hidden single NPC falls back to its room: %+v", d)
	}
	d = e.ResolveDestination(destUser(1), 700, "spawn")
	if d.Kind != DestArea || d.Pin != 12 || !d.Fallback || !reflect.DeepEqual(d.ZoneRooms, []int{12, 13}) {
		t.Errorf("no declared fallback: the spawn rooms: %+v", d)
	}
	if d := e.ResolveDestination(destUser(1), 700, "none"); d.Kind != DestUnknown {
		t.Errorf("no copy, no fallback, no spawns: %+v", d)
	}
}

func TestResolveDestination_Inference(t *testing.T) {
	zones := map[int]string{1: "z", 5: "z", 6: "z", 9: "z"}
	installDestWorld(t, destWorld{zones: zones, spawns: map[int][]int{60: {9}}})
	e := destEngine([]QuestStep{{Id: "one"}, {Id: "mixed"}, {Id: "bare"}, {Id: "order"}},
		gated("order", quests.TriggerDef{Event: "room_enter", Room: 6}),
		gated("order", quests.TriggerDef{Event: "command", Conditions: quests.Conditions{InRoom: 5}}),
		gated("one", quests.TriggerDef{Event: "room_enter", Room: 5}),
		gated("one", quests.TriggerDef{Event: "command", Conditions: quests.Conditions{InRoom: 6}}),
		gated("mixed", quests.TriggerDef{Event: "room_enter", Room: 5}),
		gated("mixed", quests.TriggerDef{Event: "mob_death", Mob: 60}),
		gated("bare", quests.TriggerDef{Event: "command", Command: "sleep"}),
	)
	d := e.ResolveDestination(destUser(1), 700, "one")
	if d.Kind != DestRoom || !d.Inferred || d.Pin != 5 || !reflect.DeepEqual(d.ZoneRooms, []int{5, 6}) {
		t.Errorf("two rooms infer one room set: %+v", d)
	}
	d = e.ResolveDestination(destUser(1), 700, "mixed")
	if d.Kind != DestRoom || d.Label != "" || d.Pin != 5 {
		t.Errorf("a room and a mob's spawn rooms: mixed kinds give a room set: %+v", d)
	}
	if d := e.ResolveDestination(destUser(1), 700, "bare"); d.Kind != DestUnknown || !d.Inferred {
		t.Errorf("no place anywhere: %+v", d)
	}
	if d := e.ResolveDestination(destUser(1), 700, "order"); !reflect.DeepEqual(d.ZoneRooms, []int{5, 6}) || d.Pin != 5 {
		t.Errorf("rooms named out of order still wash sorted: %+v", d)
	}
}

// A trigger whose has names two steps belongs to the later one only
// (Quest.CountStep): it cannot fire before the player holds both tokens.
// Quest 19's topic_heard on mob 335 has 19-shrimp and 19-hunter.
func TestResolveDestination_TriggerBelongsToItsLatestStep(t *testing.T) {
	installDestWorld(t, destWorld{zones: map[int]string{1: "z", 5: "z", 6: "z"}})
	both := quests.TriggerDef{Event: "room_enter", Room: 5,
		Conditions: quests.Conditions{Has: []string{"700-a", "700-b"}}, Actions: []quests.ActionDef{{Grant: "700-end"}}}
	e := destEngine([]QuestStep{{Id: "a"}, {Id: "b"}},
		both,
		gated("a", quests.TriggerDef{Event: "room_enter", Room: 6}),
	)
	if d := e.ResolveDestination(destUser(1), 700, "a"); d.Pin != 6 || d.ZoneRooms != nil {
		t.Errorf("step a owns only its own trigger (room 6): %+v", d)
	}
	if d := e.ResolveDestination(destUser(1), 700, "b"); d.Pin != 5 {
		t.Errorf("step b owns the trigger gated on a and b (room 5): %+v", d)
	}
}

// Instance rooms map to their template rooms: a copy's room, the player's
// room and every id NearestOf hands back or tests through in.
func TestResolveDestination_InstanceRooms(t *testing.T) {
	const inst5, inst9 = 1000000005, 1000000009
	w := destWorld{zones: map[int]string{1: "z", 5: "z", 9: "z"}, copies: map[int][]liveCopy{},
		names: map[int]string{50: "a wolf"}}
	w.copies[50] = []liveCopy{destCopy(t, inst9, false)}
	installDestWorld(t, w)
	templateRoom = func(id int) int {
		switch id {
		case inst5:
			return 5
		case inst9:
			return 9
		}
		return id
	}

	e := destEngine([]QuestStep{
		{Id: "start", Destination: quests.StepDestination{Mob: 50}},
		{Id: "room", Destination: quests.StepDestination{Room: 5}},
	})
	d := e.ResolveDestination(destUser(1), 700, "start")
	if d.Kind != DestMob || d.Pin != 9 || d.Fallback {
		t.Errorf("a copy in an instance room counts by its template room: %+v", d)
	}
	if d := e.ResolveDestination(destUser(inst5), 700, "room"); !d.Inside || d.Pin != 0 {
		t.Errorf("a player in an instance of the room is inside it: %+v", d)
	}

	var sawIn bool
	nearestOf = func(from int, in func(int) bool, zones []string) (int, int, string, bool) {
		sawIn = in(inst9)
		return inst9, inst9, "east", sawIn
	}
	d = e.ResolveDestination(destUser(1), 700, "start")
	if !sawIn || d.Pin != 9 || d.NextRoom != 9 || d.NextDir != "east" {
		t.Errorf("in maps instance ids, and Pin and NextRoom are template ids: sawIn=%v %+v", sawIn, d)
	}
}

func TestResolveDestination_MobDeclaredAreaAndZoneFallback(t *testing.T) {
	zones := map[int]string{1: "town", 4: "grove", 20: "grove", 21: "grove"}
	w := destWorld{zones: zones, copies: map[int][]liveCopy{}, names: map[int]string{50: "Velk"}}
	w.copies[50] = []liveCopy{destCopy(t, 4, true)} // hidden: not perceived
	installDestWorld(t, w)
	t.Cleanup(areas.InstallForTest([]areas.Area{{Key: "g", Name: "the grove", Rooms: []int{20, 21}}}, zones))

	e := destEngine([]QuestStep{
		{Id: "area", Destination: quests.StepDestination{Mob: 50, Area: "g"}},
		{Id: "zone", Destination: quests.StepDestination{Mob: 50, Zone: "grove"}},
	})
	d := e.ResolveDestination(destUser(1), 700, "area")
	if d.Kind != DestArea || d.Label != "the grove" || d.Npc != "Velk" || !d.Fallback {
		t.Errorf("declared area fallback: %+v", d)
	}
	d = e.ResolveDestination(destUser(1), 700, "zone")
	if d.Kind != DestZone || d.Label != "grove" || d.Npc != "Velk" || !d.Fallback {
		t.Errorf("declared zone fallback: %+v", d)
	}
}

func TestResolveDestination_NearestOfSeveralCopies(t *testing.T) {
	w := destWorld{zones: map[int]string{1: "z", 4: "z", 9: "z"}, copies: map[int][]liveCopy{},
		names: map[int]string{50: "a wolf"}}
	w.copies[50] = []liveCopy{destCopy(t, 9, false), destCopy(t, 4, false)}
	installDestWorld(t, w) // the stub nearestOf picks the lowest member by in
	e := destEngine([]QuestStep{{Id: "start", Destination: quests.StepDestination{Mob: 50}}})
	d := e.ResolveDestination(destUser(1), 700, "start")
	if d.Kind != DestMob || d.Pin != 4 || !reflect.DeepEqual(d.ZoneRooms, []int{4, 9}) {
		t.Errorf("both copies count and the nearest is pinned: %+v", d)
	}
}
