package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// secretNorthForSeenUse makes room 1 lit and its north a secret exit, locked
// when locked is true, until the test ends.
func secretNorthForSeenUse(t *testing.T, room *rooms.Room, locked bool) {
	t.Helper()
	lamp := room.Lamp
	room.Lamp = rooms.LampPtr(90)
	orig := room.Exits["north"]
	north := orig
	north.Secret = true
	if locked {
		north.Lock = gamelock.Lock{Difficulty: 3}
	}
	room.Exits["north"] = north
	t.Cleanup(func() { room.Lamp, room.Exits["north"] = lamp, orig })
}

// (d) and (f), #781 owner ruling 15: a mob that tries a locked secret exit is
// seen trying it. Bobrick and the merchant learn the exit; the skeleton's own
// record is not this rule's.
func TestMobGo_OnlookersLearnALockedSecretExitTheySeeTried(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	mob, room := getTestMobAndRoom(t)
	secretNorthForSeenUse(t, room, true)
	watcher, merchant := users.GetByUserId(2), mobs.GetInstance(200)
	events.DrainQueuedMessagesForTest(2)

	handled, err := Go("north", mob, room)
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, 1, mob.Character.RoomId, "premise: the lock held")
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(2), "\n"), "but it's locked",
		"premise: Bobrick read the line")
	require.True(t, watcher.Character.KnowsSecretExit(1, "north"))
	require.True(t, merchant.Character.KnowsSecretExit(1, "north"))
	require.False(t, mob.Character.KnowsSecretExit(1, "north"), "the actor's own record is not this rule's")
}

// The same in the dark: the line is not seen and nothing is learned.
func TestMobGo_ALockedSecretExitTriedInTheDarkTeachesNothing(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	mob, room := getTestMobAndRoom(t)
	secretNorthForSeenUse(t, room, true)
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(0)

	_, err := Go("north", mob, room)
	require.NoError(t, err)
	require.False(t, users.GetByUserId(2).Character.KnowsSecretExit(1, "north"))
	require.False(t, mobs.GetInstance(200).Character.KnowsSecretExit(1, "north"))
}

// Ruling 15: a mob seen peering through a secret exit teaches it.
func TestMobLook_OnlookersLearnASecretExitTheySeePeeredThrough(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	mob, room := getTestMobAndRoom(t)
	secretNorthForSeenUse(t, room, false)
	mob.Character.LearnSecretExit(1, "north")
	events.DrainQueuedMessagesForTest(2)

	_, err := Look("north", mob, room)
	require.NoError(t, err)
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(2), "\n"), "peers",
		"premise: Bobrick read the peer line")
	require.True(t, users.GetByUserId(2).Character.KnowsSecretExit(1, "north"))
	require.True(t, mobs.GetInstance(200).Character.KnowsSecretExit(1, "north"))
}

// moveToRoom2 moves players and mobs from room 1 to room 2 and lights room 2.
func moveToRoom2(t *testing.T, userIds []int, mobIds []int) *rooms.Room {
	t.Helper()
	room1, room2 := rooms.LoadRoom(1), rooms.LoadRoom(2)
	lamp := room2.Lamp
	room2.Lamp = rooms.LampPtr(90)
	t.Cleanup(func() { room2.Lamp = lamp })
	for _, uid := range userIds {
		room1.RemovePlayer(uid)
		users.GetByUserId(uid).Character.RoomId = 2
		room2.AddPlayer(uid)
	}
	for _, id := range mobIds {
		room1.RemoveMob(id)
		mobs.GetInstance(id).Character.RoomId = 2
		room2.AddMob(id)
	}
	return room2
}

// secretSouthIn2 makes room 2's south (the way back to room 1) secret.
func secretSouthIn2(t *testing.T, room2 *rooms.Room) {
	t.Helper()
	orig := room2.Exits["south"]
	south := orig
	south.Secret = true
	room2.Exits["south"] = south
	t.Cleanup(func() { room2.Exits["south"] = orig })
}

// Ruling 15, the far side of a mob's peer: "Skeleton is looking into the room
// from the south." teaches room 2's secret south to whoever sees it there;
// the skeleton, peering from room 1, learns nothing by it.
func TestMobLook_TheFarSideOfAPeerTeachesTheExitThere(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	mob, room := getTestMobAndRoom(t)
	room.Lamp = rooms.LampPtr(90)
	room2 := moveToRoom2(t, []int{2}, []int{200})
	secretSouthIn2(t, room2)
	events.DrainQueuedMessagesForTest(2)

	_, err := Look("north", mob, room)
	require.NoError(t, err)
	require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(2), "\n"), "is looking into the room",
		"premise: Bobrick read the far-side line")
	require.True(t, users.GetByUserId(2).Character.KnowsSecretExit(2, "south"))
	require.True(t, mobs.GetInstance(200).Character.KnowsSecretExit(2, "south"))
	require.False(t, mob.Character.KnowsSecretExit(2, "south"), "the mob peering learns nothing by it")
}

// Ruling 15 for a mob's shot: onlookers who see it leave through a secret exit
// learn it; on the far side, onlookers who see it come in learn the secret
// exit there, and the target, left out of that line, does not.
func TestMobShoot_OnlookersLearnTheSecretExitAShotCrosses(t *testing.T) {
	require.NoError(t, movenarration.LoadFrom("../../_datafiles/world/dogmud/narration/special-moves"))
	shot := actions.FireResult{
		CrossRoom: true, ExitName: "north", TargetRoomId: 2,
		TargetUserId: 2, TargetName: "Bobrick", Executed: true,
		MoveResult: combat.SkillMoveResult{Hit: true, Damage: 5, TargetMaxHP: 100},
	}
	t.Run("leaving", func(t *testing.T) {
		t.Cleanup(seedAllRegistries())
		mob, room := getTestMobAndRoom(t)
		secretNorthForSeenUse(t, room, false)
		moveToRoom2(t, []int{2}, nil)
		events.DrainQueuedMessagesForTest(1)

		sendMobShootMessages(mob, room, shot)
		require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(1), "\n"), "fires",
			"premise: Aliceia read the departure line")
		require.True(t, users.GetByUserId(1).Character.KnowsSecretExit(1, "north"), "the player onlooker")
		require.True(t, mobs.GetInstance(200).Character.KnowsSecretExit(1, "north"), "the NPC onlooker")
		require.False(t, mob.Character.KnowsSecretExit(1, "north"), "the shooter's record is not this rule's")
	})
	t.Run("arriving", func(t *testing.T) {
		t.Cleanup(seedAllRegistries())
		mob, room := getTestMobAndRoom(t)
		room2 := moveToRoom2(t, []int{1, 2}, []int{200})
		secretSouthIn2(t, room2)
		events.DrainQueuedMessagesForTest(1)

		sendMobShootMessages(mob, room, shot)
		require.Contains(t, strings.Join(events.DrainQueuedMessagesForTest(1), "\n"), "streaks in",
			"premise: Aliceia read the arrival line")
		require.True(t, users.GetByUserId(1).Character.KnowsSecretExit(2, "south"), "the player onlooker")
		require.True(t, mobs.GetInstance(200).Character.KnowsSecretExit(2, "south"), "the NPC onlooker")
		require.False(t, users.GetByUserId(2).Character.KnowsSecretExit(2, "south"), "the target is left out of that line")
	})
}
