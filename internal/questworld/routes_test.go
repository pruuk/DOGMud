package questworld

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The validator counts a quest's give_item, a quest's reward and a mob's
// topic gift as sources a player can obtain an item from (sources.go). Each
// of those routes must acquire through acquire.AcquireItem with its own
// reason, so the ItemOwnership{Gained} event fires and an item_gain trigger
// can hear the item arrive. This reads the route's source, so a route that
// goes back to a bare StoreItem fails here.
func TestCountedSourceKindsRaiseTheGainEvent(t *testing.T) {
	routes := []struct {
		kind   string
		file   string
		reason string
	}{
		{"quest give_item", "../questengine/bridge.go", "acquire.AcquireQuestGive"},
		{"quest reward", "../hooks/Quest_HandleQuestUpdate.go", "acquire.AcquireReward"},
		{"mob topic gift", "../conversation/playerstate.go", "acquire.AcquireTopic"},
	}
	for _, r := range routes {
		src, err := os.ReadFile(r.file)
		require.NoError(t, err, r.kind)
		text := string(src)
		require.Contains(t, text, "acquire.AcquireItem(", "%s route %s must acquire through the seam", r.kind, r.file)
		require.Contains(t, text, r.reason, "%s route %s must acquire with reason %s", r.kind, r.file, r.reason)
		require.False(t, strings.Contains(text, ".StoreItem("), "%s route %s must not store directly", r.kind, r.file)
	}
}
