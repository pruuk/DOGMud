package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 14: an onlooker learns the secret exit a movement line
// names when it read that line, at the decision the line was judged at. A
// shapes-only reader's line still names the exit, so it learns too; a
// reader at SightNone, or one not in the snapshot (it came in after), does
// not. The mover is skipped, and an ordinary exit records nothing.
func TestLearnSecretExitWatching(t *testing.T) {
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const roomId, full, shapes, blind, late = 99461, 99471, 99472, 99473, 99474
	const mobFull, mobBlind = 98471, 98472
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		roomId: {RoomId: roomId, Zone: "test", Exits: map[string]exit.RoomExit{
			"crack": {RoomId: 99462, Secret: true},
			"north": {RoomId: 99463},
		}},
	}, map[string]*rooms.ZoneConfig{}))
	seeded := map[int]*users.UserRecord{}
	names := map[int]string{full: "Fulsight", shapes: "Shapira", blind: "Blindwell", late: "Latecomb"}
	for uid, name := range names {
		u := users.NewTestUser(uid, name, name, 0)
		u.Character.RoomId = roomId
		seeded[uid] = u
	}
	t.Cleanup(users.SeedUsersForTest(seeded))
	room := rooms.LoadRoom(roomId)
	for uid := range seeded {
		room.AddPlayer(uid)
	}
	newMob := func(id int) *mobs.Mob {
		m := &mobs.Mob{InstanceId: id, Character: *characters.New()}
		m.Character.RoomId = roomId
		mobs.SetInstanceForTest(id, m)
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		room.AddMob(id)
		return m
	}
	seer, blindMob := newMob(mobFull), newMob(mobBlind)

	players := rooms.VisualSnapshot{full: messaging.SightFull, shapes: messaging.SightShapes, blind: messaging.SightNone}
	npcs := rooms.MobVisualSnapshot{mobFull: messaging.SightFull, mobBlind: messaging.SightNone}
	mover := seeded[full].Character

	LearnSecretExitWatching(room, "north", nil, players, npcs)
	for uid, u := range seeded {
		require.Empty(t, u.Character.Discoveries, "an ordinary exit records nothing (user %d)", uid)
	}
	require.Empty(t, seer.Character.Discoveries, "an ordinary exit records nothing (npc)")

	LearnSecretExitWatching(room, "crack", mover, players, npcs)
	require.False(t, mover.KnowsSecretExit(roomId, "crack"), "the mover's own knowledge is not this rule's")
	require.True(t, seeded[shapes].Character.KnowsSecretExit(roomId, "crack"), "a shapes-only reader's line names the exit")
	require.False(t, seeded[blind].Character.KnowsSecretExit(roomId, "crack"), "SightNone read the sound line")
	require.False(t, seeded[late].Character.KnowsSecretExit(roomId, "crack"), "not in the snapshot: not there to see it")
	require.True(t, seer.Character.KnowsSecretExit(roomId, "crack"), "an NPC onlooker records on its instance")
	require.False(t, blindMob.Character.KnowsSecretExit(roomId, "crack"))

	LearnSecretExitWatching(room, "crack", nil, players, npcs)
	require.True(t, mover.KnowsSecretExit(roomId, "crack"), "premise: skipped only as the mover")
	LearnSecretExitWatching(nil, "crack", nil, players, npcs) // must not panic
}

// #781, owner ruling 14, the mob mover's path: a player and an NPC watching a
// mob leave through a secret exit learn it, and onlookers in the room it
// enters learn the secret exit there leading back.
func TestRelocateMob_OnlookersLearnTheSecretExitTheySeeUsed(t *testing.T) {
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const from, to, instId, npcFrom, npcTo, wFrom, wTo = 99481, 99482, 98481, 98482, 98483, 99491, 99492
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		from: {RoomId: from, Zone: "test", Lamp: rooms.LampPtr(60), Exits: map[string]exit.RoomExit{"crack": {RoomId: to, Secret: true}}},
		to:   {RoomId: to, Zone: "test", Lamp: rooms.LampPtr(60), Exits: map[string]exit.RoomExit{"hollow": {RoomId: from, Secret: true}}},
	}, map[string]*rooms.ZoneConfig{}))
	watchFrom := users.NewTestUser(wFrom, "wf", "Wf", 0)
	watchFrom.Character.RoomId = from
	watchTo := users.NewTestUser(wTo, "wt", "Wt", 0)
	watchTo.Character.RoomId = to
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{wFrom: watchFrom, wTo: watchTo}))
	fromRoom, toRoom := rooms.LoadRoom(from), rooms.LoadRoom(to)
	fromRoom.AddPlayer(wFrom)
	toRoom.AddPlayer(wTo)
	place := func(id int, r *rooms.Room) *mobs.Mob {
		m := &mobs.Mob{InstanceId: id, Character: *characters.New()}
		m.Character.Name = "Mob"
		m.Character.RoomId = r.RoomId
		mobs.SetInstanceForTest(id, m)
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		r.AddMob(id)
		return m
	}
	mover, seerFrom, seerTo := place(instId, fromRoom), place(npcFrom, fromRoom), place(npcTo, toRoom)
	require.Equal(t, messaging.SightFull, fromRoom.ParticipantSight(wFrom), "fixture: lit")
	require.Equal(t, messaging.SightFull, toRoom.ParticipantSight(wTo), "fixture: lit")

	RelocateMob(mover, fromRoom, "crack", toRoom, false)

	require.True(t, watchFrom.Character.KnowsSecretExit(from, "crack"), "the player saw the mob leave by it")
	require.True(t, seerFrom.Character.KnowsSecretExit(from, "crack"), "the NPC saw the mob leave by it")
	require.True(t, watchTo.Character.KnowsSecretExit(to, "hollow"), "the player saw the mob enter from it")
	require.True(t, seerTo.Character.KnowsSecretExit(to, "hollow"), "the NPC saw the mob enter from it")
	require.Empty(t, mover.Character.Discoveries, "the mover's own knowledge is not this rule's")
}

// Ruling 14 needs a line to read: a sneaking mob sends no leave or arrive
// line (ruling D1), so its onlookers on both sides learn nothing.
func TestRelocateMob_ASneakingMoverTeachesNothing(t *testing.T) {
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const from, to, instId, npcFrom, npcTo, wFrom, wTo = 99511, 99512, 98511, 98512, 98513, 99521, 99522
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
		from: {RoomId: from, Zone: "test", Lamp: rooms.LampPtr(60), Exits: map[string]exit.RoomExit{"crack": {RoomId: to, Secret: true}}},
		to:   {RoomId: to, Zone: "test", Lamp: rooms.LampPtr(60), Exits: map[string]exit.RoomExit{"hollow": {RoomId: from, Secret: true}}},
	}, map[string]*rooms.ZoneConfig{}))
	watchFrom := users.NewTestUser(wFrom, "Wenfrom", "Wenfrom", 0)
	watchFrom.Character.RoomId = from
	watchTo := users.NewTestUser(wTo, "Wento", "Wento", 0)
	watchTo.Character.RoomId = to
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{wFrom: watchFrom, wTo: watchTo}))
	fromRoom, toRoom := rooms.LoadRoom(from), rooms.LoadRoom(to)
	fromRoom.AddPlayer(wFrom)
	toRoom.AddPlayer(wTo)
	place := func(id int, r *rooms.Room) *mobs.Mob {
		m := &mobs.Mob{InstanceId: id, Character: *characters.New()}
		m.Character.Name = "Mob"
		m.Character.RoomId = r.RoomId
		mobs.SetInstanceForTest(id, m)
		t.Cleanup(func() { mobs.SetInstanceForTest(id, nil) })
		r.AddMob(id)
		return m
	}
	mover, seerFrom, seerTo := place(instId, fromRoom), place(npcFrom, fromRoom), place(npcTo, toRoom)
	require.Equal(t, messaging.SightFull, fromRoom.ParticipantSight(wFrom), "fixture: lit")
	require.Equal(t, messaging.SightFull, toRoom.ParticipantSight(wTo), "fixture: lit")

	RelocateMob(mover, fromRoom, "crack", toRoom, true)

	require.Equal(t, to, mover.Character.RoomId, "fixture: the mob moved")
	require.False(t, watchFrom.Character.KnowsSecretExit(from, "crack"))
	require.False(t, seerFrom.Character.KnowsSecretExit(from, "crack"))
	require.False(t, watchTo.Character.KnowsSecretExit(to, "hollow"))
	require.False(t, seerTo.Character.KnowsSecretExit(to, "hollow"))
}
