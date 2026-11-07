package gmcp

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// shoutFixture: Kesh (9761) and Ana (9762) in lit room 9760 with the Crier
// (mob 761); Bo (9763) and the deafened Di (9764) next door in 9765, north of
// 9760. 9765's own exit back is south, which is what ForEachAdjacentRoom
// passes (FindExitTo from the listener's room) and what the game text says:
// "from the south direction".
func shoutFixture(t *testing.T) {
	t.Helper()
	zero := 0.0
	here := &rooms.Room{RoomId: 9760, Zone: "ShoutZone", SkyLight: &zero, Lamp: rooms.LampPtr(60),
		Exits: map[string]exit.RoomExit{"north": {RoomId: 9765}}}
	there := &rooms.Room{RoomId: 9765, Zone: "ShoutZone", SkyLight: &zero, Lamp: rooms.LampPtr(60),
		Exits: map[string]exit.RoomExit{"south": {RoomId: 9760}}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9760: here, 9765: there},
		map[string]*rooms.ZoneConfig{"ShoutZone": {Name: "ShoutZone", RoomId: 9760,
			RoomIds: map[int]struct{}{9760: {}, 9765: {}}}}))

	crier := &mobs.Mob{MobId: 1, InstanceId: 761, Zone: "ShoutZone"}
	crier.Character.Name = "Crier"
	crier.Character.RoomId = 9760
	t.Cleanup(mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Zone: "ShoutZone"}},
		map[int]*mobs.Mob{761: crier},
	))
	here.AddMob(761)

	mk := func(id int, name string, roomId int) *users.UserRecord {
		u := users.NewTestUser(id, name, name, uint64(90000+id))
		u.Character.RoomId = roomId
		return u
	}
	kesh, ana, bo, di := mk(9761, "Kesh", 9760), mk(9762, "Ana", 9760), mk(9763, "Bo", 9765), mk(9764, "Di", 9765)
	di.Deafened = true
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9761: kesh, 9762: ana, 9763: bo, 9764: di}))
	here.AddPlayer(9761)
	here.AddPlayer(9762)
	there.AddPlayer(9763)
	there.AddPlayer(9764)
}

// shoutSenders runs shoutDeliveries and maps each recipient to the Sender it reads.
func shoutSenders(evt events.Communication) map[int]string {
	got := map[int]string{}
	for _, d := range shoutDeliveries(evt, GMCPCommModule_Payload{Channel: `shout`, Sender: evt.Name, Text: evt.Message}) {
		got[d.UserId] = d.Payload.Sender
	}
	return got
}

// Spec: a shout reaches the speaker's room by name (by sight, as say) and the
// rooms next door as "Someone (<exit>)", the same words as the game text; a
// player's shout skips deafened listeners.
func TestShoutDeliveriesNearAndNextDoor(t *testing.T) {
	shoutFixture(t)
	got := shoutSenders(events.Communication{SourceUserId: 9761, CommType: `shout`, Name: `Kesh`, Message: `HERE I AM!`})
	require.Equal(t, map[int]string{9761: `Kesh`, 9762: `Kesh`, 9763: `Someone (south)`}, got)
}

// An NPC's shout is authored, so it reaches a deafened listener next door
// too, as the text lane's unfiltered SendText does.
func TestShoutDeliveries_MobShoutReachesTheDeafenedNextDoor(t *testing.T) {
	shoutFixture(t)
	got := shoutSenders(events.Communication{SourceMobInstanceId: 761, CommType: `shout`, Name: `Crier`, Message: `HEAR YE`})
	require.Equal(t, map[int]string{9761: `Crier`, 9762: `Crier`, 9763: `Someone (south)`, 9764: `Someone (south)`}, got)
}

// A shouter still hidden after shouting is "Someone" in their own room, as
// say does; the shouter reads their own name.
func TestShoutDeliveries_HiddenShouterIsSomeone(t *testing.T) {
	shoutFixture(t)
	got := shoutSenders(events.Communication{SourceUserId: 9761, CommType: `shout`, Name: `Kesh`, Message: `psst`, SpeakerHidden: true})
	require.Equal(t, map[int]string{9761: `Kesh`, 9762: `Someone`, 9763: `Someone (south)`}, got)
}
