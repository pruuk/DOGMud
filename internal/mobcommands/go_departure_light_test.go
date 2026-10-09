package mobcommands

import (
	"strconv"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const mobGoLightCond = 9783 // a carried light, literal strength 60

// #456, owner ruling 2026-10-09: a mover is seen by the light they carry on
// their own way out. A forced `go <roomId>` (callforhelp) by a mob carrying
// the only light in a cave: the watcher left in the dark saw it run off by
// that light, so reads its name rather than the footsteps.
func TestMobGo_ForcedMove_DepartureIsJudgedByTheMoversOwnLight(t *testing.T) {
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const fromRoom, toRoom, instId, watcherId = 9620, 9621, 98620, 9630
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		fromRoom: {RoomId: fromRoom, Zone: "test", Biome: "cave"},
		toRoom:   {RoomId: toRoom, Zone: "test", Biome: "cave"},
	}, map[string]*rooms.ZoneConfig{}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		mobGoLightCond: {ConditionId: mobGoLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	w := users.NewTestUser(watcherId, "watcher", "Watcher", 0)
	w.Character.RoomId = fromRoom
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{watcherId: w}))

	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Caller"
	m.Character.RoomId = fromRoom
	if !m.Character.Conditions.AddCondition(mobGoLightCond, false) {
		t.Fatal("fixture: the mob must carry the light")
	}
	mobs.SetInstanceForTest(instId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })

	from := rooms.LoadRoom(fromRoom)
	from.AddPlayer(watcherId)
	from.AddMob(instId)
	if got := from.ParticipantSight(watcherId); got != messaging.SightFull {
		t.Fatalf("fixture: the mob's light must light the room, sight %d", got)
	}
	events.DrainQueuedMessagesForTest(watcherId)

	if handled, err := Go(strconv.Itoa(toRoom), m, from); err != nil || !handled {
		t.Fatalf("Go: handled %v, err %v", handled, err)
	}

	if got := from.ParticipantSight(watcherId); got != messaging.SightNone {
		t.Fatalf("fixture: the light must leave with the mob, sight %d", got)
	}
	got := strings.Join(events.DrainQueuedMessagesForTest(watcherId), "\n")
	if !strings.Contains(got, "runs off suddenly") || strings.Contains(got, "footsteps") {
		t.Fatalf("the watcher must see the mob run off by its own light, got %q", got)
	}
}
