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
// only at the lock's current rotation, read from the live room (#781). An
// unloaded room is not read from disk, so there only the key counts.
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

	u.Character.SetKey(lockId, util.LockSequence(lockId, 4, 0))
	if knowsExitLock(u, 48501, "north", e) {
		t.Fatal("a combination from rotation 0 opened the map past a lock now at rotation 1")
	}

	u.Character.SetKey(lockId, util.LockSequence(lockId, 4, 1))
	if !knowsExitLock(u, 48501, "north", e) {
		t.Fatal("the combination at the lock's current rotation did not open the map past it")
	}

	rooms.UnloadRoomForTest(48501)
	for _, rotation := range []uint64{0, 1} {
		u.Character.SetKey(lockId, util.LockSequence(lockId, 4, rotation))
		if knowsExitLock(u, 48501, "north", e) {
			t.Fatalf("an unloaded room's combination at rotation %d was trusted without reading its rotation", rotation)
		}
	}
	u.Character.SetKey(`key-`+lockId, `20001`)
	if !knowsExitLock(u, 48501, "north", e) {
		t.Fatal("a key on the ring did not open the map past an unloaded room's lock")
	}
}
