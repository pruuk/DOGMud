package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// RelocateMob is the mob's move with no gate and no charge: walking (after
// its gates and, from 4b, its charge) and a successful flee both end in it.
func TestRelocateMob_MovesTheMobBetweenRooms(t *testing.T) {
	// Room.AddMob queues a RoomChange event as a side effect; nothing here
	// drains it, so it would otherwise leak into whatever test runs next.
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const from, to, instId = 99411, 99412, 98411
	cleanup := rooms.SeedRoomsForTest(map[int]*rooms.Room{
		from: {RoomId: from, Zone: "test", Exits: map[string]exit.RoomExit{"north": {RoomId: to}}},
		to:   {RoomId: to, Zone: "test", Exits: map[string]exit.RoomExit{"south": {RoomId: from}}},
	}, map[string]*rooms.ZoneConfig{})
	defer cleanup()

	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Walker"
	m.Character.RoomId = from
	mobs.SetInstanceForTest(instId, m)
	defer mobs.SetInstanceForTest(instId, nil)

	fromRoom, toRoom := rooms.LoadRoom(from), rooms.LoadRoom(to)
	fromRoom.AddMob(instId)

	RelocateMob(m, fromRoom, "north", toRoom, false)

	if contains := func(ids []int) bool {
		for _, id := range ids {
			if id == instId {
				return true
			}
		}
		return false
	}; contains(fromRoom.GetMobs(rooms.FindAll)) || !contains(toRoom.GetMobs(rooms.FindAll)) {
		t.Fatalf("mob not moved: from=%v to=%v", fromRoom.GetMobs(rooms.FindAll), toRoom.GetMobs(rooms.FindAll))
	}
	if m.Character.RoomId != to {
		t.Fatalf("mob RoomId = %d, want %d", m.Character.RoomId, to)
	}
}

// relocateWatchers seeds a player in each room of a RelocateMob move, plus a
// third room reached the way Room.SendTextToExits reaches its targets (an
// exit off dest, to a room with its own exit back into dest), and returns
// the messages each one received, draining them. SendTextToExits excludes
// only players in from, so the third room's watcher is the one that can
// prove the exits line actually sent (fromWatcher is always excluded from
// it, and toWatcher never was its target).
func relocateWatchers(t *testing.T, sneaking bool) (fromMsgs, toMsgs, exitMsgs []string) {
	t.Helper()
	// Room.AddMob (below, and inside RelocateMob's move to dest) queues a
	// RoomChange event as a side effect. Nothing in this file calls
	// events.ProcessEvents to drain it, so left alone it sits in the shared
	// global queue and leaks into whatever test runs next (mirrors
	// hooks.newShadowScene's cleanup for the same reason).
	t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
	const from, to, instId = 99421, 99422, 98421
	const fromWatcher, toWatcher = 99431, 99432
	const exitRoom, exitWatcher = 99423, 99433
	cleanupRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{
		from:     {RoomId: from, Zone: "test", Exits: map[string]exit.RoomExit{"north": {RoomId: to}}},
		to:       {RoomId: to, Zone: "test", Exits: map[string]exit.RoomExit{"south": {RoomId: from}, "east": {RoomId: exitRoom}}},
		exitRoom: {RoomId: exitRoom, Zone: "test", Exits: map[string]exit.RoomExit{"west": {RoomId: to}}},
	}, map[string]*rooms.ZoneConfig{})
	defer cleanupRooms()

	fw := users.NewTestUser(fromWatcher, "fromwatch", "Fromwatch", 0)
	fw.Character.RoomId = from
	tw := users.NewTestUser(toWatcher, "towatch", "Towatch", 0)
	tw.Character.RoomId = to
	ew := users.NewTestUser(exitWatcher, "exitwatch", "Exitwatch", 0)
	ew.Character.RoomId = exitRoom
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{fromWatcher: fw, toWatcher: tw, exitWatcher: ew})
	defer cleanupUsers()

	m := &mobs.Mob{InstanceId: instId, Character: *characters.New()}
	m.Character.Name = "Prowler"
	m.Character.RoomId = from
	mobs.SetInstanceForTest(instId, m)
	defer mobs.SetInstanceForTest(instId, nil)

	fromRoom, toRoom, exitRoomObj := rooms.LoadRoom(from), rooms.LoadRoom(to), rooms.LoadRoom(exitRoom)
	fromRoom.AddPlayer(fromWatcher)
	toRoom.AddPlayer(toWatcher)
	exitRoomObj.AddPlayer(exitWatcher)
	fromRoom.AddMob(instId)
	events.DrainQueuedMessagesForTest(fromWatcher)
	events.DrainQueuedMessagesForTest(toWatcher)
	events.DrainQueuedMessagesForTest(exitWatcher)
	events.DrainQueuedRoomMessagesForTest(from)
	events.DrainQueuedRoomMessagesForTest(exitRoom)

	RelocateMob(m, fromRoom, "north", toRoom, sneaking)

	fromMsgs = events.DrainQueuedMessagesForTest(fromWatcher)
	toMsgs = events.DrainQueuedMessagesForTest(toWatcher)
	// SendTextToExits queues a RoomId-keyed message (not yet expanded to the
	// recipients in exitRoom), so read it off the room, not the user.
	for _, msg := range events.DrainQueuedRoomMessagesForTest(exitRoom) {
		exitMsgs = append(exitMsgs, msg.Text)
	}
	// Drain the from-room message SendTextToExits also queues (it excludes
	// every from player, so fromWatcher never sees it) so it doesn't leak
	// into a later test that reuses room ids.
	events.DrainQueuedRoomMessagesForTest(from)

	return fromMsgs, toMsgs, exitMsgs
}

// Owner ruling D1: a sneaking mob's step is not announced, as a sneaking
// player's never was. The walking control proves the watchers can hear one,
// including the SendTextToExits line into a third room past dest.
func TestRelocateMob_ASneakingMobMovesUnannounced(t *testing.T) {
	fromMsgs, toMsgs, exitMsgs := relocateWatchers(t, false)
	if len(fromMsgs) == 0 || len(toMsgs) == 0 || len(exitMsgs) == 0 {
		t.Fatalf("control: a walking mob must be announced to all three rooms, got from=%q to=%q exits=%q", fromMsgs, toMsgs, exitMsgs)
	}

	fromMsgs, toMsgs, exitMsgs = relocateWatchers(t, true)
	if len(fromMsgs) != 0 || len(toMsgs) != 0 || len(exitMsgs) != 0 {
		t.Errorf("a sneaking mob was announced: from=%q to=%q exits=%q", fromMsgs, toMsgs, exitMsgs)
	}
}
