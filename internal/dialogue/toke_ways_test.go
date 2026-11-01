package dialogue

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// TestTokeGivesTheWay locks Crier Toke's route answers (2026-10-17 newbie-duo
// playtests): a newcomer sent to a yard or to Scout Tarn can ask Toke the way
// from Hub Square and gets the walking route. Since #375 the questions are
// topics on Toke's talk menu, open to every player (no gates), and the route
// is the topic's hint. The old `ask toke tarn/scout/steppe` words all reached
// the Scout Tarn route, which is now one topic.
func TestTokeGivesTheWay(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml")
	require.NoError(t, err)
	var df DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &df))
	require.NotEmpty(t, df.Topics)

	cases := []struct {
		key  string
		hint string
	}{
		{"way_drill_yard", "From Hub Square: north, then east four times."},
		{"way_long_terrace", "From Hub Square: north, east, east, north, up"},
		{"way_scout_tarn", "From Hub Square: west, west, south, south to the Steppe Edge."},
		{"way_mine_head", "From Hub Square: north, west five times, north, west five times, down."},
	}
	ps := &PlayerState{
		QuestOpen:    func(int) bool { return true },
		HasQuest:     func(string) bool { return false },
		HasItem:      func(int) bool { return false },
		GetQuestFlag: func(string) string { return "" },
		HasHeard:     func(string) bool { return false },
		MarkHeard:    func(string) {},
	}
	m := df.Menu("Crier Toke", ps)
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			tp := df.FindTopic(tc.key)
			require.NotNil(t, tp, "Toke has no %s topic", tc.key)
			require.True(t, df.Eligible(tp, ps), "%s must be open to a brand-new player", tc.key)
			onMenu := false
			for _, e := range m.Entries {
				if e.Id == tc.key {
					onMenu = true
				}
			}
			require.True(t, onMenu, "%s must be on Toke's talk menu", tc.key)
			require.True(t, strings.Contains(strings.Join(strings.Fields(tp.Hints), " "), tc.hint),
				"%s: hints %q, want %q", tc.key, tp.Hints, tc.hint)
		})
	}
}
