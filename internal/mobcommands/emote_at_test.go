package mobcommands

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #273: the player's `emote @<text>` strips the @; a mob's emote passed it
// through, so watchers read "Skeleton @shrugs."
func TestMobEmote_LeadingAtIsStripped(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	mob, room := getTestMobAndRoom(t)
	oldSky, oldLamp := room.SkyLight, room.Lamp
	room.SkyLight, room.Lamp = rooms.SkyLightPtr(0), rooms.LampPtr(60)
	t.Cleanup(func() { room.SkyLight, room.Lamp = oldSky, oldLamp })
	require.GreaterOrEqual(t, room.PlayerCt(), 1, "Emote sends nothing to an empty room")
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)

	handled, err := Emote("@shrugs.", mob, room)
	require.True(t, handled)
	require.NoError(t, err)

	out := strings.Join(mobSpeechHeard(1), "\n")
	require.Contains(t, out, "shrugs.", "the watcher reads the emote")
	assert.NotContains(t, out, "@")
}
