package dialogue

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// TestTokeGivesTheWay locks Crier Toke's route answers (2026-10-17 newbie-duo
// playtests): a newcomer sent to a yard or to Scout Tarn can ask Toke the way
// from Hub Square and gets the walking route, not the catch-all.
func TestTokeGivesTheWay(t *testing.T) {
	data, err := os.ReadFile("../../_datafiles/world/dogmud/dialogue/pothole_coulee/9106.yaml")
	require.NoError(t, err)
	var df DialogueFile
	require.NoError(t, yaml.Unmarshal(data, &df))

	cases := []struct {
		topic string
		hint  string
	}{
		{"drill yard", "From Hub Square: north, then east four times."},
		{"terrace", "From Hub Square: north, east, east, north, up"},
		{"tarn", "From Hub Square: west, west, south, south to the Steppe Edge."},
		{"scout", "From Hub Square: west, west, south, south to the Steppe Edge."},
		{"steppe", "From Hub Square: west, west, south, south to the Steppe Edge."},
	}
	for i, tc := range cases {
		t.Run(tc.topic, func(t *testing.T) {
			ps := &PlayerState{
				HasQuest:     func(string) bool { return false },
				HasItem:      func(int) bool { return false },
				GetQuestFlag: func(string) string { return "" },
			}
			_, hints, _, advanced := TreeAdvance(&df, 910600+i, 910600+i, tc.topic, ps)
			require.True(t, advanced, "ask toke %s must reach a node", tc.topic)
			require.True(t, strings.Contains(hints, tc.hint),
				"ask toke %s: hints %q, want %q", tc.topic, hints, tc.hint)
		})
	}
}
