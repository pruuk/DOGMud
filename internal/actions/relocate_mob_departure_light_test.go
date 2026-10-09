package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const relocateLightCond = 9782 // a carried light, literal strength 60

// #456, owner ruling 2026-10-09: a mover is seen by the light they carry on
// their own way out. The mob carries the only light in a cave and walks
// north; the watcher left in the dark saw it go by that light, so reads its
// name rather than the footsteps.
func TestRelocateMob_DepartureIsJudgedByTheMoversOwnLight(t *testing.T) {
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const from, to, instId, watcherId = 99441, 99442, 98441, 99451
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		from: {RoomId: from, Zone: "test", Biome: "cave", Exits: map[string]exit.RoomExit{"north": {RoomId: to}}},
		to:   {RoomId: to, Zone: "test", Biome: "cave", Exits: map[string]exit.RoomExit{"south": {RoomId: from}}},
	}, map[string]*rooms.ZoneConfig{}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave": {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0)},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		relocateLightCond: {ConditionId: relocateLightCond, Name: "Test Torchlight", Secret: true, TriggerCount: 1, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 60}}},
	}))
	w := users.NewTestUser(watcherId, "watcher", "Watcher", 0)
	w.Character.RoomId = from
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{watcherId: w}))

	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Lamplighter"
	m.Character.RoomId = from
	if !m.Character.Conditions.AddCondition(relocateLightCond, false) {
		t.Fatal("fixture: the mob must carry the light")
	}
	mobs.SetInstanceForTest(instId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })

	fromRoom, toRoom := rooms.LoadRoom(from), rooms.LoadRoom(to)
	fromRoom.AddPlayer(watcherId)
	fromRoom.AddMob(instId)
	if got := fromRoom.ParticipantSight(watcherId); got != messaging.SightFull {
		t.Fatalf("fixture: the mob's light must light the room, sight %d", got)
	}
	events.DrainQueuedMessagesForTest(watcherId)

	RelocateMob(m, fromRoom, "north", toRoom, false)

	if got := fromRoom.ParticipantSight(watcherId); got != messaging.SightNone {
		t.Fatalf("fixture: the light must leave with the mob, sight %d", got)
	}
	got := strings.Join(events.DrainQueuedMessagesForTest(watcherId), "\n")
	if !strings.Contains(got, "Lamplighter") || strings.Contains(got, "footsteps") {
		t.Fatalf("the watcher must see the mob leave by its own light, got %q", got)
	}
}
