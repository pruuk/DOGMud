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
// unloading has dropped, from the lock state remembered at the unload. Only
// when no source has the exit does the key alone count.
func TestKnowsExitLock_ReadsTheLiveRotation(t *testing.T) {
	room := &rooms.Room{RoomId: 48501, Zone: "LockZone", Exits: map[string]exit.RoomExit{
		"north": {RoomId: 48502, Lock: gamelock.Lock{Difficulty: 4, RotationSeed: 1}},
	}}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{48501: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(48590, "locks", "Locks", 948590)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{48590: u}))

	const lockId = `48501-north`
	t.Cleanup(util.RebootLocksUntilRotatedForTest(4, lockId))
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

	// No source has this exit (no live room, no remembered state, no
	// template): only the key counts.
	gone := nodeExit{RoomId: 48502, LockDifficulty: 4, LockId: `48599-north`}
	u.Character.SetKey(`48599-north`, util.LockSequence(`48599-north`, 4, 0))
	if knowsExitLock(u, 48599, "north", gone) {
		t.Fatal("a combination was trusted for a lock no source has")
	}
	u.Character.SetKey(`key-48599-north`, `20001`)
	if !knowsExitLock(u, 48599, "north", gone) {
		t.Fatal("a key on the ring did not open the map past a lock no source has")
	}
}
