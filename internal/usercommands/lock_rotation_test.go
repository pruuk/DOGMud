package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// A known combination is judged against the lock's current rotation (#781).
// SetLocked bumps RotationSeed, so a combination learned before the lock
// re-locked no longer opens it, and one learned at the current rotation does.

const knownLockLine = `You know this lock well`

// rotatedNorth is seenUseLockedNorth's locked north exit, opened and locked
// again once so its RotationSeed is 1.
func rotatedNorth(t *testing.T) (*users.UserRecord, *rooms.Room, string) {
	t.Helper()
	user, room, lockId := seenUseLockedNorth(t)
	t.Cleanup(util.RebootLocksUntilRotatedForTest(3, lockId))
	room.SetExitLock("north", false)
	room.SetExitLock("north", true)
	require.True(t, room.Exits["north"].Lock.IsLocked(), "fixture: north is locked again")
	require.Equal(t, uint64(1), room.Exits["north"].Lock.RotationSeed, "fixture: the re-lock rotated the lock")
	drainedText(user)
	return user, room, lockId
}

func TestGo_ACombinationFromBeforeTheRelockDoesNotPickTheLock(t *testing.T) {
	user, room, lockId := rotatedNorth(t)
	user.Character.SetKey(lockId, util.LockSequence(lockId, 3, 0))

	_, hasSequence := user.Character.HasKey(lockId, room.Exits["north"].Lock)

	_, err := Go("north", user, room, 0)
	require.NoError(t, err)
	require.NotContains(t, drainedText(user), knownLockLine)
	require.Equal(t, 1, user.Character.RoomId, "go stepped through a lock it could not open")
	require.True(t, room.Exits["north"].Lock.IsLocked(), "go opened the lock")
	require.False(t, hasSequence, "a combination from rotation 0 counts after the lock rotated")
}

func TestGo_ACombinationAtTheCurrentRotationPicksTheLock(t *testing.T) {
	user, room, lockId := rotatedNorth(t)
	user.Character.SetKey(lockId, util.LockSequence(lockId, 3, 1))

	_, hasSequence := user.Character.HasKey(lockId, room.Exits["north"].Lock)

	_, err := Go("north", user, room, 0)
	require.NoError(t, err)
	require.Contains(t, drainedText(user), knownLockLine)
	require.Equal(t, 2, user.Character.RoomId, "go did not step through the picked lock")
	require.True(t, hasSequence, "the combination at the current rotation does not count")
}

// A container: locking it with the key ring's key rotates it, which retires a
// known combination while the key keeps working for lock and unlock.
func TestLockUnlock_AContainerRelockRetiresTheCombinationNotTheKey(t *testing.T) {
	user, room, _ := seenUseLockedNorth(t)
	old := room.Containers
	room.Containers = map[string]rooms.Container{"chest": {Lock: gamelock.Lock{Difficulty: 3}}}
	t.Cleanup(func() { room.Containers = old })
	lockId := fmt.Sprintf("%d-chest", room.RoomId)
	t.Cleanup(util.RebootLocksUntilRotatedForTest(3, lockId))
	user.Character.SetKey("key-"+lockId, "1")

	_, err := Unlock("chest", user, room, 0)
	require.NoError(t, err)
	require.False(t, room.Containers["chest"].Lock.IsLocked(), "premise: the key opened it")
	user.Character.SetKey(lockId, util.LockSequence(lockId, 3, room.Containers["chest"].Lock.RotationSeed))
	hasKey, hasSequence := user.Character.HasKey(lockId, room.Containers["chest"].Lock)
	require.True(t, hasKey)
	require.True(t, hasSequence, "the combination at the current rotation does not count")

	_, err = Lock("chest", user, room, 0)
	require.NoError(t, err)
	require.True(t, room.Containers["chest"].Lock.IsLocked(), "premise: the key locked it")
	hasKey, hasSequence = user.Character.HasKey(lockId, room.Containers["chest"].Lock)
	require.True(t, hasKey, "the key stopped counting after the relock")
	require.False(t, hasSequence, "a combination from before the relock still counts")

	_, err = Unlock("chest", user, room, 0)
	require.NoError(t, err)
	require.False(t, room.Containers["chest"].Lock.IsLocked(), "the key no longer opens the relocked chest")
}
