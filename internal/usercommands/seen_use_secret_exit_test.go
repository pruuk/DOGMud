package usercommands

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/acquire"
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// #781, owner ruling 15 (seen use teaches): a sighted onlooker learns a secret
// exit from any line that shows someone using it, and only the exit: a lock's
// sequence stays on the picker's keyring.

const (
	seenUsePicks = 999971
	seenUseKey   = 999972
)

// seenUseLockedNorth is the usual square (Aliceia, Bobrick and the skeleton in
// room 1, lit) with north a secret, locked exit Aliceia has found, lockpicks
// in her pack and Skullduggery, and the timing pinned so an opened lock stays
// open (the picklock quest fixture's pins). It returns the lock id.
func seenUseLockedNorth(t *testing.T) (*users.UserRecord, *rooms.Room, string) {
	t.Helper()
	c := configs.GetConfig()
	c.Timing.RoundsPerDay = 900
	c.Timing.RoundSeconds = 4
	c.Timing.Validate()
	configs.SetConfigForTest(t, c)
	prevRound := util.GetRoundCount()
	util.SetRoundCountForTest(uint64(util.RoundCountMinimum) + 1000)
	t.Cleanup(func() { util.SetRoundCountForTest(prevRound) })
	t.Cleanup(seedAllRegistries())
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		seenUsePicks: {ItemId: seenUsePicks, Name: "test lockpicks", Type: items.Lockpicks, Uses: 3, Weight: 0.1},
		seenUseKey:   {ItemId: seenUseKey, Name: "test key", Type: items.Key, KeyLockId: "1-north", Weight: 0.1},
	}))
	user, room := getTestUserAndRoom(t)
	user.Character.Stats.Strength.ValueAdj = 100
	_ = user.Character.GetSkillLevel(skills.Skullduggery) // allocates Skills
	user.Character.Skills[string(skills.Skullduggery)] = 1
	require.True(t, acquire.AcquireItem(user, items.New(seenUsePicks), acquire.AcquireGive))

	secretExit(t, room, "north")
	north := room.Exits["north"]
	north.Lock = gamelock.Lock{Difficulty: 3}
	room.Exits["north"] = north
	user.Character.LearnSecretExit(1, "north")
	require.True(t, room.Exits["north"].Lock.IsLocked(), "fixture: north is locked")
	require.Equal(t, messaging.SightFull, room.ParticipantSight(2), "fixture: the square is lit")
	events.DrainQueuedMessagesForTest(2)
	return user, room, fmt.Sprintf("%d-%s", room.RoomId, "north")
}

// (a) and (f): each way of working the lock teaches Bobrick and the skeleton
// the exit, never the lock's sequence or a key, and leaves Aliceia's own
// record as it was.
func TestSeenUse_WorkingTheLockTeachesTheExitNotTheSequence(t *testing.T) {
	cases := map[string]func(t *testing.T, user *users.UserRecord, room *rooms.Room, lockId string){
		"picklock": func(t *testing.T, user *users.UserRecord, room *rooms.Room, lockId string) {
			user.Character.SetKey(lockId, util.LockSequence(lockId, 3, 0))
			_, err := Picklock("north", user, room, 0)
			require.NoError(t, err)
		},
		// "Aliceia tries to pick the north lock": a right pin short of open.
		"a pin short of open": func(t *testing.T, user *users.UserRecord, room *rooms.Room, lockId string) {
			t.Cleanup(user.ClearPrompt)
			drive(t, user, func() {
				_, err := Picklock("north", user, room, 0)
				require.NoError(t, err)
			}, util.LockSequence(lockId, 3, 0)[:1])
			require.True(t, room.Exits["north"].Lock.IsLocked(), "premise: still locked")
			require.NotEmpty(t, user.Character.GetKey(lockId), "premise: the pin is on Aliceia's keyring")
		},
		"the last pin": func(t *testing.T, user *users.UserRecord, room *rooms.Room, lockId string) {
			t.Cleanup(user.ClearPrompt)
			seq := util.LockSequence(lockId, 3, 0)
			run := func() {
				_, err := Picklock("north", user, room, 0)
				require.NoError(t, err)
			}
			drive(t, user, run, seq[:1], seq[1:2])
			drainedText(users.GetByUserId(2)) // the "tries to pick" lines teach already
			users.GetByUserId(2).Character.Discoveries = nil
			mobs.GetInstance(100).Character.Discoveries = nil
			user.RoutePromptInput(seq[2:], true)
			run()
			require.False(t, room.Exits["north"].Lock.IsLocked(), "premise: the last pin opened it")
		},
		"unlock with a key in the pack": func(t *testing.T, user *users.UserRecord, room *rooms.Room, lockId string) {
			require.True(t, acquire.AcquireItem(user, items.New(seenUseKey), acquire.AcquireGive))
			_, err := Unlock("north", user, room, 0)
			require.NoError(t, err)
			require.False(t, room.Exits["north"].Lock.IsLocked(), "premise: the key opened it")
		},
		"lock with a key in the pack": func(t *testing.T, user *users.UserRecord, room *rooms.Room, lockId string) {
			room.SetExitLock("north", false)
			require.True(t, acquire.AcquireItem(user, items.New(seenUseKey), acquire.AcquireGive))
			_, err := Lock("north", user, room, 0)
			require.NoError(t, err)
			require.True(t, room.Exits["north"].Lock.IsLocked(), "premise: the key locked it")
		},
		"unlock with a key": func(t *testing.T, user *users.UserRecord, room *rooms.Room, lockId string) {
			user.Character.SetKey("key-"+lockId, "1")
			_, err := Unlock("north", user, room, 0)
			require.NoError(t, err)
		},
		"lock with a key": func(t *testing.T, user *users.UserRecord, room *rooms.Room, lockId string) {
			room.SetExitLock("north", false)
			require.False(t, room.Exits["north"].Lock.IsLocked(), "premise: open before the lock")
			user.Character.SetKey("key-"+lockId, "1")
			_, err := Lock("north", user, room, 0)
			require.NoError(t, err)
		},
		// Go picks the lock with the known sequence and steps through
		// sneaking, so no leave line teaches anything: only the unlock line.
		"go picks it, sneaking": func(t *testing.T, user *users.UserRecord, room *rooms.Room, lockId string) {
			user.Character.SetKey(lockId, util.LockSequence(lockId, 3, 0))
			user.Character.SetMiscData(`sneaking`, true)
			t.Cleanup(func() { user.Character.SetMiscData(`sneaking`, nil) })
			_, err := Go("north", user, room, 0)
			require.NoError(t, err)
			require.Equal(t, 2, user.Character.RoomId, "premise: the step happened (a sneak sends no leave line, TestGo_ASneakingMoverTeachesNothing)")
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			user, room, lockId := seenUseLockedNorth(t)
			watcher, skeleton := users.GetByUserId(2), mobs.GetInstance(100)
			discoveriesBefore := fmt.Sprint(user.Character.Discoveries)

			run(t, user, room, lockId)

			require.Contains(t, drainedText(watcher), "north", "premise: Bobrick read a line naming the exit")
			require.True(t, watcher.Character.KnowsSecretExit(1, "north"), "the player onlooker learns the exit")
			require.True(t, skeleton.Character.KnowsSecretExit(1, "north"), "the NPC onlooker learns the exit")
			require.Empty(t, watcher.Character.GetKey(lockId), "never the lock's sequence")
			require.Empty(t, watcher.Character.GetKey("key-"+lockId), "never a key")
			require.Empty(t, skeleton.Character.GetKey(lockId), "never the lock's sequence (NPC)")
			require.Equal(t, discoveriesBefore, fmt.Sprint(user.Character.Discoveries), "the actor's own record is unchanged")
		})
	}
}

// (b): the same in the dark teaches nothing.
func TestSeenUse_InTheDarkNothingIsLearned(t *testing.T) {
	user, room, lockId := seenUseLockedNorth(t)
	darkenRoom(t, room)
	require.Equal(t, messaging.SightNone, room.ParticipantSight(2), "fixture: the square is dark")
	watcher, skeleton := users.GetByUserId(2), mobs.GetInstance(100)
	user.Character.SetKey("key-"+lockId, "1")

	_, err := Unlock("north", user, room, 0)
	require.NoError(t, err)
	require.False(t, room.Exits["north"].Lock.IsLocked(), "premise: the key opened it")
	require.False(t, watcher.Character.KnowsSecretExit(1, "north"))
	require.False(t, skeleton.Character.KnowsSecretExit(1, "north"))
}

// (c): a peer through a secret exit teaches whoever sees it, on both sides:
// "Aliceia peers toward the north." here, "Aliceia is looking into the room
// from the south." in room 2, where south is secret. (f): the one peering
// learns nothing on the far side.
func TestSeenUse_APeerTeachesBothSides(t *testing.T) {
	t.Run("this side", func(t *testing.T) {
		t.Cleanup(seedAllRegistries())
		user, room := getTestUserAndRoom(t)
		lamp := room.Lamp
		room.Lamp = rooms.LampPtr(100)
		t.Cleanup(func() { room.Lamp = lamp })
		secretExit(t, room, "north")
		user.Character.LearnSecretExit(1, "north")
		watcher, skeleton := users.GetByUserId(2), mobs.GetInstance(100)
		events.DrainQueuedMessagesForTest(2)

		_, err := Look("north", user, room, 0)
		require.NoError(t, err)
		require.Contains(t, drainedText(watcher), "peers", "premise: Bobrick read the peer line")
		require.True(t, watcher.Character.KnowsSecretExit(1, "north"))
		require.True(t, skeleton.Character.KnowsSecretExit(1, "north"))
	})
	t.Run("the far side", func(t *testing.T) {
		t.Cleanup(seedAllRegistries())
		user, room := getTestUserAndRoom(t)
		lamp := room.Lamp
		room.Lamp = rooms.LampPtr(100)
		t.Cleanup(func() { room.Lamp = lamp })
		watcher, skeleton, room2 := moveOnlookersTo2(t)
		secretExit(t, room2, "south")
		events.DrainQueuedMessagesForTest(2)

		_, err := Look("north", user, room, 0)
		require.NoError(t, err)
		require.Contains(t, drainedText(watcher), "is looking into the room", "premise: Bobrick read the far-side line")
		require.True(t, watcher.Character.KnowsSecretExit(2, "south"))
		require.True(t, skeleton.Character.KnowsSecretExit(2, "south"))
		require.False(t, user.Character.KnowsSecretExit(2, "south"), "the one peering learns nothing by it")
	})
}

// #781 (Task 5c fix round): with a plain exit and a secret exit both leading
// back, a far-side line names the plain one and teaches nothing. Looked
// twenty times, since the old pick followed the map's random order.
func TestSeenUse_APlainExitBackIsNamedAndTeachesNothing(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	user, room := getTestUserAndRoom(t)
	lamp := room.Lamp
	room.Lamp = rooms.LampPtr(100)
	t.Cleanup(func() { room.Lamp = lamp })
	watcher, skeleton, room2 := moveOnlookersTo2(t)
	secretExit(t, room2, "alcove")
	alcove := room2.Exits["alcove"]
	alcove.RoomId = 1 // a second, secret way back to the square
	room2.Exits["alcove"] = alcove

	for range 20 {
		events.DrainQueuedMessagesForTest(2)
		_, err := Look("north", user, room, 0)
		require.NoError(t, err)
		got := drainedText(watcher)
		require.Contains(t, got, "is looking into the room from the", "premise: the far-side line")
		require.Contains(t, got, "south", "the plain exit back is named")
		require.NotContains(t, got, "alcove")
	}
	require.False(t, watcher.Character.KnowsSecretExit(2, "alcove"), "nothing is taught")
	require.False(t, skeleton.Character.KnowsSecretExit(2, "alcove"), "nothing is taught (NPC)")
}

// (c): a shot through a secret exit teaches whoever sees it leave, and on the
// far side whoever sees it come in, but not its target, who is left out of
// that line, nor the shooter (f).
func TestSeenUse_AShotTeachesBothSides(t *testing.T) {
	shot := actions.FireResult{
		CrossRoom: true, ExitName: "north", TargetRoomId: 2,
		TargetUserId: 2, TargetName: "Bobrick",
		MoveResult: combat.SkillMoveResult{Hit: true, Damage: 5, TargetMaxHP: 100},
	}
	t.Run("leaving", func(t *testing.T) {
		t.Cleanup(seedAllRegistries())
		user, room := getTestUserAndRoom(t)
		secretExit(t, room, "north")
		user.Character.LearnSecretExit(1, "north")
		target := users.GetByUserId(2)
		room.RemovePlayer(2)
		target.Character.RoomId = 2
		rooms.LoadRoom(2).AddPlayer(2)
		skeleton := mobs.GetInstance(100)

		sendShootMessages(user, room, shot)
		require.True(t, skeleton.Character.KnowsSecretExit(1, "north"), "the NPC who saw the shot leave")
		require.False(t, target.Character.KnowsSecretExit(1, "north"), "the target was not there to see it")
	})
	t.Run("leaving, a player onlooker", func(t *testing.T) {
		t.Cleanup(seedAllRegistries())
		user, room := getTestUserAndRoom(t)
		secretExit(t, room, "north")
		user.Character.LearnSecretExit(1, "north")
		skeleton := mobs.GetInstance(100)
		room.RemoveMob(100)
		skeleton.Character.RoomId = 2
		rooms.LoadRoom(2).AddMob(100)
		watcher := users.GetByUserId(2)
		events.DrainQueuedMessagesForTest(2)

		atSkeleton := shot
		atSkeleton.TargetUserId, atSkeleton.TargetMobInstanceId = 0, 100
		atSkeleton.TargetName, atSkeleton.IsTargetMob = "Skeleton", true
		sendShootMessages(user, room, atSkeleton)
		require.Contains(t, drainedText(watcher), "fires", "premise: Bobrick read the departure line")
		require.True(t, watcher.Character.KnowsSecretExit(1, "north"), "the player who saw the shot leave")
	})
	t.Run("arriving", func(t *testing.T) {
		t.Cleanup(seedAllRegistries())
		user, room := getTestUserAndRoom(t)
		target, skeleton, room2 := moveOnlookersTo2(t)
		secretExit(t, room2, "south")
		events.DrainQueuedMessagesForTest(2)

		sendShootMessages(user, room, shot)
		require.True(t, skeleton.Character.KnowsSecretExit(2, "south"), "the NPC who saw the shot come in")
		require.False(t, target.Character.KnowsSecretExit(2, "south"), "the target is left out of that line")
		require.False(t, user.Character.KnowsSecretExit(2, "south"), "the shooter learns nothing by it")
	})
}
