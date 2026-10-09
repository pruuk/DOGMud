package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// Lighting plan 5c final review, finding 1: the dark-room fallbacks (the
// "You hear fighting close by." line, a mob's death, room and crafter idle flavour)
// chose between the visual line and the audible one by the nightvision FLAG.
// An infravision holder the Game window gives shapes was sent the sound, and a
// nightvision holder in a room its window reads as blind was sent the visual
// line. Both now follow the reader's sight.

// seedFallbackRoom moves users 1 and 2 into cave room 2 at a pinned light and
// gives user 1 the named vision condition.
func seedFallbackRoom(t *testing.T, lamp, eyesConditionId int) *rooms.Room {
	t.Helper()
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	restore := seedNarrationConditions()
	t.Cleanup(restore)

	room2 := rooms.LoadRoom(2)
	require.NotNil(t, room2)
	room2.Biome = "cave"
	room2.Lamp = rooms.LampPtr(lamp)
	for _, uid := range []int{1, 2} {
		u := users.GetByUserId(uid)
		require.NotNil(t, u)
		rooms.LoadRoom(1).RemovePlayer(uid)
		u.Character.RoomId = 2
		room2.AddPlayer(uid)
	}
	require.True(t, users.GetByUserId(1).Character.Conditions.AddCondition(eyesConditionId, true))
	require.Equal(t, lamp, room2.LightLevel())
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)
	return room2
}

// #216: every caller passes the fight's OWN room, so the blind reader is in
// the room with the fight. "nearby" told them it was somewhere else.
func TestDarkRoomCombatFallback_SaysCloseByNotNearby(t *testing.T) {
	room := seedFallbackRoom(t, 0, nightEyesConditionId)
	sendUnsightedCombatSound(room)
	got := drainPlain(1)
	require.Equal(t, 1, countContaining(got, "You hear fighting close by."), "%v", got)
	require.Zero(t, countContaining(got, "nearby"), "%v", got)
}

func TestDarkRoomCombatFallback_FollowsSight(t *testing.T) {
	const needle = "You hear fighting"

	t.Run("infravision at light 10 sees shapes, no fallback", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		sendUnsightedCombatSound(room)
		require.Zero(t, countContaining(drainPlain(1), needle), "the shapes reader follows the fight by eye")
		require.Equal(t, 1, countContaining(drainPlain(2), needle), "the normal reader hears it")
	})

	t.Run("nightvision at light 0 sees nothing, hears the fight", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		require.Equal(t, messaging.SightNone, messaging.ParticipantSight(users.GetByUserId(1).Character, room))
		sendUnsightedCombatSound(room)
		require.Equal(t, 1, countContaining(drainPlain(1), needle))
	})
}

func TestVisualElseAudible_FollowsSight(t *testing.T) {
	const visual = `<ansi fg="mobname">Skeleton</ansi> has died.`
	const sound = `You hear something collapse to the ground.`

	t.Run("infravision at light 10 reads the visual line anonymised", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		sendVisualElseAudible(room, messaging.CategoryDeath, visual, sound)
		got1, got2 := drainPlain(1), drainPlain(2)
		require.Equal(t, 1, countContaining(got1, "has died"), "shapes reader sees it: %v", got1)
		require.Zero(t, countContaining(got1, "Skeleton"), "but not who: %v", got1)
		require.Zero(t, countContaining(got1, "collapse"))
		require.Equal(t, 1, countContaining(got2, "collapse"), "normal reader hears it: %v", got2)
		require.Zero(t, countContaining(got2, "has died"))
	})

	t.Run("nightvision at light 0 hears it", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		sendVisualElseAudible(room, messaging.CategoryDeath, visual, sound)
		got1 := drainPlain(1)
		require.Equal(t, 1, countContaining(got1, "collapse"), "%v", got1)
		require.Zero(t, countContaining(got1, "has died"))
	})
}

// #216: the fight sound skipped every lit room, so a Blinded reader beside a
// fight in a lit room heard nothing. It goes to every reader who cannot make
// out shapes, whatever the room's light.
func TestDarkRoomCombatFallback_BlindedReaderInALitRoomHearsIt(t *testing.T) {
	room := seedFallbackRoom(t, 60, heatEyesConditionId)
	require.True(t, room.IsLit(), "fixture: the room must be lit")
	blind := users.GetByUserId(2)
	saved := blind.Character.Perception
	t.Cleanup(func() { blind.Character.Perception = saved })
	blind.Character.Perception = perception.NewMachine()
	require.NoError(t, blind.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(blind.Character, room))

	sendUnsightedCombatSound(room)

	require.Equal(t, 1, countContaining(drainPlain(2), "You hear fighting close by."), "the blinded reader hears the fight")
	require.Zero(t, countContaining(drainPlain(1), "You hear fighting"), "a reader who sees the fight reads it by eye")
}
