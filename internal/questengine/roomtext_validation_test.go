package questengine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoomTextProblems(t *testing.T) {
	cases := []struct {
		name        string
		text        string
		wantProblem bool
		mentions    string
	}{
		{"names the player", "{source} unlocks the strongbox.", false, ""},
		{"subjectless fragment", "unlocks the strongbox.", true, "{source}"},
		{"uses target", "{source} glares at {target}.", true, "{target}"},
		{"uses target_plain", "{source} glares at {target_plain}.", true, "{target_plain}"},
		{"uses source_plain", "{source_plain} unlocks the strongbox.", true, "{source_plain}"},
		{"unknown token", "{source} opens {thing}.", true, "{thing}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			problems := roomTextProblems(c.text)
			if !c.wantProblem {
				assert.Empty(t, problems)
				return
			}
			assert.NotEmpty(t, problems)
			assert.True(t, strings.Contains(strings.Join(problems, " | "), c.mentions),
				"a problem should mention %s, got %v", c.mentions, problems)
		})
	}
}

// withEngine swaps in a fresh engine holding one quest whose single trigger has
// one room_text action, and restores the previous engine afterwards.
func withEngine(t *testing.T, roomText string) {
	t.Helper()
	prev := globalEngine
	t.Cleanup(func() { globalEngine = prev })
	globalEngine = NewEngine()
	globalEngine.RegisterQuest(&QuestDef{
		QuestId: 90001,
		Triggers: []TriggerDef{{
			Event:   "room_interact",
			Actions: []ActionDef{{RoomText: roomText}},
		}},
	})
}

func TestValidateAllRoomText_PanicsAtStartupOnABadLine(t *testing.T) {
	withEngine(t, "unlocks the strongbox.")
	assert.Panics(t, ValidateAllRoomText,
		"a quest line that names no one must stop the boot, not fail silently in play")
}

func TestValidateAllRoomText_AcceptsAGoodLine(t *testing.T) {
	withEngine(t, "{source} unlocks the strongbox.")
	assert.NotPanics(t, ValidateAllRoomText)
}
