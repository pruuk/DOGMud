package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/mobs"
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
