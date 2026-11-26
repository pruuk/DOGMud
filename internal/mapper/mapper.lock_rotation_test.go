package mapper

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The map lets a player see past a locked exit whose combination they know
// as the lock stands now (#781): read from the live room, or for a room idle
// unloading has dropped, from the lock state remembered at the unload, or for
// a room not loaded this boot, at the node's difficulty and rotation 0.
func TestKnowsExitLock_ReadsTheLiveRotation(t *testing.T) {
	room := &rooms.Room{RoomId: 48501, Zone: "LockZone", Exits: map[string]exit.RoomExit{
		"north": {RoomId: 48502, Lock: gamelock.Lock{Difficulty: 4, RotationSeed: 1}},
	}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{48501: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(48590, "locks", "Locks", 948590)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{48590: u}))

	const lockId, coldId = `48501-north`, `48599-north`
	t.Cleanup(util.RebootLocksUntilRotatedForTest(4, lockId, coldId))
	e := nodeExit{RoomId: 48502, LockDifficulty: 4, LockId: lockId}

	for _, name := range []string{"loaded", "unloaded"} {
		if name == "unloaded" {
			rooms.UnloadRoomForTest(48501)
			if rooms.IsRoomLoaded(48501) {
				t.Fatal("premise: the room unloaded")
			}
		}
		u.Character.SetKey(lockId, util.LockSequence(lockId, 4, 0))
		if knowsExitLock(u, 48501, "north", e) {
			t.Fatalf("%s: a combination from rotation 0 opened the map past a lock now at rotation 1", name)
		}
		u.Character.SetKey(lockId, util.LockSequence(lockId, 4, 1))
		if !knowsExitLock(u, 48501, "north", e) {
			t.Fatalf("%s: the combination at the lock's current rotation did not open the map past it", name)
		}
	}

	// A room not loaded this boot has no live lock and no remembered state:
	// its exit is read at the node's difficulty and rotation 0, the
	// template's, since no template sets an exit's rotationseed.
	cold := nodeExit{RoomId: 48502, LockDifficulty: 4, LockId: coldId}
	u.Character.SetKey(coldId, util.LockSequence(coldId, 4, 1))
	if knowsExitLock(u, 48599, "north", cold) {
		t.Fatal("a cold room's exit was judged at a rotation other than 0")
	}
	u.Character.SetKey(coldId, util.LockSequence(coldId, 4, 0))
	if !knowsExitLock(u, 48599, "north", cold) {
		t.Fatal("the template's combination did not open the map past a cold room's exit")
	}
}
