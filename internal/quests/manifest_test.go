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
