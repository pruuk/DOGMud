package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The newbie cert quests First Sign (41) and First Shot (50) gate their steps
// on the `command` success event for forage, track and sneak (owner ruling
// 2026-10-17: a step fires on success, never on a refusal). Before this fix
// forage and track sent the event however the command ended, and sneak sent
// none at all, so First Shot's sneak step could never be finished.

func TestForage_QuestCommandEventOnlyWhenTheForageRuns(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	origBiome := room.Biome
	t.Cleanup(func() { room.Biome = origBiome })
	user.Character.Cooldowns = characters.Cooldowns{}

	room.Biome = "city" // nothing grows here
	fired := listenForCommand(t, user, 999211, "forage")
	_, err := Forage("", user, room, 0)
	require.NoError(t, err)
	require.False(t, fired(), "a forage refused for the ground must not send the forage command event")

	room.Biome = "cave"
	fired = listenForCommand(t, user, 999212, "forage")
	_, err = Forage("", user, room, 0)
	require.NoError(t, err)
	require.True(t, fired(), "a forage that searches the ground must send the forage command event")

	fired = listenForCommand(t, user, 999213, "forage")
	_, err = Forage("", user, room, 0)
	require.NoError(t, err)
	require.False(t, fired(), "a forage refused for cooldown must not send the forage command event")
}

func TestTrack_QuestCommandEventOnlyWhenTheTrackRuns(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, _, room := speechWrapperScene(t) // a lit room: the tracker can see the ground
	user.Character.Cooldowns = characters.Cooldowns{}
	origPerception := user.Character.Stats.Perception.Base
	t.Cleanup(func() {
		user.Character.Stats.Perception.Base = origPerception
		user.Character.RecalculateStats()
	})
	user.Character.Stats.Perception.Base = 100000 // reads the ground every time
	user.Character.RecalculateStats()

	fired := listenForCommand(t, user, 999221, "track")
	_, err := Track("stop", user, room, 0)
	require.NoError(t, err)
	require.False(t, fired(), "track stop reads no ground and must not send the track command event")

	fired = listenForCommand(t, user, 999222, "track")
	_, err = Track("", user, room, 0)
	require.NoError(t, err)
	require.True(t, fired(), "a track that reads the ground must send the track command event")

	require.False(t, user.Character.CooldownReady(skills.Search.String()), "fixture: the read spent the cooldown")
	fired = listenForCommand(t, user, 999223, "track")
	_, err = Track("", user, room, 0)
	require.NoError(t, err)
	require.False(t, fired(), "a track refused for cooldown must not send the track command event")
}

func TestSneak_QuestCommandEventWhenHidden(t *testing.T) {
	room := &rooms.Room{RoomId: 9950, Zone: "SneakQuest", SkyLight: rooms.SkyLightPtr(0), Lamp: rooms.LampPtr(60)}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{9950: room}, map[string]*rooms.ZoneConfig{}))

	sneaker := users.NewTestUser(9951, "sneakq", "SneakQ", 0)
	sneaker.Character.Skills = map[string]int{string(skills.Skullduggery): 1}
	sneaker.Character.RoomId = room.RoomId
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9951: sneaker}))
	room.AddPlayer(9951)
	events.DrainQueuedMessagesForTest(9951)

	// A cooldown refusal: the sneak never happens.
	sneaker.Character.TryCooldown(skills.Skullduggery.String(`sneak`), "5 rounds")
	fired := listenForCommand(t, sneaker, 999231, "sneak")
	_, err := Sneak("", sneaker, room, 0)
	require.NoError(t, err)
	require.False(t, fired(), "a sneak refused for cooldown must not send the sneak command event")

	sneaker.Character.Cooldowns = characters.Cooldowns{}
	fired = listenForCommand(t, sneaker, 999232, "sneak")
	_, err = Sneak("", sneaker, room, 0)
	require.NoError(t, err)
	require.True(t, fired(), "a sneak that hides the player must send the sneak command event")
}
