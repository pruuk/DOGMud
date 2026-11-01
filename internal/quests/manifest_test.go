package quests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStepTokens(t *testing.T) {
	qs := []Quest{{QuestId: 53, Steps: []QuestStep{{Id: "start"}, {Id: "end"}}}, {QuestId: 7, Steps: []QuestStep{{Id: "start"}}}}
	require.Equal(t, []string{"53-start", "53-end", "7-start"}, StepTokens(qs))
}

func TestParseStepManifest_NamesItsFile(t *testing.T) {
	_, err := ParseStepManifest([]byte("1-a\n1-a\n"))
	require.ErrorContains(t, err, "step-ids.txt")
}

// A quest is retired whole when every step id it ever had sits under
// "# retired" and none is still a key (#375 phase 7: quest 29 Two Roads).
// One retired step of a live quest does not retire the quest.
func TestRetiredQuests_OnlyWholeQuests(t *testing.T) {
	m, err := ParseStepManifest([]byte("30-start\n30-end\n# retired\n29-start\n29-end\n30-old\n"))
	require.NoError(t, err)
	require.Equal(t, map[int]bool{29: true}, retiredQuests(m))

	m, err = ParseStepManifest([]byte("30-start\n30-end\n"))
	require.NoError(t, err)
	require.Nil(t, retiredQuests(m))
}
