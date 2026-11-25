package usercommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-11-25: every relock gets a new combination, a relock on
// the timer included. A combination learned before stops working at the next
// relock, whatever caused it.

const keyringPickLine = `Your keyring already has this lock on it.`

// timerRelockedNorth is seenUseLockedNorth's north exit, opened and left
// until its timer relocks it. preOpen is the combination it had before it
// opened; the lock seed is rerolled so that and the current combination
// differ (a 3-pin lock has 8).
func timerRelockedNorth(t *testing.T) (user *users.UserRecord, room *rooms.Room, lockId string, preOpen string) {
	t.Helper()
	user, room, lockId = seenUseLockedNorth(t)
	room.SetExitLock("north", false)
	opened := room.Exits["north"].Lock.UnlockedRound
	util.SetRoundCountForTest(gametime.GetDate(opened).AddPeriod(gamelock.DefaultRelockTime))
	lock := room.Exits["north"].Lock
	require.True(t, lock.IsLocked(), "fixture: the timer relocked north")
	require.Equal(t, uint64(0), lock.RotationSeed, "fixture: nothing called SetLocked")
	// A timer relock that did not rotate leaves no two rotations to tell
	// apart; the tests then fail on what go and picklock do, as they should.
	if r := lock.Rotation(); r != 0 {
		t.Cleanup(util.RebootLocksUntilDistinctForTest(3, []string{lockId}, 0, r))
	}
	drainedText(user)
	return user, room, lockId, util.LockSequence(lockId, 3, 0)
}

func TestGo_ATimerRelockRetiresTheCombination(t *testing.T) {
	user, room, lockId, preOpen := timerRelockedNorth(t)
	user.Character.SetKey(lockId, preOpen)

	_, err := Go("north", user, room, 0)
	require.NoError(t, err)
	require.NotContains(t, drainedText(user), knownLockLine)
	require.Equal(t, 1, user.Character.RoomId, "go stepped through a lock it could not open")
	require.True(t, room.Exits["north"].Lock.IsLocked(), "go opened the lock")
}

func TestPicklock_ATimerRelockNeedsAFreshPick(t *testing.T) {
	user, room, lockId, preOpen := timerRelockedNorth(t)
	t.Cleanup(user.ClearPrompt)
	user.Character.SetKey(lockId, preOpen)

	_, err := Picklock("north", user, room, 0)
	require.NoError(t, err)
	require.NotContains(t, drainedText(user), keyringPickLine)
	require.True(t, room.Exits["north"].Lock.IsLocked(), "picklock opened the relocked lock with the old combination")
}

func TestGoAndPicklock_AFreshPickAfterATimerRelockWorks(t *testing.T) {
	t.Run("go", func(t *testing.T) {
		user, room, lockId, _ := timerRelockedNorth(t)
		user.Character.SetKey(lockId, room.Exits["north"].Lock.Sequence(lockId))

		_, err := Go("north", user, room, 0)
		require.NoError(t, err)
		require.Contains(t, drainedText(user), knownLockLine)
		require.Equal(t, 2, user.Character.RoomId, "go did not step through the freshly picked lock")
	})
	t.Run("picklock", func(t *testing.T) {
		user, room, lockId, _ := timerRelockedNorth(t)
		t.Cleanup(user.ClearPrompt)
		user.Character.SetKey(lockId, room.Exits["north"].Lock.Sequence(lockId))

		_, err := Picklock("north", user, room, 0)
		require.NoError(t, err)
		require.True(t, strings.Contains(drainedText(user), keyringPickLine), "picklock did not take the fresh combination")
		require.False(t, room.Exits["north"].Lock.IsLocked(), "picklock did not open the lock")
	})
}
