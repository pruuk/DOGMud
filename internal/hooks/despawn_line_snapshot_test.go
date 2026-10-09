package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #456: the quit line is judged by the room before the quitter left. The
// playtest's quitter wore a lit torch at night; the watcher, who had just
// read their name, read "drag a figure away", because the line went out
// after RemovePlayer had taken the torch's light with them.

const despawnTestLine = `Suddenly and without warning spirits and demons reach up through the ground and drag <ansi fg="username">Aliceia</ansi> away!`

func TestDespawnLine_JudgedByTheQuittersOwnLight(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedNarrationConditions()
	defer restore()
	darken(t, 1)
	room := rooms.LoadRoom(1)
	quitter := users.GetByUserId(1)
	require.True(t, quitter.Character.Conditions.AddCondition(lanternConditionId, false))
	require.Equal(t, messaging.SightFull, room.ParticipantSight(2), "fixture: the quitter's light lights the room")
	drainPlain(2)

	removeAndAnnounceDespawn(room, quitter, despawnTestLine)

	require.Equal(t, messaging.SightNone, room.ParticipantSight(2), "fixture: the light left with the quitter")
	assert.Equal(t, 1, countContaining(drainPlain(2), "drag Aliceia away!"),
		"the watcher saw the quitter by their own light as they went")
}

// A hidden quitter is forced visible by logout before HandleLeave runs
// (Logout_AwarenessCleanup.go). The readers who could not make them out
// before that are excluded from the line: it would otherwise name, to a
// room that never saw them, someone it did not know was there.
func TestDespawnLine_ReadersWhoDidNotPerceiveTheQuitterReadNothing(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(90)
	quitter := users.GetByUserId(1)
	hidePlayer(t, quitter)
	drainPlain(2)

	onPlayerDespawnForAwareness(events.PlayerDespawn{UserId: 1, RoomId: 1})
	require.False(t, quitter.Character.IsHidden(), "fixture: logout forces the quitter visible")
	removeAndAnnounceDespawn(room, quitter, despawnTestLine)

	assert.Equal(t, 0, countContaining(drainPlain(2), "drag"),
		"a reader who could not see the hidden quitter read their quit line")
}
