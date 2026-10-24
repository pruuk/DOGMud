package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// A quest token landing changes which NPCs carry a talk marker for the
// player, so the player's GMCP NPC list must be refreshed (#375 R9).
func TestHandleQuestUpdate_TokenLandingRefreshesNpcMarkers(t *testing.T) {
	defer setupBridgeTestQuest(t)()

	u := users.NewTestUser(511, "markeruser", "MarkerUser", 5011)
	defer users.SeedUsersForTest(map[int]*users.UserRecord{511: u})()
	events.DrainQueuedNpcMarkersChangedForTest(0)

	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 511, QuestToken: "999001-start"}))
	require.True(t, u.Character.HasQuest("999001-start"))
	got := events.DrainQueuedNpcMarkersChangedForTest(0)
	require.Equal(t, []events.NpcMarkersChanged{{UserId: 511}}, got)

	// A MarkerOnly event follows a step the quest engine already set
	// synchronously (GameBridge.GrantQuest), so the markers must refresh
	// here too: nothing else refreshes them for an engine-advanced step.
	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 511, QuestToken: "999001-end", MarkerOnly: true}))
	require.Equal(t, []events.NpcMarkersChanged{{UserId: 511}}, events.DrainQueuedNpcMarkersChangedForTest(0))

	// Removing a token changes the markers too.
	require.Equal(t, events.Continue, HandleQuestUpdate(events.Quest{UserId: 511, QuestToken: "-999001-start"}))
	require.False(t, u.Character.HasQuest("999001-start"))
	require.Equal(t, []events.NpcMarkersChanged{{UserId: 511}}, events.DrainQueuedNpcMarkersChangedForTest(0))
}
