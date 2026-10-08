package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
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
	sendDarkRoomCombatFallback(room)
	got := drainPlain(1)
	require.Equal(t, 1, countContaining(got, "You hear fighting close by."), "%v", got)
	require.Zero(t, countContaining(got, "nearby"), "%v", got)
}

func TestDarkRoomCombatFallback_FollowsSight(t *testing.T) {
	const needle = "You hear fighting"

	t.Run("infravision at light 10 sees shapes, no fallback", func(t *testing.T) {
		room := seedFallbackRoom(t, 10, heatEyesConditionId)
		sendDarkRoomCombatFallback(room)
		require.Zero(t, countContaining(drainPlain(1), needle), "the shapes reader follows the fight by eye")
		require.Equal(t, 1, countContaining(drainPlain(2), needle), "the normal reader hears it")
	})

	t.Run("nightvision at light 0 sees nothing, hears the fight", func(t *testing.T) {
		room := seedFallbackRoom(t, 0, nightEyesConditionId)
		require.Equal(t, messaging.SightNone, messaging.ParticipantSight(users.GetByUserId(1).Character, room))
		sendDarkRoomCombatFallback(room)
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
